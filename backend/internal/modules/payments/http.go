package payments

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/payment"
)

// Routes — klienti (requireAuth), Finance (requireFinance), webhook-u publik (nënshkrim).
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireFinance httpx.Middleware) {
	mux.Handle("POST /api/v1/wallet/topup", requireAuth(principal.Handler(s.handleTopUp)))
	mux.Handle("GET /api/v1/payments/intents/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/admin/payments/intents/{id}/refund", requireFinance(principal.Handler(s.handleRefund)))
	mux.HandleFunc("POST /api/v1/payments/webhook/"+s.provider.Name(), s.handleWebhook)
}

// DevRoutes — VETËM development me PAYMENT_PROVIDER=devlog: simulon webhook-un e ofruesit (i nënshkruar).
func (s *Service) DevRoutes(mux *http.ServeMux, dev *payment.DevLog) {
	simulate := func(outcome string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := uuid.Parse(r.PathValue("id"))
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrNotFound)
				return
			}
			var providerID *string
			var amount int64
			if err := s.pool.QueryRow(r.Context(), `SELECT provider_intent_id, amount_minor FROM payment_intents WHERE id = $1`, id).Scan(&providerID, &amount); err != nil || providerID == nil {
				httpx.WriteError(w, r, httpx.ErrNotFound)
				return
			}
			obj := map[string]any{"id": *providerID, "object": "payment_intent", "amount": amount, "currency": "eur", "status": "succeeded"}
			typ := "payment_intent.succeeded"
			if outcome == "fail" {
				obj["status"] = "requires_payment_method"
				obj["last_payment_error"] = map[string]string{"code": "card_declined", "decline_code": "insufficient_funds"}
				typ = "payment_intent.payment_failed"
			}
			payload, _ := json.Marshal(map[string]any{"id": "evt_dev_" + uuid.NewString(), "type": typ, "data": map[string]any{"object": obj}})
			if err := s.HandleWebhook(r.Context(), payload, dev.SignDev(payload, time.Now())); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}
	mux.HandleFunc("POST /api/v1/dev/payments/{id}/succeed", simulate("succeed"))
	mux.HandleFunc("POST /api/v1/dev/payments/{id}/fail", simulate("fail"))
}

func (s *Service) handleTopUp(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in TopUpInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	intent, err := s.CreateTopUp(r.Context(), a, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, intent)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	intent, err := s.Get(r.Context(), a, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (s *Service) handleRefund(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	var in struct {
		AmountMinor int64  `json:"amount_minor"`
		Reason      string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refundID, err := s.Refund(r.Context(), a, id, in.AmountMinor, in.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"refund_id": refundID})
}

func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrValidation.With(err))
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Signature")
	}
	if err := s.HandleWebhook(r.Context(), payload, sig); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
