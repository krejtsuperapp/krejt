package rides

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Offer — oferta e dispatch-it siç e sheh shoferi (me fitimin e tij, jo vetëm çmimin).
type Offer struct {
	ID             uuid.UUID `json:"id"`
	RideID         uuid.UUID `json:"ride_id"`
	Round          int       `json:"round"`
	ExpiresAt      time.Time `json:"expires_at"`
	DistanceM      int       `json:"distance_m"` // shoferi → pikë marrjeje
	ETAS           int       `json:"eta_s"`
	CategoryID     string    `json:"category"`
	Pickup         geo.Point `json:"pickup"`
	PickupAddress  *string   `json:"pickup_address"`
	Dropoff        geo.Point `json:"dropoff"`
	DropoffAddress *string   `json:"dropoff_address"`
	RideDistanceM  int       `json:"ride_distance_m"`
	RideDurationS  int       `json:"ride_duration_s"`
	PriceMinor     int64     `json:"price_minor"`
	EarningsMinor  int64     `json:"earnings_minor"`
	Currency       string    `json:"currency"`
	PaymentMethod  string    `json:"payment_method"`
}

// Offers — ofertat e hapura të shoferit (deri te Centrifugo: klienti i shoferit i kërkon çdo 3 s).
func (s *Service) Offers(ctx context.Context, a principal.Actor) ([]Offer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.ride_id, o.round, o.expires_at, o.distance_m, o.eta_s, r.category_id,
		       r.pickup_lat, r.pickup_lng, r.pickup_address, r.dropoff_lat, r.dropoff_lng, r.dropoff_address,
		       r.distance_m, r.duration_s, r.price_quoted_minor, r.currency, r.payment_method, p.commission_bp
		FROM ride_offers o
		JOIN rides r ON r.id = o.ride_id
		JOIN ride_quotes q ON q.id = r.quote_id
		JOIN pricing_rules p ON p.id = q.pricing_rule_id
		WHERE o.driver_id = $1 AND o.state = 'offered' AND o.expires_at > now() AND r.state = 'matching'
		ORDER BY o.created_at`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		var o Offer
		var bp int
		if err := rows.Scan(&o.ID, &o.RideID, &o.Round, &o.ExpiresAt, &o.DistanceM, &o.ETAS, &o.CategoryID,
			&o.Pickup.Lat, &o.Pickup.Lng, &o.PickupAddress, &o.Dropoff.Lat, &o.Dropoff.Lng, &o.DropoffAddress,
			&o.RideDistanceM, &o.RideDurationS, &o.PriceMinor, &o.Currency, &o.PaymentMethod, &bp); err != nil {
			return nil, err
		}
		o.EarningsMinor = o.PriceMinor - pricing.Commission(o.PriceMinor, bp)
		out = append(out, o)
	}
	return out, rows.Err()
}

// AcceptOffer — shoferi pranon: udhëtimi caktohet (një udhëtim aktiv për shofer — indeks unik), ofertat
// e tjera tërhiqen, shoferi shënohet busy në Redis.
func (s *Service) AcceptOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) (*Ride, error) {
	if _, err := s.drivers.Approved(ctx, a.UserID); err != nil {
		return nil, err
	}
	var out *Ride
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var rideID uuid.UUID
		var state string
		var expires time.Time
		err := tx.QueryRow(ctx, `SELECT ride_id, state, expires_at FROM ride_offers WHERE id = $1 AND driver_id = $2 FOR UPDATE`, offerID, a.UserID).
			Scan(&rideID, &state, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if state != "offered" || s.now().After(expires) {
			return ErrOfferGone
		}
		r, err := scanRide(tx.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 FOR UPDATE`, rideID))
		if err != nil {
			return err
		}
		if r.State != StateMatching {
			_, _ = tx.Exec(ctx, `UPDATE ride_offers SET state = 'withdrawn', responded_at = now() WHERE id = $1`, offerID)
			return ErrOfferGone
		}
		r, err = scanRide(tx.QueryRow(ctx, `
			UPDATE rides SET state = 'assigned', driver_id = $2, assigned_at = now(), updated_at = now()
			WHERE id = $1 RETURNING `+rideCols, rideID, a.UserID))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrActiveRide // shoferi ka tashmë një udhëtim aktiv
			}
			return err
		}
		out = r
		if _, err := tx.Exec(ctx, `UPDATE ride_offers SET state = 'accepted', responded_at = now() WHERE id = $1`, offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE ride_offers SET state = 'withdrawn', responded_at = now() WHERE ride_id = $1 AND state = 'offered'`, rideID); err != nil {
			return err
		}
		from := StateMatching
		if err := rideEvent(ctx, tx, rideID, &from, StateAssigned, "driver", &a.UserID, map[string]any{"offer_id": offerID}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideAssigned", map[string]any{"ride_id": rideID, "driver_id": a.UserID, "customer_id": r.CustomerID})
	})
	if err != nil {
		return nil, err
	}
	_ = s.loc.SetBusy(ctx, a.UserID, out.ID)
	out = forActor(out, a)
	return s.decorate(ctx, out)
}

// DeclineOffer — shoferi refuzon; dispatch-i kalon te tjetri.
func (s *Service) DeclineOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var rideID uuid.UUID
		err := tx.QueryRow(ctx, `UPDATE ride_offers SET state = 'declined', responded_at = now()
			WHERE id = $1 AND driver_id = $2 AND state = 'offered' RETURNING ride_id`, offerID, a.UserID).Scan(&rideID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOfferGone
		}
		if err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideOfferDeclined", map[string]any{"ride_id": rideID, "driver_id": a.UserID, "offer_id": offerID})
	})
}

// ActiveForDriver — udhëtimi aktual i shoferit (nil kur s'ka).
func (s *Service) ActiveForDriver(ctx context.Context, a principal.Actor) (*Ride, error) {
	r, err := scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides
		WHERE driver_id = $1 AND state IN ('assigned','arrived','in_progress') LIMIT 1`, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, forActor(r, a))
}

