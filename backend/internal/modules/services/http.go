package services

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — klienti (requireAuth), mjeshtri (requireAuth, kontrolli i miratimit bëhet brenda) dhe
// Operacionet (requireOps) për miratimin e mjeshtrave.
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireOps httpx.Middleware) {
	mux.Handle("GET /api/v1/services/categories", requireAuth(principal.Handler(s.handleCategories)))
	mux.Handle("POST /api/v1/services/requests", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/services/requests", requireAuth(principal.Handler(s.handleHistory)))
	mux.Handle("GET /api/v1/services/requests/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/services/requests/{id}/accept", requireAuth(principal.Handler(s.handleAccept)))
	mux.Handle("POST /api/v1/services/requests/{id}/cancel", requireAuth(principal.Handler(s.handleCancel)))

	mux.Handle("POST /api/v1/services/provider", requireAuth(principal.Handler(s.handleApply)))
	mux.Handle("GET /api/v1/services/provider", requireAuth(principal.Handler(s.handleProfile)))
	mux.Handle("GET /api/v1/services/provider/open", requireAuth(principal.Handler(s.handleOpen)))
	mux.Handle("GET /api/v1/services/provider/jobs", requireAuth(principal.Handler(s.handleJobs)))
	mux.Handle("POST /api/v1/services/provider/requests/{id}/offer", requireAuth(principal.Handler(s.handleOffer)))
	mux.Handle("POST /api/v1/services/provider/offers/{id}/withdraw", requireAuth(principal.Handler(s.handleWithdraw)))
	mux.Handle("POST /api/v1/services/provider/requests/{id}/start", requireAuth(principal.Handler(s.step(StateInProgress))))
	mux.Handle("POST /api/v1/services/provider/requests/{id}/complete", requireAuth(principal.Handler(s.step(StateCompleted))))
	mux.Handle("POST /api/v1/services/provider/requests/{id}/release", requireAuth(principal.Handler(s.step(StateOpen))))

	mux.Handle("GET /api/v1/admin/service-providers", requireOps(principal.Handler(s.handleProviders)))
	mux.Handle("PATCH /api/v1/admin/service-providers/{id}", requireOps(principal.Handler(s.handleSetStatus)))
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

func (s *Service) handleCategories(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	items, err := s.Categories(r.Context())
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req, err := s.Create(r.Context(), a, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, req)
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
	req, err := s.Get(r.Context(), a, id)
	respond(w, r, req, err)
}

func (s *Service) handleAccept(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		OfferID uuid.UUID `json:"offer_id"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req, err := s.AcceptOffer(r.Context(), a, id, in.OfferID)
	respond(w, r, req, err)
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
	_ = httpx.DecodeJSON(r, &in)
	req, err := s.Cancel(r.Context(), a, id, in.Reason)
	respond(w, r, req, err)
}

// --- mjeshtri -------------------------------------------------------------------------------

func (s *Service) handleApply(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in ApplyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.Apply(r.Context(), a, in)
	respond(w, r, p, err)
}

func (s *Service) handleProfile(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	p, err := s.ProviderProfile(r.Context(), a.UserID)
	respond(w, r, p, err)
}

func (s *Service) handleOpen(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.OpenRequests(r.Context(), a, limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleJobs(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.MyJobs(r.Context(), a, limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleOffer(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in OfferInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.MakeOffer(r.Context(), a, id, in)
	respond(w, r, o, err)
}

func (s *Service) handleWithdraw(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.WithdrawOffer(r.Context(), a, id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) step(to string) func(http.ResponseWriter, *http.Request, principal.Actor) {
	return func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		_ = httpx.DecodeJSON(r, &in)
		req, err := s.Step(r.Context(), a, id, to, in.Reason)
		respond(w, r, req, err)
	}
}

func (s *Service) handleProviders(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.Providers(r.Context(), r.URL.Query().Get("status"), limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleSetStatus(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.SetProviderStatus(r.Context(), a, id, in.Status, in.Reason)
	respond(w, r, p, err)
}
