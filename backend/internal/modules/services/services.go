package services

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
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrInsufficient   = &httpx.APIError{Code: "INSUFFICIENT_FUNDS", MessageKey: "errors.wallet.insufficient_funds", HTTPStatus: http.StatusPaymentRequired}
	ErrInvalidState   = &httpx.APIError{Code: "SERVICE_INVALID_STATE", MessageKey: "errors.services.invalid_state", HTTPStatus: http.StatusConflict}
	ErrNotApproved    = &httpx.APIError{Code: "PROVIDER_NOT_APPROVED", MessageKey: "errors.services.provider_not_approved", HTTPStatus: http.StatusForbidden}
	ErrWrongCategory  = &httpx.APIError{Code: "PROVIDER_WRONG_CATEGORY", MessageKey: "errors.services.wrong_category", HTTPStatus: http.StatusForbidden}
	ErrOfferGone      = &httpx.APIError{Code: "OFFER_GONE", MessageKey: "errors.orders.offer_gone", HTTPStatus: http.StatusConflict}
	ErrProviderBusy   = &httpx.APIError{Code: "PROVIDER_BUSY", MessageKey: "errors.services.provider_busy", HTTPStatus: http.StatusConflict}
	ErrAlreadyOffered = &httpx.APIError{Code: "ALREADY_OFFERED", MessageKey: "errors.services.already_offered", HTTPStatus: http.StatusConflict}
)

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
	now    func() time.Time
}

func New(pool *pgxpool.Pool, led *ledger.Service) *Service {
	return &Service{pool: pool, ledger: led, now: time.Now}
}

// --- tipat ---------------------------------------------------------------------------------

type Category struct {
	ID      string `json:"id"`
	NameKey string `json:"name_key"`
	Sort    int    `json:"sort"`
}

// ProviderCard — çfarë sheh klienti për mjeshtrin: emri, qyteti, vlerësimi. Pa telefon privat.
type ProviderCard struct {
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	BusinessName *string   `json:"business_name"`
	City         string    `json:"city"`
	Rating       *float64  `json:"rating"`
	RatingCount  int       `json:"rating_count"`
	JobsDone     int       `json:"jobs_done"`
}

type Provider struct {
	UserID          uuid.UUID `json:"user_id"`
	Status          string    `json:"status"`
	Categories      []string  `json:"categories"`
	BusinessName    *string   `json:"business_name"`
	Bio             *string   `json:"bio"`
	City            string    `json:"city"`
	PhonePublic     *string   `json:"phone_public"`
	Rating          *float64  `json:"rating"`
	RatingCount     int       `json:"rating_count"`
	JobsDone        int       `json:"jobs_done"`
	SuspendedReason *string   `json:"suspended_reason"`
	CreatedAt       time.Time `json:"created_at"`
}

type Request struct {
	ID                  uuid.UUID     `json:"id"`
	Code                string        `json:"code"`
	CustomerID          uuid.UUID     `json:"customer_id"`
	CategoryID          string        `json:"category_id"`
	ProviderID          *uuid.UUID    `json:"provider_id"`
	State               string        `json:"state"`
	Title               string        `json:"title"`
	Description         string        `json:"description"`
	AddressLine1        string        `json:"address_line1"`
	Address             geo.Point     `json:"address"`
	AddressInstructions *string       `json:"address_instructions"`
	PreferredAt         *time.Time    `json:"preferred_at"`
	PhotoKeys           []string      `json:"photo_keys"`
	PaymentMethod       string        `json:"payment_method"`
	PaymentStatus       string        `json:"payment_status"`
	PriceMinor          *int64        `json:"price_minor"`
	CommissionMinor     int64         `json:"-"`
	Currency            string        `json:"currency"`
	CancelledBy         *string       `json:"cancelled_by"`
	CancellationReason  *string       `json:"cancellation_reason"`
	CreatedAt           time.Time     `json:"created_at"`
	BookedAt            *time.Time    `json:"booked_at"`
	StartedAt           *time.Time    `json:"started_at"`
	CompletedAt         *time.Time    `json:"completed_at"`
	CancelledAt         *time.Time    `json:"cancelled_at"`
	Provider            *ProviderCard `json:"provider,omitempty"`
	Offers              []Offer       `json:"offers,omitempty"`
}