// Arrived / Start / Complete — hapat e shoferit. Complete mbyll udhëtimin me çmimin e paracaktuar
// dhe e shlyen (settle) menjëherë; nëse shlyerja dështon, mbetet 'pending' dhe riprovohet nga worker-i.
func (s *Service) Arrived(ctx context.Context, a principal.Actor, rideID uuid.UUID) (*Ride, error) {
	return s.driverTransition(ctx, a, rideID, StateArrived)
}

var ErrPickupCode = &httpx.APIError{Code: "PICKUP_CODE_INVALID", MessageKey: "errors.rides.pickup_code_invalid", HTTPStatus: http.StatusUnprocessableEntity}

// Start — nisja kërkon kodin e marrjes së klientit (4 shifra) ose QR-in e nënshkruar (§25, §60).
func (s *Service) Start(ctx context.Context, a principal.Actor, rideID uuid.UUID, code, qrToken string) (*Ride, error) {
	if qrToken != "" && s.qr != nil {
		claims, err := s.qr.Parse(qrToken)
		if err != nil || claims["purpose"] != "ride_pickup" || claims["ride"] != rideID.String() {
			return nil, ErrPickupCode
		}
		code, _ = claims["code"].(string)
	}
	code = strings.TrimSpace(code)
	var expected *string
	err := s.pool.QueryRow(ctx, `SELECT pickup_code FROM rides WHERE id = $1 AND driver_id = $2`, rideID, a.UserID).Scan(&expected)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if expected != nil && code != *expected {
		return nil, ErrPickupCode
	}
	return s.driverTransition(ctx, a, rideID, StateInProgress)
}

