// Package dataexport — eksporti i të dhënave personale (§16). Përdoruesi e kërkon, worker-i e
// ndërton në sfond, dhe skedari shkarkohet me URL të nënshkruar jetëshkurtër. API-ja nuk i shërben
// kurrë bajtët vetë, njësoj si te dokumentet e shoferit.
//
// Eksporti përmban vetëm të dhënat e vetë përdoruesit. Emrat dhe numrat e palëve të tjera —
// shoferi i një udhëtimi, korrieri i një porosie — nuk hyjnë: ato janë të dhënat e tyre, jo të tij.
package dataexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

var (
	// ErrTooSoon — një eksport në ditë. Ndërtimi lexon gjithë historikun e përdoruesit, ndaj
	// përsëritja e shpeshtë do të ishte një mënyrë e lehtë për ta ngarkuar bazën.
	ErrTooSoon = &httpx.APIError{
		Code:       "EXPORT_TOO_SOON",
		MessageKey: "errors.export.too_soon",
		HTTPStatus: http.StatusTooManyRequests,
	}
	ErrInProgress = &httpx.APIError{
		Code:       "EXPORT_IN_PROGRESS",
		MessageKey: "errors.export.in_progress",
		HTTPStatus: http.StatusConflict,
	}
)

const (
	// Sa shpesh lejohet një eksport i ri.
	MinInterval = 24 * time.Hour
	// Sa gjatë rri skedari para se të fshihet.
	RetentionPeriod = 7 * 24 * time.Hour
	// Sa gjatë vlen lidhja e shkarkimit.
	downloadTTL = 10 * time.Minute
)

type Service struct {
	pool    *pgxpool.Pool
	storage storage.Provider
	now     func() time.Time
}

func New(pool *pgxpool.Pool, store storage.Provider) *Service {
	return &Service{pool: pool, storage: store, now: time.Now}
}

// Export — gjendja e një kërkese, ashtu siç e sheh përdoruesi.
type Export struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	SizeBytes   *int64     `json:"size_bytes"`
	// DownloadURL vendoset vetëm kur eksporti është gati dhe skedari s'ka skaduar.
	DownloadURL string `json:"download_url,omitempty"`
}

// Request — regjistron një kërkesë të re. Nuk ndërton asgjë: atë e bën worker-i.
func (s *Service) Request(ctx context.Context, a principal.Actor) (*Export, error) {
	// Krahasimi i kohës bëhet nga baza, jo nga aplikacioni: rreshtat i vulos ajo, ndaj vetëm ajo
	// e di me siguri sa kohë ka kaluar.
	var lastStatus string
	var tooSoon bool
	err := s.pool.QueryRow(ctx, `
		SELECT status, requested_at > now() - $2::interval
		FROM data_exports WHERE user_id = $1 ORDER BY requested_at DESC LIMIT 1`,
		a.UserID, MinInterval.String()).Scan(&lastStatus, &tooSoon)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if lastStatus == "pending" || lastStatus == "building" {
			return nil, ErrInProgress
		}
		// Një kërkesë e dështuar mund të përsëritet menjëherë: dështimi nuk është faji i përdoruesit.
		if lastStatus != "failed" && tooSoon {
			return nil, ErrTooSoon
		}
	}

	id := uuid.New()
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO data_exports (id, user_id) VALUES ($1, $2)`, id, a.UserID); err != nil {
		return nil, err
	}
	return s.Latest(ctx, a.UserID)
}

// Latest — kërkesa e fundit e përdoruesit, me lidhje shkarkimi kur ka çfarë të shkarkohet.
func (s *Service) Latest(ctx context.Context, userID uuid.UUID) (*Export, error) {
	var e Export
	var objectKey *string
	var expired bool
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, requested_at, completed_at, expires_at, size_bytes, object_key,
		       COALESCE(expires_at < now(), false)
		FROM data_exports WHERE user_id = $1 ORDER BY requested_at DESC LIMIT 1`, userID).
		Scan(&e.ID, &e.Status, &e.RequestedAt, &e.CompletedAt, &e.ExpiresAt, &e.SizeBytes, &objectKey, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if e.Status == "ready" && objectKey != nil {
		// Skedari mund të ketë skaduar pa e prekur ende puna periodike; atëherë nuk premtojmë lidhje.
		if expired {
			e.Status = "expired"
			return &e, nil
		}
		url, err := s.storage.PresignDownload(ctx, *objectKey, downloadTTL)
		if err != nil {
			return nil, err
		}
		e.DownloadURL = url
	}
	return &e, nil
}

// Bundle — përmbajtja e eksportit. Fushat janë të njëjtat emra si te API-ja, që skedari të lexohet
// pa pasur nevojë për një fjalor përkthimi.
type Bundle struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Profile     map[string]any    `json:"profile"`
	Addresses   []map[string]any  `json:"addresses"`
	Rides       []map[string]any  `json:"rides"`
	Orders      []map[string]any  `json:"orders"`
	Wallet      []map[string]any  `json:"wallet_transactions"`
	Support     []map[string]any  `json:"support_tickets"`
	Sessions    []map[string]any  `json:"sessions"`
	Preferences []map[string]any  `json:"notification_preferences"`
	Notes       map[string]string `json:"notes"`
}

