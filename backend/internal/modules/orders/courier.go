package orders

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

// Location — gjendja e korrierit në Redis (moduli location), për busy/available.
type Location interface {
	SetBusy(ctx context.Context, driverID uuid.UUID, rideID uuid.UUID) error
	SetAvailable(ctx context.Context, driverID uuid.UUID) error
}

func (s *Service) WithLocation(l Location) *Service {
	s.loc = l
	return s
}

// Offer — oferta e dorëzimit siç e sheh korrieri.
type Offer struct {
	ID               uuid.UUID  `json:"id"`
	OrderID          uuid.UUID  `json:"order_id"`
	Code             string     `json:"code"`
	Round            int        `json:"round"`
	ExpiresAt        time.Time  `json:"expires_at"`
	DistanceM        int        `json:"distance_m"` // korrieri → merchant
	ETAS             int        `json:"eta_s"`
	MerchantName     string     `json:"merchant_name"`
	MerchantAddress  string     `json:"merchant_address"`
	MerchantLocation geo.Point  `json:"merchant_location"`
	DropoffAddress   *string    `json:"dropoff_address"`
	Dropoff          *geo.Point `json:"dropoff,omitempty"`
	EarningsMinor    int64      `json:"earnings_minor"` // tarifa e dërgesës i shkon korrierit
	Currency         string     `json:"currency"`
	PaymentMethod    string     `json:"payment_method"`
	TotalMinor       int64      `json:"total_minor"` // sa duhet mbledhur në cash
}

func (s *Service) Offers(ctx context.Context, a principal.Actor) ([]Offer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.order_id, ord.code, o.round, o.expires_at, o.distance_m, o.eta_s, m.name, m.address_line1, m.lat, m.lng,
		       ord.address_line1, ord.address_lat, ord.address_lng, ord.delivery_fee_minor, ord.currency, ord.payment_method, ord.total_minor
		FROM order_offers o
		JOIN orders ord ON ord.id = o.order_id
		JOIN merchants m ON m.id = ord.merchant_id
		WHERE o.courier_id = $1 AND o.state = 'offered' AND o.expires_at > now() AND ord.state = 'ready'
		ORDER BY o.created_at`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		var o Offer
		var lat, lng *float64
		if err := rows.Scan(&o.ID, &o.OrderID, &o.Code, &o.Round, &o.ExpiresAt, &o.DistanceM, &o.ETAS, &o.MerchantName, &o.MerchantAddress,
			&o.MerchantLocation.Lat, &o.MerchantLocation.Lng, &o.DropoffAddress, &lat, &lng, &o.EarningsMinor, &o.Currency, &o.PaymentMethod, &o.TotalMinor); err != nil {
			return nil, err
		}
		if lat != nil && lng != nil {
			o.Dropoff = &geo.Point{Lat: *lat, Lng: *lng}
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AcceptOffer — korrieri pranon dorëzimin (një porosi aktive për korrier — indeks unik).
func (s *Service) AcceptOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) (*Order, error) {
	var out *Order
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var orderID uuid.UUID
		var state string
		var expires time.Time
		err := tx.QueryRow(ctx, `SELECT order_id, state, expires_at FROM order_offers WHERE id = $1 AND courier_id = $2 FOR UPDATE`, offerID, a.UserID).Scan(&orderID, &state, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if state != "offered" || s.now().After(expires) {
			return ErrOfferGone
		}
		o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 FOR UPDATE`, orderID))
		if err != nil {
			return err
		}
		if o.State != StateReady {
			_, _ = tx.Exec(ctx, `UPDATE order_offers SET state = 'withdrawn', responded_at = now() WHERE id = $1`, offerID)
			return ErrOfferGone
		}
		o, err = scanOrder(tx.QueryRow(ctx, `UPDATE orders SET state = 'courier_assigned', courier_id = $2, updated_at = now() WHERE id = $1 RETURNING `+orderCols, orderID, a.UserID))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrCourierAssigned
			}
			return err
		}
		out = o
		if _, err := tx.Exec(ctx, `UPDATE order_offers SET state = 'accepted', responded_at = now() WHERE id = $1`, offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE order_offers SET state = 'withdrawn', responded_at = now() WHERE order_id = $1 AND state = 'offered'`, orderID); err != nil {
			return err
		}
		from := StateReady
		if err := orderEvent(ctx, tx, orderID, &from, StateCourierAssigned, "courier", &a.UserID, map[string]any{"offer_id": offerID}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "order", orderID.String(), "OrderCourierAssigned", map[string]any{
			"order_id": orderID, "code": o.Code, "customer_id": o.CustomerID, "merchant_id": o.MerchantID, "courier_id": a.UserID})
	})
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetBusy(ctx, a.UserID, out.ID)
	}
	return s.withItems(ctx, out)
}

func (s *Service) DeclineOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var orderID uuid.UUID
		err := tx.QueryRow(ctx, `UPDATE order_offers SET state = 'declined', responded_at = now()
			WHERE id = $1 AND courier_id = $2 AND state = 'offered' RETURNING order_id`, offerID, a.UserID).Scan(&orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOfferGone
		}
		if err != nil {
			return err
		}
		return events.Emit(ctx, tx, "order", orderID.String(), "OrderOfferDeclined", map[string]any{"order_id": orderID, "courier_id": a.UserID, "offer_id": offerID})
	})
}

// ActiveForCourier — porosia aktuale e korrierit (nil kur s'ka).
func (s *Service) ActiveForCourier(ctx context.Context, a principal.Actor) (*Order, error) {
	o, err := scanOrder(s.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE courier_id = $1 AND state IN ('courier_assigned','picked_up') LIMIT 1`, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.withItems(ctx, o)
}

