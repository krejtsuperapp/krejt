package parcels

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — klienti (requireAuth) dhe korrieri (requireDriver).
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireDriver httpx.Middleware) {
	mux.Handle("POST /api/v1/parcels/quote", requireAuth(principal.Handler(s.handleQuote)))
	mux.Handle("POST /api/v1/parcels", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/parcels", requireAuth(principal.Handler(s.handleHistory)))
	mux.Handle("GET /api/v1/parcels/active", requireAuth(principal.Handler(s.handleActive)))
	mux.Handle("GET /api/v1/parcels/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/parcels/{id}/cancel", requireAuth(principal.Handler(s.handleCancel)))

	mux.Handle("GET /api/v1/courier/parcel-offers", requireDriver(principal.Handler(s.handleOffers)))
	mux.Handle("POST /api/v1/courier/parcel-offers/{id}/accept", requireDriver(principal.Handler(s.handleAccept)))
	mux.Handle("POST /api/v1/courier/parcel-offers/{id}/decline", requireDriver(principal.Handler(s.handleDecline)))
	mux.Handle("GET /api/v1/courier/parcels/active", requireDriver(principal.Handler(s.handleCourierActive)))
	mux.Handle("POST /api/v1/courier/parcels/{id}/pickup", requireDriver(principal.Handler(s.handlePickUp)))
	mux.Handle("POST /api/v1/courier/parcels/{id}/deliver", requireDriver(principal.Handler(s.handleDeliver)))
	mux.Handle("POST /api/v1/courier/parcels/{id}/release", requireDriver(principal.Handler(s.handleRelease)))
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
	var in QuoteInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q, err := s.Quote(r.Context(), a.UserID, in)
	respond(w, r, q, err)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.Create(r.Context(), a, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, p)
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

func (s *Service) handleActive(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	p, err := s.Active(r.Context(), a)
	respond(w, r, map[string]any{"parcel": p}, err)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.Get(r.Context(), a, id)
	respond(w, r, p, err)
}

func (s *Service) handleCancel(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	_ = httpx.DecodeJSON(r, &in) // trupi është opsional
	p, err := s.Cancel(r.Context(), a, id, in.Reason)
	respond(w, r, p, err)
}

// --- korrieri ---------------------------------------------------------------------------------

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
	p, err := s.AcceptOffer(r.Context(), a, id)
	respond(w, r, p, err)
}

func (s *Service) handleDecline(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.DeclineOffer(r.Context(), a, id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleCourierActive(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	p, err := s.ActiveForCourier(r.Context(), a)
	respond(w, r, map[string]any{"parcel": p}, err)
}

type codeInput struct {
	Code string `json:"code"`
}

func (s *Service) handlePickUp(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in codeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.PickUp(r.Context(), a, id, in.Code)
	respond(w, r, p, err)
}

func (s *Service) handleDeliver(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in codeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.Deliver(r.Context(), a, id, in.Code)
	respond(w, r, p, err)
}

func (s *Service) handleRelease(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	_ = httpx.DecodeJSON(r, &in)
	p, err := s.Release(r.Context(), a, id, in.Reason)
	respond(w, r, p, err)
}