// BuildNext — merr një kërkesë në pritje dhe e ndërton. Kthen false kur nuk kishte asgjë për të bërë.
// Thirret nga worker-i; API-ja nuk e ekzekuton kurrë vetë, sepse mund të zgjasë.
func (s *Service) BuildNext(ctx context.Context) (bool, error) {
	var id, userID uuid.UUID
	// FOR UPDATE SKIP LOCKED: dy worker-a nuk e marrin të njëjtën kërkesë.
	err := s.pool.QueryRow(ctx, `
		UPDATE data_exports SET status = 'building', started_at = now()
		WHERE id = (
			SELECT id FROM data_exports WHERE status = 'pending'
			ORDER BY requested_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING id, user_id`).Scan(&id, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	key, size, buildErr := s.build(ctx, id, userID)
	if buildErr != nil {
		_, _ = s.pool.Exec(ctx,
			`UPDATE data_exports SET status = 'failed', error_code = $2, completed_at = now() WHERE id = $1`,
			id, "build_failed")
		return true, buildErr
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE data_exports
		SET status = 'ready', object_key = $2, size_bytes = $3, completed_at = now(),
		    expires_at = now() + $4::interval
		WHERE id = $1`, id, key, size, RetentionPeriod.String())
	return true, err
}

func (s *Service) build(ctx context.Context, id, userID uuid.UUID) (string, int64, error) {
	bundle := Bundle{
		GeneratedAt: s.now().UTC(),
		Notes: map[string]string{
			"scope": "Vetëm të dhënat e kësaj llogarie. Të dhënat e shoferëve, korrierëve dhe " +
				"vendeve nuk përfshihen, sepse janë të dhënat e tyre.",
			"money": "Të gjitha shumat janë numra të plotë në cent.",
		},
	}

	var err error
	if bundle.Profile, err = s.one(ctx, `
		SELECT id, phone_e164 AS phone, email, full_name, locale, status, created_at
		FROM users WHERE id = $1`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Addresses, err = s.rows(ctx, `
		SELECT id, label, name, line1, line2, city, postal_code, lat, lng, instructions, is_default, created_at
		FROM user_addresses WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Rides, err = s.rows(ctx, `
		SELECT id, state, category_id, payment_method, payment_status, pickup_address, dropoff_address,
		       distance_m, duration_s, price_quoted_minor, price_final_minor, cancellation_fee_minor,
		       currency, requested_at, completed_at, cancelled_at
		FROM rides WHERE customer_id = $1 ORDER BY requested_at`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Orders, err = s.rows(ctx, `
		SELECT id, code, state, fulfillment, payment_method, payment_status, items_total_minor,
		       delivery_fee_minor, total_minor, currency, created_at, delivered_at
		FROM orders WHERE customer_id = $1 ORDER BY created_at`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Wallet, err = s.rows(ctx, `
		SELECT t.id, t.kind, t.reference, e.credit_minor - e.debit_minor AS amount_minor, e.currency, e.created_at
		FROM ledger_entries e
		JOIN ledger_accounts a ON a.id = e.account_id
		JOIN ledger_transactions t ON t.id = e.tx_id
		WHERE a.code = $1 ORDER BY e.created_at`, ledger.UserWalletCode(userID)); err != nil {
		return "", 0, err
	}
	if bundle.Support, err = s.rows(ctx, `
		SELECT id, category, subject, status, priority, created_at, resolved_at
		FROM support_tickets WHERE user_id = $1 ORDER BY created_at`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Sessions, err = s.rows(ctx, `
		SELECT id, device_name, platform, last_seen_at, created_at
		FROM sessions WHERE user_id = $1 AND revoked_at IS NULL ORDER BY created_at`, userID); err != nil {
		return "", 0, err
	}
	if bundle.Preferences, err = s.rows(ctx, `
		SELECT category, push, email, sms FROM notification_preferences WHERE user_id = $1 ORDER BY category`,
		userID); err != nil {
		return "", 0, err
	}

	body, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", 0, err
	}

	key := fmt.Sprintf("exports/%s/%s.json", userID, id)
	if !storage.ValidKey(key) {
		return "", 0, fmt.Errorf("dataexport: çelës i pavlefshëm %q", key)
	}
	if err := s.storage.PutBytes(ctx, key, "application/json", body); err != nil {
		return "", 0, err
	}
	return key, int64(len(body)), nil
}

// ExpireOld — fshin skedarët e skaduar dhe i shënon kërkesat si të skaduara. Rreshti mbetet,
// që përdoruesi ta shohë se eksporti u bë dhe kur.
func (s *Service) ExpireOld(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, object_key FROM data_exports
		WHERE status = 'ready' AND expires_at IS NOT NULL AND expires_at < now() LIMIT 100`)
	if err != nil {
		return 0, err
	}
	type item struct {
		id  uuid.UUID
		key string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.key); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, it := range items {
		// Fshirja e objektit është e para: një rresht i shënuar si i skaduar me skedarin ende
		// aty do të thoshte se të dhënat rrinë më gjatë se sa premtohet.
		if err := s.storage.Delete(ctx, it.key); err != nil {
			return 0, err
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE data_exports SET status = 'expired', object_key = NULL WHERE id = $1`, it.id); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

// readable — drajveri i kthen disa lloje si bajta të papërpunuar; skedari duhet të lexohet
// nga një njeri, jo nga një program, ndaj identifikuesit dalin si tekst e jo si varg numrash.
func readable(v any) any {
	switch t := v.(type) {
	case [16]byte:
		return uuid.UUID(t).String()
	case []byte:
		return string(t)
	default:
		return v
	}
}

func (s *Service) one(ctx context.Context, sql string, args ...any) (map[string]any, error) {
	out, err := s.rows(ctx, sql, args...)
	if err != nil || len(out) == 0 {
		return nil, err
	}
	return out[0], nil
}

func (s *Service) rows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		m := make(map[string]any, len(fields))
		for i, f := range fields {
			m[string(f.Name)] = readable(values[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
