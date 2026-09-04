// Package business — KREJT Business (§34): llogaria e një ndërmarrjeje, punonjësit dhe kufijtë.
//
// Paraja e ndërmarrjes jeton te libri si çdo llogari tjetër, me kodin `business:{id}:wallet`.
// Kjo nuk është hollësi zbatimi: do të thotë se shpenzimi i një punonjësi është një regjistrim i
// zakonshëm me dy hyrje, që bilanci del i saktë me të njëjtat rregulla si kudo, dhe se asnjë rrugë
// e dytë paralele nuk duhet mbajtur në këmbë.
package business

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// WalletCode — kodi i llogarisë së ndërmarrjes te libri.
func WalletCode(businessID uuid.UUID) string { return "business:" + businessID.String() + ":wallet" }

var (
	ErrNotMember = &httpx.APIError{
		Code: "NOT_A_MEMBER", MessageKey: "errors.business.not_member", HTTPStatus: http.StatusForbidden,
	}
	ErrNotAdmin = &httpx.APIError{
		Code: "BUSINESS_FORBIDDEN", MessageKey: "errors.business.forbidden", HTTPStatus: http.StatusForbidden,
	}
	ErrLastOwner = &httpx.APIError{
		Code: "LAST_OWNER", MessageKey: "errors.business.last_owner", HTTPStatus: http.StatusConflict,
	}
	ErrLimitReached = &httpx.APIError{
		Code: "BUSINESS_LIMIT_REACHED", MessageKey: "errors.business.limit_reached", HTTPStatus: http.StatusConflict,
	}
	ErrInsufficient = &httpx.APIError{
		Code: "BUSINESS_INSUFFICIENT_FUNDS", MessageKey: "errors.business.insufficient", HTTPStatus: http.StatusConflict,
	}
)

type Business struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	TaxID        *string   `json:"tax_id"`
	AddressLine1 *string   `json:"address_line1"`
	City         *string   `json:"city"`
	BillingEmail *string   `json:"billing_email"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`

	// Vetëm te përgjigjet ku roli i kërkuesit dihet.
	Role string `json:"role,omitempty"`
}

type Member struct {
	UserID       uuid.UUID `json:"user_id"`
	Name         *string   `json:"name"`
	Phone        *string   `json:"phone"`
	Role         string    `json:"role"`
	MonthlyLimit *int64    `json:"monthly_limit_minor"`
	Active       bool      `json:"active"`

	// Sa ka shpenzuar këtë muaj; llogaritet nga shpenzimet, jo nga një numërator i ruajtur.
	SpentThisMonth int64 `json:"spent_this_month_minor"`
}

type Charge struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	UserName  *string   `json:"user_name"`
	Kind      string    `json:"kind"`
	SubjectID uuid.UUID `json:"subject_id"`
	Amount    int64     `json:"amount_minor"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
	now    func() time.Time
}

func New(pool *pgxpool.Pool, l *ledger.Service) *Service {
	return &Service{pool: pool, ledger: l, now: time.Now}
}

type CreateInput struct {
	Name         string `json:"name"`
	TaxID        string `json:"tax_id"`
	AddressLine1 string `json:"address_line1"`
	City         string `json:"city"`
	BillingEmail string `json:"billing_email"`
}

func clean(s string, max int) string {
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

// Create — kush e hap ndërmarrjen bëhet pronari i saj. Të dyja në një transaksion: një ndërmarrje
// pa asnjë pronar nuk do të kishte kush ta administronte.
func (s *Service) Create(ctx context.Context, a principal.Actor, in CreateInput) (*Business, error) {
	in.Name = clean(in.Name, 120)
	if n := utf8.RuneCountInString(in.Name); n < 2 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"name": "invalid"})
	}
	in.TaxID = clean(in.TaxID, 40)
	in.AddressLine1 = clean(in.AddressLine1, 160)
	in.City = clean(in.City, 80)
	in.BillingEmail = strings.ToLower(clean(in.BillingEmail, 160))
	if in.BillingEmail != "" && !strings.Contains(in.BillingEmail, "@") {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"billing_email": "invalid"})
	}

	var out *Business
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var b Business
		if err := tx.QueryRow(ctx, `
			INSERT INTO businesses (name, tax_id, address_line1, city, billing_email)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, name, tax_id, address_line1, city, billing_email, status, created_at`,
			in.Name, nullable(in.TaxID), nullable(in.AddressLine1), nullable(in.City), nullable(in.BillingEmail)).
			Scan(&b.ID, &b.Name, &b.TaxID, &b.AddressLine1, &b.City, &b.BillingEmail, &b.Status, &b.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO business_members (business_id, user_id, role) VALUES ($1, $2, 'owner')`,
			b.ID, a.UserID); err != nil {
			return err
		}
		b.Role = "owner"
		out = &b
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Llogaria te libri hapet jashtë transaksionit të krijimit: EnsureAccount është idempotente,
	// dhe një dështim këtu nuk duhet ta zhbëjë ndërmarrjen e sapokrijuar.
	if err := s.ledger.EnsureAccount(ctx, WalletCode(out.ID), "business", nil, "liability", "EUR"); err != nil {
		return nil, err
	}
	return out, nil
}

