// Package ratelimit — kufizim i shpejtësisë (§51 rate limiting / brute-force) mbi Redis:
// dritare fikse për çelës (IP ose përdorues), INCR + EXPIRE atomik, përgjigje 429 me Retry-After.
// Nëse Redis-i nuk përgjigjet, kërkesa kalon (fail-open) dhe logohet — kufizimi nuk duhet të rrëzojë shërbimin.
package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/platform/httpx"
)

type Limiter struct {
	rdb redis.UniversalClient
	log *slog.Logger
	now func() time.Time
}

func New(rdb redis.UniversalClient, log *slog.Logger) *Limiter {
	return &Limiter{rdb: rdb, log: log, now: time.Now}
}

// Allow — a lejohet kërkesa për çelësin (scope:id) me `limit` në `window`? Kthen edhe sa sekonda mbeten.
func (l *Limiter) Allow(ctx context.Context, scope, id string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error) {
	if id == "" {
		return true, 0, nil
	}
	now := l.now()
	slot := now.Unix() / int64(window.Seconds())
	key := fmt.Sprintf("rl:%s:%s:%d", scope, id, slot)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window+time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err
	}
	if incr.Val() > int64(limit) {
		end := time.Unix((slot+1)*int64(window.Seconds()), 0)
		return false, end.Sub(now), nil
	}
	return true, 0, nil
}

// PerIP — middleware për trafikun publik (para autentikimit).
func (l *Limiter) PerIP(limit int, window time.Duration) httpx.Middleware {
	return l.middleware("ip", limit, window, func(r *http.Request) string { return httpx.ClientIP(r) })
}

// PerUser — middleware pas RequireAuth (çelësi: id-ja e përdoruesit nga JWT).
func (l *Limiter) PerUser(limit int, window time.Duration) httpx.Middleware {
	return l.middleware("user", limit, window, func(r *http.Request) string {
		if c, ok := auth.ClaimsFrom(r.Context()); ok {
			return c.Subject
		}
		return ""
	})
}

func (l *Limiter) middleware(scope string, limit int, window time.Duration, keyOf func(*http.Request) string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			if p == "/healthz" || p == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}
			allowed, retry, err := l.Allow(r.Context(), scope, keyOf(r), limit, window)
			if err != nil {
				l.log.Warn("ratelimit: redis unavailable, failing open", "scope", scope, "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
				httpx.WriteError(w, r, httpx.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
