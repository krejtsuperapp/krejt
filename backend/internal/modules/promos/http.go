package promos

import (
	"net/http"
	"strconv"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — klienti kontrollon një kod para checkout-it; Operacionet i administrojnë kuponat.
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireOps httpx.Middleware) {
	mux.Handle("POST /api/v1/coupons/check", requireAuth(principal.Handler(s.handleCheck)))
	mux.Handle("GET /api/v1/admin/coupons", requireOps(principal.Handler(s.handleList)))
	mux.Handle("POST /api/v1/admin/coupons", requireOps(principal.Handler(s.handleUpsert)))
	mux.Handle("PATCH /api/v1/admin/coupons/{code}", requireOps(principal.Handler(s.handlePatch)))
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

type checkInput struct {
	Code        string `json:"code"`
	Scope       string `json:"scope"` // food | parcels
	AmountMinor int64  `json:"amount_minor"`
}

func (s *Service) handleCheck(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in checkInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if in.Scope != ScopeFood && in.Scope != ScopeParcels {
		httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"scope": "invalid"}))
		return
	}
	applied, err := s.Apply(r.Context(), in.Code, a.UserID, in.Scope, in.AmountMinor)
	respond(w, r, applied, err)
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.List(r.Context(), limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleUpsert(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	var in UpsertInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	c, err := s.Upsert(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, c)
}

func (s *Service) handlePatch(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	var in struct {
		Active *bool `json:"active"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if in.Active == nil {
		httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"active": "required"}))
		return
	}
	c, err := s.SetActive(r.Context(), r.PathValue("code"), *in.Active)
	respond(w, r, c, err)
}