// PickUp — korrieri e mori porosinë te merchant-i (kodi i porosisë verifikohet).
func (s *Service) PickUp(ctx context.Context, a principal.Actor, orderID uuid.UUID, code string) (*Order, error) {
	return s.courierTransition(ctx, a, orderID, StatePickedUp, code, "")
}

// Deliver — dorëzimi te klienti; shlyerja bëhet menjëherë.
func (s *Service) Deliver(ctx context.Context, a principal.Actor, orderID uuid.UUID) (*Order, error) {
	o, err := s.courierTransition(ctx, a, orderID, StateDelivered, "", "")
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetAvailable(ctx, a.UserID)
	}
	_ = s.settle(ctx, o)
	o, err = scanOrder(s.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1`, orderID))
	if err != nil {
		return nil, err
	}
	return s.withItems(ctx, o)
}

// ReleaseByCourier — korrieri heq dorë para marrjes → porosia kthehet 'ready' (ricaktim).
func (s *Service) ReleaseByCourier(ctx context.Context, a principal.Actor, orderID uuid.UUID, reason string) (*Order, error) {
	o, err := s.courierTransition(ctx, a, orderID, StateReady, "", clip(reason, 200))
	if err != nil {
		return nil, err
	}
	if s.loc != nil {
		_ = s.loc.SetAvailable(ctx, a.UserID)
	}
	return o, nil
}

func (s *Service) courierTransition(ctx context.Context, a principal.Actor, orderID uuid.UUID, to, code, reason string) (*Order, error) {
	var out *Order
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 AND courier_id = $2 FOR UPDATE`, orderID, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(o.State, to) {
			return ErrInvalidState
		}
		if to == StatePickedUp && !equalFold(code, o.Code) {
			return httpx.ErrValidation.WithFields(map[string]string{"code": "invalid"})
		}
		from := o.State
		set := ""
		switch to {
		case StatePickedUp:
			set = `state = 'picked_up', picked_up_at = now()`
		case StateDelivered:
			set = `state = 'delivered', delivered_at = now(), payment_status = 'pending'`
		case StateReady:
			set = `state = 'ready', courier_id = NULL`
		default:
			return ErrInvalidState
		}
		o, err = scanOrder(tx.QueryRow(ctx, `UPDATE orders SET `+set+`, updated_at = now() WHERE id = $1 RETURNING `+orderCols, orderID))
		if err != nil {
			return err
		}
		out = o
		if to == StateReady {
			if _, err := tx.Exec(ctx, `UPDATE order_offers SET state = 'declined', responded_at = now() WHERE order_id = $1 AND courier_id = $2`, orderID, a.UserID); err != nil {
				return err
			}
		}
		if err := orderEvent(ctx, tx, orderID, &from, to, "courier", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		evt := map[string]string{StatePickedUp: "OrderPickedUp", StateDelivered: "OrderDelivered", StateReady: "OrderCourierReleased"}[to]
		return events.Emit(ctx, tx, "order", orderID.String(), evt, map[string]any{
			"order_id": orderID, "code": o.Code, "customer_id": o.CustomerID, "merchant_id": o.MerchantID, "courier_id": a.UserID, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	return s.withItems(ctx, out)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 32
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
