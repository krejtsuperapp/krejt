// Package promos — kupona zbritjeje (§35): kod, përqindje ose shumë fikse, minimum porosie, afat,
// numër përdorimesh (gjithsej dhe për përdorues), fushë (ushqim | pako | të gjitha). Zbritjen e llogarit
// vetëm serveri; klienti dërgon kodin dhe sheh shumën e zbritur. Kostoja e zbritjes i bie platformës
// (llogaria `krejt:marketing`), jo partnerit apo korrierit.
package promos

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/httpx"
)

var (
	ErrInvalid       = &httpx.APIError{Code: "COUPON_INVALID", MessageKey: "errors.promos.invalid", HTTPStatus: http.StatusUnprocessableEntity}
	ErrExpired       = &httpx.APIError{Code: "COUPON_EXPIRED", MessageKey: "errors.promos.expired", HTTPStatus: http.StatusUnprocessableEntity}
	ErrMinOrder      = &httpx.APIError{Code: "COUPON_MIN_ORDER", MessageKey: "errors.promos.min_order", HTTPStatus: http.StatusUnprocessableEntity}
	ErrUsedUp        = &httpx.APIError{Code: "COUPON_USED_UP", MessageKey: "errors.promos.used_up", HTTPStatus: http.StatusUnprocessableEntity}
	ErrNotApplicable = &httpx.APIError{Code: "COUPON_NOT_APPLICABLE", MessageKey: "errors.promos.not_applicable", HTTPStatus: http.StatusUnprocessableEntity}
)

// MarketingAccount — llogaria e ledger-it që mban koston e zbritjeve.
const MarketingAccount = "krejt:marketing"

const (
	ScopeAll     = "all"
	ScopeFood    = "food"
	ScopeParcels = "parcels"
)

type Coupon struct {
	Code           string     `json:"code"`
	Kind           string     `json:"kind"` // percent | fixed
	PercentBP      int        `json:"percent_bp"`
	AmountMinor    int64      `json:"amount_minor"`
	MinOrderMinor  int64      `json:"min_order_minor"`
	Scope          string     `json:"scope"` // all | food | parcels
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	MaxUses        *int       `json:"max_uses"`
	MaxUsesPerUser *int       `json:"max_uses_per_user"`
	UsesCount      int        `json:"uses_count"`
	Active         bool       `json:"active"`
	Note           *string    `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Applied — rezultati i një kuponi mbi një shumë: sa zbritet dhe cili kupon.
type Applied struct {
	Code          string `json:"code"`
	Kind          string `json:"kind"`
	DiscountMinor int64  `json:"discount_minor"`
}

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

const couponCols = `code, kind, percent_bp, amount_minor, min_order_minor, scope, starts_at, ends_at, max_uses, max_uses_per_user, uses_count, active, note, created_at, updated_at`

func scanCoupon(row pgx.Row) (*Coupon, error) {
	var c Coupon
	if err := row.Scan(&c.Code, &c.Kind, &c.PercentBP, &c.AmountMinor, &c.MinOrderMinor, &c.Scope, &c.StartsAt, &c.EndsAt,
		&c.MaxUses, &c.MaxUsesPerUser, &c.UsesCount, &c.Active, &c.Note, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// Normalize — kodi shkruhet kudo njësoj: shkronja të mëdha, pa hapësira, pa shenja.
func Normalize(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(code)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Discount — zbritja e një kuponi mbi një bazë; kurrë mbi bazën, kurrë negative.
func Discount(c Coupon, baseMinor int64) int64 {
	if baseMinor <= 0 {
		return 0
	}
	var d int64
	switch c.Kind {
	case "percent":
		d = (baseMinor*int64(c.PercentBP) + 5000) / 10000
	case "fixed":
		d = c.AmountMinor
	}
	if d > baseMinor {
		d = baseMinor
	}
	if d < 0 {
		d = 0
	}
	return d
}

// check — rregullat që nuk varen nga baza e shumës.
func (s *Service) check(c *Coupon, scope string, userUses int) error {
	now := s.now()
	if !c.Active {
		return ErrInvalid
	}
	if c.Scope != ScopeAll && c.Scope != scope {
		return ErrNotApplicable
	}
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return ErrExpired
	}
	if c.EndsAt != nil && now.After(*c.EndsAt) {
		return ErrExpired
	}
	if c.MaxUses != nil && c.UsesCount >= *c.MaxUses {
		return ErrUsedUp
	}
	if c.MaxUsesPerUser != nil && userUses >= *c.MaxUsesPerUser {
		return ErrUsedUp
	}
	return nil
}

// Apply — valido kuponin për këtë përdorues/fushë/shumë dhe kthe zbritjen (pa e konsumuar).
func (s *Service) Apply(ctx context.Context, code string, userID uuid.UUID, scope string, baseMinor int64) (*Applied, error) {
	code = Normalize(code)
	if code == "" || len(code) > 32 {
		return nil, ErrInvalid
	}
	c, err := scanCoupon(s.pool.QueryRow(ctx, `SELECT `+couponCols+` FROM coupons WHERE code = $1`, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	var userUses int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM coupon_redemptions WHERE coupon_code = $1 AND user_id = $2`, code, userID).Scan(&userUses); err != nil {
		return nil, err
	}
	if err := s.check(c, scope, userUses); err != nil {
		return nil, err
	}
	if baseMinor < c.MinOrderMinor {
		return nil, ErrMinOrder
	}
	return &Applied{Code: c.Code, Kind: c.Kind, DiscountMinor: Discount(*c, baseMinor)}, nil
}

