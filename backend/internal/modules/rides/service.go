package rides

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/drivers"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrActiveRide               = &httpx.APIError{Code: "RIDE_ALREADY_ACTIVE", MessageKey: "errors.rides.already_active", HTTPStatus: http.StatusConflict}
	ErrPaymentMethodUnavailable = &httpx.APIError{Code: "PAYMENT_METHOD_UNAVAILABLE", MessageKey: "errors.rides.payment_method_unavailable", HTTPStatus: http.StatusUnprocessableEntity}
	ErrInsufficientFunds        = &httpx.APIError{Code: "INSUFFICIENT_FUNDS", MessageKey: "errors.wallet.insufficient_funds", HTTPStatus: http.StatusPaymentRequired}
	ErrInvalidState             = &httpx.APIError{Code: "RIDE_INVALID_STATE", MessageKey: "errors.rides.invalid_state", HTTPStatus: http.StatusConflict}
	ErrOfferGone                = &httpx.APIError{Code: "OFFER_GONE", MessageKey: "errors.rides.offer_gone", HTTPStatus: http.StatusConflict}
)

// Businesses — sa i duhet udhëtimit nga KREJT Business: a lejohet ky shpenzim, dhe nga cila
// llogari. Ndërfaqe e ngushtë me qëllim — udhëtimi nuk ka pse të njohë punonjësit ose faturat.
type Businesses interface {
	Authorize(ctx context.Context, businessID, userID uuid.UUID, amount int64) (string, error)
	RecordCharge(ctx context.Context, tx pgx.Tx, businessID, userID uuid.UUID, kind string, subjectID, ledgerTx uuid.UUID, amount int64) error
}

// Flags — feature flags (§65): `rides.request` = çaktivizim emergjent i shërbimit.
type Flags interface {
	Enabled(ctx context.Context, key string, userID uuid.UUID) bool
}

var ErrServiceDisabled = &httpx.APIError{Code: "SERVICE_DISABLED", MessageKey: "errors.rides.service_disabled", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}

type Service struct {
	pool     *pgxpool.Pool
	loc      *location.Service
	ledger   *ledger.Service
	drivers  *drivers.Service
	pricing  *pricing.Service
	flags    Flags
	velocity Velocity
	business Businesses
	qr       QRSigner
	now      func() time.Time
}

// QRSigner — nënshkruan/lexon QR-in e marrjes (HS256; i njëjti lëshues si token-at e realtime-it).
type QRSigner interface {
	SignClaims(purpose string, claims map[string]any, ttl time.Duration) (string, time.Time, error)
	Parse(token string) (jwt.MapClaims, error)
}

func (s *Service) WithQR(q QRSigner) *Service {
	s.qr = q
	return s
}

func New(pool *pgxpool.Pool, loc *location.Service, led *ledger.Service, drv *drivers.Service, pr *pricing.Service) *Service {
	return &Service{pool: pool, loc: loc, ledger: led, drivers: drv, pricing: pr, now: time.Now}
}

// WithFlags — kur është vendosur, kërkesat e reja kontrollojnë flag-un `rides.request`.
func (s *Service) WithFlags(f Flags) *Service {
	s.flags = f
	return s
}

// Velocity — kufi shpejtësie për përdorues (fraud §67): p.sh. 10 kërkesa udhëtimi në orë.
type Velocity interface {
	Allow(ctx context.Context, userID uuid.UUID, action string, limit int, window time.Duration) error
}

func (s *Service) WithVelocity(v Velocity) *Service {
	s.velocity = v
	return s
}

// WithBusiness — pa të, metoda `business` refuzohet si e padisponueshme në vend që të dështojë
// më vonë me një gabim që përdoruesi nuk e kupton.
func (s *Service) WithBusiness(b Businesses) *Service {
	s.business = b
	return s
}

