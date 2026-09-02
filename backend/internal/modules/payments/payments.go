// Package payments — pagesat me kartë (§24) dhe mbushja e wallet-it të mbyllur (§5): qëllim pagese
// te ofruesi, konfirmim VETËM nga webhook-u i nënshkruar (dublikatat injorohen), kreditim në ledger
// me çelës idempotence, rimbursime nga Finance. Asnjë P2P, asnjë sukses i simuluar.
package payments

import (
	"context"
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

	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/payment"
)

var (
	ErrAmount        = &httpx.APIError{Code: "TOPUP_AMOUNT_INVALID", MessageKey: "errors.wallet.topup_amount", HTTPStatus: http.StatusUnprocessableEntity}
	ErrDailyLimit    = &httpx.APIError{Code: "TOPUP_DAILY_LIMIT", MessageKey: "errors.wallet.topup_daily_limit", HTTPStatus: http.StatusUnprocessableEntity}
	ErrProviderDown  = &httpx.APIError{Code: "PAYMENT_PROVIDER_UNAVAILABLE", MessageKey: "errors.payments.provider_unavailable", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
	ErrBadSignature  = &httpx.APIError{Code: "WEBHOOK_SIGNATURE_INVALID", MessageKey: "errors.payments.webhook_signature", HTTPStatus: http.StatusBadRequest}
	ErrNotRefundable = &httpx.APIError{Code: "NOT_REFUNDABLE", MessageKey: "errors.payments.not_refundable", HTTPStatus: http.StatusConflict}
)

// Limitet e mbushjes (§67 fraud v1): për transaksion dhe në 24 orë. Konfigurohen më vonë nga admin/feature flags.
const (
	MinTopUpMinor   = 100    // 1,00 €
	MaxTopUpMinor   = 50000  // 500,00 €
	DailyTopUpMinor = 100000 // 1.000,00 €
)

// Flags — feature flags (§65): `wallet.topup` = çaktivizim emergjent i mbushjes.
type Flags interface {
	Enabled(ctx context.Context, key string, userID uuid.UUID) bool
}

var ErrTopUpDisabled = &httpx.APIError{Code: "TOPUP_DISABLED", MessageKey: "errors.wallet.topup_disabled", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}

type Service struct {
	pool     *pgxpool.Pool
	ledger   *ledger.Service
	provider payment.Provider
	flags    Flags
	now      func() time.Time
}

func New(pool *pgxpool.Pool, led *ledger.Service, p payment.Provider) *Service {
	return &Service{pool: pool, ledger: led, provider: p, now: time.Now}
}

func (s *Service) WithFlags(f Flags) *Service {
	s.flags = f
	return s
}

type Intent struct {
	ID           uuid.UUID  `json:"id"`
	Purpose      string     `json:"purpose"`
	AmountMinor  int64      `json:"amount_minor"`
	Currency     string     `json:"currency"`
	Provider     string     `json:"provider"`
	Status       string     `json:"status"`
	FailureCode  *string    `json:"failure_code"`
	ClientSecret string     `json:"client_secret,omitempty"` // vetëm në krijim
	CreatedAt    time.Time  `json:"created_at"`
	SucceededAt  *time.Time `json:"succeeded_at"`
}

const intentCols = `id, purpose, amount_minor, currency, provider, status, failure_code, created_at, succeeded_at`

func scanIntent(row pgx.Row) (*Intent, error) {
	var x Intent
	if err := row.Scan(&x.ID, &x.Purpose, &x.AmountMinor, &x.Currency, &x.Provider, &x.Status, &x.FailureCode, &x.CreatedAt, &x.SucceededAt); err != nil {
		return nil, err
	}
	x.Currency = strings.TrimSpace(x.Currency)
	return &x, nil
}

type TopUpInput struct {
	AmountMinor int64 `json:"amount_minor"`
}

// CreateTopUp — qëllim pagese për mbushje; idempotent me Idempotency-Key (ripërsëritja kthen të njëjtin qëllim,
// pa client_secret të ri). Shuma kontrollohet server-side; asnjë bilanc nuk ndryshon këtu.
func (s *Service) CreateTopUp(ctx context.Context, a principal.Actor, idemKey string, in TopUpInput) (*Intent, error) {
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" || len(idemKey) > 100 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"idempotency_key": "required"})
	}
	if in.AmountMinor < MinTopUpMinor || in.AmountMinor > MaxTopUpMinor || in.AmountMinor%50 != 0 {
		return nil, ErrAmount
	}
	if s.flags != nil && !s.flags.Enabled(ctx, "wallet.topup", a.UserID) {
		return nil, ErrTopUpDisabled
	}
	existing, err := scanIntent(s.pool.QueryRow(ctx, `SELECT `+intentCols+` FROM payment_intents WHERE user_id = $1 AND idempotency_key = $2`, a.UserID, idemKey))
	if err == nil {
		if existing.AmountMinor != in.AmountMinor {
			return nil, httpx.ErrIdempotency
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var today int64
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_minor), 0) FROM payment_intents
		WHERE user_id = $1 AND purpose = 'wallet_topup' AND status IN ('created','requires_action','processing','succeeded')
		  AND created_at > now() - interval '24 hours'`, a.UserID).Scan(&today); err != nil {
		return nil, err
	}
	if today+in.AmountMinor > DailyTopUpMinor {
		return nil, ErrDailyLimit
	}
	// 1) rreshti lokal (status created) — që webhook-u të ketë ku të lidhet edhe nëse ne rrëzohemi pas thirrjes
	intent, err := scanIntent(s.pool.QueryRow(ctx, `
		INSERT INTO payment_intents (user_id, purpose, amount_minor, currency, provider, idempotency_key, metadata)
		VALUES ($1, 'wallet_topup', $2, 'EUR', $3, $4, jsonb_build_object('ip', $5::text))
		RETURNING `+intentCols, a.UserID, in.AmountMinor, s.provider.Name(), idemKey, a.IP))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // garë e së njëjtës kërkesë
			return s.CreateTopUp(ctx, a, idemKey, in)
		}
		return nil, err
	}
	// 2) ofruesi (idempotent me çelësin tonë)
	res, err := s.provider.CreateIntent(ctx, payment.IntentRequest{
		AmountMinor: in.AmountMinor, Currency: "EUR", IdempotencyKey: "topup:" + intent.ID.String(),
		Metadata:    map[string]string{"intent_id": intent.ID.String(), "user_id": a.UserID.String(), "purpose": "wallet_topup"},
		Description: "KREJT Wallet top-up",
	})
	if err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE payment_intents SET status = 'failed', failure_code = 'provider_error', updated_at = now() WHERE id = $1 AND status = 'created'`, intent.ID)
		return nil, ErrProviderDown.With(err)
	}
	if err := s.pool.QueryRow(ctx, `UPDATE payment_intents SET provider_intent_id = $2, status = $3, updated_at = now() WHERE id = $1 RETURNING `+intentCols,
		intent.ID, res.ProviderIntentID, res.Status).Scan(&intent.ID, &intent.Purpose, &intent.AmountMinor, &intent.Currency, &intent.Provider, &intent.Status, &intent.FailureCode, &intent.CreatedAt, &intent.SucceededAt); err != nil {
		return nil, err
	}
	intent.Currency = strings.TrimSpace(intent.Currency)
	intent.ClientSecret = res.ClientSecret
	if _, err := s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata)
		VALUES ($1, 'wallet.topup_started', 'payment_intent', $2, jsonb_build_object('amount_minor', $3::bigint, 'provider', $4::text))`,
		a.UserID, intent.ID.String(), in.AmountMinor, s.provider.Name()); err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *Service) Get(ctx context.Context, a principal.Actor, id uuid.UUID) (*Intent, error) {
	x, err := scanIntent(s.pool.QueryRow(ctx, `SELECT `+intentCols+` FROM payment_intents WHERE id = $1 AND user_id = $2`, id, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return x, err
}

// HandleWebhook — verifikon nënshkrimin, regjistron ngjarjen një herë, zbaton rezultatin.
// Kthen 200 edhe për ngjarje të panjohura/dublikata (ofruesi s'duhet t'i ridërgojë).
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	ev, err := s.provider.ParseWebhook(payload, signature, s.now())
	if err != nil {
		if errors.Is(err, payment.ErrBadSignature) {
			return ErrBadSignature
		}
		return httpx.ErrValidation.With(err)
	}
	if ev.ID == "" {
		return httpx.ErrValidation.WithFields(map[string]string{"event": "missing_id"})
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO payment_webhook_events (provider, event_id, event_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		s.provider.Name(), ev.ID, ev.Type)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // dublikatë: e përpunuar më parë
	}
	perr := s.apply(ctx, ev)
	var errText *string
	if perr != nil {
		t := perr.Error()
		errText = &t
	}
	_, _ = s.pool.Exec(ctx, `UPDATE payment_webhook_events SET processed_at = now(), error = $3 WHERE provider = $1 AND event_id = $2`, s.provider.Name(), ev.ID, errText)
	return perr
}

func (s *Service) apply(ctx context.Context, ev payment.WebhookEvent) error {
	switch {
	case ev.ProviderRefundID != "":
		st := "pending"
		switch ev.Status {
		case "succeeded":
			st = "succeeded"
		case "failed", "canceled":
			st = "failed"
		}
		_, err := s.pool.Exec(ctx, `UPDATE payment_refunds SET status = $2, updated_at = now() WHERE provider_refund_id = $1`, ev.ProviderRefundID, st)
		return err
	case ev.ProviderIntentID == "":
		return nil
	}
	var intent Intent
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id, user_id, purpose, amount_minor, currency, status FROM payment_intents WHERE provider_intent_id = $1`, ev.ProviderIntentID).
		Scan(&intent.ID, &userID, &intent.Purpose, &intent.AmountMinor, &intent.Currency, &intent.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // qëllim jo i yni (p.sh. nga dashboard-i i ofruesit) — injorohet, por mbetet i regjistruar
	}
	if err != nil {
		return err
	}
	intent.Currency = strings.TrimSpace(intent.Currency)
	if _, err := s.pool.Exec(ctx, `UPDATE payment_webhook_events SET intent_id = $3 WHERE provider = $1 AND event_id = $2`, s.provider.Name(), ev.ID, intent.ID); err != nil {
		return err
	}
	switch ev.Status {
	case "succeeded":
		if ev.AmountMinor != intent.AmountMinor || (ev.Currency != "" && ev.Currency != intent.Currency) {
			// shuma e ofruesit ndryshon nga e jona: nuk kreditohet; Finance e sheh në rakordim
			_, err := s.pool.Exec(ctx, `UPDATE payment_intents SET status = 'failed', failure_code = 'amount_mismatch', updated_at = now() WHERE id = $1 AND status <> 'succeeded'`, intent.ID)
			return err
		}
		return s.credit(ctx, intent.ID, userID, intent.AmountMinor, intent.Currency)
	case "canceled":
		_, err := s.pool.Exec(ctx, `UPDATE payment_intents SET status = 'canceled', updated_at = now() WHERE id = $1 AND status NOT IN ('succeeded','canceled')`, intent.ID)
		return err
	default:
		if ev.FailureCode != "" {
			_, err := s.pool.Exec(ctx, `UPDATE payment_intents SET status = 'failed', failure_code = $2, updated_at = now() WHERE id = $1 AND status NOT IN ('succeeded','canceled')`, intent.ID, ev.FailureCode)
			return err
		}
		_, err := s.pool.Exec(ctx, `UPDATE payment_intents SET status = $2, updated_at = now() WHERE id = $1 AND status NOT IN ('succeeded','canceled','failed')`, intent.ID, ev.Status)
		return err
	}
}

