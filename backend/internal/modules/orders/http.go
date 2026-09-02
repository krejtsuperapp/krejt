package orders

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Routes — klienti (requireAuth), merchant-i (requireAuth + anëtarësi), korrieri (requireDriver).
func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireDriver httpx.Middleware) {
	mux.Handle("POST /api/v1/orders/quote", requireAuth(principal.Handler(s.handleQuote)))
	mux.Handle("POST /api/v1/orders", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/orders", requireAuth(principal.Handler(s.handleHistory)))
	mux.Handle("GET /api/v1/orders/{id}", requireAuth(principal.Handler(s.handleGet)))
	mux.Handle("POST /api/v1/orders/{id}/cancel", requireAuth(principal.Handler(s.handleCancel)))

	mux.Handle("GET /api/v1/merchant/{id}/orders", requireAuth(principal.Handler(s.handleMerchantQueue)))
	mux.Handle("POST /api/v1/merchant/orders/{id}/transition", requireAuth(principal.Handler(s.handleMerchantTransition)))

	mux.Handle("GET /api/v1/courier/offers", requireDriver(principal.Handler(s.handleOffers)))
	mux.Handle("POST /api/v1/courier/offers/{id}/accept", requireDriver(principal.Handler(s.handleAccept)))
	mux.Handle("POST /api/v1/courier/offers/{id}/decline", requireDriver(principal.Handler(s.handleDecline)))
	mux.Handle("GET /api/v1/courier/orders/active", requireDriver(principal.Handler(s.handleActive)))
	mux.Handle("POST /api/v1/courier/orders/{id}/pickup", requireDriver(principal.Handler(s.handlePickUp)))
	mux.Handle("POST /api/v1/courier/orders/{id}/deliver", requireDriver(principal.Handler(s.handleDeliver)))
	mux.Handle("POST /api/v1/courier/orders/{id}/release", requireDriver(principal.Handler(s.handleRelease)))
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

func (s *Service) handleQuote(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	var in CheckoutInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q, err := s.Quote(r.Context(), in)
	respond(w, r, q, err)
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in CheckoutInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.Create(r.Context(), a, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, o)
}

func (s *Service) handleHistory(w http.ResponseWriter, r *http.Request, a principal.Actor) {
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
	items, err := s.History(r.Context(), a, before, limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.Get(r.Context(), a, id)
	respond(w, r, o, err)
}

func (s *Service) handleCancel(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	o, err := s.CancelByCustomer(r.Context(), a, id, body.Reason)
	respond(w, r, o, err)
}

func (s *Service) handleMerchantQueue(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.MerchantQueue(r.Context(), a, id, r.URL.Query().Get("include_closed") == "true", limit)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleMerchantTransition(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		To          string `json:"to"`
		PrepTimeMin int    `json:"prep_time_min"`
		Reason      string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.MerchantTransition(r.Context(), a, id, in.To, in.PrepTimeMin, in.Reason)
	respond(w, r, o, err)
}

func (s *Service) handleOffers(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	items, err := s.Offers(r.Context(), a)
	respond(w, r, map[string]any{"items": items}, err)
}

func (s *Service) handleAccept(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.AcceptOffer(r.Context(), a, id)
	respond(w, r, o, err)
}

func (s *Service) handleDecline(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err == nil {
		err = s.DeclineOffer(r.Context(), a, id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleActive(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	o, err := s.ActiveForCourier(r.Context(), a)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"order": o})
}

func (s *Service) handlePickUp(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.PickUp(r.Context(), a, id, in.Code)
	respond(w, r, o, err)
}

func (s *Service) handleDeliver(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	o, err := s.Deliver(r.Context(), a, id)
	respond(w, r, o, err)
}

func (s *Service) handleRelease(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	o, err := s.ReleaseByCourier(r.Context(), a, id, body.Reason)
	respond(w, r, o, err)
}