type Offer struct {
	ID         uuid.UUID     `json:"id"`
	RequestID  uuid.UUID     `json:"request_id"`
	ProviderID uuid.UUID     `json:"provider_id"`
	PriceMinor int64         `json:"price_minor"`
	Currency   string        `json:"currency"`
	Note       *string       `json:"note"`
	CanStartAt *time.Time    `json:"can_start_at"`
	State      string        `json:"state"`
	CreatedAt  time.Time     `json:"created_at"`
	Provider   *ProviderCard `json:"provider,omitempty"`
}

const requestCols = `id, code, customer_id, category_id, provider_id, state, title, description,
	address_line1, address_lat, address_lng, address_instructions, preferred_at, photo_keys,
	payment_method, payment_status, price_minor, commission_minor, currency, cancelled_by, cancellation_reason,
	created_at, booked_at, started_at, completed_at, cancelled_at`

func scanRequest(row pgx.Row) (*Request, error) {
	var r Request
	if err := row.Scan(&r.ID, &r.Code, &r.CustomerID, &r.CategoryID, &r.ProviderID, &r.State, &r.Title, &r.Description,
		&r.AddressLine1, &r.Address.Lat, &r.Address.Lng, &r.AddressInstructions, &r.PreferredAt, &r.PhotoKeys,
		&r.PaymentMethod, &r.PaymentStatus, &r.PriceMinor, &r.CommissionMinor, &r.Currency, &r.CancelledBy, &r.CancellationReason,
		&r.CreatedAt, &r.BookedAt, &r.StartedAt, &r.CompletedAt, &r.CancelledAt); err != nil {
		return nil, err
	}
	return &r, nil
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

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
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

// --- katalogu -----------------------------------------------------------------------------

func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name_key, sort FROM service_categories WHERE active ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.NameKey, &c.Sort); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) providerCard(ctx context.Context, id uuid.UUID) *ProviderCard {
	var c ProviderCard
	var name *string
	if err := s.pool.QueryRow(ctx, `SELECT p.user_id, u.full_name, p.business_name, p.city, p.rating, p.rating_count, p.jobs_done
		FROM service_providers p JOIN users u ON u.id = p.user_id WHERE p.user_id = $1`, id).
		Scan(&c.UserID, &name, &c.BusinessName, &c.City, &c.Rating, &c.RatingCount, &c.JobsDone); err != nil {
		return nil
	}
	if name != nil {
		c.Name = *name
	}
	return &c
}

// --- mjeshtri -----------------------------------------------------------------------------

type ApplyInput struct {
	Categories   []string `json:"categories"`
	BusinessName string   `json:"business_name"`
	Bio          string   `json:"bio"`
	City         string   `json:"city"`
	PhonePublic  string   `json:"phone_public"`
}

