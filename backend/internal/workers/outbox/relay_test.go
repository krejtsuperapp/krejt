package outbox

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/logx"
)

func TestBackoff(t *testing.T) {
	cases := map[int]time.Duration{0: 2 * time.Second, 1: 2 * time.Second, 3: 8 * time.Second, 9: 512 * time.Second, 10: 10 * time.Minute, 40: 10 * time.Minute}
	for attempt, want := range cases {
		if got := Backoff(attempt); got != want {
			t.Errorf("Backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

// fakePublisher dështon për event_type-et në fail; regjistron vetëm ngjarjet e agregatit të testit
// (paketat e tjera të testeve mund të shkruajnë në outbox paralelisht kundër së njëjtës DB).
type fakePublisher struct {
	agg       string
	fail      map[string]bool
	published []string
}

func (f *fakePublisher) Publish(_ context.Context, ev events.Event) error {
	if ev.AggregateID != f.agg {
		return nil
	}
	if f.fail[ev.EventType] {
		return errors.New("sns down")
	}
	f.published = append(f.published, ev.EventType)
	return nil
}

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestRelayPublishesRetriesAndKeepsAggregateOrder(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	agg := "test-" + uuid.NewString()
	defer pool.Exec(context.Background(), `DELETE FROM outbox_events WHERE aggregate_id = $1`, agg)

	for _, typ := range []string{"First", "Second", "Third"} {
		if err := events.Emit(ctx, pool, "test", agg, typ, map[string]string{"k": typ}); err != nil {
			t.Fatal(err)
		}
	}
	state := func(typ string) (published bool, attempts int, lastErr *string) {
		var p *time.Time
		if err := pool.QueryRow(ctx, `SELECT published_at, attempts, last_error FROM outbox_events WHERE aggregate_id=$1 AND event_type=$2`, agg, typ).
			Scan(&p, &attempts, &lastErr); err != nil {
			t.Fatal(err)
		}
		return p != nil, attempts, lastErr
	}

	pub := &fakePublisher{agg: agg, fail: map[string]bool{"Second": true}}
	r := New(pool, pub, logx.New("test", "development"))

	// 1) First publikohet; Second dështon → Third (i njëjti agregat) NUK preket
	if _, err := r.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if ok, _, _ := state("First"); !ok {
		t.Fatal("First duhej publikuar")
	}
	if ok, att, le := state("Second"); ok || att != 1 || le == nil || *le != "sns down" {
		t.Fatalf("Second: published=%v attempts=%d last_error=%v", ok, att, le)
	}
	if ok, att, _ := state("Third"); ok || att != 0 {
		t.Fatalf("Third u prek megjithëse Second dështoi (published=%v attempts=%d)", ok, att)
	}

	// 2) gjatë backoff-it Second nuk riprovohet
	if _, err := r.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if _, att, _ := state("Second"); att != 1 {
		t.Fatalf("Second u riprovua para kohe (attempts=%d)", att)
	}

	// 3) SNS "rikthehet": kalojmë kohën → Second dhe Third publikohen me radhë
	pub.fail = nil
	if _, err := pool.Exec(ctx, `UPDATE outbox_events SET next_attempt_at = now() WHERE aggregate_id=$1`, agg); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	got := pub.published
	if len(got) != 3 || got[0] != "First" || got[1] != "Second" || got[2] != "Third" {
		t.Fatalf("renditja = %v", got)
	}
	var unpublished int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id=$1 AND published_at IS NULL`, agg).Scan(&unpublished); err != nil {
		t.Fatal(err)
	}
	if unpublished != 0 {
		t.Fatalf("%d ngjarje mbetën të papublikuara", unpublished)
	}
	if _, _, le := state("Second"); le != nil {
		t.Fatalf("last_error duhej pastruar pas suksesit: %v", *le)
	}
}
