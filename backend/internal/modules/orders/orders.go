package orders

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/catalog"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/promos"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrMerchantClosed  = &httpx.APIError{Code: "MERCHANT_CLOSED", MessageKey: "errors.orders.merchant_closed", HTTPStatus: http.StatusConflict}
	ErrMinOrder        = &httpx.APIError{Code: "ORDER_BELOW_MINIMUM", MessageKey: "errors.orders.below_minimum", HTTPStatus: http.StatusUnprocessableEntity}
	ErrInsufficient    = &httpx.APIError{Code: "INSUFFICIENT_FUNDS", MessageKey: "errors.wallet.insufficient_funds", HTTPStatus: http.StatusPaymentRequired}
	ErrInvalidState    = &httpx.APIError{Code: "ORDER_INVALID_STATE", MessageKey: "errors.orders.invalid_state", HTTPStatus: http.StatusConflict}
	ErrPaymentMethod   = &httpx.APIError{Code: "PAYMENT_METHOD_UNAVAILABLE", MessageKey: "errors.orders.payment_method_unavailable", HTTPStatus: http.StatusUnprocessableEntity}
	ErrAddressRequired = &httpx.APIError{Code: "ADDRESS_REQUIRED", MessageKey: "errors.orders.address_required", HTTPStatus: http.StatusUnprocessableEntity}
	ErrOfferGone       = &httpx.APIError{Code: "OFFER_GONE", MessageKey: "errors.orders.offer_gone", HTTPStatus: http.StatusConflict}
	ErrCourierAssigned = &httpx.APIError{Code: "COURIER_ALREADY_BUSY", MessageKey: "errors.orders.courier_busy", HTTPStatus: http.StatusConflict}
	// ErrPickupCode — kodi 6-shkronjësh i marrjes nuk përputhet; mesazh i vetin, jo "kontrollo të dhënat".
	ErrPickupCode = &httpx.APIError{Code: "PICKUP_CODE_INVALID", MessageKey: "errors.orders.pickup_code_invalid", HTTPStatus: http.StatusUnprocessableEntity}
)

// Merchants — çka i duhet porosive nga moduli merchants (ndërfaqe: pa varësi ciklike).
type Merchants interface {
	Membership(ctx context.Context, userID, merchantID uuid.UUID) (string, error)
}

// Catalog — vlerësimi server-side i rreshtave.
type Catalog interface {
	Price(ctx context.Context, merchantID uuid.UUID, sel catalog.Selection) (*catalog.PricedLine, error)
}

type Service struct {
	pool    *pgxpool.Pool
	ledger  *ledger.Service
	promos  *promos.Service
	catalog Catalog
	members Merchants
	loc     Location
	now     func() time.Time
}

func New(pool *pgxpool.Pool, led *ledger.Service, cat Catalog, m Merchants) *Service {
	return &Service{pool: pool, ledger: led, catalog: cat, members: m, now: time.Now}
}

// WithPromos — kuponat te checkout-i; pa të, kodi i kuponit refuzohet.
func (s *Service) WithPromos(p *promos.Service) *Service {
	s.promos = p
	return s
}

type Item struct {
	ID         uuid.UUID   `json:"id"`
	ProductID  uuid.UUID   `json:"product_id"`
	Name       string      `json:"name"`
	Options    []string    `json:"options"`
	OptionIDs  []uuid.UUID `json:"option_ids"`
	UnitMinor  int64       `json:"unit_minor"`
	Quantity   int         `json:"quantity"`
	TotalMinor int64       `json:"total_minor"`
}

