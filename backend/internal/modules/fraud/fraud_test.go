package fraud

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/principal"
)

func TestFraudRulesAndBlock(t *testing.T) {
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
	svc := New(pool, nil)
	if raddr := os.Getenv("TEST_REDIS_ADDR"); raddr != "" {
		rdb, err := cache.Connect(ctx, config.Redis{Host: raddr})
		if err != nil {
			t.Fatal(err)
		}
		defer rdb.Close()
		svc.rdb = rdb
	}
	newUser := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38338"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	customer, ops := newUser(), newUser()
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (user_id, device_id, platform, refresh_token_hash, refresh_expires_at) VALUES ($1, 'd', 'android', '\x00', now() + interval '30 days')`, customer); err != nil {
		t.Fatal(err)
	}

	// kufiri i shpejtësisë (kur ka Redis)
	if svc.rdb != nil {
		for i := 0; i < 3; i++ {
			if err := svc.Allow(ctx, customer, "test_action", 3, time.Minute); err != nil {
				t.Fatalf("allow %d: %v", i, err)
			}
		}
		if err := svc.Allow(ctx, customer, "test_action", 3, time.Minute); !errors.Is(err, ErrVelocity) {
			t.Fatalf("i 4-ti: %v", err)
		}
	}

	// anulime në seri të klientit: 5 udhëtime të anuluara në 24 h → flag low; ridorëzim i ngjarjes → pa dublikatë
	var ruleID, quoteID uuid.UUID
	pool.QueryRow(ctx, `SELECT id FROM pricing_rules WHERE service_area_id='prishtina' AND category_id='economy' LIMIT 1`).Scan(&ruleID)
	if err := pool.QueryRow(ctx, `INSERT INTO ride_quotes (customer_id, service_area_id, category_id, pricing_rule_id, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_minor, currency, surge_bp, expires_at)
		VALUES ($1,'prishtina','economy',$2,42.66,21.16,42.67,21.17,3000,600,300,'EUR',10000, now() + interval '2 minutes') RETURNING id`, customer, ruleID).Scan(&quoteID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM rides WHERE customer_id = $1`, customer)
		pool.Exec(context.Background(), `DELETE FROM ride_quotes WHERE id = $1`, quoteID)
	})
	for i := 0; i < 5; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO rides (customer_id, quote_id, service_area_id, category_id, state, payment_method, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_quoted_minor, currency, idempotency_key, cancelled_by, cancelled_at)
			VALUES ($1,$2,'prishtina','economy','cancelled','cash',42.66,21.16,42.67,21.17,3000,600,300,'EUR',$3,'customer', now())`, customer, quoteID, "f-"+uuid.NewString()); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]any{"ride_id": uuid.New(), "by": "customer", "customer_id": customer})
	ev := events.Event{ID: uuid.New(), EventType: "RideCancelled", OccurredAt: time.Now(), Payload: raw}
	if err := svc.Handle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	flags, err := svc.Flags(ctx, "open", &customer, 10)
	if err != nil || len(flags) != 1 || flags[0].Kind != "customer_cancel_burst" || flags[0].Severity != "low" {
		t.Fatalf("flags: %+v err=%v", flags, err)
	}
	// zgjidhja nga ops
	f, err := svc.Resolve(ctx, principal.Actor{UserID: ops}, flags[0].ID, "confirmed", "kontaktuar")
	if err != nil || f.Status != "confirmed" || f.ResolvedAt == nil {
		t.Fatalf("resolve: %+v err=%v", f, err)
	}
	if _, err := svc.Resolve(ctx, principal.Actor{UserID: ops}, flags[0].ID, "banana", ""); err == nil {
		t.Fatal("status i pavlefshëm")
	}
	// bllokimi: statusi + sesionet e shkyçura; zhbllokimi
	if err := svc.Block(ctx, principal.Actor{UserID: ops}, customer, "", true); err == nil {
		t.Fatal("bllokim pa arsye")
	}
	if err := svc.Block(ctx, principal.Actor{UserID: ops}, customer, "abuzim me anulime", true); err != nil {
		t.Fatal(err)
	}
	var status string
	var active int
	pool.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, customer).Scan(&status)
	pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, customer).Scan(&active)
	if status != "blocked" || active != 0 {
		t.Fatalf("pas bllokimit: status=%s sesione=%d", status, active)
	}
	if err := svc.Block(ctx, principal.Actor{UserID: ops}, customer, "", false); err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, customer).Scan(&status)
	if status != "active" {
		t.Fatalf("pas zhbllokimit: %s", status)
	}
}
