package parcels

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/promos"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/maps"
)

var (
	ErrInsufficient    = &httpx.APIError{Code: "INSUFFICIENT_FUNDS", MessageKey: "errors.wallet.insufficient_funds", HTTPStatus: http.StatusPaymentRequired}
	ErrInvalidState    = &httpx.APIError{Code: "PARCEL_INVALID_STATE", MessageKey: "errors.parcels.invalid_state", HTTPStatus: http.StatusConflict}
	ErrQuoteExpired    = &httpx.APIError{Code: "QUOTE_EXPIRED", MessageKey: "errors.parcels.quote_expired", HTTPStatus: http.StatusConflict}
	ErrOfferGone       = &httpx.APIError{Code: "OFFER_GONE", MessageKey: "errors.orders.offer_gone", HTTPStatus: http.StatusConflict}
	ErrCourierAssigned = &httpx.APIError{Code: "COURIER_ALREADY_BUSY", MessageKey: "errors.orders.courier_busy", HTTPStatus: http.StatusConflict}
	ErrPickupCode      = &httpx.APIError{Code: "PICKUP_CODE_INVALID", MessageKey: "errors.orders.pickup_code_invalid", HTTPStatus: http.StatusUnprocessableEntity}
	ErrDeliveryCode    = &httpx.APIError{Code: "DELIVERY_CODE_INVALID", MessageKey: "errors.parcels.delivery_code_invalid", HTTPStatus: http.StatusUnprocessableEntity}
)

const QuoteTTL = 2 * time.Minute

// Location — gjendja e korrierit në Redis (moduli location).
type Location interface {
	SetBusyParcel(ctx context.Context, driverID uuid.UUID, parcelID uuid.UUID) error
	SetAvailable(ctx context.Context, driverID uuid.UUID) error
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
	maps   maps.Provider
	loc    Location
	promos *promos.Service
	now    func() time.Time
}

func New(pool *pgxpool.Pool, led *ledger.Service, m maps.Provider) *Service {
	return &Service{pool: pool, ledger: led, maps: m, now: time.Now}
}

func (s *Service) WithLocation(l Location) *Service {
	s.loc = l
	return s
}

func (s *Service) WithPromos(p *promos.Service) *Service {
	s.promos = p
	return s
}

// CourierCard — çfarë sheh klienti për korrierin.
type CourierCard struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	VehicleMake  string    `json:"vehicle_make"`
	VehicleModel string    `json:"vehicle_model"`
	VehiclePlate string    `json:"vehicle_plate"`
	VehicleColor string    `json:"vehicle_color"`
}

type Parcel struct {
	ID                 uuid.UUID    `json:"id"`
	Code               string       `json:"code"`
	PickupCode         string       `json:"pickup_code,omitempty"`   // vetëm klienti
	DeliveryCode       string       `json:"delivery_code,omitempty"` // vetëm klienti
	CustomerID         uuid.UUID    `json:"customer_id"`
	CourierID          *uuid.UUID   `json:"courier_id"`
	State              string       `json:"state"`
	Size               string       `json:"size"`
	PaymentMethod      string       `json:"payment_method"`
	PaymentStatus      string       `json:"payment_status"`
	Pickup             geo.Point    `json:"pickup"`
	PickupAddress      *string      `json:"pickup_address"`
	PickupContactName  *string      `json:"pickup_contact_name"`
	PickupContactPhone *string      `json:"pickup_contact_phone"`
	Dropoff            geo.Point    `json:"dropoff"`
	DropoffAddress     *string      `json:"dropoff_address"`
	RecipientName      string       `json:"recipient_name"`
	RecipientPhone     string       `json:"recipient_phone"`
	Note               *string      `json:"note"`
	DistanceM          int          `json:"distance_m"`
	DurationS          int          `json:"duration_s"`
	PriceMinor         int64        `json:"price_minor"`
	DiscountMinor      int64        `json:"discount_minor"`
	CouponCode         *string      `json:"coupon_code"`
	CommissionMinor    int64        `json:"-"`
	Currency           string       `json:"currency"`
	CancelledBy        *string      `json:"cancelled_by"`
	CancellationReason *string      `json:"cancellation_reason"`
	CreatedAt          time.Time    `json:"created_at"`
	AssignedAt         *time.Time   `json:"assigned_at"`
	PickedUpAt         *time.Time   `json:"picked_up_at"`
	DeliveredAt        *time.Time   `json:"delivered_at"`
	CancelledAt        *time.Time   `json:"cancelled_at"`
	Courier            *CourierCard `json:"courier,omitempty"`
}

