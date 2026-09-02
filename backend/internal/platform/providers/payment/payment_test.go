package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func sign(secret string, ts time.Time, payload []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t + "."))
	mac.Write(payload)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestStripeCreateIntentAndRefund(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk_test_x" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/v1/payment_intents":
			if r.Form.Get("amount") != "1500" || r.Form.Get("currency") != "eur" || r.Form.Get("automatic_payment_methods[enabled]") != "true" ||
				r.Form.Get("metadata[intent_id]") != "abc" || r.Header.Get("Idempotency-Key") != "topup:abc" {
				http.Error(w, "form", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pi_123", "client_secret": "pi_123_secret", "status": "requires_payment_method"})
		case "/v1/refunds":
			if r.Form.Get("payment_intent") != "pi_123" || r.Form.Get("amount") != "500" {
				http.Error(w, "form", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "re_1", "status": "pending"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s, err := NewStripe("sk_test_x", "whsec_x")
	if err != nil {
		t.Fatal(err)
	}
	s.base = srv.URL
	res, err := s.CreateIntent(context.Background(), IntentRequest{AmountMinor: 1500, Currency: "EUR", IdempotencyKey: "topup:abc", Metadata: map[string]string{"intent_id": "abc"}})
	if err != nil || res.ProviderIntentID != "pi_123" || res.ClientSecret != "pi_123_secret" || res.Status != "created" {
		t.Fatalf("intent: %+v err=%v", res, err)
	}
	ref, err := s.Refund(context.Background(), RefundRequest{ProviderIntentID: "pi_123", AmountMinor: 500, IdempotencyKey: "refund:1"})
	if err != nil || ref.ProviderRefundID != "re_1" || ref.Status != "pending" {
		t.Fatalf("refund: %+v err=%v", ref, err)
	}
}

func TestStripeWebhookSignature(t *testing.T) {
	s, _ := NewStripe("sk", "whsec_test")
	now := time.Now()
	payload := []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"object":{"id":"pi_9","object":"payment_intent","status":"succeeded","amount":2000,"currency":"eur"}}}`)
	ev, err := s.ParseWebhook(payload, sign("whsec_test", now, payload), now)
	if err != nil || ev.ID != "evt_1" || ev.ProviderIntentID != "pi_9" || ev.Status != "succeeded" || ev.AmountMinor != 2000 || ev.Currency != "EUR" {
		t.Fatalf("parse: %+v err=%v", ev, err)
	}
	if _, err := s.ParseWebhook(payload, sign("wrong", now, payload), now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("sekret i gabuar: %v", err)
	}
	if _, err := s.ParseWebhook(payload, sign("whsec_test", now.Add(-10*time.Minute), payload), now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("i vjetër (replay): %v", err)
	}
	tampered := append([]byte{}, payload...)
	tampered[len(tampered)-3] = '9'
	if _, err := s.ParseWebhook(tampered, sign("whsec_test", now, payload), now); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("payload i ndryshuar: %v", err)
	}
	failed := []byte(`{"id":"evt_2","type":"payment_intent.payment_failed","data":{"object":{"id":"pi_9","object":"payment_intent","status":"requires_payment_method","amount":2000,"currency":"eur","last_payment_error":{"code":"card_declined","decline_code":"insufficient_funds"}}}}`)
	ev, err = s.ParseWebhook(failed, sign("whsec_test", now, failed), now)
	if err != nil || ev.FailureCode != "insufficient_funds" || ev.Status != "created" {
		t.Fatalf("failed: %+v err=%v", ev, err)
	}
	if _, err := NewFromEnv("production", "devlog", "", "", nil); err == nil {
		t.Fatal("devlog në production duhej refuzuar")
	}
}