// Ride — pamja e udhëtimit (e njëjta për klientin dhe shoferin; klienti sheh shoferin, jo komisionin).
type Ride struct {
	ID                   uuid.UUID  `json:"id"`
	CustomerID           uuid.UUID  `json:"customer_id"`
	DriverID             *uuid.UUID `json:"driver_id"`
	CategoryID           string     `json:"category"`
	State                string     `json:"state"`
	PaymentMethod        string     `json:"payment_method"`
	PaymentStatus        string     `json:"payment_status"`
	Pickup               geo.Point  `json:"pickup"`
	PickupAddress        *string    `json:"pickup_address"`
	Dropoff              geo.Point  `json:"dropoff"`
	DropoffAddress       *string    `json:"dropoff_address"`
	DistanceM            int        `json:"distance_m"`
	DurationS            int        `json:"duration_s"`
	PriceQuotedMinor     int64      `json:"price_quoted_minor"`
	PriceFinalMinor      *int64     `json:"price_final_minor"`
	CommissionMinor      *int64     `json:"-"`
	CancellationFeeMinor int64      `json:"cancellation_fee_minor"`
	Currency             string     `json:"currency"`
	Note                 *string    `json:"note"`
	MatchingAttempts     int        `json:"-"`
	CancelledBy          *string    `json:"cancelled_by"`
	CancellationReason   *string    `json:"cancellation_reason"`
	RequestedAt          time.Time  `json:"requested_at"`
	AssignedAt           *time.Time `json:"assigned_at"`
	ArrivedAt            *time.Time `json:"arrived_at"`
	StartedAt            *time.Time `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	CancelledAt          *time.Time `json:"cancelled_at"`
	QuoteID              uuid.UUID  `json:"-"`
	PickupCode           *string    `json:"pickup_code,omitempty"` // vetëm për klientin (§25)
	// Ndërmarrja që e paguan; mbahet te udhëtimi e nuk lexohet nga anëtarësia në kohën e faturimit,
	// sepse një punonjës mund të largohet pasi ka udhëtuar dhe ai udhëtim mbetet i saj.
	BusinessID *uuid.UUID  `json:"business_id,omitempty"`
	Driver     *DriverCard `json:"driver,omitempty"`
}

// DriverCard — çfarë sheh klienti për shoferin (§18 driver profile) + lokacioni i gjallë.
type DriverCard struct {
	ID           uuid.UUID  `json:"id"`
	Name         *string    `json:"name"`
	VehicleMake  string     `json:"vehicle_make"`
	VehicleModel string     `json:"vehicle_model"`
	VehiclePlate string     `json:"vehicle_plate"`
	VehicleColor string     `json:"vehicle_color"`
	Rating       *float64   `json:"rating"`
	Location     *geo.Point `json:"location,omitempty"`
	LocationAt   *time.Time `json:"location_at,omitempty"`
}

const rideCols = `id, customer_id, driver_id, category_id, state, payment_method, payment_status,
	pickup_lat, pickup_lng, pickup_address, dropoff_lat, dropoff_lng, dropoff_address, distance_m, duration_s,
	price_quoted_minor, price_final_minor, commission_minor, cancellation_fee_minor, currency, note, matching_attempts,
	cancelled_by, cancellation_reason, requested_at, assigned_at, arrived_at, started_at, completed_at, cancelled_at, quote_id, pickup_code, business_id`

func scanRide(row pgx.Row) (*Ride, error) {
	var r Ride
	if err := row.Scan(&r.ID, &r.CustomerID, &r.DriverID, &r.CategoryID, &r.State, &r.PaymentMethod, &r.PaymentStatus,
		&r.Pickup.Lat, &r.Pickup.Lng, &r.PickupAddress, &r.Dropoff.Lat, &r.Dropoff.Lng, &r.DropoffAddress, &r.DistanceM, &r.DurationS,
		&r.PriceQuotedMinor, &r.PriceFinalMinor, &r.CommissionMinor, &r.CancellationFeeMinor, &r.Currency, &r.Note, &r.MatchingAttempts,
		&r.CancelledBy, &r.CancellationReason, &r.RequestedAt, &r.AssignedAt, &r.ArrivedAt, &r.StartedAt, &r.CompletedAt, &r.CancelledAt, &r.QuoteID, &r.PickupCode, &r.BusinessID); err != nil {
		return nil, err
	}
	r.Currency = strings.TrimSpace(r.Currency)
	return &r, nil
}

type RequestInput struct {
	QuoteID       uuid.UUID `json:"quote_id"`
	PaymentMethod string    `json:"payment_method"` // cash | wallet | business (card: pas modulit payments)
	Note          string    `json:"note"`
	// Kërkohet vetëm me metodën `business`; anëtarësia dhe kufiri verifikohen para krijimit.
	BusinessID *uuid.UUID `json:"business_id"`
}

// Request — krijon kërkesën nga një quote (çmimi vjen nga serveri), idempotente me Idempotency-Key.
func (s *Service) Request(ctx context.Context, a principal.Actor, idemKey string, in RequestInput) (*Ride, error) {
	fields := map[string]string{}
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" || len(idemKey) > 100 {
		fields["idempotency_key"] = "required"
	}
	switch in.PaymentMethod {
	case "cash", "wallet":
	case "business":
		// Ndërmarrja jepet gjithmonë me metodën e vet; pa të, udhëtimi do të mbetej i papagueshëm.
		if in.BusinessID == nil {
			fields["business_id"] = "required"
		} else if s.business == nil {
			return nil, ErrPaymentMethodUnavailable
		}
	case "card":
		return nil, ErrPaymentMethodUnavailable
	default:
		fields["payment_method"] = "invalid"
	}
	if in.QuoteID == uuid.Nil {
		fields["quote_id"] = "required"
	}
	in.Note = strings.Join(strings.Fields(in.Note), " ")
	if utf8.RuneCountInString(in.Note) > 200 {
		fields["note"] = "too_long"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	if s.flags != nil && !s.flags.Enabled(ctx, "rides.request", a.UserID) {
		return nil, ErrServiceDisabled
	}
	if s.velocity != nil {
		if err := s.velocity.Allow(ctx, a.UserID, "ride_request", 10, time.Hour); err != nil {
			return nil, err
		}
	}

	// idempotencë: e njëjta kërkesë kthen të njëjtin udhëtim
	existing, err := scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE customer_id = $1 AND idempotency_key = $2`, a.UserID, idemKey))
	if err == nil {
		if existing.QuoteID != in.QuoteID {
			return nil, httpx.ErrIdempotency
		}
		return s.decorate(ctx, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var out *Ride
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q, err := pricing.LoadQuote(ctx, tx, in.QuoteID, a.UserID)
		if err != nil {
			return err
		}
		if in.PaymentMethod == "wallet" {
			bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(a.UserID))
			if err != nil && !errors.Is(err, ledger.ErrAccountMissing) {
				return err
			}
			if int64(bal.Minor) < q.PriceMinor {
				return ErrInsufficientFunds
			}
		}
		if in.PaymentMethod == "business" {
			// Anëtarësia, kufiri mujor dhe bilanci kontrollohen para se udhëtimi të ekzistojë:
			// një shofer i caktuar një udhëtimi që nuk paguhet dot është kohë e humbur e tij.
			if _, err := s.business.Authorize(ctx, *in.BusinessID, a.UserID, q.PriceMinor); err != nil {
				return err
			}
		}
		r, err := scanRide(tx.QueryRow(ctx, `
			INSERT INTO rides (customer_id, quote_id, service_area_id, category_id, state, payment_method,
			  pickup_lat, pickup_lng, pickup_address, dropoff_lat, dropoff_lng, dropoff_address,
			  distance_m, duration_s, price_quoted_minor, currency, note, idempotency_key, pickup_code, business_id)
			VALUES ($1,$2,$3,$4,'matching',$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING `+rideCols,
			a.UserID, q.ID, q.AreaID, q.CategoryID, in.PaymentMethod,
			q.Pickup.Lat, q.Pickup.Lng, q.PickupAddress, q.Dropoff.Lat, q.Dropoff.Lng, q.DropoffAddress,
			q.DistanceM, q.DurationS, q.PriceMinor, q.Currency, nullable(in.Note), idemKey, newPickupCode(),
			in.BusinessID))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrActiveRide
			}
			return err
		}
		out = r
		if err := rideEvent(ctx, tx, r.ID, nil, StateMatching, "customer", &a.UserID, map[string]any{"payment_method": in.PaymentMethod}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", r.ID.String(), "RideRequested", map[string]any{
			"ride_id": r.ID, "customer_id": a.UserID, "category": r.CategoryID, "area": q.AreaID, "attempt": 1})
	})
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, out)
}

