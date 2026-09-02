// Package otel — observability (§50): OpenTelemetry gjurmë + metrika drejt OTLP (Grafana Cloud),
// span për çdo kërkesë HTTP, query të PostgreSQL dhe komandë Redis; trace_id hyn në log dhe në
// zarfin e gabimit. Konfigurimi standard nga mjedisi: OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_HEADERS,
// OTEL_TRACES_SAMPLER_ARG (përqindja). Pa endpoint → asgjë nuk eksportohet (development).
package otel

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"krejt.app/backend/internal/platform/logx"
)

// Init — ngre ofruesit e gjurmëve dhe metrikave; kthen funksionin e mbylljes (flush në shutdown).
// Kur OTEL_EXPORTER_OTLP_ENDPOINT mungon, instalohet vetëm propagimi (trace id-të rrjedhin, asgjë s'eksportohet).
func Init(ctx context.Context, service, env, version string, log *slog.Logger) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", service),
		attribute.String("service.version", version),
		attribute.String("deployment.environment", env),
	))
	if err != nil {
		return nil, err
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		log.Info("otel: no OTLP endpoint configured — traces/metrics not exported")
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.NeverSample())))
		return func(context.Context) error { return nil }, nil
	}
	ratio := 0.2
	if v, err := strconv.ParseFloat(os.Getenv("OTEL_TRACES_SAMPLER_ARG"), 64); err == nil && v >= 0 && v <= 1 {
		ratio = v
	}
	texp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(texp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(tp)
	mexp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexp, sdkmetric.WithInterval(30*time.Second))))
	otel.SetMeterProvider(mp)
	log.Info("otel: exporting to OTLP", "endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), "sample_ratio", ratio)
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		e1 := tp.Shutdown(ctx)
		e2 := mp.Shutdown(ctx)
		if e1 != nil {
			return e1
		}
		return e2
	}, nil
}

// HTTPMiddleware — span për çdo kërkesë (otelhttp) + trace_id në kontekstin e log-ut/gabimit.
func HTTPMiddleware(service string) func(http.Handler) http.Handler {
	inner := otelhttp.NewMiddleware(service, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if p := r.Pattern; p != "" {
			return p
		}
		return r.Method + " " + r.URL.Path
	}))
	return func(next http.Handler) http.Handler {
		return inner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sc := trace.SpanContextFromContext(r.Context()); sc.HasTraceID() {
				r = r.WithContext(logx.WithTraceID(r.Context(), sc.TraceID().String()))
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// --- pgx ------------------------------------------------------------------------------

// PgxTracer — span për çdo query (emri: fjala e parë e SQL-së; teksti i plotë vetëm te atributi, pa argumente).
type PgxTracer struct{}

type pgxSpanKey struct{}

func (PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	name := "db.query"
	if len(data.SQL) > 0 {
		verb := data.SQL
		for i, c := range verb {
			if c == ' ' || c == '\n' || c == '\t' {
				verb = verb[:i]
				break
			}
			if i > 12 {
				verb = verb[:12]
				break
			}
		}
		name = "db " + verb
	}
	ctx, span := otel.Tracer("pgx").Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system", "postgresql"), attribute.String("db.statement", truncate(data.SQL, 300))))
	return context.WithValue(ctx, pgxSpanKey{}, span)
}

func (PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span, ok := ctx.Value(pgxSpanKey{}).(trace.Span)
	if !ok {
		return
	}
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, "query failed")
	}
	span.End()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