const parcelCols = `id, code, pickup_code, delivery_code, customer_id, courier_id, state, size, payment_method, payment_status,
	pickup_lat, pickup_lng, pickup_address, pickup_contact_name, pickup_contact_phone,
	dropoff_lat, dropoff_lng, dropoff_address, recipient_name, recipient_phone, note,
	distance_m, duration_s, price_minor, commission_minor, currency, cancelled_by, cancellation_reason,
	created_at, assigned_at, picked_up_at, delivered_at, cancelled_at, discount_minor, coupon_code`

func scanParcel(row pgx.Row) (*Parcel, error) {
	var p Parcel
	if err := row.Scan(&p.ID, &p.Code, &p.PickupCode, &p.DeliveryCode, &p.CustomerID, &p.CourierID, &p.State, &p.Size, &p.PaymentMethod, &p.PaymentStatus,
		&p.Pickup.Lat, &p.Pickup.Lng, &p.PickupAddress, &p.PickupContactName, &p.PickupContactPhone,
		&p.Dropoff.Lat, &p.Dropoff.Lng, &p.DropoffAddress, &p.RecipientName, &p.RecipientPhone, &p.Note,
		&p.DistanceM, &p.DurationS, &p.PriceMinor, &p.CommissionMinor, &p.Currency, &p.CancelledBy, &p.CancellationReason,
		&p.CreatedAt, &p.AssignedAt, &p.PickedUpAt, &p.DeliveredAt, &p.CancelledAt, &p.DiscountMinor, &p.CouponCode); err != nil {
		return nil, err
	}
	return &p, nil
}

// withCourier — karta e korrierit për klientin (emri dhe automjeti), pa telefon.
func (s *Service) withCourier(ctx context.Context, p *Parcel) *Parcel {
	if p.CourierID == nil {
		return p
	}
	var c CourierCard
	var name *string
	if err := s.pool.QueryRow(ctx, `SELECT u.id, u.full_name, d.vehicle_make, d.vehicle_model, d.vehicle_plate, d.vehicle_color
		FROM drivers d JOIN users u ON u.id = d.user_id WHERE d.user_id = $1`, *p.CourierID).
		Scan(&c.ID, &name, &c.VehicleMake, &c.VehicleModel, &c.VehiclePlate, &c.VehicleColor); err == nil {
		if name != nil {
			c.Name = *name
		}
		p.Courier = &c
	}
	return p
}

// forCourier — korrieri nuk i sheh kodet: ia thonë dërguesi dhe marrësi.
func forCourier(p *Parcel) *Parcel {
	if p == nil {
		return nil
	}
	c := *p
	c.PickupCode, c.DeliveryCode = "", ""
	c.Courier = nil
	return &c
}

func newCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	out := make([]byte, 6)
	for i, x := range b {
		out[i] = alphabet[int(x)%len(alphabet)]
	}
	return string(out)
}

func newDigits() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	out := make([]byte, 4)
	for i, x := range b {
		out[i] = '0' + x%10
	}
	return string(out)
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

func (s *Service) pricing(ctx context.Context, size string) (Pricing, error) {
	var p Pricing
	err := s.pool.QueryRow(ctx, `SELECT size, base_minor, per_km_minor, commission_bp, currency FROM parcel_pricing WHERE size = $1`, size).
		Scan(&p.Size, &p.BaseMinor, &p.PerKmMinor, &p.CommissionBP, &p.Currency)
	return p, err
}

