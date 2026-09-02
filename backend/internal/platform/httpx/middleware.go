package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/logx"
)

type Middleware func(http.Handler) http.Handler

// Chain aplikon middleware-t në rendin e dhënë (i pari është më i jashtmi).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestID — X-Request-Id nga klienti ose i ri; trace id nga traceparent (W3C) kur ekziston.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" || len(id) > 64 {
				id = "req_" + uuid.NewString()
			}
			ctx := logx.WithRequestID(r.Context(), id)
			if tp := r.Header.Get("traceparent"); len(tp) >= 55 {
				ctx = logx.WithTraceID(ctx, tp[3:35])
			}
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Recover kap panic-et: log teknik brenda, INTERNAL i pastër jashtë (§57).
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logx.From(r.Context(), log).Error("panic", "panic", rec, "stack", string(debug.Stack()))
					WriteError(w, r, ErrInternal)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders — header-a të sigurt për çdo përgjigje (§51).
func SecureHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cache-Control", "no-store")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout — asnjë kërkesë nuk mban një worker përgjithmonë.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }
func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// AccessLog — një rresht JSON për kërkesë: metoda, rruga, statusi, latenca, request_id (§50).
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			logx.From(r.Context(), log).Info("http",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"latency_ms", time.Since(start).Milliseconds(), "bytes", sw.bytes,
				"ip", clientIP(r), "ua", r.UserAgent())
		})
	}
}

func clientIP(r *http.Request) string {
	// Cloudflare → ALB → task: CF-Connecting-IP është burimi i besueshëm kur vjen nga Cloudflare.
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
