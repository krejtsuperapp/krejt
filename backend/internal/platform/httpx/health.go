package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Health — /healthz (liveness: procesi jeton) dhe /readyz (readiness: DB + Redis përgjigjen) (§44).
type Health struct {
	pool *pgxpool.Pool
	rdb  redis.UniversalClient
}

func NewHealth(pool *pgxpool.Pool, rdb redis.UniversalClient) *Health {
	return &Health{pool: pool, rdb: rdb}
}

func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"db": "ok", "redis": "ok"}
	status := http.StatusOK
	if err := h.pool.Ping(ctx); err != nil {
		checks["db"] = "down"
		status = http.StatusServiceUnavailable
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down"
		status = http.StatusServiceUnavailable
	}
	WriteJSON(w, status, map[string]any{"status": map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK], "checks": checks})
}
