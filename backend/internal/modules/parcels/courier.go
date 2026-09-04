package parcels

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Offer — oferta e një pakoje siç e sheh korrieri (pa kodet).
type Offer struct {
	ID             uuid.UUID `json:"id"`
	ParcelID       uuid.UUID `json:"parcel_id"`
	Code           string    `json:"code"`
	Round          int       `json:"round"`
	ExpiresAt      time.Time `json:"expires_at"`
	DistanceM      int       `json:"distance_m"` // korrieri → nisja
	ETAS           int       `json:"eta_s"`
	Size           string    `json:"size"`
	PickupAddress  *string   `json:"pickup_address"`
	Pickup         geo.Point `json:"pickup"`
	DropoffAddress *string   `json:"dropoff_address"`
	Dropoff        geo.Point `json:"dropoff"`
	RouteM         int       `json:"route_m"` // nisja → destinacioni
	EarningsMinor  int64     `json:"earnings_minor"`
	Currency       string    `json:"currency"`
	PaymentMethod  string    `json:"payment_method"`
	TotalMinor     int64     `json:"total_minor"` // sa mblidhet në cash
}

func (s *Service) Offers(ctx context.Context, a principal.Actor) ([]Offer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.parcel_id, p.code, o.round, o.expires_at, o.distance_m, o.eta_s, p.size,
		       p.pickup_address, p.pickup_lat, p.pickup_lng, p.dropoff_address, p.dropoff_lat, p.dropoff_lng, p.distance_m,
		       p.price_minor - p.commission_minor, p.currency, p.payment_method, p.price_minor - p.discount_minor
		FROM parcel_offers o JOIN parcels p ON p.id = o.parcel_id
		WHERE o.courier_id = $1 AND o.state = 'offered' AND o.expires_at > now() AND p.state = 'requested'
		ORDER BY o.created_at`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.ParcelID, &o.Code, &o.Round, &o.ExpiresAt, &o.DistanceM, &o.ETAS, &o.Size,
			&o.PickupAddress, &o.Pickup.Lat, &o.Pickup.Lng, &o.DropoffAddress, &o.Dropoff.Lat, &o.Dropoff.Lng, &o.RouteM,
			&o.EarningsMinor, &o.Currency, &o.PaymentMethod, &o.TotalMinor); err != nil {
			return nil, err
		}
		if o.PaymentMethod != "cash" {
			o.TotalMinor = 0
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AcceptOffer — korrieri pranon pakon (një pako aktive për korrier — indeks unik).
func (s *Service) AcceptOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) (*Parcel, error) {
	var out *Parcel
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var parcelID uuid.UUID
		var state string
		var expires time.Time
		err := tx.QueryRow(ctx, `SELECT parcel_id, state, expires_at FROM parcel_offers WHERE id = $1 AND courier_id = $2 FOR UPDATE`, offerID, a.UserID).Scan(&parcelID, &state, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if state != "offered" || s.now().After(expires) {
			return ErrOfferGone
		}
		p, err := scanParcel(tx.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE id = $1 FOR UPDATE`, parcelID))
		if err != nil {
			return err
		}
		if p.State != StateRequested {
			_, _ = tx.Exec(ctx, `UPDATE parcel_offers SET state = 'withdrawn', responded_at = now() WHERE id = $1`, offerID)
			return ErrOfferGone
		}
		p, err = scanParcel(tx.QueryRow(ctx, `UPDATE parcels SET state = 'courier_assigned', courier_id = $2, assigned_at = now(), updated_at = now() WHERE id = $1 RETURNING `+parcelCols, parcelID, a.UserID))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrCourierAssigned
			}
			return err
		}
		out = p
		if _, err := tx.Exec(ctx, `UPDATE parcel_offers SET state = 'accepted', responded_at = now() WHERE id = $1`, offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE parcel_offers SET state = 'withdrawn', responded_at = now() WHERE parcel_id = $1 AND state = 'offered'`, parcelID); err != nil {
			return err
		}
		from := StateRequested
		if err := parcelEvent(ctx, tx, parcelID, &from, StateCourierAssigned, "courier", &a.UserID, map[string]any{"offer_id": offerID}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "parcel", parcelID.String(), "ParcelCourierAssigned", map[string]any{
			"parcel_id": parcelID, "code": p.Code, "customer_id": p.CustomerID, "courier_id": a.UserID})
	})
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetBusyParcel(ctx, a.UserID, out.ID)
	}
	return forCourier(out), nil
}

func (s *Service) DeclineOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var parcelID uuid.UUID
		err := tx.QueryRow(ctx, `UPDATE parcel_offers SET state = 'declined', responded_at = now()
			WHERE id = $1 AND courier_id = $2 AND state = 'offered' RETURNING parcel_id`, offerID, a.UserID).Scan(&parcelID)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		return events.Emit(ctx, tx, "parcel", parcelID.String(), "ParcelOfferDeclined", map[string]any{"parcel_id": parcelID, "courier_id": a.UserID, "offer_id": offerID})
	})
}

func (s *Service) ActiveForCourier(ctx context.Context, a principal.Actor) (*Parcel, error) {
	p, err := scanParcel(s.pool.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE courier_id = $1 AND state IN ('courier_assigned','picked_up') LIMIT 1`, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return forCourier(p), nil
}

