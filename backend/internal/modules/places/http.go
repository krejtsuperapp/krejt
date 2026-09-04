package places

import (
	"net/http"
	"strconv"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — të gjitha kërkojnë kyçje: kufizuesi për përdorues i mbron kuotat e ofruesit.
func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("GET /api/v1/places/search", requireAuth(principal.Handler(s.handleSearch)))
	mux.Handle("GET /api/v1/places/reverse", requireAuth(principal.Handler(s.handleReverse)))
	mux.Handle("POST /api/v1/places/route", requireAuth(principal.Handler(s.handleRoute)))
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func queryPoint(r *http.Request) *geo.Point {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(q.Get("lng"), 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	return &geo.Point{Lat: lat, Lng: lng}
}

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	items, err := s.Search(r.Context(), q.Get("q"), queryPoint(r), limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleReverse(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	p := queryPoint(r)
	if p == nil {
		httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"lat": "required", "lng": "required"}))
		return
	}
	place, err := s.Reverse(r.Context(), *p)
	respond(w, r, map[string]any{"place": place}, err)
}

type routeInput struct {
	From geo.Point `json:"from"`
	To   geo.Point `json:"to"`
}

func (s *Service) handleRoute(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	var in routeInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	route, err := s.RoutePath(r.Context(), in.From, in.To)
	respond(w, r, route, err)
}