type Order struct {
	ID                  uuid.UUID  `json:"id"`
	Code                string     `json:"code"`
	CustomerID          uuid.UUID  `json:"customer_id"`
	MerchantID          uuid.UUID  `json:"merchant_id"`
	MerchantName        string     `json:"merchant_name,omitempty"`
	MerchantLocation    *geo.Point `json:"merchant_location,omitempty"` // për hartën e ndjekjes
	CourierID           *uuid.UUID `json:"courier_id"`
	State               string     `json:"state"`
	Fulfillment         string     `json:"fulfillment"`
	PaymentMethod       string     `json:"payment_method"`
	PaymentStatus       string     `json:"payment_status"`
	ItemsTotalMinor     int64      `json:"items_total_minor"`
	DeliveryFeeMinor    int64      `json:"delivery_fee_minor"`
	DiscountMinor       int64      `json:"discount_minor"`
	TotalMinor          int64      `json:"total_minor"`
	CommissionMinor     *int64     `json:"-"`
	Currency            string     `json:"currency"`
	AddressLine1        *string    `json:"address_line1"`
	Address             *geo.Point `json:"address,omitempty"`
	AddressInstructions *string    `json:"address_instructions"`
	Note                *string    `json:"note"`
	PrepTimeMin         int        `json:"prep_time_min"`
	ReadyAtEstimate     *time.Time `json:"ready_at_estimate"`
	CancelledBy         *string    `json:"cancelled_by"`
	CancellationReason  *string    `json:"cancellation_reason"`
	CreatedAt           time.Time  `json:"created_at"`
	AcceptedAt          *time.Time `json:"accepted_at"`
	ReadyAt             *time.Time `json:"ready_at"`
	PickedUpAt          *time.Time `json:"picked_up_at"`
	DeliveredAt         *time.Time `json:"delivered_at"`
	CancelledAt         *time.Time `json:"cancelled_at"`
	Items               []Item     `json:"items,omitempty"`
}

const orderCols = `id, code, customer_id, merchant_id, courier_id, state, fulfillment, payment_method, payment_status,
	items_total_minor, delivery_fee_minor, discount_minor, total_minor, commission_minor, currency,
	address_line1, address_lat, address_lng, address_instructions, note, prep_time_min, ready_at_estimate,
	cancelled_by, cancellation_reason, created_at, accepted_at, ready_at, picked_up_at, delivered_at, cancelled_at`

func scanOrder(row pgx.Row) (*Order, error) {
	var o Order
	var lat, lng *float64
	if err := row.Scan(&o.ID, &o.Code, &o.CustomerID, &o.MerchantID, &o.CourierID, &o.State, &o.Fulfillment, &o.PaymentMethod, &o.PaymentStatus,
		&o.ItemsTotalMinor, &o.DeliveryFeeMinor, &o.DiscountMinor, &o.TotalMinor, &o.CommissionMinor, &o.Currency,
		&o.AddressLine1, &lat, &lng, &o.AddressInstructions, &o.Note, &o.PrepTimeMin, &o.ReadyAtEstimate,
		&o.CancelledBy, &o.CancellationReason, &o.CreatedAt, &o.AcceptedAt, &o.ReadyAt, &o.PickedUpAt, &o.DeliveredAt, &o.CancelledAt); err != nil {
		return nil, err
	}
	o.Currency = strings.TrimSpace(o.Currency)
	if lat != nil && lng != nil {
		o.Address = &geo.Point{Lat: *lat, Lng: *lng}
	}
	return &o, nil
}

// newCode — kod i shkurtër i lexueshëm (pa 0/O/1/I) për ekranin e merchant-it dhe kuponin.
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

// --- checkout ---------------------------------------------------------------------------------

type CheckoutInput struct {
	MerchantID    uuid.UUID           `json:"merchant_id"`
	Items         []catalog.Selection `json:"items"`
	PaymentMethod string              `json:"payment_method"` // cash | wallet
	Fulfillment   string              `json:"fulfillment"`    // courier | pickup (merchant_delivers vendoset nga merchant-i)
	AddressLine1  string              `json:"address_line1"`
	Address       *geo.Point          `json:"address"`
	Instructions  string              `json:"instructions"`
	Note          string              `json:"note"`
	CouponCode    string              `json:"coupon_code"`

	customerID uuid.UUID // vendoset nga handler-i; kuponat kanë kufij për përdorues
}

// Quote — përmbledhja e shportës pa krijuar porosi (ekrani i checkout-it).
type Quote struct {
	MerchantID       uuid.UUID            `json:"merchant_id"`
	Items            []catalog.PricedLine `json:"items"`
	ItemsTotalMinor  int64                `json:"items_total_minor"`
	DeliveryFeeMinor int64                `json:"delivery_fee_minor"`
	DiscountMinor    int64                `json:"discount_minor"`
	CouponCode       string               `json:"coupon_code,omitempty"`
	TotalMinor       int64                `json:"total_minor"`
	MinOrderMinor    int64                `json:"min_order_minor"`
	Currency         string               `json:"currency"`
	PrepTimeMin      int                  `json:"prep_time_min"`
	OpenNow          bool                 `json:"open_now"`
}