// credit — kreditimi i wallet-it (idempotent me "topup:{intent}"): debit karta (clearing), kredit wallet-i.
func (s *Service) credit(ctx context.Context, intentID, userID uuid.UUID, amount int64, currency string) error {
	code := ledger.UserWalletCode(userID)
	if err := s.ledger.EnsureAccount(ctx, code, "user", &userID, "liability", currency); err != nil {
		return err
	}
	if _, err := s.ledger.Post(ctx, ledger.Transaction{Kind: "wallet_topup_card", Reference: "payment_intent:" + intentID.String(),
		IdempotencyKey: "topup:" + intentID.String(), Currency: currency,
		Postings: []ledger.Posting{{AccountCode: "krejt:card_clearing", Debit: money.Minor(amount)}, {AccountCode: code, Credit: money.Minor(amount)}}}); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE payment_intents SET status = 'succeeded', succeeded_at = now(), updated_at = now() WHERE id = $1 AND status <> 'succeeded'`, intentID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_type, action, target_type, target_id, metadata)
			VALUES ('system', 'wallet.topup_succeeded', 'payment_intent', $1, jsonb_build_object('amount_minor', $2::bigint, 'user_id', $3::text))`,
			intentID.String(), amount, userID.String()); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "wallet", userID.String(), "WalletToppedUp", map[string]any{"user_id": userID, "intent_id": intentID, "amount_minor": amount, "currency": currency})
	})
}