func (s *Service) Complete(ctx context.Context, a principal.Actor, rideID uuid.UUID) (*Ride, error) {
	r, err := s.driverTransition(ctx, a, rideID, StateCompleted)
	if err != nil {
		return nil, err
	}
	_ = s.loc.SetAvailable(ctx, a.UserID)
	_ = s.settle(ctx, r)
	r, err = scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1`, rideID))
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, forActor(r, a))
}

func (s *Service) driverTransition(ctx context.Context, a principal.Actor, rideID uuid.UUID, to string) (*Ride, error) {
	var out *Ride
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRide(tx.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 AND driver_id = $2 FOR UPDATE`, rideID, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(r.State, to) {
			return ErrInvalidState
		}
		from := r.State
		var set string
		args := []any{rideID}
		switch to {
		case StateArrived:
			set = `state = 'arrived', arrived_at = now()`
		case StateInProgress:
			set = `state = 'in_progress', started_at = now()`
		case StateCompleted:
			var bp int
			if err := tx.QueryRow(ctx, `SELECT p.commission_bp FROM ride_quotes q JOIN pricing_rules p ON p.id = q.pricing_rule_id WHERE q.id = $1`, r.QuoteID).Scan(&bp); err != nil {
				return err
			}
			price := r.PriceQuotedMinor // çmim i paracaktuar (V1): asnjë rillogaritje pas udhëtimit
			set = `state = 'completed', completed_at = now(), price_final_minor = $2, commission_minor = $3, payment_status = 'pending'`
			args = append(args, price, pricing.Commission(price, bp))
		default:
			return ErrInvalidState
		}
		r, err = scanRide(tx.QueryRow(ctx, `UPDATE rides SET `+set+`, updated_at = now() WHERE id = $1 RETURNING `+rideCols, args...))
		if err != nil {
			return err
		}
		out = r
		if err := rideEvent(ctx, tx, rideID, &from, to, "driver", &a.UserID, nil); err != nil {
			return err
		}
		evType := map[string]string{StateArrived: "RideDriverArrived", StateInProgress: "RideStarted", StateCompleted: "RideCompleted"}[to]
		return events.Emit(ctx, tx, "ride", rideID.String(), evType, map[string]any{
			"ride_id": rideID, "driver_id": a.UserID, "customer_id": r.CustomerID, "price_final_minor": r.PriceFinalMinor})
	})
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, forActor(out, a))
}

// CancelByDriver — shoferi heq dorë pas caktimit: udhëtimi kthehet në kërkim (ricaktim §18) pa e prekur
// klientin; shoferi shënohet (sinjal për fraud/risk §67) dhe nuk merr më ofertë për këtë udhëtim.
func (s *Service) CancelByDriver(ctx context.Context, a principal.Actor, rideID uuid.UUID, reason string) (*Ride, error) {
	reason = clip(reason, 200)
	var out *Ride
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRide(tx.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 AND driver_id = $2 FOR UPDATE`, rideID, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(r.State, StateMatching) {
			return ErrInvalidState
		}
		from := r.State
		r, err = scanRide(tx.QueryRow(ctx, `
			UPDATE rides SET state = 'matching', driver_id = NULL, assigned_at = NULL, arrived_at = NULL,
			  matching_attempts = matching_attempts + 1, requested_at = now(), updated_at = now()
			WHERE id = $1 RETURNING `+rideCols, rideID))
		if err != nil {
			return err
		}
		out = r
		if _, err := tx.Exec(ctx, `UPDATE ride_offers SET state = 'declined', responded_at = now() WHERE ride_id = $1 AND driver_id = $2`, rideID, a.UserID); err != nil {
			return err
		}
		if err := rideEvent(ctx, tx, rideID, &from, StateMatching, "driver", &a.UserID, map[string]any{"reason": reason, "reassign": true}); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata)
			VALUES ($1, 'driver.cancelled_ride', 'ride', $2, jsonb_build_object('reason', $3::text))`, a.UserID, rideID.String(), reason); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideRequested", map[string]any{
			"ride_id": rideID, "customer_id": r.CustomerID, "category": r.CategoryID, "attempt": r.MatchingAttempts + 1, "reassign": true, "previous_driver_id": a.UserID})
	})
	if err != nil {
		return nil, err
	}
	_ = s.loc.SetAvailable(ctx, a.UserID)
	return forActor(out, a), nil
}