// --- quote ------------------------------------------------------------------------------------

type QuoteInput struct {
	Size           string    `json:"size"`
	Pickup         geo.Point `json:"pickup"`
	PickupAddress  string    `json:"pickup_address"`
	Dropoff        geo.Point `json:"dropoff"`
	DropoffAddress string    `json:"dropoff_address"`
}

type Quote struct {
	ID         uuid.UUID `json:"id"`
	Size       string    `json:"size"`
	DistanceM  int       `json:"distance_m"`
	DurationS  int       `json:"duration_s"`
	PriceMinor int64     `json:"price_minor"`
	Currency   string    `json:"currency"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Quote — çmimi i dërgesës, server-side, me afat 2 min (si te udhëtimet).
func (s *Service) Quote(ctx context.Context, customerID uuid.UUID, in QuoteInput) (*Quote, error) {
	fields := map[string]string{}
	if !ValidSize(in.Size) {
		fields["size"] = "invalid"
	}
	if !in.Pickup.Valid() || !geo.InKosovo(in.Pickup) {
		fields["pickup"] = "invalid"
	}
	if !in.Dropoff.Valid() || !geo.InKosovo(in.Dropoff) {
		fields["dropoff"] = "invalid"
	}
	if len(fields) == 0 && geo.Haversine(in.Pickup, in.Dropoff) < 50 {
		fields["dropoff"] = "same_as_pickup"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	pr, err := s.pricing(ctx, in.Size)
	if err != nil {
		return nil, err
	}
	route, err := s.maps.Route(ctx, in.Pickup, in.Dropoff)
	if err != nil {
		if errors.Is(err, maps.ErrNoRoute) {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"dropoff": "no_route"})
		}
		return nil, httpx.ErrUnavailable.With(err)
	}
	q := &Quote{Size: in.Size, DistanceM: route.DistanceM, DurationS: route.DurationS, PriceMinor: Price(pr, route.DistanceM), Currency: pr.Currency, ExpiresAt: s.now().Add(QuoteTTL)}
	if err := s.pool.QueryRow(ctx, `INSERT INTO parcel_quotes (customer_id, size, pickup_lat, pickup_lng, pickup_address, dropoff_lat, dropoff_lng, dropoff_address,
		distance_m, duration_s, price_minor, commission_bp, currency, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
		customerID, in.Size, in.Pickup.Lat, in.Pickup.Lng, nullable(clip(in.PickupAddress, 200)), in.Dropoff.Lat, in.Dropoff.Lng, nullable(clip(in.DropoffAddress, 200)),
		q.DistanceM, q.DurationS, q.PriceMinor, pr.CommissionBP, q.Currency, q.ExpiresAt).Scan(&q.ID); err != nil {
		return nil, err
	}
	return q, nil
}

// --- create -----------------------------------------------------------------------------------

type CreateInput struct {
	QuoteID            uuid.UUID `json:"quote_id"`
	PaymentMethod      string    `json:"payment_method"` // cash | wallet
	PickupContactName  string    `json:"pickup_contact_name"`
	PickupContactPhone string    `json:"pickup_contact_phone"`
	RecipientName      string    `json:"recipient_name"`
	RecipientPhone     string    `json:"recipient_phone"`
	Note               string    `json:"note"`
	CouponCode         string    `json:"coupon_code"`
}

