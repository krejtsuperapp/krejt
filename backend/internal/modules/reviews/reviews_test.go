package reviews

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

func TestValidate(t *testing.T) {
	in := Input{Rating: 5, Tags: []string{" Friendly ", "clean_car", "friendly", "late"}, Comment: "  shumë   mirë "}
	if f := validate(&in, CustomerTags); len(f) != 0 || len(in.Tags) != 3 || in.Comment != "shumë mirë" {
		t.Fatalf("validate: %v %+v", f, in)
	}
	bad := Input{Rating: 6, Tags: []string{"no_show"}, Comment: string(make([]rune, 301))}
	f := validate(&bad, CustomerTags)
	if f["rating"] != "invalid" || f["tags"] != "invalid" || f["comment"] != "too_long" {
		t.Fatalf("bad: %v", f)
	}
	many := Input{Rating: 1, Tags: []string{"polite", "on_time", "clear_pickup", "late", "rude", "no_show"}}
	if f := validate(&many, DriverTags); f["tags"] != "too_many" {
		t.Fatalf("many: %v", f)
	}
}

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestReviewFlow(t *testing.T) {
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
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38347"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	customer, driver, stranger := newUser(), newUser(), newUser()
	if _, err := pool.Exec(ctx, `INSERT INTO drivers (user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
		VALUES ($1, 'approved', 'VW', 'Golf', '02-999-ZZ', 'gri', '{economy}')`, driver); err != nil {
		t.Fatal(err)
	}
	var ruleID, quoteID, rideID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM pricing_rules WHERE service_area_id='prishtina' AND category_id='economy' LIMIT 1`).Scan(&ruleID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO ride_quotes (customer_id, service_area_id, category_id, pricing_rule_id, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_minor, currency, surge_bp, expires_at)
		VALUES ($1,'prishtina','economy',$2,42.66,21.16,42.67,21.17,3000,600,300,'EUR',10000, now() + interval '2 minutes') RETURNING id`, customer, ruleID).Scan(&quoteID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO rides (customer_id, driver_id, quote_id, service_area_id, category_id, state, payment_method, payment_status,
		pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, distance_m, duration_s, price_quoted_minor, price_final_minor, currency, idempotency_key, completed_at)
		VALUES ($1,$2,$3,'prishtina','economy','in_progress','cash','cash',42.66,21.16,42.67,21.17,3000,600,300,300,'EUR',$4, NULL) RETURNING id`,
		customer, driver, quoteID, "rev-"+uuid.NewString()).Scan(&rideID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM rides WHERE id = $1`, rideID)
		pool.Exec(context.Background(), `DELETE FROM ride_quotes WHERE id = $1`, quoteID)
	})
	c, d, x := principal.Actor{UserID: customer}, principal.Actor{UserID: driver}, principal.Actor{UserID: stranger}

	// para përfundimit: jo
	if _, err := svc.Create(ctx, c, rideID, Input{Rating: 5}); !errors.Is(err, ErrRideNotReviewable) {
		t.Fatalf("in_progress: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rides SET state = 'completed', completed_at = now() WHERE id = $1`, rideID); err != nil {
		t.Fatal(err)
	}
	// i huaji: NOT_FOUND (BOLA)
	if _, err := svc.Create(ctx, x, rideID, Input{Rating: 5}); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("i huaji: %v", err)
	}
	// klienti → shoferi (etiketë e shoferit refuzohet)
	if _, err := svc.Create(ctx, c, rideID, Input{Rating: 5, Tags: []string{"no_show"}}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("etiketë e gabuar: %v", err)
	}
	rev, err := svc.Create(ctx, c, rideID, Input{Rating: 4, Tags: []string{"friendly", "clean_car"}, Comment: "Faleminderit!"})
	if err != nil || rev.ReviewerRole != "customer" || rev.Rating != 4 {
		t.Fatalf("customer review: %+v err=%v", rev, err)
	}
	if _, err := svc.Create(ctx, c, rideID, Input{Rating: 1}); !errors.Is(err, ErrAlreadyReviewed) {
		t.Fatalf("dyfish: %v", err)
	}
	var sum, cnt int
	pool.QueryRow(ctx, `SELECT rating_sum, rating_count FROM drivers WHERE user_id = $1`, driver).Scan(&sum, &cnt)
	if sum != 4 || cnt != 1 {
		t.Fatalf("agregati i shoferit: %d/%d", sum, cnt)
	}
	// shoferi → klienti
	drev, err := svc.Create(ctx, d, rideID, Input{Rating: 2, Tags: []string{"late"}, Comment: "vonesë 10 min"})
	if err != nil || drev.ReviewerRole != "driver" {
		t.Fatalf("driver review: %+v err=%v", drev, err)
	}
	pool.QueryRow(ctx, `SELECT rating_sum, rating_count FROM users WHERE id = $1`, customer).Scan(&sum, &cnt)
	if sum != 2 || cnt != 1 {
		t.Fatalf("agregati i klientit: %d/%d", sum, cnt)
	}
	// klienti raporton vlerësimin e shoferit → fshihet nga pamja e tij, i vetë mbetet
	if err := svc.Report(ctx, c, drev.ID, "fyese"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Report(ctx, c, drev.ID, "sërish"); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("raport i dytë: %v", err)
	}
	list, err := svc.ForRide(ctx, c, rideID)
	if err != nil || len(list) != 1 || list[0].ID != rev.ID {
		t.Fatalf("lista e klientit: %+v err=%v", list, err)
	}
	if list, _ = svc.ForRide(ctx, d, rideID); len(list) != 2 {
		t.Fatalf("lista e shoferit: %+v", list)
	}
	// dritarja 7-ditore
	if _, err := pool.Exec(ctx, `UPDATE rides SET completed_at = now() - interval '8 days' WHERE id = $1`, rideID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM ride_reviews WHERE ride_id = $1 AND reviewer_id = $2`, rideID, driver); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, d, rideID, Input{Rating: 5}); !errors.Is(err, ErrRideNotReviewable) {
		t.Fatalf("pas 7 ditësh: %v", err)
	}
}