// settle — shlyerja në ledger (idempotente me çelës për udhëtim):
//   - wallet: debit wallet-i i klientit (çmimi), kredit wallet-i i shoferit (çmimi − komisioni), kredit komisioni;
//   - cash: shoferi e mban cash-in; debit wallet-i i shoferit me komisionin (borxh ndaj platformës), kredit komisioni;
//   - anulim me tarifë (wallet): debit klienti, kredit komisioni.
func (s *Service) settle(ctx context.Context, r *Ride) error {
	var status string
	var tx ledger.Transaction
	switch {
	case r.State == StateCancelled && r.CancellationFeeMinor > 0 && r.PaymentMethod == "wallet":
		fee := money.Minor(r.CancellationFeeMinor)
		tx = ledger.Transaction{Kind: "ride_cancellation_fee", Reference: "ride:" + r.ID.String(), IdempotencyKey: "ride:" + r.ID.String() + ":cancel_fee", Currency: r.Currency,
			Postings: []ledger.Posting{{AccountCode: ledger.UserWalletCode(r.CustomerID), Debit: fee}, {AccountCode: "krejt:commission", Credit: fee}}}
		status = "paid"
	case r.State == StateCompleted && r.DriverID != nil && r.PriceFinalMinor != nil:
		price := money.Minor(*r.PriceFinalMinor)
		commission := money.Minor(0)
		if r.CommissionMinor != nil {
			commission = money.Minor(*r.CommissionMinor)
		}
		driverWallet := "driver:" + r.DriverID.String() + ":wallet"
		did := *r.DriverID
		if err := s.ledger.EnsureAccount(ctx, driverWallet, "driver", &did, "liability", r.Currency); err != nil {
			return err
		}
		if r.PaymentMethod == "cash" {
			status = "cash"
			if commission > 0 {
				tx = ledger.Transaction{Kind: "ride_cash_commission", Reference: "ride:" + r.ID.String(), IdempotencyKey: "ride:" + r.ID.String() + ":fare", Currency: r.Currency,
					Postings: []ledger.Posting{{AccountCode: driverWallet, Debit: commission}, {AccountCode: "krejt:commission", Credit: commission}}}
			}
		} else {
			bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(r.CustomerID))
			if err != nil {
				return err
			}
			if bal.Minor < price {
				return s.setPaymentStatus(ctx, r, "failed")
			}
			status = "paid"
			tx = ledger.Transaction{Kind: "ride_fare", Reference: "ride:" + r.ID.String(), IdempotencyKey: "ride:" + r.ID.String() + ":fare", Currency: r.Currency,
				Postings: []ledger.Posting{
					{AccountCode: ledger.UserWalletCode(r.CustomerID), Debit: price},
					{AccountCode: driverWallet, Credit: price - commission},
					{AccountCode: "krejt:commission", Credit: commission}}}
			if commission == 0 {
				tx.Postings = tx.Postings[:2]
			}
		}
	default:
		return fmt.Errorf("rides: settle: gjendje e papritur %s/%s", r.State, r.PaymentStatus)
	}
	if len(tx.Postings) > 0 {
		if _, err := s.ledger.Post(ctx, tx); err != nil {
			return err
		}
	}
	return s.setPaymentStatus(ctx, r, status)
}

func (s *Service) setPaymentStatus(ctx context.Context, r *Ride, status string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE rides SET payment_status = $2, updated_at = now() WHERE id = $1 AND payment_status = 'pending'`, r.ID, status); err != nil {
			return err
		}
		ev := "RidePaymentSettled"
		if status == "failed" {
			ev = "RidePaymentFailed"
		}
		return events.Emit(ctx, tx, "ride", r.ID.String(), ev, map[string]any{"ride_id": r.ID, "customer_id": r.CustomerID, "status": status, "method": r.PaymentMethod})
	})
}

// SettlePending — worker-i riprovon shlyerjet e mbetura 'pending' (p.sh. pas një ndërprerjeje).
func (s *Service) SettlePending(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+rideCols+` FROM rides
		WHERE payment_status = 'pending' AND state IN ('completed','cancelled') AND updated_at < now() - interval '10 seconds'
		ORDER BY updated_at LIMIT 50`)
	if err != nil {
		return 0, err
	}
	var list []*Ride
	for rows.Next() {
		r, err := scanRide(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, r)
	}
	rows.Close()
	n := 0
	for _, r := range list {
		if err := s.settle(ctx, r); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
