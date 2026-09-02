package payments

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/wallet"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/payment"
)

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestTopUpWebhookAndRefund(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	prov, err := payment.NewFromEnv("development", "devlog", "", "", logx.New("test", "development"))
	if err != nil {
		t.Fatal(err)
	}
	dev := prov.(*payment.DevLog)
	led := ledger.New(pool)
	svc := New(pool, led, prov)
	wal := wallet.New(pool, led, wallet.Limits{MinTopUpMinor: MinTopUpMinor, MaxTopUpMinor: MaxTopUpMinor, DailyTopUpMinor: DailyTopUpMinor})

	var uid, finID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38343"+uuid.NewString()[:6]).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38342"+uuid.NewString()[:6]).Scan(&finID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Exec(context.Background(), `DELETE FROM payment_webhook_events WHERE intent_id IN (SELECT id FROM payment_intents WHERE user_id = $1)`, uid)
		pool.Exec(context.Background(), `DELETE FROM payment_refunds WHERE intent_id IN (SELECT id FROM payment_intents WHERE user_id = $1)`, uid)
		pool.Exec(context.Background(), `DELETE FROM payment_intents WHERE user_id = $1`, uid)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, uid, finID)
	}()
	a := principal.Actor{UserID: uid, IP: "203.0.113.5"}
	fin := principal.Actor{UserID: finID}

	// shuma: nën minimum / mbi maksimum / jo shumëfish i 0,50 €
	for _, bad := range []int64{50, 50001, 1234} {
		if _, err := svc.CreateTopUp(ctx, a, "k-"+uuid.NewString(), TopUpInput{AmountMinor: bad}); !errors.Is(err, ErrAmount) {
			t.Fatalf("shuma %d: %v", bad, err)
		}
	}
	idem := "topup-" + uuid.NewString()
	intent, err := svc.CreateTopUp(ctx, a, idem, TopUpInput{AmountMinor: 2000})
	if err != nil || intent.Status != "created" || intent.ClientSecret == "" || intent.Provider != "devlog" {
		t.Fatalf("intent: %+v err=%v", intent, err)
	}
	again, err := svc.CreateTopUp(ctx, a, idem, TopUpInput{AmountMinor: 2000})
	if err != nil || again.ID != intent.ID || again.ClientSecret != "" {
		t.Fatalf("idempotencë: %+v err=%v", again, err)
	}
	if _, err := svc.CreateTopUp(ctx, a, idem, TopUpInput{AmountMinor: 3000}); !errors.Is(err, httpx.ErrIdempotency) {
		t.Fatalf("i njëjti çelës, shumë tjetër: %v", err)
	}
	// bilanci s'ndryshon para webhook-ut
	if ov, _ := wal.Overview(ctx, a); ov.BalanceMinor != 0 || !ov.ClosedLoop {
		t.Fatalf("bilanci para webhook-ut: %+v", ov)
	}

	// webhook: nënshkrim i gabuar → refuzohet; i saktë → kreditohet; dublikatë → asgjë
	var providerID string
	pool.QueryRow(ctx, `SELECT provider_intent_id FROM payment_intents WHERE id = $1`, intent.ID).Scan(&providerID)
	payload, _ := json.Marshal(map[string]any{"id": "evt_" + uuid.NewString(), "type": "payment_intent.succeeded",
		"data": map[string]any{"object": map[string]any{"id": providerID, "object": "payment_intent", "status": "succeeded", "amount": 2000, "currency": "eur"}}})
	if err := svc.HandleWebhook(ctx, payload, "t=1,v1=deadbeef"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("nënshkrim i gabuar: %v", err)
	}
	if err := svc.HandleWebhook(ctx, payload, dev.SignDev(payload, time.Now())); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleWebhook(ctx, payload, dev.SignDev(payload, time.Now())); err != nil {
		t.Fatal(err)
	}
	ov, _ := wal.Overview(ctx, a)
	if ov.BalanceMinor != 2000 {
		t.Fatalf("bilanci pas webhook-ut (dhe dublikatës) = %d, pritej 2000", ov.BalanceMinor)
	}
	got, _ := svc.Get(ctx, a, intent.ID)
	if got.Status != "succeeded" || got.SucceededAt == nil {
		t.Fatalf("statusi: %+v", got)
	}
	// shumë tjetër nga ofruesi → nuk kreditohet
	i2, _ := svc.CreateTopUp(ctx, a, "k2-"+uuid.NewString(), TopUpInput{AmountMinor: 1000})
	pool.QueryRow(ctx, `SELECT provider_intent_id FROM payment_intents WHERE id = $1`, i2.ID).Scan(&providerID)
	bad, _ := json.Marshal(map[string]any{"id": "evt_" + uuid.NewString(), "type": "payment_intent.succeeded",
		"data": map[string]any{"object": map[string]any{"id": providerID, "object": "payment_intent", "status": "succeeded", "amount": 999, "currency": "eur"}}})
	if err := svc.HandleWebhook(ctx, bad, dev.SignDev(bad, time.Now())); err != nil {
		t.Fatal(err)
	}
	if g, _ := svc.Get(ctx, a, i2.ID); g.Status != "failed" || g.FailureCode == nil || *g.FailureCode != "amount_mismatch" {
		t.Fatalf("mospërputhje shume: %+v", g)
	}
	if ov, _ = wal.Overview(ctx, a); ov.BalanceMinor != 2000 {
		t.Fatalf("bilanci pas mospërputhjes = %d", ov.BalanceMinor)
	}
	// limiti ditor: 2000 + 1000 (failed nuk numërohet) … 100000 total
	if _, err := svc.CreateTopUp(ctx, a, "k3-"+uuid.NewString(), TopUpInput{AmountMinor: 50000}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateTopUp(ctx, a, "k4-"+uuid.NewString(), TopUpInput{AmountMinor: 50000}); !errors.Is(err, ErrDailyLimit) {
		t.Fatalf("limiti ditor: %v", err)
	}
	// rimbursim: mbi shumën → jo; 500 → wallet-i bie, refund succeeded (devlog)
	if _, err := svc.Refund(ctx, fin, intent.ID, 2500, "gabim"); !errors.Is(err, ErrNotRefundable) {
		t.Fatalf("mbi shumën: %v", err)
	}
	rid, err := svc.Refund(ctx, fin, intent.ID, 500, "kërkesë e klientit")
	if err != nil || rid == uuid.Nil {
		t.Fatalf("refund: %v", err)
	}
	if ov, _ = wal.Overview(ctx, a); ov.BalanceMinor != 1500 {
		t.Fatalf("bilanci pas rimbursimit = %d, pritej 1500", ov.BalanceMinor)
	}
	txs, err := wal.Transactions(ctx, a, nil, 10)
	if err != nil || len(txs) != 2 || txs[0].Kind != "wallet_topup_refund" || txs[0].AmountMinor != -500 || txs[1].AmountMinor != 2000 {
		t.Fatalf("historiku: %+v err=%v", txs, err)
	}
}