// PickUp — korrieri e merr pakon nga dërguesi; kodi 4-shifror i marrjes duhet të përputhet.
func (s *Service) PickUp(ctx context.Context, a principal.Actor, id uuid.UUID, code string) (*Parcel, error) {
	return s.transition(ctx, a, id, StatePickedUp, code, "")
}

// Deliver — dorëzimi te marrësi me kodin e tij; shlyerja ndodh menjëherë.
func (s *Service) Deliver(ctx context.Context, a principal.Actor, id uuid.UUID, code string) (*Parcel, error) {
	p, err := s.transition(ctx, a, id, StateDelivered, code, "")
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetAvailable(ctx, a.UserID)
	}
	_ = s.settle(ctx, p)
	p, err = scanParcel(s.pool.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return forCourier(p), nil
}

// Release — korrieri heq dorë para marrjes → pakoja kthehet 'requested' (ricaktim).
func (s *Service) Release(ctx context.Context, a principal.Actor, id uuid.UUID, reason string) (*Parcel, error) {
	p, err := s.transition(ctx, a, id, StateRequested, "", clip(reason, 200))
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetAvailable(ctx, a.UserID)
	}
	return p, nil
}

func (s *Service) transition(ctx context.Context, a principal.Actor, id uuid.UUID, to, code, reason string) (*Parcel, error) {
	var out *Parcel
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := scanParcel(tx.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE id = $1 AND courier_id = $2 FOR UPDATE`, id, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(p.State, to) {
			return ErrInvalidState
		}
		if to == StatePickedUp && code != p.PickupCode {
			return ErrPickupCode
		}
		if to == StateDelivered && code != p.DeliveryCode {
			return ErrDeliveryCode
		}
		from := p.State
		set := ""
		switch to {
		case StatePickedUp:
			set = `state = 'picked_up', picked_up_at = now()`
		case StateDelivered:
			set = `state = 'delivered', delivered_at = now(), payment_status = 'pending'`
		case StateRequested:
			set = `state = 'requested', courier_id = NULL, assigned_at = NULL`
		default:
			return ErrInvalidState
		}
		p, err = scanParcel(tx.QueryRow(ctx, `UPDATE parcels SET `+set+`, updated_at = now() WHERE id = $1 RETURNING `+parcelCols, id))
		if err != nil {
			return err
		}
		out = p
		if to == StateRequested {
			if _, err := tx.Exec(ctx, `UPDATE parcel_offers SET state = 'declined', responded_at = now() WHERE parcel_id = $1 AND courier_id = $2`, id, a.UserID); err != nil {
				return err
			}
		}
		if err := parcelEvent(ctx, tx, id, &from, to, "courier", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		evt := map[string]string{StatePickedUp: "ParcelPickedUp", StateDelivered: "ParcelDelivered", StateRequested: "ParcelCourierReleased"}[to]
		return events.Emit(ctx, tx, "parcel", id.String(), evt, map[string]any{
			"parcel_id": id, "code": p.Code, "customer_id": p.CustomerID, "courier_id": a.UserID, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	return forCourier(out), nil
}
