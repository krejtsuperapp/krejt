package merchants

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — publike (zbulim, profil), staf (requireAuth + anëtarësi e verifikuar në shërbim), Operacionet.
func (s *Service) Routes(mux *http.ServeMux, optionalAuth, requireAuth, requireOps httpx.Middleware) {
	mux.Handle("GET /api/v1/merchants", optionalAuth(http.HandlerFunc(s.handleDiscover)))
	mux.Handle("GET /api/v1/merchants/{slug}", optionalAuth(http.HandlerFunc(s.handleBySlug)))

	mux.Handle("POST /api/v1/merchant/apply", requireAuth(principal.Handler(s.handleApply)))
	mux.Handle("GET /api/v1/merchant/mine", requireAuth(principal.Handler(s.handleMine)))
	mux.Handle("GET /api/v1/merchant/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("PATCH /api/v1/merchant/{id}", requireAuth(principal.Handler(s.handleUpdate)))
	mux.Handle("PUT /api/v1/merchant/{id}/hours", requireAuth(principal.Handler(s.handleHours)))
	mux.Handle("POST /api/v1/merchant/{id}/staff", requireAuth(principal.Handler(s.handleAddStaff)))
	mux.Handle("DELETE /api/v1/merchant/{id}/staff/{user_id}", requireAuth(principal.Handler(s.handleRemoveStaff)))

	mux.Handle("GET /api/v1/admin/merchants", requireOps(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := s.Pending(r.Context(), limit)
		respond(w, r, map[string]any{"items": items}, err)
	})))
	mux.Handle("PATCH /api/v1/admin/merchants/{id}", requireOps(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		m, err := s.SetStatus(r.Context(), a, id, in.Action, in.Reason)
		respond(w, r, m, err)
	})))
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

func (s *Service) handleDiscover(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := DiscoverFilter{Type: q.Get("type"), Query: q.Get("q"), Cuisine: q.Get("cuisine")}
	if lat, err1 := strconv.ParseFloat(q.Get("lat"), 64); err1 == nil {
		if lng, err2 := strconv.ParseFloat(q.Get("lng"), 64); err2 == nil {
			f.At = &geo.Point{Lat: lat, Lng: lng}
		}
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	items, err := s.Discover(r.Context(), f)
	respond(w, r, map[string]any{"items": items, "types": Types}, err)
}

func (s *Service) handleBySlug(w http.ResponseWriter, r *http.Request) {
	m, err := s.BySlug(r.Context(), r.PathValue("slug"))
	respond(w, r, m, err)
}

func (s *Service) handleApply(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in ApplyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.Apply(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

func (s *Service) handleMine(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	items, err := s.Mine(r.Context(), a)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.Get(r.Context(), a, id)
	respond(w, r, m, err)
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in ProfileUpdate
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	m, err := s.UpdateProfile(r.Context(), a, id, in)
	respond(w, r, m, err)
}

func (s *Service) handleHours(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Hours []Hours `json:"hours"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hours, err := s.SetHours(r.Context(), a, id, in.Hours)
	respond(w, r, map[string]any{"hours": hours}, err)
}

func (s *Service) handleAddStaff(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Phone string `json:"phone"`
		Role  string `json:"role"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.AddStaff(r.Context(), a, id, in.Phone, in.Role); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleRemoveStaff(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	uid, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	if err := s.RemoveStaff(r.Context(), a, id, uid); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
