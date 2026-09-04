package payment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// Pa ofrues pagese: aplikacioni punon me cash dhe me kuletë, kurse mbushja me kartë refuzohet
// hapur. Kjo lejohet edhe në prodhim — mospasja e kartës nuk është shtirje.
func TestNoneProviderAllowedInProduction(t *testing.T) {
	p, err := NewFromEnv("production", "none", "", "", quietLog())
	if err != nil {
		t.Fatalf("none duhet lejuar në prodhim: %v", err)
	}
	if p.Name() != "none" {
		t.Fatalf("emri: %s", p.Name())
	}
	if _, err := p.CreateIntent(context.Background(), IntentRequest{AmountMinor: 1000}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("mbushja duhet refuzuar: %v", err)
	}
	if _, err := p.Refund(context.Background(), RefundRequest{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("rimbursimi duhet refuzuar: %v", err)
	}
	if _, err := p.ParseWebhook(nil, "", time.Now()); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("webhook-u duhet refuzuar: %v", err)
	}
}

func TestDevlogStillRefusedInProduction(t *testing.T) {
	if _, err := NewFromEnv("production", "devlog", "", "", quietLog()); err == nil {
		t.Fatal("devlog nuk lejohet në prodhim")
	}
	if _, err := NewFromEnv("staging", "devlog", "", "", quietLog()); err != nil {
		t.Fatalf("devlog lejohet në staging: %v", err)
	}
}
