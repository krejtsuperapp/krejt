// Package logx — log JSON të strukturuara (§50). Çdo rresht mban service, env dhe,
// kur ekziston, request_id / trace_id. Fushat sekrete maskohen gjithmonë.
package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyTraceID
	keyUserID
)

// fushat që nuk logohen kurrë (§50): fjalëkalime, tokena, çelësa, të dhëna karte
var redacted = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true, "access_token": true, "refresh_token": true,
	"authorization": true, "api_key": true, "apikey": true, "card": true, "pan": true, "cvv": true, "otp": true, "code": true,
}

func New(service, env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if redacted[strings.ToLower(a.Key)] {
				return slog.String(a.Key, "[REDACTED]")
			}
			return a
		},
	})
	return slog.New(h).With("service", service, "env", env)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyTraceID, id)
}
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}

func RequestID(ctx context.Context) string { v, _ := ctx.Value(keyRequestID).(string); return v }
func TraceID(ctx context.Context) string   { v, _ := ctx.Value(keyTraceID).(string); return v }
func UserID(ctx context.Context) string    { v, _ := ctx.Value(keyUserID).(string); return v }

// From kthen logger-in me fushat e kontekstit të kërkesës.
func From(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if id := RequestID(ctx); id != "" {
		l = l.With("request_id", id)
	}
	if id := TraceID(ctx); id != "" {
		l = l.With("trace_id", id)
	}
	if id := UserID(ctx); id != "" {
		l = l.With("user_id", id)
	}
	return l
}
