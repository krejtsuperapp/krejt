package rides

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — klienti (requireAuth) dhe shoferi (requireDriver: RIDE_DRIVER | TAXI_DRIVER).
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireDriver httpx.Middleware) {
	mux.Handle("POST /api/v1/rides/quote", requireAuth(principal.Handler(s.handleQuote)))
	mux.Handle("POST /api/v1/rides", requireAuth(principal.Handler(s.handleRequest)))
	mux.Handle("GET /api/v1/rides", requireAuth(principal.Handler(s.handleHistory)))
	mux.Handle("GET /api/v1/rides/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/rides/{id}/cancel", requireAuth(principal.Handler(s.handleCancel)))
	mux.Handle("GET /api/v1/rides/{id}/qr", requireAuth(principal.Handler(s.handleQR)))

	mux.Handle("GET /api/v1/driver/offers", requireDriver(principal.Handler(s.handleOffers)))
	mux.Handle("POST /api/v1/driver/offers/{id}/accept", requireDriver(principal.Handler(s.handleAccept)))
	mux.Handle("POST /api/v1/driver/offers/{id}/decline", requireDriver(principal.Handler(s.handleDecline)))
	mux.Handle("GET /api/v1/driver/rides/active", requireDriver(principal.Handler(s.handleActive)))
	mux.Handle("POST /api/v1/driver/rides/{id}/arrived", requireDriver(principal.Handler(s.step(s.Arrived))))
	mux.Handle("POST /api/v1/driver/rides/{id}/start", requireDriver(principal.Handler(s.handleStart)))
	mux.Handle("POST /api/v1/driver/rides/{id}/complete", requireDriver(principal.Handler(s.step(s.Complete))))
	mux.Handle("POST /api/v1/driver/rides/{id}/cancel", requireDriver(principal.Handler(s.handleDriverCancel)))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (s *Service) handleQuote(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in pricing.QuoteInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q, err := s.pricing.Quote(r.Context(), a.UserID, in)
	respond(w, r, q, err)
}

func (s *Service) handleRequest(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in RequestInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ride, err := s.Request(r.Context(), a, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, ride)
}

func (s *Service) handleHistory(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var before *time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"before": "invalid"}))
			return
		}
		before = &t
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.History(r.Context(), a, before, limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ride, err := s.Get(r.Context(), a, id)
	respond(w, r, ride, err)
}

type cancelBody struct {
	Reason string `json:"reason"`
}

func (s *Service) handleCancel(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body cancelBody
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	ride, err := s.CancelByCustomer(r.Context(), a, id, body.Reason)
	respond(w, r, ride, err)
}

func (s *Service) handleOffers(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	items, err := s.Offers(r.Context(), a)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleAccept(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ride, err := s.AcceptOffer(r.Context(), a, id)
	respond(w, r, ride, err)
}

func (s *Service) handleDecline(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err == nil {
		err = s.DeclineOffer(r.Context(), a, id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleActive(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	ride, err := s.ActiveForDriver(r.Context(), a)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if ride == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ride": nil})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ride": ride})
}

func (s *Service) step(fn func(context.Context, principal.Actor, uuid.UUID) (*Ride, error)) func(http.ResponseWriter, *http.Request, principal.Actor) {
	return func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		ride, err := fn(r.Context(), a, id)
		respond(w, r, ride, err)
	}
}

func (s *Service) handleStart(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		Code    string `json:"code"`
		QRToken string `json:"qr_token"`
	}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	ride, err := s.Start(r.Context(), a, id, body.Code, body.QRToken)
	respond(w, r, ride, err)
}

func (s *Service) handleQR(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tok, exp, err := s.QRToken(r.Context(), a, id)
	respond(w, r, map[string]any{"token": tok, "expires_at": exp}, err)
}

func (s *Service) handleDriverCancel(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body cancelBody
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	ride, err := s.CancelByDriver(r.Context(), a, id, body.Reason)
	respond(w, r, ride, err)
}