// Mine — ndërmarrjet ku ky përdorues është anëtar aktiv.
func (s *Service) Mine(ctx context.Context, a principal.Actor) ([]Business, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.name, b.tax_id, b.address_line1, b.city, b.billing_email, b.status, b.created_at, m.role
		FROM businesses b
		JOIN business_members m ON m.business_id = b.id AND m.user_id = $1 AND m.active
		WHERE b.status = 'active'
		ORDER BY b.created_at`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Business{}
	for rows.Next() {
		var b Business
		if err := rows.Scan(&b.ID, &b.Name, &b.TaxID, &b.AddressLine1, &b.City, &b.BillingEmail,
			&b.Status, &b.CreatedAt, &b.Role); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// role — roli i një përdoruesi te një ndërmarrje, ose gabim kur nuk është anëtar aktiv.
func (s *Service) role(ctx context.Context, businessID, userID uuid.UUID) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM business_members WHERE business_id = $1 AND user_id = $2 AND active`,
		businessID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		// Një ndërmarrje ku nuk je anëtar nuk ekziston për ty.
		return "", ErrNotMember
	}
	return role, err
}

func (s *Service) requireAdmin(ctx context.Context, businessID, userID uuid.UUID) error {
	role, err := s.role(ctx, businessID, userID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return ErrNotAdmin
	}
	return nil
}

// Get — profili i ndërmarrjes bashkë me rolin e kërkuesit; ekrani e di menjëherë çfarë mund të bëjë.
func (s *Service) Get(ctx context.Context, a principal.Actor, businessID uuid.UUID) (*Business, error) {
	role, err := s.role(ctx, businessID, a.UserID)
	if err != nil {
		return nil, err
	}
	var b Business
	err = s.pool.QueryRow(ctx, `
		SELECT id, name, tax_id, address_line1, city, billing_email, status, created_at
		FROM businesses WHERE id = $1`, businessID).
		Scan(&b.ID, &b.Name, &b.TaxID, &b.AddressLine1, &b.City, &b.BillingEmail, &b.Status, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Role = role
	return &b, nil
}

// Charges — shpenzimet e ndërmarrjes, nga më i riu, me shumën e periudhës.
//
// Totali llogaritet mbi të gjithë rreshtat e periudhës e jo mbi faqen: një total që ndryshon sa
// herë shtyp "më shumë" nuk është total.
func (s *Service) Charges(ctx context.Context, a principal.Actor, businessID uuid.UUID, before *time.Time, limit int) ([]Charge, int64, error) {
	if _, err := s.role(ctx, businessID, a.UserID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_minor), 0) FROM business_charges
		 WHERE business_id = $1 AND created_at >= $2`, businessID, monthStart(s.now())).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.user_id, u.full_name, c.kind, c.subject_id, c.amount_minor, c.currency, c.created_at
		FROM business_charges c
		JOIN users u ON u.id = c.user_id
		WHERE c.business_id = $1 AND ($3::timestamptz IS NULL OR c.created_at < $3)
		ORDER BY c.created_at DESC LIMIT $2`, businessID, limit, before)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Charge{}
	for rows.Next() {
		var c Charge
		if err := rows.Scan(&c.ID, &c.UserID, &c.UserName, &c.Kind, &c.SubjectID, &c.Amount,
			&c.Currency, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