// Redeem — konsumo kuponin brenda transaksionit të porosisë/pakos (rreshti kyçet, numëratori rritet).
func (s *Service) Redeem(ctx context.Context, tx pgx.Tx, code string, userID uuid.UUID, reference string, discountMinor int64) error {
	code = Normalize(code)
	c, err := scanCoupon(tx.QueryRow(ctx, `SELECT `+couponCols+` FROM coupons WHERE code = $1 FOR UPDATE`, code))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalid
	}
	if err != nil {
		return err
	}
	var userUses int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM coupon_redemptions WHERE coupon_code = $1 AND user_id = $2`, code, userID).Scan(&userUses); err != nil {
		return err
	}
	if err := s.check(c, ScopeAll, userUses); err != nil && !errors.Is(err, ErrNotApplicable) {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO coupon_redemptions (coupon_code, user_id, reference, discount_minor) VALUES ($1,$2,$3,$4)`,
		code, userID, reference, discountMinor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE coupons SET uses_count = uses_count + 1, updated_at = now() WHERE code = $1`, code)
	return err
}

// --- administrimi (OPERATIONS) -------------------------------------------------------------------

type UpsertInput struct {
	Code           string     `json:"code"`
	Kind           string     `json:"kind"`
	PercentBP      int        `json:"percent_bp"`
	AmountMinor    int64      `json:"amount_minor"`
	MinOrderMinor  int64      `json:"min_order_minor"`
	Scope          string     `json:"scope"`
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	MaxUses        *int       `json:"max_uses"`
	MaxUsesPerUser *int       `json:"max_uses_per_user"`
	Active         *bool      `json:"active"`
	Note           string     `json:"note"`
}

func (in *UpsertInput) validate() error {
	fields := map[string]string{}
	in.Code = Normalize(in.Code)
	if n := len(in.Code); n < 3 || n > 32 {
		fields["code"] = "length"
	}
	switch in.Kind {
	case "percent":
		if in.PercentBP <= 0 || in.PercentBP > 10000 {
			fields["percent_bp"] = "range"
		}
	case "fixed":
		if in.AmountMinor <= 0 {
			fields["amount_minor"] = "range"
		}
	default:
		fields["kind"] = "invalid"
	}
	if in.Scope == "" {
		in.Scope = ScopeAll
	}
	if in.Scope != ScopeAll && in.Scope != ScopeFood && in.Scope != ScopeParcels {
		fields["scope"] = "invalid"
	}
	if in.MinOrderMinor < 0 {
		fields["min_order_minor"] = "range"
	}
	if in.StartsAt != nil && in.EndsAt != nil && in.EndsAt.Before(*in.StartsAt) {
		fields["ends_at"] = "before_start"
	}
	if len(fields) > 0 {
		return httpx.ErrValidation.WithFields(fields)
	}
	return nil
}

func (s *Service) List(ctx context.Context, limit int) ([]Coupon, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT `+couponCols+` FROM coupons ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Coupon{}
	for rows.Next() {
		c, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// Upsert — krijon ose përditëson kuponin me këtë kod (numëratori i përdorimeve nuk prekjet).
func (s *Service) Upsert(ctx context.Context, in UpsertInput) (*Coupon, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var note *string
	if n := strings.TrimSpace(in.Note); n != "" {
		note = &n
	}
	return scanCoupon(s.pool.QueryRow(ctx, `
		INSERT INTO coupons (code, kind, percent_bp, amount_minor, min_order_minor, scope, starts_at, ends_at, max_uses, max_uses_per_user, active, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (code) DO UPDATE SET kind = EXCLUDED.kind, percent_bp = EXCLUDED.percent_bp, amount_minor = EXCLUDED.amount_minor,
		  min_order_minor = EXCLUDED.min_order_minor, scope = EXCLUDED.scope, starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at,
		  max_uses = EXCLUDED.max_uses, max_uses_per_user = EXCLUDED.max_uses_per_user, active = EXCLUDED.active, note = EXCLUDED.note, updated_at = now()
		RETURNING `+couponCols,
		in.Code, in.Kind, in.PercentBP, in.AmountMinor, in.MinOrderMinor, in.Scope, in.StartsAt, in.EndsAt, in.MaxUses, in.MaxUsesPerUser, active, note))
}

func (s *Service) SetActive(ctx context.Context, code string, active bool) (*Coupon, error) {
	c, err := scanCoupon(s.pool.QueryRow(ctx, `UPDATE coupons SET active = $2, updated_at = now() WHERE code = $1 RETURNING `+couponCols, Normalize(code), active))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return c, err
}
