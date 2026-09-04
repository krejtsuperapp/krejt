// Package payment — PaymentProvider (§24): qëllim pagese, rimbursim, verifikim webhook-u. Zbatimi i
// parë: Stripe (entiteti në BE sipas stack-ut); Raiffeisen (gateway lokal) vjen pas kontratës, pas së
// njëjtës ndërfaqe. Suksesi i pagesës vjen VETËM nga webhook-u i nënshkruar — kurrë nga klienti.
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type IntentRequest struct {
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	Metadata       map[string]string // intent_id, user_id, purpose — për rakordim
	Description    string
}

type IntentResult struct {
	ProviderIntentID string
	ClientSecret     string // i kthehet aplikacionit për konfirmim (Stripe SDK); nuk ruhet
	Status           string // created | requires_action | processing | succeeded | failed | canceled
}

type RefundRequest struct {
	ProviderIntentID string
	AmountMinor      int64
	IdempotencyKey   string
	Reason           string
}

type RefundResult struct {
	ProviderRefundID string
	Status           string // pending | succeeded | failed
}

// WebhookEvent — ngjarje e normalizuar nga ofruesi.
type WebhookEvent struct {
	ID               string
	Type             string // payment_intent.succeeded | payment_intent.payment_failed | payment_intent.canceled | refund.updated | other
	ProviderIntentID string
	ProviderRefundID string
	Status           string
	AmountMinor      int64
	Currency         string
	FailureCode      string
}

var ErrBadSignature = errors.New("payment: nënshkrim webhook-u i pavlefshëm")

// ErrDisabled — nuk ka ofrues pagese: cash-i dhe kuleta punojnë, mbushja me kartë jo.
var ErrDisabled = errors.New("payment: pagesat me kartë nuk janë të hapura")

type Provider interface {
	Name() string
	CreateIntent(ctx context.Context, in IntentRequest) (IntentResult, error)
	Refund(ctx context.Context, in RefundRequest) (RefundResult, error)
	// ParseWebhook verifikon nënshkrimin dhe normalizon ngjarjen.
	ParseWebhook(payload []byte, signatureHeader string, now time.Time) (WebhookEvent, error)
}

// --- Stripe ------------------------------------------------------------------------

type Stripe struct {
	secretKey     string
	webhookSecret string
	base          string
	http          *http.Client
}

func NewStripe(secretKey, webhookSecret string) (*Stripe, error) {
	if secretKey == "" || webhookSecret == "" {
		return nil, errors.New("payment: STRIPE_SECRET_KEY / STRIPE_WEBHOOK_SECRET mungojnë")
	}
	return &Stripe{secretKey: secretKey, webhookSecret: webhookSecret, base: "https://api.stripe.com", http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (s *Stripe) Name() string { return "stripe" }

func (s *Stripe) post(ctx context.Context, path string, form url.Values, idem string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+s.secretKey)
	req.Header.Set("Stripe-Version", "2024-06-20")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("payment: stripe %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return fmt.Errorf("payment: stripe %s: HTTP %d %s %s", path, resp.StatusCode, e.Error.Type, e.Error.Code)
	}
	return json.Unmarshal(body, out)
}

func (s *Stripe) CreateIntent(ctx context.Context, in IntentRequest) (IntentResult, error) {
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(in.AmountMinor, 10))
	form.Set("currency", strings.ToLower(in.Currency))
	form.Set("automatic_payment_methods[enabled]", "true")
	if in.Description != "" {
		form.Set("description", in.Description)
	}
	for k, v := range in.Metadata {
		form.Set("metadata["+k+"]", v)
	}
	var out struct {
		ID           string `json:"id"`
		ClientSecret string `json:"client_secret"`
		Status       string `json:"status"`
	}
	if err := s.post(ctx, "/v1/payment_intents", form, in.IdempotencyKey, &out); err != nil {
		return IntentResult{}, err
	}
	return IntentResult{ProviderIntentID: out.ID, ClientSecret: out.ClientSecret, Status: normalizeStripeStatus(out.Status)}, nil
}

func (s *Stripe) Refund(ctx context.Context, in RefundRequest) (RefundResult, error) {
	form := url.Values{}
	form.Set("payment_intent", in.ProviderIntentID)
	form.Set("amount", strconv.FormatInt(in.AmountMinor, 10))
	if in.Reason != "" {
		form.Set("metadata[reason]", in.Reason)
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"` // pending | succeeded | failed | canceled
	}
	if err := s.post(ctx, "/v1/refunds", form, in.IdempotencyKey, &out); err != nil {
		return RefundResult{}, err
	}
	st := "pending"
	switch out.Status {
	case "succeeded":
		st = "succeeded"
	case "failed", "canceled":
		st = "failed"
	}
	return RefundResult{ProviderRefundID: out.ID, Status: st}, nil
}

