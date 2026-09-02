// Package httpx — serveri HTTP, middleware-t dhe formati i vetëm i gabimeve (§55, §57).
// Klienti merr gjithmonë: code, message_key (për përkthim), request_id, trace_id, retryable,
// dhe për validim: fields {emri_i_fushës: arsyeja} për gabime inline (§57).
// Stack trace nuk ekspozohet kurrë; detajet teknike mbeten në log.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"krejt.app/backend/internal/platform/logx"
)

type APIError struct {
	Code       string            `json:"code"`
	MessageKey string            `json:"message_key"`
	HTTPStatus int               `json:"http_status"`
	Retryable  bool              `json:"retryable"`
	Fields     map[string]string `json:"fields,omitempty"`
	Err        error             `json:"-"`
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *APIError) Unwrap() error { return e.Err }

// Is — dy APIError janë "i njëjti gabim" kur kanë të njëjtin Code (kopjet me With/WithFields përputhen).
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	return ok && t.Code == e.Code
}

// With bashkangjit shkakun teknik (vetëm për log).
func (e *APIError) With(err error) *APIError {
	c := *e
	c.Err = err
	return &c
}

// WithFields bashkangjit gabimet për fushë (§57 inline validation): {"email": "invalid"}.
func (e *APIError) WithFields(fields map[string]string) *APIError {
	c := *e
	c.Fields = fields
	return &c
}

var (
	ErrNotFound      = &APIError{Code: "NOT_FOUND", MessageKey: "errors.not_found", HTTPStatus: http.StatusNotFound}
	ErrValidation    = &APIError{Code: "VALIDATION_FAILED", MessageKey: "errors.validation", HTTPStatus: http.StatusUnprocessableEntity}
	ErrUnauthorized  = &APIError{Code: "UNAUTHORIZED", MessageKey: "errors.unauthorized", HTTPStatus: http.StatusUnauthorized}
	ErrForbidden     = &APIError{Code: "FORBIDDEN", MessageKey: "errors.forbidden", HTTPStatus: http.StatusForbidden}
	ErrConflict      = &APIError{Code: "CONFLICT", MessageKey: "errors.conflict", HTTPStatus: http.StatusConflict}
	ErrRateLimited   = &APIError{Code: "RATE_LIMITED", MessageKey: "errors.rate_limited", HTTPStatus: http.StatusTooManyRequests, Retryable: true}
	ErrInternal      = &APIError{Code: "INTERNAL", MessageKey: "errors.internal", HTTPStatus: http.StatusInternalServerError, Retryable: true}
	ErrUnavailable   = &APIError{Code: "UNAVAILABLE", MessageKey: "errors.unavailable", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
	ErrMaintenance   = &APIError{Code: "MAINTENANCE", MessageKey: "errors.maintenance", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
	ErrIdempotency   = &APIError{Code: "IDEMPOTENCY_KEY_REUSED", MessageKey: "errors.idempotency", HTTPStatus: http.StatusConflict}
	ErrUpdateRequire = &APIError{Code: "UPDATE_REQUIRED", MessageKey: "errors.update_required", HTTPStatus: http.StatusUpgradeRequired}
)

type errorEnvelope struct {
	Error struct {
		Code       string            `json:"code"`
		MessageKey string            `json:"message_key"`
		HTTPStatus int               `json:"http_status"`
		RequestID  string            `json:"request_id,omitempty"`
		TraceID    string            `json:"trace_id,omitempty"`
		Retryable  bool              `json:"retryable"`
		Fields     map[string]string `json:"fields,omitempty"`
	} `json:"error"`
}

// ErrorReporter — gjurmuesi i gabimeve (Sentry); merr paniqet dhe gabimet e brendshme (5xx me shkak).
var reporter func(ctx context.Context, err error)

func SetErrorReporter(f func(ctx context.Context, err error)) { reporter = f }

func report(ctx context.Context, err error) {
	if reporter != nil && err != nil {
		reporter(ctx, err)
	}
}

// WriteError shkruan gabimin në formatin e vetëm. Gabimet e panjohura bëhen INTERNAL (pa detaje).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		api = ErrInternal.With(err)
	}
	if api.HTTPStatus >= 500 && api.Err != nil {
		report(r.Context(), api.Err)
	}
	var env errorEnvelope
	env.Error.Code = api.Code
	env.Error.MessageKey = api.MessageKey
	env.Error.HTTPStatus = api.HTTPStatus
	env.Error.RequestID = logx.RequestID(r.Context())
	env.Error.TraceID = logx.TraceID(r.Context())
	env.Error.Retryable = api.Retryable
	env.Error.Fields = api.Fields
	WriteJSON(w, api.HTTPStatus, env)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON lexon trupin JSON (max 64 KiB, fusha të panjohura refuzohen); gabimi kthehet si VALIDATION_FAILED.
func DecodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return ErrValidation.With(err).WithFields(map[string]string{"body": "invalid_json"})
	}
	return nil
}

// ClientIP — IP-ja e klientit (pas ALB/Cloudflare: X-Forwarded-For i pari).
func ClientIP(r *http.Request) string { return clientIP(r) }
