package appconfig

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0}, {"1.2.3", "1.10.0", -1}, {"2.0.0", "1.99.99", 1}, {"v1.2", "1.2.0", 0},
		{"1.2.3-beta+7", "1.2.3", 0}, {"", "0.0.0", 0}, {"1", "0.9.9", 1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestBucketDeterministic(t *testing.T) {
	u := uuid.New()
	first := bucket("x", u)
	if first < 0 || first > 99 {
		t.Fatalf("bucket jashtë 0..99: %d", first)
	}
	for i := 0; i < 10; i++ {
		if bucket("x", u) != first {
			t.Fatal("bucket jo determinist")
		}
	}
	if bucket("y", u) == first && bucket("z", u) == first && bucket("w", u) == first {
		t.Fatal("çelësa të ndryshëm dhanë të njëjtin bucket katër herë — dyshohet hash i dobët")
	}
	// shpërndarje afërsisht uniforme mbi shumë përdorues
	hits := 0
	for i := 0; i < 2000; i++ {
		if bucket("rides.surge_dynamic", uuid.New()) < 25 {
			hits++
		}
	}
	if hits < 380 || hits > 620 {
		t.Fatalf("25%% rollout mbi 2000 përdorues: %d", hits)
	}
}

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestConfigGateAndFlags(t *testing.T) {
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
	s := New(pool)
	var adminID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38341"+uuid.NewString()[:6]).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	admin := principal.Actor{UserID: adminID}
	defer func() {
		_, _ = s.SetVersion(context.Background(), admin, "customer", "android", VersionUpdate{MinVersion: strPtr("0.0.0"), RecommendedVersion: strPtr("0.0.0"), Maintenance: boolPtr(false)})
		_, _ = s.SetFlag(context.Background(), admin, "rides.surge_dynamic", FlagUpdate{Enabled: boolPtr(false), RolloutPercent: intPtr(100)})
		pool.Exec(context.Background(), `UPDATE app_versions SET updated_by = NULL WHERE updated_by = $1`, adminID)
		pool.Exec(context.Background(), `UPDATE feature_flags SET updated_by = NULL WHERE updated_by = $1`, adminID)
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, adminID)
	}()

	// porta: pa header-a kalon; version i vjetër → UPDATE_REQUIRED; mirëmbajtje → MAINTENANCE
	if _, err := s.SetVersion(ctx, admin, "customer", "android", VersionUpdate{MinVersion: strPtr("1.4.0"), RecommendedVersion: strPtr("1.6.0")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVersion(ctx, admin, "customer", "android", VersionUpdate{MinVersion: strPtr("abc")}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("version i pavlefshëm: %v", err)
	}
	gate := s.Gate()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	call := func(app, platform, version, path string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if app != "" {
			req.Header.Set("X-App-Id", app)
			req.Header.Set("X-App-Platform", platform)
			req.Header.Set("X-App-Version", version)
		}
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := call("", "", "", "/api/v1/rides"); c != http.StatusNoContent {
		t.Fatalf("pa header-a: %d", c)
	}
	if c := call("customer", "android", "1.3.9", "/api/v1/rides"); c != http.StatusUpgradeRequired {
		t.Fatalf("version i vjetër: %d", c)
	}
	if c := call("customer", "android", "1.5.0", "/api/v1/rides"); c != http.StatusNoContent {
		t.Fatalf("version ok: %d", c)
	}
	if c := call("customer", "android", "1.3.9", "/api/v1/config"); c != http.StatusNoContent {
		t.Fatalf("/config kalon gjithmonë: %d", c)
	}
	pub, err := s.PublicConfig(ctx, ClientInfo{App: "customer", Platform: "android", Version: "1.5.0"}, uuid.Nil)
	if err != nil || pub.UpdateState != "recommended" || pub.App == nil || pub.App.MinVersion != "1.4.0" {
		t.Fatalf("public: %+v err=%v", pub, err)
	}
	if !pub.Flags["rides.request"] || !pub.Flags["wallet.topup"] {
		t.Fatalf("flag-et publike: %+v", pub.Flags)
	}
	if _, ok := pub.Flags["rides.surge_dynamic"]; ok {
		t.Fatal("flag jo-publik u ekspozua")
	}
	if _, err := s.SetVersion(ctx, admin, "customer", "android", VersionUpdate{Maintenance: boolPtr(true), MaintenanceMessage: strPtr("maintenance.rides_upgrade")}); err != nil {
		t.Fatal(err)
	}
	if c := call("customer", "android", "9.9.9", "/api/v1/rides"); c != http.StatusServiceUnavailable {
		t.Fatalf("mirëmbajtje: %d", c)
	}

	// flag-et: i panjohur → false; rollout 0 → false; 100 → true; 50 → determinist
	uid := uuid.New()
	if s.Enabled(ctx, "nuk.ekziston", uid) {
		t.Fatal("flag i panjohur duhej false")
	}
	if s.Enabled(ctx, "rides.surge_dynamic", uid) {
		t.Fatal("surge është off")
	}
	if _, err := s.SetFlag(ctx, admin, "rides.surge_dynamic", FlagUpdate{Enabled: boolPtr(true), RolloutPercent: intPtr(0)}); err != nil {
		t.Fatal(err)
	}
	if s.Enabled(ctx, "rides.surge_dynamic", uid) {
		t.Fatal("rollout 0%")
	}
	if _, err := s.SetFlag(ctx, admin, "rides.surge_dynamic", FlagUpdate{RolloutPercent: intPtr(100)}); err != nil {
		t.Fatal(err)
	}
	if !s.Enabled(ctx, "rides.surge_dynamic", uid) {
		t.Fatal("rollout 100%")
	}
	if _, err := s.SetFlag(ctx, admin, "rides.surge_dynamic", FlagUpdate{RolloutPercent: intPtr(50)}); err != nil {
		t.Fatal(err)
	}
	first := s.Enabled(ctx, "rides.surge_dynamic", uid)
	for i := 0; i < 5; i++ {
		if s.Enabled(ctx, "rides.surge_dynamic", uid) != first {
			t.Fatal("rollout jo determinist")
		}
	}
	if _, err := s.SetFlag(ctx, admin, "rides.surge_dynamic", FlagUpdate{RolloutPercent: intPtr(101)}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("101%%: %v", err)
	}
	if _, err := s.SetFlag(ctx, admin, "nuk.ekziston", FlagUpdate{Enabled: boolPtr(true)}); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("flag i panjohur: %v", err)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
