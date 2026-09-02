package dataexport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

func TestDataExportFlow(t *testing.T) {
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
	fs, err := storage.NewDevFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	svc := New(pool, fs)
	svc.now = func() time.Time { return now }

	var userID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (phone_e164, locale, full_name) VALUES ($1, 'sq', 'Arta Krasniqi') RETURNING id`,
		"+38346"+uuid.NewString()[:6]).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	if _, err := pool.Exec(ctx, `
		INSERT INTO user_addresses (user_id, label, line1, city, lat, lng)
		VALUES ($1, 'home', 'Rruga B 12', 'Prishtinë', 42.66, 21.16)`, userID); err != nil {
		t.Fatal(err)
	}

	actor := principal.Actor{UserID: userID}

	// Pa asnjë kërkesë, gjendja nuk ekziston.
	if _, err := svc.Latest(ctx, userID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("pritej ErrNotFound, u kthye %v", err)
	}

	first, err := svc.Request(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "pending" || first.DownloadURL != "" {
		t.Fatalf("kërkesa e re duhet 'pending' pa lidhje: %+v", first)
	}

	// Një kërkesë e dytë sa është e para e hapur refuzohet.
	if _, err := svc.Request(ctx, actor); !errors.Is(err, ErrInProgress) {
		t.Fatalf("pritej ErrInProgress, u kthye %v", err)
	}

	did, err := svc.BuildNext(ctx)
	if err != nil || !did {
		t.Fatalf("BuildNext: did=%v err=%v", did, err)
	}
	// Radha bosh nuk është gabim.
	if did, err := svc.BuildNext(ctx); did || err != nil {
		t.Fatalf("radha duhet bosh: did=%v err=%v", did, err)
	}

	ready, err := svc.Latest(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "ready" || ready.DownloadURL == "" {
		t.Fatalf("eksporti duhet gati me lidhje: %+v", ready)
	}
	if ready.SizeBytes == nil || *ready.SizeBytes == 0 {
		t.Fatal("madhësia duhet e njohur")
	}
	if ready.ExpiresAt == nil || !ready.ExpiresAt.Equal(now.Add(RetentionPeriod)) {
		t.Fatalf("skadimi duhet %v, është %v", now.Add(RetentionPeriod), ready.ExpiresAt)
	}

	// Përmbajtja: profili dhe adresa e përdoruesit, dhe asgjë e askujt tjetër.
	var key string
	if err := pool.QueryRow(ctx, `SELECT object_key FROM data_exports WHERE id = $1`, ready.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	body, ctype, err := fs.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(body)
	body.Close()
	if ctype != "application/json" {
		t.Fatalf("lloji i skedarit: %s", ctype)
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatalf("skedari duhet JSON i vlefshëm: %v", err)
	}
	if bundle.Profile["full_name"] != "Arta Krasniqi" {
		t.Fatalf("profili mungon: %+v", bundle.Profile)
	}
	if len(bundle.Addresses) != 1 || bundle.Addresses[0]["line1"] != "Rruga B 12" {
		t.Fatalf("adresat: %+v", bundle.Addresses)
	}
	// Listat bosh dalin si vargje, jo si null: skedari lexohet pa raste të veçanta.
	if bundle.Rides == nil || bundle.Orders == nil || bundle.Support == nil {
		t.Fatal("listat bosh duhet vargje bosh, jo null")
	}

	// Brenda 24 orëve nuk lejohet një eksport i dytë.
	if _, err := svc.Request(ctx, actor); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("pritej ErrTooSoon, u kthye %v", err)
	}

	// Një ditë më vonë lejohet.
	svc.now = func() time.Time { return now.Add(MinInterval + time.Minute) }
	if _, err := svc.Request(ctx, actor); err != nil {
		t.Fatalf("pas 24 orëve duhet lejuar: %v", err)
	}
	// Kërkesa e re është e hapur; e mbyllim që të mos pengojë pjesën tjetër.
	if _, err := svc.BuildNext(ctx); err != nil {
		t.Fatal(err)
	}

	// Pas skadimit skedari fshihet dhe gjendja bëhet 'expired'.
	svc.now = func() time.Time { return now.Add(RetentionPeriod + 2*time.Hour) }
	if _, err := pool.Exec(ctx,
		`UPDATE data_exports SET expires_at = now() - interval '1 hour' WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	n, err := svc.ExpireOld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("duhej fshirë të paktën një skedar")
	}
	if _, _, err := fs.Open(key); err == nil {
		t.Fatal("skedari i skaduar duhet fshirë nga magazina")
	}
	after, err := svc.Latest(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "expired" || after.DownloadURL != "" {
		t.Fatalf("pas skadimit: %+v", after)
	}
}

func TestFailedRequestCanBeRetriedImmediately(t *testing.T) {
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
	fs, err := storage.NewDevFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(pool, fs)

	var userID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`,
		"+38346"+uuid.NewString()[:6]).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })

	if _, err := svc.Request(ctx, principal.Actor{UserID: userID}); err != nil {
		t.Fatal(err)
	}
	// Një dështim nuk është faji i përdoruesit, ndaj nuk e bllokon për 24 orë.
	if _, err := pool.Exec(ctx,
		`UPDATE data_exports SET status = 'failed', completed_at = now() WHERE user_id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Request(ctx, principal.Actor{UserID: userID}); err != nil {
		t.Fatalf("pas dështimit duhet lejuar menjëherë: %v", err)
	}
}
