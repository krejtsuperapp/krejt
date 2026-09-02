package drivers

import (
	"net/http"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — requireAuth: çdo përdorues i kyçur (aplikon); requireDriver: RIDE_DRIVER ose TAXI_DRIVER;
// requireOps: OPERATIONS (miratimi).
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireDriver, requireOps httpx.Middleware) {
	mux.Handle("POST /api/v1/driver/profile", requireAuth(principal.Handler(s.handleApply)))
	mux.Handle("GET /api/v1/driver/profile", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/driver/online", requireDriver(principal.Handler(s.handleOnline)))
	mux.Handle("POST /api/v1/driver/offline", requireDriver(principal.Handler(s.handleOffline)))
	mux.Handle("POST /api/v1/driver/location", requireDriver(principal.Handler(s.handleLocation)))
	mux.Handle("GET /api/v1/admin/drivers", requireOps(principal.Handler(s.handlePending)))
	mux.Handle("PATCH /api/v1/admin/drivers/{id}", requireOps(principal.Handler(s.handleAdminPatch)))
}

func (s *Service) handleApply(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in ApplyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := s.Apply(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	p, err := s.Get(r.Context(), a.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (s *Service) handleOnline(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	st, err := s.GoOnline(r.Context(), a)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (s *Service) handleOffline(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	if err := s.GoOffline(r.Context(), a); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleLocation(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var body struct {
		Samples []location.Sample `json:"samples"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	n, err := s.Ingest(r.Context(), a, body.Samples)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"accepted": n})
}

func (s *Service) handlePending(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	items, err := s.Pending(r.Context(), 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleAdminPatch(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	var body struct {
		Action     string   `json:"action"` // approve | suspend
		Categories []string `json:"categories"`
		Reason     string   `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var p *Profile
	switch body.Action {
	case "approve":
		p, err = s.Approve(r.Context(), a, id, body.Categories)
	case "suspend":
		p, err = s.Suspend(r.Context(), a, id, body.Reason)
	default:
		err = httpx.ErrValidation.WithFields(map[string]string{"action": "invalid"})
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
