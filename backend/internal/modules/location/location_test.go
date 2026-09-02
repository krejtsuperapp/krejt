package location

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
)

// Qendra e Prishtinës (testet e rides përdorin Fushë Kosovën — larg, që të mos përzihen kandidatët).
var center = geo.Point{Lat: 42.6629, Lng: 21.1655}

func TestLocationIngestNearestAndStaleness(t *testing.T) {
	raddr := os.Getenv("TEST_REDIS_ADDR")
	if raddr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rdb, err := cache.Connect(ctx, config.Redis{Host: raddr})
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	s := New(rdb, nil) // pa Postgres: persistenca gjatë udhëtimit nuk ushtrohet këtu
	now := time.Now()
	s.now = func() time.Time { return now }

	near, far := uuid.New(), uuid.New()
	defer s.SetOffline(context.Background(), near)
	defer s.SetOffline(context.Background(), far)

	// offline → refuzohet
	if _, err := s.Ingest(ctx, near, []Sample{{Lat: center.Lat, Lng: center.Lng, RecordedAtMs: now.UnixMilli()}}); err != ErrDriverOffline {
		t.Fatalf("offline: %v", err)
	}
	if err := s.SetOnline(ctx, near, []string{"economy", "comfort"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOnline(ctx, far, []string{"economy"}); err != nil {
		t.Fatal(err)
	}
	// mostra jashtë rendit + dublikatë + jashtë Kosovës: pranohen vetëm 2 (t-2, t-1 → e fundit t-1)
	ms := now.UnixMilli()
	n, err := s.Ingest(ctx, near, []Sample{
		{Lat: center.Lat, Lng: center.Lng, RecordedAtMs: ms - 1000},
		{Lat: center.Lat + 0.001, Lng: center.Lng, RecordedAtMs: ms - 2000},
		{Lat: center.Lat, Lng: center.Lng, RecordedAtMs: ms - 1000},
		{Lat: 41.33, Lng: 19.82, RecordedAtMs: ms},
	})
	if err != nil || n != 2 {
		t.Fatalf("ingest: n=%d err=%v", n, err)
	}
	// e vjetër se e fundit → hidhet
	if n, _ := s.Ingest(ctx, near, []Sample{{Lat: center.Lat, Lng: center.Lng, RecordedAtMs: ms - 5000}}); n != 0 {
		t.Fatalf("mostra e vjetër u pranua: %d", n)
	}
	if _, err := s.Ingest(ctx, far, []Sample{{Lat: center.Lat + 0.05, Lng: center.Lng, RecordedAtMs: ms}}); err != nil { // ~5.5 km
		t.Fatal(err)
	}

	c, err := s.Nearest(ctx, "economy", center, 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) < 1 || c[0].DriverID != near || c[0].DistanceM > 50 {
		t.Fatalf("nearest: %+v", c)
	}
	for _, x := range c {
		if x.DriverID == far {
			t.Fatal("shoferi i largët brenda 3 km?")
		}
	}
	if c, _ := s.Nearest(ctx, "comfort", center, 3, 10); len(c) < 1 || c[0].DriverID != near {
		t.Fatalf("comfort: %+v", c)
	}
	if eta, ok, _ := s.NearestETA(ctx, "economy", center); !ok || eta < 60 {
		t.Fatalf("eta: %d %v", eta, ok)
	}

	// busy → jo kandidat; available → sërish
	rid := uuid.New()
	if err := s.SetBusy(ctx, near, rid); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Nearest(ctx, "economy", center, 3, 10); len(c) > 0 && c[0].DriverID == near {
		t.Fatal("busy u kthye si kandidat")
	}
	if st, _ := s.State(ctx, near); st == nil || st.Status != "busy" || st.RideID != rid.String() {
		t.Fatalf("state: %+v", st)
	}
	if err := s.SetAvailable(ctx, near); err != nil {
		t.Fatal(err)
	}

	// i ndenjur (>60 s pa mostër) → jo kandidat dhe pastrohet nga GEO
	s.now = func() time.Time { return now.Add(2 * time.Minute) }
	if c, _ := s.Nearest(ctx, "economy", center, 3, 10); len(c) > 0 && c[0].DriverID == near {
		t.Fatal("i ndenjuri u kthye si kandidat")
	}
	if err := s.SetOffline(ctx, near); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.State(ctx, near); st != nil {
		t.Fatalf("pas offline: %+v", st)
	}
}