// ParseWebhook — Stripe-Signature: t=<unix>,v1=<hex hmac sha256("<t>.<payload>")>, tolerancë 5 min.
func (s *Stripe) ParseWebhook(payload []byte, sigHeader string, now time.Time) (WebhookEvent, error) {
	var ts string
	var sigs []string
	for _, part := range strings.Split(sigHeader, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return WebhookEvent{}, ErrBadSignature
	}
	if d := now.Sub(time.Unix(tsInt, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return WebhookEvent{}, ErrBadSignature
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	want := hex.EncodeToString(mac.Sum(nil))
	valid := false
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(want)) {
			valid = true
		}
	}
	if !valid {
		return WebhookEvent{}, ErrBadSignature
	}
	var ev struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID            string `json:"id"`
				Object        string `json:"object"`
				Status        string `json:"status"`
				Amount        int64  `json:"amount"`
				Currency      string `json:"currency"`
				PaymentIntent string `json:"payment_intent"`
				LastError     *struct {
					Code        string `json:"code"`
					DeclineCode string `json:"decline_code"`
				} `json:"last_payment_error"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return WebhookEvent{}, fmt.Errorf("payment: webhook json: %w", err)
	}
	out := WebhookEvent{ID: ev.ID, Type: ev.Type, AmountMinor: ev.Data.Object.Amount, Currency: strings.ToUpper(ev.Data.Object.Currency)}
	switch ev.Data.Object.Object {
	case "payment_intent":
		out.ProviderIntentID = ev.Data.Object.ID
		out.Status = normalizeStripeStatus(ev.Data.Object.Status)
		if ev.Data.Object.LastError != nil {
			out.FailureCode = ev.Data.Object.LastError.DeclineCode
			if out.FailureCode == "" {
				out.FailureCode = ev.Data.Object.LastError.Code
			}
		}
	case "refund":
		out.ProviderRefundID = ev.Data.Object.ID
		out.ProviderIntentID = ev.Data.Object.PaymentIntent
		out.Status = ev.Data.Object.Status
	}
	return out, nil
}

func normalizeStripeStatus(s string) string {
	switch s {
	case "requires_payment_method", "requires_confirmation":
		return "created"
	case "requires_action", "requires_capture":
		return "requires_action"
	case "processing":
		return "processing"
	case "succeeded":
		return "succeeded"
	case "canceled":
		return "canceled"
	}
	return "created"
}

// --- None: nisje vetëm me cash --------------------------------------------------------
// Pa ofrues pagese aplikacioni punon me cash dhe me kuletë; vetëm mbushja me kartë refuzohet,
// hapur dhe me arsyen e vet. Lejohet në prodhim: mospasja e kartës nuk është shtirje.

type NoneProvider struct{}

func (NoneProvider) Name() string { return "none" }

func (NoneProvider) CreateIntent(context.Context, IntentRequest) (IntentResult, error) {
	return IntentResult{}, ErrDisabled
}

func (NoneProvider) Refund(context.Context, RefundRequest) (RefundResult, error) {
	return RefundResult{}, ErrDisabled
}

func (NoneProvider) ParseWebhook([]byte, string, time.Time) (WebhookEvent, error) {
	return WebhookEvent{}, ErrBadSignature
}

// --- DevLog (VETËM development) -----------------------------------------------------
// Krijon qëllime "pi_dev_…" pa asnjë ofrues; suksesi vjen VETËM nga endpoint-i dev i simulimit të
// webhook-ut (POST /api/v1/dev/payments/{id}/{succeed|fail}) — asnjë pagesë nuk "kalon" vetvetiu.

type DevLog struct {
	log    *slog.Logger
	secret string
}

func (d *DevLog) Name() string { return "devlog" }

func (d *DevLog) CreateIntent(_ context.Context, in IntentRequest) (IntentResult, error) {
	id := "pi_dev_" + strings.ReplaceAll(in.IdempotencyKey, ":", "_")
	d.log.Info("DEV ONLY — payment intent (no provider)", "id", id, "amount_minor", in.AmountMinor, "currency", in.Currency)
	return IntentResult{ProviderIntentID: id, ClientSecret: id + "_secret_dev", Status: "created"}, nil
}

func (d *DevLog) Refund(_ context.Context, in RefundRequest) (RefundResult, error) {
	d.log.Info("DEV ONLY — refund (no provider)", "intent", in.ProviderIntentID, "amount_minor", in.AmountMinor)
	return RefundResult{ProviderRefundID: "re_dev_" + in.IdempotencyKey, Status: "succeeded"}, nil
}

// ParseWebhook — i njëjti format nënshkrimi si Stripe (t=,v1=) me sekretin dev, që endpoint-i dev
// të kalojë nga e njëjta rrugë verifikimi.
func (d *DevLog) ParseWebhook(payload []byte, sigHeader string, now time.Time) (WebhookEvent, error) {
	return (&Stripe{webhookSecret: d.secret}).ParseWebhook(payload, sigHeader, now)
}

// SignDev — nënshkrim për endpoint-in dev (vetëm development).
func (d *DevLog) SignDev(payload []byte, now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(d.secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// NewFromEnv — PAYMENT_PROVIDER: stripe (parazgjedhje) | none (vetëm cash/kuletë, edhe në prodhim) |
// devlog (development dhe staging; kurrë production).
func NewFromEnv(env, provider, stripeSecret, stripeWebhookSecret string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "stripe", "":
		return NewStripe(stripeSecret, stripeWebhookSecret)
	case "none":
		log.Info("payment: PAYMENT_PROVIDER=none — vetëm cash dhe kuletë; mbushja me kartë e mbyllur")
		return NoneProvider{}, nil
	case "devlog":
		if env == "production" {
			return nil, fmt.Errorf("payment: devlog nuk lejohet në production (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — PAYMENT_PROVIDER=devlog: pagesat nuk kalojnë kurrë vetvetiu; suksesi vetëm nga endpoint-i dev")
		return &DevLog{log: log, secret: "krejt_dev_only_webhook_secret"}, nil
	default:
		return nil, fmt.Errorf("payment: ofrues i panjohur %q", provider)
	}
}
