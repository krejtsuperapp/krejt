package business

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — të gjitha kërkojnë kyçje; anëtarësia dhe roli verifikohen te shërbimi, sepse ato varen
// nga ndërmarrja e kërkesës dhe jo nga kapacitetet e token-it.
func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/businesses", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/businesses", requireAuth(principal.Handler(s.handleMine)))
	mux.Handle("GET /api/v1/businesses/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("GET /api/v1/businesses/{id}/members", requireAuth(principal.Handler(s.handleMembers)))
	mux.Handle("POST /api/v1/businesses/{id}/members", requireAuth(principal.Handler(s.handleAddMember)))
	mux.Handle("DELETE /api/v1/businesses/{id}/members/{user_id}", requireAuth(principal.Handler(s.handleRemoveMember)))
	mux.Handle("GET /api/v1/businesses/{id}/charges", requireAuth(principal.Handler(s.handleCharges)))
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
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

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := s.Create(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, b)
}

func (s *Service) handleMine(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	items, err := s.Mine(r.Context(), a)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := s.Get(r.Context(), a, id)
	respond(w, r, b, err)
}

func (s *Service) handleMembers(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items, err := s.Members(r.Context(), a, id)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleAddMember(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in MemberInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.AddMember(r.Context(), a, id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Service) handleRemoveMember(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userID, err := pathID(r, "user_id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.RemoveMember(r.Context(), a, id, userID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleCharges(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r, "id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
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
	items, total, err := s.Charges(r.Context(), a, id, before, limit)
	respond(w, r, map[string]any{"items": items, "total_minor": total}, err)
}
