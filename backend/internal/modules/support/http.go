package support

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireSupport httpx.Middleware) {
	mux.Handle("POST /api/v1/support/tickets", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/support/tickets", requireAuth(principal.Handler(s.handleMine)))
	mux.Handle("GET /api/v1/support/tickets/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/support/tickets/{id}/messages", requireAuth(principal.Handler(s.handleReply)))
	mux.Handle("POST /api/v1/support/tickets/{id}/close", requireAuth(principal.Handler(s.handleClose)))
	mux.Handle("POST /api/v1/safety/reports", requireAuth(principal.Handler(s.handleSafety)))

	mux.Handle("GET /api/v1/admin/support/tickets", requireSupport(principal.Handler(s.handleQueue)))
	mux.Handle("GET /api/v1/admin/support/tickets/{id}", requireSupport(principal.Handler(s.handleAgentGet)))
	mux.Handle("POST /api/v1/admin/support/tickets/{id}/messages", requireSupport(principal.Handler(s.handleAgentReply)))
	mux.Handle("PATCH /api/v1/admin/support/tickets/{id}", requireSupport(principal.Handler(s.handleAgentUpdate)))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in CreateInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	t, err := s.Create(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (s *Service) handleMine(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	items, err := s.Mine(r.Context(), a)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "categories": Categories})
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	t, err := s.Get(r.Context(), a, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

type bodyIn struct {
	Body string `json:"body"`
}

func (s *Service) handleReply(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in bodyIn
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.Reply(r.Context(), a, id, in.Body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Service) handleClose(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err == nil {
		err = s.Close(r.Context(), a, id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleSafety(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in SafetyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rep, err := s.ReportSafety(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rep)
}

func (s *Service) handleQueue(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	q := r.URL.Query()
	f := QueueFilter{Status: q.Get("status"), Priority: q.Get("priority")}
	if q.Get("assigned") == "me" {
		id := a.UserID
		f.AssignedTo = &id
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	items, err := s.Queue(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleAgentGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	t, err := s.AgentGet(r.Context(), a, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (s *Service) handleAgentReply(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in bodyIn
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.AgentReply(r.Context(), a, id, in.Body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Service) handleAgentUpdate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in AgentUpdate
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	t, err := s.AgentUpdate(r.Context(), a, id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}
