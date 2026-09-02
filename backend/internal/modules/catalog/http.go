package catalog

import (
	"net/http"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func (s *Service) Routes(mux *http.ServeMux, optionalAuth, requireAuth httpx.Middleware) {
	mux.Handle("GET /api/v1/merchants/{id}/menu", optionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		m, err := s.Menu(r.Context(), id, false)
		respond(w, r, m, err)
	})))

	mux.Handle("GET /api/v1/merchant/{id}/menu", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := s.requireStaff(r.Context(), a, id); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		m, err := s.Menu(r.Context(), id, true)
		respond(w, r, m, err)
	})))
	mux.Handle("POST /api/v1/merchant/{id}/categories", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		var in CategoryInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		c, err := s.UpsertCategory(r.Context(), a, id, nil, in)
		created(w, r, c, err)
	})))
	mux.Handle("PUT /api/v1/merchant/{id}/categories/{cat}", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		cat, err := uuid.Parse(r.PathValue("cat"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in CategoryInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		c, err := s.UpsertCategory(r.Context(), a, id, &cat, in)
		respond(w, r, c, err)
	})))
	mux.Handle("DELETE /api/v1/merchant/{id}/categories/{cat}", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		cat, err := uuid.Parse(r.PathValue("cat"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		noContent(w, r, s.DeleteCategory(r.Context(), a, id, cat))
	})))
	mux.Handle("POST /api/v1/merchant/{id}/products", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		var in ProductInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		p, err := s.UpsertProduct(r.Context(), a, id, nil, in)
		created(w, r, p, err)
	})))
	mux.Handle("PUT /api/v1/merchant/{id}/products/{product}", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		pid, err := uuid.Parse(r.PathValue("product"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in ProductInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		p, err := s.UpsertProduct(r.Context(), a, id, &pid, in)
		respond(w, r, p, err)
	})))
	mux.Handle("PATCH /api/v1/merchant/{id}/products/{product}/availability", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		pid, err := uuid.Parse(r.PathValue("product"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in struct {
			Available bool `json:"available"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		noContent(w, r, s.SetAvailability(r.Context(), a, id, pid, in.Available))
	})))
	mux.Handle("DELETE /api/v1/merchant/{id}/products/{product}", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := pathID(r)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		pid, err := uuid.Parse(r.PathValue("product"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		noContent(w, r, s.DeleteProduct(r.Context(), a, id, pid))
	})))
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

func created(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func noContent(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