type merchantRow struct {
	name        string
	status      string
	accepting   bool
	fulfillment string
	minOrder    int64
	deliveryFee int64
	prepTime    int
	commission  int
	ownerID     uuid.UUID
}

func (s *Service) merchant(ctx context.Context, id uuid.UUID) (*merchantRow, error) {
	var m merchantRow
	err := s.pool.QueryRow(ctx, `SELECT name, status, accepting_orders, fulfillment_mode, min_order_minor, delivery_fee_minor, prep_time_min, commission_bp, owner_user_id
		FROM merchants WHERE id = $1`, id).Scan(&m.name, &m.status, &m.accepting, &m.fulfillment, &m.minOrder, &m.deliveryFee, &m.prepTime, &m.commission, &m.ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return &m, err
}

// openNow — orari i merchant-it (i njëjti rregull si te moduli merchants; kërkesë e vetme).
func (s *Service) openNow(ctx context.Context, id uuid.UUID) (bool, error) {
	var open bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM merchant_hours h WHERE h.merchant_id = $1 AND (
		    (h.closes > h.opens AND h.weekday = EXTRACT(DOW FROM now())::int AND now()::time >= h.opens AND now()::time < h.closes)
		    OR (h.closes <= h.opens AND ((h.weekday = EXTRACT(DOW FROM now())::int AND now()::time >= h.opens)
		        OR (h.weekday = ((EXTRACT(DOW FROM now())::int + 6) % 7) AND now()::time < h.closes)))))`, id).Scan(&open)
	return open, err
}

func (s *Service) price(ctx context.Context, in *CheckoutInput) (*Quote, *merchantRow, error) {
	if len(in.Items) == 0 || len(in.Items) > 50 {
		return nil, nil, httpx.ErrValidation.WithFields(map[string]string{"items": "invalid"})
	}
	m, err := s.merchant(ctx, in.MerchantID)
	if err != nil {
		return nil, nil, err
	}
	if m.status != "active" || !m.accepting {
		return nil, nil, ErrMerchantClosed
	}
	open, err := s.openNow(ctx, in.MerchantID)
	if err != nil {
		return nil, nil, err
	}
	q := &Quote{MerchantID: in.MerchantID, Items: []catalog.PricedLine{}, MinOrderMinor: m.minOrder, Currency: "EUR", PrepTimeMin: m.prepTime, OpenNow: open}
	for _, sel := range in.Items {
		line, err := s.catalog.Price(ctx, in.MerchantID, sel)
		if err != nil {
			return nil, nil, err
		}
		q.Items = append(q.Items, *line)
		q.ItemsTotalMinor += line.TotalMinor
		q.Currency = line.Currency
	}
	fulfillment := in.Fulfillment
	if fulfillment == "" {
		fulfillment = m.fulfillment
	}
	if fulfillment != "pickup" {
		q.DeliveryFeeMinor = m.deliveryFee
	}
	if code := promos.Normalize(in.CouponCode); code != "" {
		if s.promos == nil {
			return nil, nil, promos.ErrInvalid
		}
		applied, err := s.promos.Apply(ctx, code, in.customerID, promos.ScopeFood, q.ItemsTotalMinor)
		if err != nil {
			return nil, nil, err
		}
		q.DiscountMinor = applied.DiscountMinor
		q.CouponCode = applied.Code
	}
	q.TotalMinor = q.ItemsTotalMinor + q.DeliveryFeeMinor - q.DiscountMinor
	return q, m, nil
}

// Quote — çmimi i shportës (pa krijuar porosi). Përdoruesi duhet për kufijtë e kuponit.
func (s *Service) Quote(ctx context.Context, customerID uuid.UUID, in CheckoutInput) (*Quote, error) {
	in.customerID = customerID
	q, _, err := s.price(ctx, &in)
	return q, err
}

// Create — checkout: çmimi rillogaritet në server, wallet-i kontrollohet, porosia krijohet 'pending_merchant'.
func (s *Service) Create(ctx context.Context, a principal.Actor, idemKey string, in CheckoutInput) (*Order, error) {
	idemKey = strings.TrimSpace(idemKey)
	in.customerID = a.UserID
	fields := map[string]string{}
	if idemKey == "" || len(idemKey) > 100 {
		fields["idempotency_key"] = "required"
	}
	switch in.PaymentMethod {
	case "cash", "wallet":
	case "card":
		return nil, ErrPaymentMethod
	default:
		fields["payment_method"] = "invalid"
	}
	in.Note = strings.Join(strings.Fields(in.Note), " ")
	if utf8.RuneCountInString(in.Note) > 300 {
		fields["note"] = "too_long"
	}
	in.Instructions = strings.Join(strings.Fields(in.Instructions), " ")
	if utf8.RuneCountInString(in.Instructions) > 200 {
		fields["instructions"] = "too_long"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	// idempotencë
	if existing, err := scanOrder(s.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE customer_id = $1 AND idempotency_key = $2`, a.UserID, idemKey)); err == nil {
		return s.withItems(ctx, existing)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	q, m, err := s.price(ctx, &in)
	if err != nil {
		return nil, err
	}
	if !q.OpenNow {
		return nil, ErrMerchantClosed
	}
	if q.ItemsTotalMinor < m.minOrder {
		return nil, ErrMinOrder.WithFields(map[string]string{"min_order_minor": "not_met"})
	}
	fulfillment := in.Fulfillment
	if fulfillment == "" {
		fulfillment = m.fulfillment
	}
	if fulfillment != "pickup" {
		if in.Address == nil || !in.Address.Valid() || !geo.InKosovo(*in.Address) || strings.TrimSpace(in.AddressLine1) == "" {
			return nil, ErrAddressRequired
		}
	}
	if in.PaymentMethod == "wallet" {
		bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(a.UserID))
		if err != nil && !errors.Is(err, ledger.ErrAccountMissing) {
			return nil, err
		}
		if int64(bal.Minor) < q.TotalMinor {
			return nil, ErrInsufficient
		}
	}

	var out *Order
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var lat, lng *float64
		if in.Address != nil {
			lat, lng = &in.Address.Lat, &in.Address.Lng
		}
		ready := s.now().Add(time.Duration(m.prepTime) * time.Minute)
		var o *Order
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			o, err = scanOrder(tx.QueryRow(ctx, `
				INSERT INTO orders (code, customer_id, merchant_id, state, fulfillment, payment_method, items_total_minor, delivery_fee_minor,
				  total_minor, currency, address_line1, address_lat, address_lng, address_instructions, note, prep_time_min, ready_at_estimate, idempotency_key,
				  discount_minor, coupon_code)
				VALUES ($1,$2,$3,'pending_merchant',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING `+orderCols,
				newCode(), a.UserID, in.MerchantID, fulfillment, in.PaymentMethod, q.ItemsTotalMinor, q.DeliveryFeeMinor, q.TotalMinor, q.Currency,
				nullable(in.AddressLine1), lat, lng, nullable(in.Instructions), nullable(in.Note), m.prepTime, ready, idemKey,
				q.DiscountMinor, nullable(q.CouponCode)))
			if err == nil {
				break
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "code") {
				continue // kod i përplasur → provo sërish
			}
			return err
		}
		if err != nil {
			return err
		}
		if q.CouponCode != "" {
			if err := s.promos.Redeem(ctx, tx, q.CouponCode, a.UserID, "order:"+o.ID.String(), q.DiscountMinor); err != nil {
				return err
			}
		}
		for _, line := range q.Items {
			if _, err := tx.Exec(ctx, `INSERT INTO order_items (order_id, product_id, name, options, option_ids, unit_minor, quantity, total_minor)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, o.ID, line.ProductID, line.Name, line.Options, line.OptionIDs, line.UnitMinor, line.Quantity, line.TotalMinor); err != nil {
				return err
			}
		}
		out = o
		if err := orderEvent(ctx, tx, o.ID, nil, StatePendingMerchant, "customer", &a.UserID, map[string]any{"payment_method": in.PaymentMethod, "fulfillment": fulfillment}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "order", o.ID.String(), "OrderCreated", map[string]any{
			"order_id": o.ID, "code": o.Code, "customer_id": a.UserID, "merchant_id": in.MerchantID, "owner_id": m.ownerID, "total_minor": q.TotalMinor})
	})
	if err != nil {
		return nil, err
	}
	out.MerchantName = m.name
	return s.withItems(ctx, out)
}

func (s *Service) withItems(ctx context.Context, o *Order) (*Order, error) {
	if o == nil {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, product_id, name, options, option_ids, unit_minor, quantity, total_minor FROM order_items WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	o.Items = []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Name, &it.Options, &it.OptionIDs, &it.UnitMinor, &it.Quantity, &it.TotalMinor); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Emri dhe pozicioni i partnerit: harta e ndjekjes i vizaton pa thirrje tjetër.
	var mname string
	var mlat, mlng float64
	if err := s.pool.QueryRow(ctx, `SELECT name, lat, lng FROM merchants WHERE id = $1`, o.MerchantID).Scan(&mname, &mlat, &mlng); err == nil {
		if o.MerchantName == "" {
			o.MerchantName = mname
		}
		o.MerchantLocation = &geo.Point{Lat: mlat, Lng: mlng}
	}
	return o, nil
}

// --- klienti -----------------------------------------------------------------------------------

func (s *Service) Get(ctx context.Context, a principal.Actor, id uuid.UUID) (*Order, error) {
	o, err := scanOrder(s.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 AND (customer_id = $2 OR courier_id = $2
		OR EXISTS (SELECT 1 FROM merchant_staff st WHERE st.merchant_id = orders.merchant_id AND st.user_id = $2))`, id, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.withItems(ctx, o)
}

func (s *Service) History(ctx context.Context, a principal.Actor, before *time.Time, limit int) ([]Order, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if before == nil {
		t := s.now().Add(time.Hour)
		before = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT `+orderCols+` FROM orders WHERE customer_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`, a.UserID, *before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// CancelByCustomer — vetëm derisa merchant-i të fillojë përgatitjen; pa tarifë në V1.
func (s *Service) CancelByCustomer(ctx context.Context, a principal.Actor, id uuid.UUID, reason string) (*Order, error) {
	reason = clip(reason, 200)
	var out *Order
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 AND customer_id = $2 FOR UPDATE`, id, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CustomerCanCancel(o.State) {
			return ErrInvalidState
		}
		from := o.State
		o, err = scanOrder(tx.QueryRow(ctx, `UPDATE orders SET state = 'cancelled', cancelled_by = 'customer', cancellation_reason = $2,
			payment_status = 'none', cancelled_at = now(), updated_at = now() WHERE id = $1 RETURNING `+orderCols, id, nullable(reason)))
		if err != nil {
			return err
		}
		out = o
		if err := orderEvent(ctx, tx, id, &from, StateCancelled, "customer", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "order", id.String(), "OrderCancelled", map[string]any{"order_id": id, "by": "customer", "merchant_id": o.MerchantID, "customer_id": o.CustomerID})
	})
	if err != nil {
		return nil, err
	}
	return s.withItems(ctx, out)
}

