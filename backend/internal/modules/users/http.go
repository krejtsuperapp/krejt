package users

import (
	"net/http"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/platform/httpx"
)

type handler func(w http.ResponseWriter, r *http.Request, a Actor)

// Routes regjistron /api/v1/users/me/* — të gjitha pas RequireAuth (çdo kapacitet).
func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	h := func(fn handler) http.Handler {
		return requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, ok := actorFrom(r)
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthorized)
				return
			}
			fn(w, r, a)
		}))
	}
	mux.Handle("PATCH /api/v1/users/me", h(s.handleUpdateProfile))
	mux.Handle("DELETE /api/v1/users/me", h(s.handleDeleteAccount))
	mux.Handle("GET /api/v1/users/me/addresses", h(s.handleListAddresses))
	mux.Handle("POST /api/v1/users/me/addresses", h(s.handleCreateAddress))
	mux.Handle("PUT /api/v1/users/me/addresses/{id}", h(s.handleUpdateAddress))
	mux.Handle("DELETE /api/v1/users/me/addresses/{id}", h(s.handleDeleteAddress))
	mux.Handle("GET /api/v1/users/me/notification-preferences", h(s.handleGetPreferences))
	mux.Handle("PUT /api/v1/users/me/notification-preferences", h(s.handleSetPreferences))
	mux.Handle("GET /api/v1/users/me/sessions", h(s.handleListSessions))
	mux.Handle("DELETE /api/v1/users/me/sessions/{id}", h(s.handleRevokeSession))
}

func actorFrom(r *http.Request) (Actor, bool) {
	c, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		return Actor{}, false
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return Actor{}, false
	}
	sid, err := uuid.Parse(c.SessionID)
	if err != nil {
		return Actor{}, false
	}
	return Actor{UserID: uid, SessionID: sid, IP: httpx.ClientIP(r)}, true
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

func (s *Service) handleUpdateProfile(w http.ResponseWriter, r *http.Request, a Actor) {
	var in ProfileUpdate
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.UpdateProfile(r.Context(), a, in)
	respond(w, r, p, err)
}

func (s *Service) handleDeleteAccount(w http.ResponseWriter, r *http.Request, a Actor) {
	if err := s.DeleteAccount(r.Context(), a); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListAddresses(w http.ResponseWriter, r *http.Request, a Actor) {
	items, err := s.ListAddresses(r.Context(), a.UserID)
	respond(w, r, map[string]any{"items": items, "max": MaxAddresses}, err)
}

func (s *Service) handleCreateAddress(w http.ResponseWriter, r *http.Request, a Actor) {
	var in AddressInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	x, err := s.CreateAddress(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (s *Service) handleUpdateAddress(w http.ResponseWriter, r *http.Request, a Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in AddressInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	x, err := s.UpdateAddress(r.Context(), a, id, in)
	respond(w, r, x, err)
}

func (s *Service) handleDeleteAddress(w http.ResponseWriter, r *http.Request, a Actor) {
	id, err := pathID(r)
	if err == nil {
		err = s.DeleteAddress(r.Context(), a, id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleGetPreferences(w http.ResponseWriter, r *http.Request, a Actor) {
	items, err := s.Preferences(r.Context(), a.UserID)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleSetPreferences(w http.ResponseWriter, r *http.Request, a Actor) {
	var body struct {
		Items []Preference `json:"items"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items, err := s.SetPreferences(r.Context(), a, body.Items)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleListSessions(w http.ResponseWriter, r *http.Request, a Actor) {
	items, err := s.Sessions(r.Context(), a)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleRevokeSession(w http.ResponseWriter, r *http.Request, a Actor) {
	id, err := pathID(r)
	if err == nil {
		err = s.RevokeSession(r.Context(), a, id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
