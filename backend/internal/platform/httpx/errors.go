// Package httpx — serveri HTTP, middleware-t dhe formati i vetëm i gabimeve (§57 e vjetër / §55).
// Klienti merr gjithmonë: code, message_key (për përkthim), request_id, trace_id, retryable.
// Stack trace nuk ekspozohet kurrë; detajet teknike mbeten në log.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"krejt.app/backend/internal/platform/logx"
)

type APIError struct {
	Code       string `json:"code"`
	MessageKey string `json:"message_key"`
	HTTPStatus int    `json:"http_status"`
	Retryable  bool   `json:"retryable"`
	Err        error  `json:"-"`
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *APIError) Unwrap() error { return e.Err }

// With bashkangjit shkakun teknik (vetëm për log).
func (e *APIError) With(err error) *APIError {
	c := *e
	c.Err = err
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
		Code       string `json:"code"`
		MessageKey string `json:"message_key"`
		HTTPStatus int    `json:"http_status"`
		RequestID  string `json:"request_id,omitempty"`
		TraceID    string `json:"trace_id,omitempty"`
		Retryable  bool   `json:"retryable"`
	} `json:"error"`
}

// WriteError shkruan gabimin në formatin e vetëm. Gabimet e panjohura bëhen INTERNAL (pa detaje).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		api = ErrInternal.With(err)
	}
	var env errorEnvelope
	env.Error.Code = api.Code
	env.Error.MessageKey = api.MessageKey
	env.Error.HTTPStatus = api.HTTPStatus
	env.Error.RequestID = logx.RequestID(r.Context())
	env.Error.TraceID = logx.TraceID(r.Context())
	env.Error.Retryable = api.Retryable
	WriteJSON(w, api.HTTPStatus, env)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