// Refund — Finance rimburson (pjesërisht ose plotësisht) një mbushje të suksesshme: wallet-i debitohet
// (duhet të ketë mjaftueshëm — paratë e shpenzuara nuk kthehen dot), karta kreditohet përmes ofruesit.
func (s *Service) Refund(ctx context.Context, finance principal.Actor, intentID uuid.UUID, amount int64, reason string) (uuid.UUID, error) {
	reason = strings.Join(strings.Fields(reason), " ")
	if amount <= 0 || reason == "" || utf8.RuneCountInString(reason) > 300 {
		return uuid.Nil, httpx.ErrValidation.WithFields(map[string]string{"amount_minor": "invalid", "reason": "required"})
	}
	var userID uuid.UUID
	var providerIntent *string
	var total, refunded int64
	var status, currency string
	err := s.pool.QueryRow(ctx, `
		SELECT i.user_id, i.provider_intent_id, i.amount_minor, i.status, i.currency,
		       COALESCE((SELECT SUM(amount_minor) FROM payment_refunds r WHERE r.intent_id = i.id AND r.status <> 'failed'), 0)
		FROM payment_intents i WHERE i.id = $1`, intentID).Scan(&userID, &providerIntent, &total, &status, &currency, &refunded)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, httpx.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	currency = strings.TrimSpace(currency)
	if status != "succeeded" || providerIntent == nil || refunded+amount > total {
		return uuid.Nil, ErrNotRefundable
	}
	bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(userID))
	if err != nil {
		return uuid.Nil, err
	}
	if int64(bal.Minor) < amount {
		return uuid.Nil, ErrNotRefundable.WithFields(map[string]string{"wallet": "insufficient_balance"})
	}
	refundID := uuid.New()
	idem := "refund:" + refundID.String()
	if _, err := s.pool.Exec(ctx, `INSERT INTO payment_refunds (id, intent_id, amount_minor, reason, requested_by, idempotency_key) VALUES ($1, $2, $3, $4, $5, $6)`,
		refundID, intentID, amount, reason, finance.UserID, idem); err != nil {
		return uuid.Nil, err
	}
	// ledger para ofruesit: nëse ofruesi dështon, rreshti mbetet 'pending' me borxh të rikthyer nga Finance
	if _, err := s.ledger.Post(ctx, ledger.Transaction{Kind: "wallet_topup_refund", Reference: "refund:" + refundID.String(), IdempotencyKey: idem, Currency: currency,
		Postings: []ledger.Posting{{AccountCode: ledger.UserWalletCode(userID), Debit: money.Minor(amount)}, {AccountCode: "krejt:card_clearing", Credit: money.Minor(amount)}}}); err != nil {
		return uuid.Nil, err
	}
	res, err := s.provider.Refund(ctx, payment.RefundRequest{ProviderIntentID: *providerIntent, AmountMinor: amount, IdempotencyKey: idem, Reason: reason})
	if err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE payment_refunds SET status = 'failed', updated_at = now() WHERE id = $1`, refundID)
		// paratë i kthehen wallet-it (rimbursimi nuk ndodhi)
		_, _ = s.ledger.Post(ctx, ledger.Transaction{Kind: "wallet_topup_refund_reversal", Reference: "refund:" + refundID.String(), IdempotencyKey: idem + ":reversal", Currency: currency,
			Postings: []ledger.Posting{{AccountCode: "krejt:card_clearing", Debit: money.Minor(amount)}, {AccountCode: ledger.UserWalletCode(userID), Credit: money.Minor(amount)}}})
		return uuid.Nil, ErrProviderDown.With(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE payment_refunds SET provider_refund_id = $2, status = $3, updated_at = now() WHERE id = $1`, refundID, res.ProviderRefundID, res.Status); err != nil {
		return uuid.Nil, err
	}
	meta, _ := json.Marshal(map[string]any{"amount_minor": amount, "reason": reason, "user_id": userID})
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'wallet.refund', 'payment_refund', $2, $3)`, finance.UserID, refundID.String(), meta)
	return refundID, nil
}