// --- merchant-i ---------------------------------------------------------------------------------

func (s *Service) MerchantQueue(ctx context.Context, a principal.Actor, merchantID uuid.UUID, includeClosed bool, limit int) ([]Order, error) {
	if _, err := s.members.Membership(ctx, a.UserID, merchantID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT `+orderCols+` FROM orders
		WHERE merchant_id = $1 AND ($2 OR state IN ('pending_merchant','accepted','preparing','ready','courier_assigned','picked_up'))
		ORDER BY created_at DESC LIMIT $3`, merchantID, includeClosed, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		withItems, err := s.withItems(ctx, o)
		if err != nil {
			return nil, err
		}
		out = append(out, *withItems)
	}
	return out, rows.Err()
}

// MerchantTransition — accept (me kohë përgatitjeje), preparing, ready, delivered (pickup/merchant_delivers),
// reject / cancel me arsye. Kalimet ndjekin makinën e gjendjeve.
func (s *Service) MerchantTransition(ctx context.Context, a principal.Actor, id uuid.UUID, to string, prepTimeMin int, reason string) (*Order, error) {
	reason = clip(reason, 200)
	var out *Order
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1 FOR UPDATE`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := s.members.Membership(ctx, a.UserID, o.MerchantID); err != nil {
			return httpx.ErrNotFound
		}
		if !CanTransition(o.State, to) {
			return ErrInvalidState
		}
		if to == StateDelivered && o.Fulfillment == "courier" {
			return ErrInvalidState // dorëzimin e konfirmon korrieri
		}
		if (to == StateCancelled || to == StateRejected) && reason == "" {
			return httpx.ErrValidation.WithFields(map[string]string{"reason": "required"})
		}
		from := o.State
		set := ""
		args := []any{id}
		switch to {
		case StateAccepted:
			if prepTimeMin < 5 || prepTimeMin > 180 {
				prepTimeMin = o.PrepTimeMin
			}
			set = `state = 'accepted', accepted_at = now(), prep_time_min = $2, ready_at_estimate = now() + make_interval(mins => $2)`
			args = append(args, prepTimeMin)
		case StatePreparing:
			set = `state = 'preparing'`
		case StateReady:
			set = `state = 'ready', ready_at = now()`
		case StateDelivered:
			set = `state = 'delivered', delivered_at = now(), payment_status = 'pending'`
		case StateCancelled, StateRejected:
			set = `state = $2, cancelled_by = 'merchant', cancellation_reason = $3, payment_status = 'none', cancelled_at = now()`
			args = append(args, to, reason)
		default:
			return ErrInvalidState
		}
		o, err = scanOrder(tx.QueryRow(ctx, `UPDATE orders SET `+set+`, updated_at = now() WHERE id = $1 RETURNING `+orderCols, args...))
		if err != nil {
			return err
		}
		out = o
		if err := orderEvent(ctx, tx, id, &from, to, "merchant", &a.UserID, map[string]any{"reason": reason, "prep_time_min": o.PrepTimeMin}); err != nil {
			return err
		}
		evt := map[string]string{StateAccepted: "OrderAccepted", StatePreparing: "OrderPreparing", StateReady: "OrderReady",
			StateDelivered: "OrderDelivered", StateCancelled: "OrderCancelled", StateRejected: "OrderRejected"}[to]
		return events.Emit(ctx, tx, "order", id.String(), evt, map[string]any{
			"order_id": id, "code": o.Code, "customer_id": o.CustomerID, "merchant_id": o.MerchantID, "courier_id": o.CourierID,
			"by": "merchant", "reason": reason, "ready_at_estimate": o.ReadyAtEstimate, "fulfillment": o.Fulfillment})
	})
	if err != nil {
		return nil, err
	}
	if out.State == StateDelivered {
		_ = s.settle(ctx, out)
		out, _ = scanOrder(s.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1`, id))
	}
	return s.withItems(ctx, out)
}

// --- shlyerja ------------------------------------------------------------------------------------

// settle — dorëzim: wallet → debit klienti (total), kredit merchant-i (artikujt − komision), kredit komisioni,
// kredit tarifa e dërgesës te KREJT; cash → merchant-i/korrieri mban paratë, komisioni + tarifa i debitohen
// merchant-it (borxh ndaj platformës). Idempotente me çelës për porosi.
func (s *Service) settle(ctx context.Context, o *Order) error {
	if o.State != StateDelivered || o.PaymentStatus != "pending" {
		return nil
	}
	var commissionBP int
	var ownerID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT commission_bp, owner_user_id FROM merchants WHERE id = $1`, o.MerchantID).Scan(&commissionBP, &ownerID); err != nil {
		return err
	}
	commission := (o.ItemsTotalMinor*int64(commissionBP) + 5000) / 10000
	merchantWallet := "merchant:" + o.MerchantID.String() + ":wallet"
	mid := o.MerchantID
	if err := s.ledger.EnsureAccount(ctx, merchantWallet, "merchant", &mid, "liability", o.Currency); err != nil {
		return err
	}
	idem := "order:" + o.ID.String() + ":settle"
	ref := "order:" + o.ID.String()
	var tx ledger.Transaction
	status := "cash"
	if o.PaymentMethod == "wallet" {
		bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(o.CustomerID))
		if err != nil {
			return err
		}
		if int64(bal.Minor) < o.TotalMinor {
			_, err := s.pool.Exec(ctx, `UPDATE orders SET payment_status = 'failed', updated_at = now() WHERE id = $1 AND payment_status = 'pending'`, o.ID)
			return err
		}
		status = "paid"
		postings := []ledger.Posting{
			{AccountCode: ledger.UserWalletCode(o.CustomerID), Debit: money.Minor(o.TotalMinor)},
			{AccountCode: merchantWallet, Credit: money.Minor(o.ItemsTotalMinor - commission)},
		}
		if commission > 0 {
			postings = append(postings, ledger.Posting{AccountCode: "krejt:commission", Credit: money.Minor(commission)})
		}
		if o.DeliveryFeeMinor > 0 {
			postings = append(postings, ledger.Posting{AccountCode: "krejt:delivery_fees", Credit: money.Minor(o.DeliveryFeeMinor)})
		}
		if o.DiscountMinor > 0 {
			postings = append(postings, ledger.Posting{AccountCode: promos.MarketingAccount, Debit: money.Minor(o.DiscountMinor)})
		}
		tx = ledger.Transaction{Kind: "order_payment", Reference: ref, IdempotencyKey: idem, Currency: o.Currency, Postings: postings}
	} else {
		owed := commission + o.DeliveryFeeMinor
		if owed > 0 || o.DiscountMinor > 0 {
			postings := []ledger.Posting{}
			if owed > 0 {
				postings = append(postings, ledger.Posting{AccountCode: merchantWallet, Debit: money.Minor(owed)})
			}
			if commission > 0 {
				postings = append(postings, ledger.Posting{AccountCode: "krejt:commission", Credit: money.Minor(commission)})
			}
			if o.DeliveryFeeMinor > 0 {
				postings = append(postings, ledger.Posting{AccountCode: "krejt:delivery_fees", Credit: money.Minor(o.DeliveryFeeMinor)})
			}
			// Klienti pagoi cash më pak për shkak të kuponit; diferencën partnerit ia mbulon platforma.
			if o.DiscountMinor > 0 {
				postings = append(postings, ledger.Posting{AccountCode: merchantWallet, Credit: money.Minor(o.DiscountMinor)},
					ledger.Posting{AccountCode: promos.MarketingAccount, Debit: money.Minor(o.DiscountMinor)})
			}
			tx = ledger.Transaction{Kind: "order_cash_fees", Reference: ref, IdempotencyKey: idem, Currency: o.Currency, Postings: postings}
		}
	}
	if len(tx.Postings) > 0 {
		if _, err := s.ledger.Post(ctx, tx); err != nil {
			return err
		}
	}
	return pgx.BeginFunc(ctx, s.pool, func(dbtx pgx.Tx) error {
		if _, err := dbtx.Exec(ctx, `UPDATE orders SET payment_status = $2, commission_minor = $3, updated_at = now() WHERE id = $1 AND payment_status = 'pending'`, o.ID, status, commission); err != nil {
			return err
		}
		return events.Emit(ctx, dbtx, "order", o.ID.String(), "OrderPaymentSettled", map[string]any{
			"order_id": o.ID, "customer_id": o.CustomerID, "merchant_id": o.MerchantID, "status": status, "method": o.PaymentMethod, "total_minor": o.TotalMinor})
	})
}

// SettlePending — riprovim nga worker-i (si te udhëtimet).
func (s *Service) SettlePending(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+orderCols+` FROM orders WHERE payment_status = 'pending' AND state = 'delivered' AND updated_at < now() - interval '10 seconds' ORDER BY updated_at LIMIT 50`)
	if err != nil {
		return 0, err
	}
	var list []*Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, o)
	}
	rows.Close()
	n := 0
	for _, o := range list {
		if err := s.settle(ctx, o); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func orderEvent(ctx context.Context, tx events.Execer, orderID uuid.UUID, from *string, to, actorType string, actorID *uuid.UUID, meta map[string]any) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO order_events (order_id, from_state, to_state, actor_type, actor_id, metadata) VALUES ($1,$2,$3,$4,$5,$6)`,
		orderID, from, to, actorType, actorID, metaJSON)
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