// Apply — aplikimi si mjeshtër; miratimin e jep Operacionet, ndaj gjendja nis 'pending'.
func (s *Service) Apply(ctx context.Context, a principal.Actor, in ApplyInput) (*Provider, error) {
	fields := map[string]string{}
	in.City = clip(in.City, 60)
	if in.City == "" {
		fields["city"] = "required"
	}
	valid, err := s.validCategories(ctx)
	if err != nil {
		return nil, err
	}
	cats := []string{}
	for _, c := range in.Categories {
		if valid[c] && !contains(cats, c) {
			cats = append(cats, c)
		}
	}
	if len(cats) == 0 {
		fields["categories"] = "required"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	var p Provider
	err = s.pool.QueryRow(ctx, `
		INSERT INTO service_providers (user_id, categories, business_name, bio, city, phone_public)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (user_id) DO UPDATE SET categories = EXCLUDED.categories, business_name = EXCLUDED.business_name,
		  bio = EXCLUDED.bio, city = EXCLUDED.city, phone_public = EXCLUDED.phone_public, updated_at = now()
		RETURNING user_id, status, categories, business_name, bio, city, phone_public, rating, rating_count, jobs_done, suspended_reason, created_at`,
		a.UserID, cats, nullable(clip(in.BusinessName, 80)), nullable(clip(in.Bio, 400)), in.City, nullable(clip(in.PhonePublic, 20))).
		Scan(&p.UserID, &p.Status, &p.Categories, &p.BusinessName, &p.Bio, &p.City, &p.PhonePublic, &p.Rating, &p.RatingCount, &p.JobsDone, &p.SuspendedReason, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := events.Emit(ctx, s.pool, "service_provider", a.UserID.String(), "ServiceProviderApplied",
		map[string]any{"provider_id": a.UserID, "categories": cats}); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) ProviderProfile(ctx context.Context, id uuid.UUID) (*Provider, error) {
	var p Provider
	err := s.pool.QueryRow(ctx, `SELECT user_id, status, categories, business_name, bio, city, phone_public, rating, rating_count, jobs_done, suspended_reason, created_at
		FROM service_providers WHERE user_id = $1`, id).
		Scan(&p.UserID, &p.Status, &p.Categories, &p.BusinessName, &p.Bio, &p.City, &p.PhonePublic, &p.Rating, &p.RatingCount, &p.JobsDone, &p.SuspendedReason, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return &p, err
}

// SetProviderStatus — Operacionet miratojnë ose pezullojnë një mjeshtër.
func (s *Service) SetProviderStatus(ctx context.Context, a principal.Actor, id uuid.UUID, status, reason string) (*Provider, error) {
	if status != "approved" && status != "suspended" && status != "pending" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"status": "invalid"})
	}
	var p Provider
	err := s.pool.QueryRow(ctx, `UPDATE service_providers SET status = $2, suspended_reason = $3, updated_at = now()
		WHERE user_id = $1 RETURNING user_id, status, categories, business_name, bio, city, phone_public, rating, rating_count, jobs_done, suspended_reason, created_at`,
		id, status, nullable(clip(reason, 200))).
		Scan(&p.UserID, &p.Status, &p.Categories, &p.BusinessName, &p.Bio, &p.City, &p.PhonePublic, &p.Rating, &p.RatingCount, &p.JobsDone, &p.SuspendedReason, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, events.Emit(ctx, s.pool, "service_provider", id.String(), "ServiceProviderStatusChanged",
		map[string]any{"provider_id": id, "status": status, "reason": reason, "by": a.UserID})
}

func (s *Service) validCategories(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM service_categories WHERE active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// --- kërkesa e klientit ---------------------------------------------------------------------

type CreateInput struct {
	CategoryID          string     `json:"category_id"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	AddressLine1        string     `json:"address_line1"`
	Address             *geo.Point `json:"address"`
	AddressInstructions string     `json:"address_instructions"`
	PreferredAt         *time.Time `json:"preferred_at"`
	PhotoKeys           []string   `json:"photo_keys"`
	PaymentMethod       string     `json:"payment_method"`
}

func (s *Service) Create(ctx context.Context, a principal.Actor, idemKey string, in CreateInput) (*Request, error) {
	idemKey = strings.TrimSpace(idemKey)
	fields := map[string]string{}
	if idemKey == "" || len(idemKey) > 100 {
		fields["idempotency_key"] = "required"
	}
	in.Title = clip(in.Title, 80)
	in.Description = clip(in.Description, 1000)
	in.AddressLine1 = clip(in.AddressLine1, 200)
	if in.Title == "" {
		fields["title"] = "required"
	}
	if utf8.RuneCountInString(in.Description) < 10 {
		fields["description"] = "too_short"
	}
	if in.AddressLine1 == "" {
		fields["address_line1"] = "required"
	}
	if in.Address == nil || !in.Address.Valid() || !geo.InKosovo(*in.Address) {
		fields["address"] = "invalid"
	}
	if in.PaymentMethod != "cash" && in.PaymentMethod != "wallet" {
		fields["payment_method"] = "invalid"
	}
	valid, err := s.validCategories(ctx)
	if err != nil {
		return nil, err
	}
	if !valid[in.CategoryID] {
		fields["category_id"] = "invalid"
	}
	if len(in.PhotoKeys) > 5 {
		fields["photo_keys"] = "too_many"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	if existing, err := scanRequest(s.pool.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE customer_id = $1 AND idempotency_key = $2`, a.UserID, idemKey)); err == nil {
		return existing, nil
	}
	if in.PhotoKeys == nil {
		in.PhotoKeys = []string{}
	}

	var out *Request
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRequest(tx.QueryRow(ctx, `
			INSERT INTO service_requests (code, customer_id, category_id, state, title, description,
			  address_line1, address_lat, address_lng, address_instructions, preferred_at, photo_keys, payment_method, idempotency_key)
			VALUES ($1,$2,$3,'open',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING `+requestCols,
			newCode(), a.UserID, in.CategoryID, in.Title, in.Description,
			in.AddressLine1, in.Address.Lat, in.Address.Lng, nullable(clip(in.AddressInstructions, 200)),
			in.PreferredAt, in.PhotoKeys, in.PaymentMethod, idemKey))
		if err != nil {
			return err
		}
		out = r
		if err := requestEvent(ctx, tx, r.ID, nil, StateOpen, "customer", &a.UserID, nil); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "service_request", r.ID.String(), "ServiceRequested", map[string]any{
			"request_id": r.ID, "code": r.Code, "customer_id": a.UserID, "category_id": in.CategoryID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, a principal.Actor, id uuid.UUID) (*Request, error) {
	r, err := scanRequest(s.pool.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE id = $1 AND (customer_id = $2 OR provider_id = $2)`, id, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if r.ProviderID != nil {
		r.Provider = s.providerCard(ctx, *r.ProviderID)
	}
	// Ofertat i sheh vetëm klienti, dhe vetëm derisa të zgjedhë njërën.
	if r.CustomerID == a.UserID && r.State == StateOpen {
		offers, err := s.offers(ctx, id)
		if err != nil {
			return nil, err
		}
		r.Offers = offers
	}
	return r, nil
}

func (s *Service) offers(ctx context.Context, requestID uuid.UUID) ([]Offer, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, request_id, provider_id, price_minor, currency, note, can_start_at, state, created_at
		FROM service_offers WHERE request_id = $1 AND state = 'offered' ORDER BY price_minor`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.RequestID, &o.ProviderID, &o.PriceMinor, &o.Currency, &o.Note, &o.CanStartAt, &o.State, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.Provider = s.providerCard(ctx, o.ProviderID)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Service) History(ctx context.Context, a principal.Actor, limit int) ([]Request, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT `+requestCols+` FROM service_requests WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2`, a.UserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		if r.ProviderID != nil {
			r.Provider = s.providerCard(ctx, *r.ProviderID)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// AcceptOffer — klienti zgjedh një ofertë; kërkesa bëhet 'booked' dhe ofertat e tjera tërhiqen.
func (s *Service) AcceptOffer(ctx context.Context, a principal.Actor, requestID, offerID uuid.UUID) (*Request, error) {
	var out *Request
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE id = $1 AND customer_id = $2 FOR UPDATE`, requestID, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if r.State != StateOpen {
			return ErrInvalidState
		}
		var providerID uuid.UUID
		var price int64
		var currency, offerState string
		err = tx.QueryRow(ctx, `SELECT provider_id, price_minor, currency, state FROM service_offers WHERE id = $1 AND request_id = $2 FOR UPDATE`, offerID, requestID).
			Scan(&providerID, &price, &currency, &offerState)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if offerState != "offered" {
			return ErrOfferGone
		}
		if r.PaymentMethod == "wallet" {
			bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(a.UserID))
			if err != nil && !errors.Is(err, ledger.ErrAccountMissing) {
				return err
			}
			if int64(bal.Minor) < price {
				return ErrInsufficient
			}
		}
		var commissionBP int
		if err := tx.QueryRow(ctx, `SELECT commission_bp FROM service_categories WHERE id = $1`, r.CategoryID).Scan(&commissionBP); err != nil {
			return err
		}
		r, err = scanRequest(tx.QueryRow(ctx, `UPDATE service_requests SET state = 'booked', provider_id = $2, accepted_offer_id = $3,
			price_minor = $4, commission_minor = $5, currency = $6, payment_status = 'none', booked_at = now(), updated_at = now()
			WHERE id = $1 RETURNING `+requestCols, requestID, providerID, offerID, price, Commission(price, commissionBP), currency))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrProviderBusy
			}
			return err
		}
		out = r
		if _, err := tx.Exec(ctx, `UPDATE service_offers SET state = 'accepted', responded_at = now() WHERE id = $1`, offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE service_offers SET state = 'withdrawn', responded_at = now() WHERE request_id = $1 AND id <> $2 AND state = 'offered'`, requestID, offerID); err != nil {
			return err
		}
		from := StateOpen
		if err := requestEvent(ctx, tx, requestID, &from, StateBooked, "customer", &a.UserID, map[string]any{"offer_id": offerID}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "service_request", requestID.String(), "ServiceBooked", map[string]any{
			"request_id": requestID, "code": r.Code, "customer_id": a.UserID, "provider_id": providerID, "price_minor": price})
	})
	if err != nil {
		return nil, err
	}
	if out.ProviderID != nil {
		out.Provider = s.providerCard(ctx, *out.ProviderID)
	}
	return out, nil
}

func (s *Service) Cancel(ctx context.Context, a principal.Actor, id uuid.UUID, reason string) (*Request, error) {
	var out *Request
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE id = $1 AND customer_id = $2 FOR UPDATE`, id, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CustomerCanCancel(r.State) {
			return ErrInvalidState
		}
		from := r.State
		r, err = scanRequest(tx.QueryRow(ctx, `UPDATE service_requests SET state = 'cancelled', cancelled_by = 'customer', cancellation_reason = $2,
			cancelled_at = now(), updated_at = now() WHERE id = $1 RETURNING `+requestCols, id, nullable(clip(reason, 200))))
		if err != nil {
			return err
		}
		out = r
		if _, err := tx.Exec(ctx, `UPDATE service_offers SET state = 'withdrawn', responded_at = now() WHERE request_id = $1 AND state = 'offered'`, id); err != nil {
			return err
		}
		if err := requestEvent(ctx, tx, id, &from, StateCancelled, "customer", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "service_request", id.String(), "ServiceCancelled", map[string]any{
			"request_id": id, "code": r.Code, "customer_id": a.UserID, "provider_id": r.ProviderID, "by": "customer"})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func requestEvent(ctx context.Context, tx events.Execer, id uuid.UUID, from *string, to, actorType string, actorID *uuid.UUID, meta map[string]any) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO service_events (request_id, from_state, to_state, actor_type, actor_id, metadata) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, from, to, actorType, actorID, metaJSON)
	return err
}

