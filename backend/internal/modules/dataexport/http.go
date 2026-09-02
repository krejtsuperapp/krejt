package dataexport

import (
	"errors"
	"net/http"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/users/me/export", requireAuth(principal.Handler(s.handleRequest)))
	mux.Handle("GET /api/v1/users/me/export", requireAuth(principal.Handler(s.handleLatest)))
}

func (s *Service) handleRequest(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	out, err := s.Request(r.Context(), a)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 202: kërkesa u pranua, skedari ende nuk ekziston.
	httpx.WriteJSON(w, http.StatusAccepted, out)
}

func (s *Service) handleLatest(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	out, err := s.Latest(r.Context(), a.UserID)
	if err != nil {
		// Asnjë kërkesë ende nuk është gabim: është gjendja fillestare e çdo llogarie.
		if errors.Is(err, httpx.ErrNotFound) {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "none"})
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