// Get — udhëtimi, i dukshëm vetëm për klientin ose shoferin e caktuar (BOLA §52).
func (s *Service) Get(ctx context.Context, a principal.Actor, rideID uuid.UUID) (*Ride, error) {
	r, err := scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2)`, rideID, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.decorate(ctx, forActor(r, a))
}

// History — udhëtimet e klientit, nga më i riu (cursor: before = requested_at).
func (s *Service) History(ctx context.Context, a principal.Actor, before *time.Time, limit int) ([]Ride, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if before == nil {
		t := s.now().Add(time.Hour)
		before = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT `+rideCols+` FROM rides WHERE customer_id = $1 AND requested_at < $2
		ORDER BY requested_at DESC LIMIT $3`, a.UserID, *before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ride{}
	for rows.Next() {
		r, err := scanRide(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// CancelByCustomer — anulim me politikë tarifash: falas gjatë kërkimit dhe brenda periudhës së hirit
// pas caktimit; pastaj tarifa e rregullit. Nuk anulohet dot udhëtimi i nisur.
func (s *Service) CancelByCustomer(ctx context.Context, a principal.Actor, rideID uuid.UUID, reason string) (*Ride, error) {
	reason = clip(reason, 200)
	var out *Ride
	var driverID *uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRide(tx.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 AND customer_id = $2 FOR UPDATE`, rideID, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(r.State, StateCancelled) {
			return ErrInvalidState
		}
		var fee int64
		if (r.State == StateAssigned || r.State == StateArrived) && r.AssignedAt != nil {
			var feeMinor int64
			var grace int
			if err := tx.QueryRow(ctx, `SELECT p.cancellation_fee_minor, p.cancellation_grace_seconds
				FROM ride_quotes q JOIN pricing_rules p ON p.id = q.pricing_rule_id WHERE q.id = $1`, r.QuoteID).Scan(&feeMinor, &grace); err != nil {
				return err
			}
			if s.now().Sub(*r.AssignedAt) > time.Duration(grace)*time.Second {
				fee = feeMinor
			}
		}
		payStatus := "none"
		if fee > 0 && r.PaymentMethod == "wallet" {
			payStatus = "pending" // tarifa arkëtohet nga wallet-i pas commit-it (settle)
		}
		from := r.State
		r, err = scanRide(tx.QueryRow(ctx, `
			UPDATE rides SET state = 'cancelled', cancelled_by = 'customer', cancellation_reason = $2,
			  cancellation_fee_minor = $3, payment_status = $4, cancelled_at = now(), updated_at = now()
			WHERE id = $1 RETURNING `+rideCols, rideID, nullable(reason), fee, payStatus))
		if err != nil {
			return err
		}
		out, driverID = r, r.DriverID
		if _, err := tx.Exec(ctx, `UPDATE ride_offers SET state = 'withdrawn', responded_at = now() WHERE ride_id = $1 AND state = 'offered'`, rideID); err != nil {
			return err
		}
		if err := rideEvent(ctx, tx, rideID, &from, StateCancelled, "customer", &a.UserID, map[string]any{"fee_minor": fee, "reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideCancelled", map[string]any{
			"ride_id": rideID, "by": "customer", "driver_id": driverID, "fee_minor": fee})
	})
	if err != nil {
		return nil, err
	}
	if driverID != nil {
		_ = s.loc.SetAvailable(ctx, *driverID)
	}
	if out.PaymentStatus == "pending" {
		_ = s.settle(ctx, out) // dështimi mbetet 'pending' dhe riprovohet nga SettlePending
		out, _ = scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1`, rideID))
	}
	return s.decorate(ctx, out)
}