// --- shlyerja ------------------------------------------------------------------------------

// settle — si te udhëtimet: wallet → klienti paguan, mjeshtri merr çmimin pa komisionin;
// cash → mjeshtri e mban paranë dhe i detyrohet platformës komisionin.
func (s *Service) settle(ctx context.Context, r *Request) error {
	if r.State != StateCompleted || r.PaymentStatus != "pending" || r.ProviderID == nil || r.PriceMinor == nil {
		return nil
	}
	price := money.Minor(*r.PriceMinor)
	commission := money.Minor(r.CommissionMinor)
	// I njëjti kod llogarie si te shoferët dhe korrierët: pagesat e mjeshtrave kalojnë nga e njëjta
	// makineri payout-esh, ndryshe fitimi do të mbetej në një llogari që askush nuk e paguan.
	wallet := "driver:" + r.ProviderID.String() + ":wallet"
	pid := *r.ProviderID
	if err := s.ledger.EnsureAccount(ctx, wallet, "driver", &pid, "liability", r.Currency); err != nil {
		return err
	}
	ref := "service:" + r.ID.String()
	idem := ref + ":fee"
	var tx ledger.Transaction
	status := "cash"
	if r.PaymentMethod == "wallet" {
		bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(r.CustomerID))
		if err != nil {
			return err
		}
		if bal.Minor < price {
			return s.setPaymentStatus(ctx, r, "failed")
		}
		status = "paid"
		postings := []ledger.Posting{
			{AccountCode: ledger.UserWalletCode(r.CustomerID), Debit: price},
			{AccountCode: wallet, Credit: price - commission},
		}
		if commission > 0 {
			postings = append(postings, ledger.Posting{AccountCode: "krejt:commission", Credit: commission})
		}
		tx = ledger.Transaction{Kind: "service_fee", Reference: ref, IdempotencyKey: idem, Currency: r.Currency, Postings: postings}
	} else if commission > 0 {
		tx = ledger.Transaction{Kind: "service_cash_commission", Reference: ref, IdempotencyKey: idem, Currency: r.Currency,
			Postings: []ledger.Posting{{AccountCode: wallet, Debit: commission}, {AccountCode: "krejt:commission", Credit: commission}}}
	}
	if len(tx.Postings) > 0 {
		if _, err := s.ledger.Post(ctx, tx); err != nil {
			return err
		}
	}
	return s.setPaymentStatus(ctx, r, status)
}

func (s *Service) setPaymentStatus(ctx context.Context, r *Request, status string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE service_requests SET payment_status = $2, updated_at = now() WHERE id = $1 AND payment_status = 'pending'`, r.ID, status); err != nil {
			return err
		}
		ev := "ServicePaymentSettled"
		if status == "failed" {
			ev = "ServicePaymentFailed"
		}
		return events.Emit(ctx, tx, "service_request", r.ID.String(), ev, map[string]any{
			"request_id": r.ID, "customer_id": r.CustomerID, "provider_id": r.ProviderID, "status": status})
	})
}

// SettlePending — riprovon shlyerjet e mbetura (worker).
func (s *Service) SettlePending(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+requestCols+` FROM service_requests
		WHERE payment_status = 'pending' AND state = 'completed' AND updated_at < now() - interval '10 seconds' ORDER BY updated_at LIMIT 50`)
	if err != nil {
		return 0, err
	}
	var list []*Request
	for rows.Next() {
		r, err := scanRequest(rows)
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
