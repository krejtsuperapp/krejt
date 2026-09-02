package appconfig

import (
	"net/http"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — /config publik (me përdorues opsional për flag-et), admin me OPERATIONS/SUPER_ADMIN.
func (s *Service) Routes(mux *http.ServeMux, optionalAuth, requireOps httpx.Middleware) {
	mux.Handle("GET /api/v1/config", optionalAuth(http.HandlerFunc(s.handleConfig)))
	mux.Handle("GET /api/v1/admin/flags", requireOps(principal.Handler(s.handleFlags)))
	mux.Handle("PATCH /api/v1/admin/flags/{key}", requireOps(principal.Handler(s.handleSetFlag)))
	mux.Handle("GET /api/v1/admin/app-versions", requireOps(principal.Handler(s.handleVersions)))
	mux.Handle("PUT /api/v1/admin/app-versions/{app}/{platform}", requireOps(principal.Handler(s.handleSetVersion)))
}

func (s *Service) handleConfig(w http.ResponseWriter, r *http.Request) {
	userID := uuid.Nil
	if c, ok := auth.ClaimsFrom(r.Context()); ok {
		if id, err := uuid.Parse(c.Subject); err == nil {
			userID = id
		}
	}
	out, err := s.PublicConfig(r.Context(), ClientFrom(r), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Service) handleFlags(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	items, err := s.Flags(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleSetFlag(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in FlagUpdate
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f, err := s.SetFlag(r.Context(), a, r.PathValue("key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, f)
}

func (s *Service) handleVersions(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	items, err := s.Versions(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleSetVersion(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in VersionUpdate
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	v, err := s.SetVersion(r.Context(), a, r.PathValue("app"), r.PathValue("platform"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
