// Package errtrack — gjurmimi i gabimeve (§50 Sentry): paniqet dhe gabimet e brendshme (5xx) raportohen
// me request_id/trace_id; asnjë sekret apo trup kërkese nuk dërgohet. Pa DSN → i çaktivizuar.
package errtrack

import (
	"context"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"

	"krejt.app/backend/internal/platform/logx"
)

// Init — kthen funksionin e mbylljes (flush).
func Init(dsn, env, release, service string, log *slog.Logger) (func(), error) {
	if dsn == "" {
		log.Info("errtrack: SENTRY_DSN mungon — raportimi i gabimeve i çaktivizuar")
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          service + "@" + release,
		ServerName:       service,
		AttachStacktrace: true,
		TracesSampleRate: 0, // gjurmët i mban OpenTelemetry
	}); err != nil {
		return nil, err
	}
	log.Info("errtrack: Sentry aktiv", "env", env)
	return func() { sentry.Flush(2 * time.Second) }, nil
}

// Report — raporton një gabim me kontekstin e kërkesës (request_id, trace_id, user_id — vetëm id, jo PII).
func Report(ctx context.Context, err error, tags map[string]string) {
	if err == nil || sentry.CurrentHub().Client() == nil {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		if v := logx.RequestID(ctx); v != "" {
			scope.SetTag("request_id", v)
		}
		if v := logx.TraceID(ctx); v != "" {
			scope.SetTag("trace_id", v)
		}
		if v := logx.UserID(ctx); v != "" {
			scope.SetUser(sentry.User{ID: v})
		}
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		sentry.CaptureException(err)
	})
}
