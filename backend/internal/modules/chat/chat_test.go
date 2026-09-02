package chat

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func TestChatFlow(t *testing.T) {
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
	svc := New(pool)
	newUser := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, full_name, locale) VALUES ($1, 'Test', 'sq') RETURNING id`, "+38339"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	customer, driver, stranger := newUser(), newUser(), newUser()
	if _, err := pool.Exec(ctx, `INSERT INTO drivers (user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
		VALUES ($1, 'approved', 'Ford', 'Focus', '04-555-CC', 'blu', '{economy}')`, driver); err != nil {
		t.Fatal(err)
	}
	var ruleID, quoteID, rideID uuid.UUID
	pool.QueryRow(ctx, `SELECT id FROM pricing_rules WHERE service_area_id='prishtina' AND category_id='economy' LIMIT 1`).Scan(&ruleID)
	if err := pool.QueryRow(ctx, `INSERT INTO ride_quotes (customer_id, service_area_id, category_id, pricing_rule_id, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_minor, currency, surge_bp, expires_at)
		VALUES ($1,'prishtina','economy',$2,42.66,21.16,42.67,21.17,3000,600,300,'EUR',10000, now() + interval '2 minutes') RETURNING id`, customer, ruleID).Scan(&quoteID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO rides (customer_id, driver_id, quote_id, service_area_id, category_id, state, payment_method,
		pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_quoted_minor, currency, idempotency_key, assigned_at)
		VALUES ($1,NULL,$2,'prishtina','economy','matching','cash',42.66,21.16,42.67,21.17,3000,600,300,'EUR',$3, NULL) RETURNING id`,
		customer, quoteID, "chat-"+uuid.NewString()).Scan(&rideID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM rides WHERE id = $1`, rideID)
		pool.Exec(context.Background(), `DELETE FROM ride_quotes WHERE id = $1`, quoteID)
	})
	c, d, x := principal.Actor{UserID: customer}, principal.Actor{UserID: driver}, principal.Actor{UserID: stranger}

	// pa shofer → mbyllur; i huaji → NOT_FOUND
	if _, err := svc.Send(ctx, c, rideID, "Ku je?"); !errors.Is(err, ErrChatClosed) {
		t.Fatalf("pa shofer: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rides SET driver_id = $2, state = 'assigned', assigned_at = now() WHERE id = $1`, rideID, driver); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(ctx, x, rideID, "hej"); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("i huaji: %v", err)
	}
	if _, err := svc.Send(ctx, c, rideID, "   "); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("bosh: %v", err)
	}
	m1, err := svc.Send(ctx, c, rideID, "Jam te hyrja kryesore")
	if err != nil || m1.SenderRole != "customer" || !m1.Mine {
		t.Fatalf("send: %+v err=%v", m1, err)
	}
	m2, err := svc.Send(ctx, d, rideID, "Po vij, 2 min")
	if err != nil || m2.SenderRole != "driver" {
		t.Fatalf("send driver: %+v err=%v", m2, err)
	}
	// ngjarja mbart marrësin dhe parapamjen
	var recipient string
	pool.QueryRow(ctx, `SELECT payload->>'recipient_id' FROM outbox_events WHERE event_type = 'RideChatMessage' AND payload->>'message_id' = $1`, m2.ID.String()).Scan(&recipient)
	if recipient != customer.String() {
		t.Fatalf("marrësi: %s", recipient)
	}
	// klienti lexon: 1 i palexuar → pas listimit 0; shoferi sheh mesazhin e tij si të lexuar
	if n, _ := svc.Unread(ctx, c, rideID); n != 1 {
		t.Fatalf("unread para: %d", n)
	}
	list, err := svc.List(ctx, c, rideID, nil, 0)
	if err != nil || len(list) != 2 || !list[0].Mine || list[1].Mine {
		t.Fatalf("lista: %+v err=%v", list, err)
	}
	if n, _ := svc.Unread(ctx, c, rideID); n != 0 {
		t.Fatalf("unread pas: %d", n)
	}
	dl, _ := svc.List(ctx, d, rideID, nil, 0)
	if dl[1].ReadAt == nil {
		t.Fatal("mesazhi i shoferit duhej i lexuar")
	}
	after := m1.CreatedAt
	if tail, _ := svc.List(ctx, c, rideID, &after, 0); len(tail) != 1 || tail[0].ID != m2.ID {
		t.Fatalf("after: %+v", tail)
	}
	// pas përfundimit: brenda 24 h hapur, pas 24 h mbyllur
	if _, err := pool.Exec(ctx, `UPDATE rides SET state = 'completed', completed_at = now() WHERE id = $1`, rideID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Send(ctx, d, rideID, "Faleminderit!"); err != nil {
		t.Fatalf("brenda 24 h: %v", err)
	}
	svc.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if _, err := svc.Send(ctx, c, rideID, "…"); !errors.Is(err, ErrChatClosed) {
		t.Fatalf("pas 24 h: %v", err)
	}
	// retention: 90 ditë
	svc.now = func() time.Time { return time.Now().Add(91 * 24 * time.Hour) }
	n, err := svc.RetentionSweep(ctx)
	if err != nil || n < 3 {
		t.Fatalf("retention: n=%d err=%v", n, err)
	}
}
