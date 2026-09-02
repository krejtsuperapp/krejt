package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/logx"
)

func TestPerIPLimit(t *testing.T) {
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
	l := New(rdb, logx.New("test", "development"))
	fixed := time.Date(2026, 9, 2, 10, 0, 5, 0, time.UTC)
	l.now = func() time.Time { return fixed }

	h := l.PerIP(3, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	ip := "198.51.100." + uuid.NewString()[:2] // IP e veçantë për këtë ekzekutim (çelësat në Redis të ndarë)
	call := func(path string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Header().Get("Retry-After")
	}
	for i := 0; i < 3; i++ {
		if c, _ := call("/api/v1/x"); c != http.StatusNoContent {
			t.Fatalf("kërkesa %d: %d", i+1, c)
		}
	}
	c, retry := call("/api/v1/x")
	if c != http.StatusTooManyRequests || retry == "" {
		t.Fatalf("e 4-ta: %d retry=%q", c, retry)
	}
	if c, _ := call("/healthz"); c != http.StatusNoContent {
		t.Fatalf("shëndeti nuk kufizohet: %d", c)
	}
	// dritarja tjetër → sërish e lejuar
	l.now = func() time.Time { return fixed.Add(time.Minute) }
	if c, _ := call("/api/v1/x"); c != http.StatusNoContent {
		t.Fatalf("dritare e re: %d", c)
	}
}