func (s *Service) Create(ctx context.Context, a principal.Actor, idemKey string, in CreateInput) (*Parcel, error) {
	idemKey = strings.TrimSpace(idemKey)
	fields := map[string]string{}
	if idemKey == "" || len(idemKey) > 100 {
		fields["idempotency_key"] = "required"
	}
	if in.PaymentMethod != "cash" && in.PaymentMethod != "wallet" {
		fields["payment_method"] = "invalid"
	}
	in.RecipientName = clip(in.RecipientName, 80)
	in.RecipientPhone = strings.TrimSpace(in.RecipientPhone)
	if in.RecipientName == "" {
		fields["recipient_name"] = "required"
	}
	if n := len(in.RecipientPhone); n < 8 || n > 20 || !strings.HasPrefix(in.RecipientPhone, "+") {
		fields["recipient_phone"] = "invalid"
	}
	if in.QuoteID == uuid.Nil {
		fields["quote_id"] = "required"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	if existing, err := scanParcel(s.pool.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE customer_id = $1 AND idempotency_key = $2`, a.UserID, idemKey)); err == nil {
		return s.withCourier(ctx, existing), nil
	}

	var q struct {
		size, currency               string
		pickup, dropoff              geo.Point
		pickupAddr, dropoffAddr      *string
		distanceM, durationS, commBP int
		priceMinor                   int64
		expiresAt                    time.Time
	}
	err := s.pool.QueryRow(ctx, `SELECT size, currency, pickup_lat, pickup_lng, pickup_address, dropoff_lat, dropoff_lng, dropoff_address, distance_m, duration_s, commission_bp, price_minor, expires_at
		FROM parcel_quotes WHERE id = $1 AND customer_id = $2`, in.QuoteID, a.UserID).
		Scan(&q.size, &q.currency, &q.pickup.Lat, &q.pickup.Lng, &q.pickupAddr, &q.dropoff.Lat, &q.dropoff.Lng, &q.dropoffAddr, &q.distanceM, &q.durationS, &q.commBP, &q.priceMinor, &q.expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"quote_id": "invalid"})
	}
	if err != nil {
		return nil, err
	}
	if s.now().After(q.expiresAt) {
		return nil, ErrQuoteExpired
	}
	var discount int64
	coupon := promos.Normalize(in.CouponCode)
	if coupon != "" {
		if s.promos == nil {
			return nil, promos.ErrInvalid
		}
		applied, err := s.promos.Apply(ctx, coupon, a.UserID, promos.ScopeParcels, q.priceMinor)
		if err != nil {
			return nil, err
		}
		discount, coupon = applied.DiscountMinor, applied.Code
	}
	if in.PaymentMethod == "wallet" {
		bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(a.UserID))
		if err != nil {
			return nil, err
		}
		if int64(bal.Minor) < q.priceMinor-discount {
			return nil, ErrInsufficient
		}
	}

	var out *Parcel
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := scanParcel(tx.QueryRow(ctx, `INSERT INTO parcels (code, pickup_code, delivery_code, customer_id, quote_id, state, size, payment_method,
			pickup_lat, pickup_lng, pickup_address, pickup_contact_name, pickup_contact_phone,
			dropoff_lat, dropoff_lng, dropoff_address, recipient_name, recipient_phone, note,
			distance_m, duration_s, price_minor, commission_minor, currency, idempotency_key, discount_minor, coupon_code)
			VALUES ($1,$2,$3,$4,$5,'requested',$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26) RETURNING `+parcelCols,
			newCode(), newDigits(), newDigits(), a.UserID, in.QuoteID, q.size, in.PaymentMethod,
			q.pickup.Lat, q.pickup.Lng, q.pickupAddr, nullable(clip(in.PickupContactName, 80)), nullable(clip(in.PickupContactPhone, 20)),
			q.dropoff.Lat, q.dropoff.Lng, q.dropoffAddr, in.RecipientName, in.RecipientPhone, nullable(clip(in.Note, 300)),
			q.distanceM, q.durationS, q.priceMinor, Commission(q.priceMinor, q.commBP), q.currency, idemKey, discount, nullable(coupon)))
		if err != nil {
			return err
		}
		out = p
		if coupon != "" {
			if err := s.promos.Redeem(ctx, tx, coupon, a.UserID, "parcel:"+p.ID.String(), discount); err != nil {
				return err
			}
		}
		if err := parcelEvent(ctx, tx, p.ID, nil, StateRequested, "customer", &a.UserID, nil); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "parcel", p.ID.String(), "ParcelRequested", map[string]any{
			"parcel_id": p.ID, "code": p.Code, "customer_id": a.UserID, "price_minor": p.PriceMinor})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- klienti ----------------------------------------------------------------------------------

func (s *Service) Get(ctx context.Context, a principal.Actor, id uuid.UUID) (*Parcel, error) {
	p, err := scanParcel(s.pool.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE id = $1 AND (customer_id = $2 OR courier_id = $2)`, id, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if p.CustomerID != a.UserID {
		return forCourier(p), nil
	}
	return s.withCourier(ctx, p), nil
}

func (s *Service) Active(ctx context.Context, a principal.Actor) (*Parcel, error) {
	p, err := scanParcel(s.pool.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE customer_id = $1 AND state IN ('requested','courier_assigned','picked_up') ORDER BY created_at DESC LIMIT 1`, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.withCourier(ctx, p), nil
}

func (s *Service) History(ctx context.Context, a principal.Actor, before *time.Time, limit int) ([]Parcel, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if before == nil {
		t := s.now().Add(time.Minute)
		before = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT `+parcelCols+` FROM parcels WHERE customer_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`, a.UserID, *before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Parcel{}
	for rows.Next() {
		p, err := scanParcel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s.withCourier(ctx, p))
	}
	return out, rows.Err()
}

// Cancel — klienti anulon para marrjes; pa tarifë në V1. Korrieri i caktuar lirohet.
func (s *Service) Cancel(ctx context.Context, a principal.Actor, id uuid.UUID, reason string) (*Parcel, error) {
	var out *Parcel
	var courier *uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := scanParcel(tx.QueryRow(ctx, `SELECT `+parcelCols+` FROM parcels WHERE id = $1 AND customer_id = $2 FOR UPDATE`, id, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CustomerCanCancel(p.State) {
			return ErrInvalidState
		}
		courier = p.CourierID
		from := p.State
		p, err = scanParcel(tx.QueryRow(ctx, `UPDATE parcels SET state = 'cancelled', payment_status = 'none', cancelled_by = 'customer', cancellation_reason = $2,
			cancelled_at = now(), updated_at = now() WHERE id = $1 RETURNING `+parcelCols, id, nullable(clip(reason, 200))))
		if err != nil {
			return err
		}
		out = p
		if _, err := tx.Exec(ctx, `UPDATE parcel_offers SET state = 'withdrawn', responded_at = now() WHERE parcel_id = $1 AND state = 'offered'`, id); err != nil {
			return err
		}
		if err := parcelEvent(ctx, tx, id, &from, StateCancelled, "customer", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "parcel", id.String(), "ParcelCancelled", map[string]any{
			"parcel_id": id, "code": p.Code, "customer_id": a.UserID, "courier_id": courier, "by": "customer"})
	})
	if err != nil {
		return nil, err
	}
	if courier != nil && s.loc != nil {
		_ = s.loc.SetAvailable(ctx, *courier)
	}
	return s.withCourier(ctx, out), nil
}

// --- shlyerja ---------------------------------------------------------------------------------

// settle — si te udhëtimet: wallet → klienti paguan çmimin, korrieri merr çmimin − komisionin;
// cash → korrieri e mban cash-in dhe i detyrohet platformës komisionin.
func (s *Service) settle(ctx context.Context, p *Parcel) error {
	if p.State != StateDelivered || p.PaymentStatus != "pending" || p.CourierID == nil {
		return nil
	}
	price := money.Minor(p.PriceMinor)
	commission := money.Minor(p.CommissionMinor)
	discount := money.Minor(p.DiscountMinor)
	courierWallet := "driver:" + p.CourierID.String() + ":wallet"
	cid := *p.CourierID
	if err := s.ledger.EnsureAccount(ctx, courierWallet, "driver", &cid, "liability", p.Currency); err != nil {
		return err
	}
	ref := "parcel:" + p.ID.String()
	idem := ref + ":fare"
	var tx ledger.Transaction
	status := "cash"
	if p.PaymentMethod == "wallet" {
		bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(p.CustomerID))
		if err != nil {
			return err
		}
		if bal.Minor < price-discount {
			return s.setPaymentStatus(ctx, p, "failed")
		}
		status = "paid"
		postings := []ledger.Posting{
			{AccountCode: ledger.UserWalletCode(p.CustomerID), Debit: price - discount},
			{AccountCode: courierWallet, Credit: price - commission},
		}
		if commission > 0 {
			postings = append(postings, ledger.Posting{AccountCode: "krejt:commission", Credit: commission})
		}
		if discount > 0 {
			postings = append(postings, ledger.Posting{AccountCode: promos.MarketingAccount, Debit: discount})
		}
		tx = ledger.Transaction{Kind: "parcel_fare", Reference: ref, IdempotencyKey: idem, Currency: p.Currency, Postings: postings}
	} else if commission > 0 || discount > 0 {
		// Cash: korrieri mblodhi çmimin minus zbritjen; komisionin ia detyrohet platformës, zbritjen ia mbulon ajo.
		postings := []ledger.Posting{}
		if commission > 0 {
			postings = append(postings, ledger.Posting{AccountCode: courierWallet, Debit: commission}, ledger.Posting{AccountCode: "krejt:commission", Credit: commission})
		}
		if discount > 0 {
			postings = append(postings, ledger.Posting{AccountCode: courierWallet, Credit: discount}, ledger.Posting{AccountCode: promos.MarketingAccount, Debit: discount})
		}
		tx = ledger.Transaction{Kind: "parcel_cash_commission", Reference: ref, IdempotencyKey: idem, Currency: p.Currency, Postings: postings}
	}
	if len(tx.Postings) > 0 {
		if _, err := s.ledger.Post(ctx, tx); err != nil {
			return err
		}
	}
	return s.setPaymentStatus(ctx, p, status)
}

func (s *Service) setPaymentStatus(ctx context.Context, p *Parcel, status string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE parcels SET payment_status = $2, updated_at = now() WHERE id = $1 AND payment_status = 'pending'`, p.ID, status); err != nil {
			return err
		}
		ev := "ParcelPaymentSettled"
		if status == "failed" {
			ev = "ParcelPaymentFailed"
		}
		return events.Emit(ctx, tx, "parcel", p.ID.String(), ev, map[string]any{
			"parcel_id": p.ID, "customer_id": p.CustomerID, "courier_id": p.CourierID, "status": status, "price_minor": p.PriceMinor})
	})
}

// SettlePending — riprovon shlyerjet e mbetura (worker, çdo 10 s).
func (s *Service) SettlePending(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+parcelCols+` FROM parcels WHERE payment_status = 'pending' AND state = 'delivered' AND updated_at < now() - interval '10 seconds' ORDER BY updated_at LIMIT 50`)
	if err != nil {
		return 0, err
	}
	var list []*Parcel
	for rows.Next() {
		p, err := scanParcel(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, p)
	}
	rows.Close()
	n := 0
	for _, p := range list {
		if err := s.settle(ctx, p); err != nil {
			return n, fmt.Errorf("parcels: settle %s: %w", p.ID, err)
		}
		n++
	}
	return n, nil
}

func parcelEvent(ctx context.Context, tx events.Execer, parcelID uuid.UUID, from *string, to, actorType string, actorID *uuid.UUID, meta map[string]any) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO parcel_events (parcel_id, from_state, to_state, actor_type, actor_id, metadata) VALUES ($1,$2,$3,$4,$5,$6)`,
		parcelID, from, to, actorType, actorID, metaJSON)
	return err
}