// newPickupCode — 4 shifra (crypto/rand): klienti ia tregon/skanon shoferit para nisjes.
func newPickupCode() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	n := int(binary.BigEndian.Uint16(b)) % 10000
	return fmt.Sprintf("%04d", n)
}

// forActor — kodi i marrjes shfaqet vetëm te klienti; shoferi e verifikon, nuk e sheh.
func forActor(r *Ride, a principal.Actor) *Ride {
	if r != nil && r.CustomerID != a.UserID {
		r.PickupCode = nil
	}
	return r
}

// QRToken — QR i nënshkruar, jetëshkurtër (5 min), me kodin e marrjes brenda (§25: pa sekrete, referencë e nënshkruar).
func (s *Service) QRToken(ctx context.Context, a principal.Actor, rideID uuid.UUID) (string, time.Time, error) {
	if s.qr == nil {
		return "", time.Time{}, httpx.ErrUnavailable
	}
	r, err := scanRide(s.pool.QueryRow(ctx, `SELECT `+rideCols+` FROM rides WHERE id = $1 AND customer_id = $2`, rideID, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, httpx.ErrNotFound
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if r.PickupCode == nil || !IsActive(r.State) {
		return "", time.Time{}, ErrInvalidState
	}
	return s.qr.SignClaims("ride_pickup", map[string]any{"ride": rideID.String(), "code": *r.PickupCode}, 5*time.Minute)
}

// decorate — shton kartën e shoferit dhe lokacionin e gjallë kur ka kuptim.
func (s *Service) decorate(ctx context.Context, r *Ride) (*Ride, error) {
	if r == nil || r.DriverID == nil {
		return r, nil
	}
	var c DriverCard
	var sum, cnt int
	err := s.pool.QueryRow(ctx, `
		SELECT d.user_id, u.full_name, d.vehicle_make, d.vehicle_model, d.vehicle_plate, d.vehicle_color, d.rating_sum, d.rating_count
		FROM drivers d JOIN users u ON u.id = d.user_id WHERE d.user_id = $1`, *r.DriverID).
		Scan(&c.ID, &c.Name, &c.VehicleMake, &c.VehicleModel, &c.VehiclePlate, &c.VehicleColor, &sum, &cnt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if cnt > 0 {
		v := float64(int(float64(sum)/float64(cnt)*100+0.5)) / 100
		c.Rating = &v
	}
	if IsActive(r.State) {
		if st, err := s.loc.State(ctx, *r.DriverID); err == nil && st != nil && st.Point.Valid() {
			p, t := st.Point, st.RecordedAt
			c.Location, c.LocationAt = &p, &t
		}
	}
	r.Driver = &c
	return r, nil
}

func rideEvent(ctx context.Context, tx events.Execer, rideID uuid.UUID, from *string, to, actorType string, actorID *uuid.UUID, meta map[string]any) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO ride_events (ride_id, from_state, to_state, actor_type, actor_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)`, rideID, from, to, actorType, actorID, metaJSON)
	return err
}

func clip(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
