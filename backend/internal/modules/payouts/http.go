package payouts

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func (s *Service) Routes(mux *http.ServeMux, requireDriver, requireFinance httpx.Middleware) {
	mux.Handle("GET /api/v1/driver/earnings", requireDriver(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		e, err := s.Earnings(r.Context(), a.UserID)
		respond(w, r, e, err)
	})))
	mux.Handle("GET /api/v1/driver/bank-account", requireDriver(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		b, err := s.BankAccount(r.Context(), a.UserID)
		respond(w, r, map[string]any{"bank_account": b}, err)
	})))
	mux.Handle("PUT /api/v1/driver/bank-account", requireDriver(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		var in BankAccountInput
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		b, err := s.SetBankAccount(r.Context(), a, in)
		respond(w, r, b, err)
	})))
	mux.Handle("GET /api/v1/driver/payouts", requireDriver(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id := a.UserID
		items, err := s.Items(r.Context(), nil, &id, 50)
		respond(w, r, map[string]any{"items": items}, err)
	})))

	mux.Handle("GET /api/v1/admin/payouts/batches", requireFinance(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := s.Batches(r.Context(), limit)
		respond(w, r, map[string]any{"items": items}, err)
	})))
	mux.Handle("POST /api/v1/admin/payouts/batches", requireFinance(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		var in struct {
			PeriodStart string `json:"period_start"`
			PeriodEnd   string `json:"period_end"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		start, err1 := time.Parse("2006-01-02", in.PeriodStart)
		end, err2 := time.Parse("2006-01-02", in.PeriodEnd)
		if err1 != nil || err2 != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"period": "invalid"}))
			return
		}
		b, err := s.CreateBatch(r.Context(), a, start, end)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, b)
	})))
	mux.Handle("GET /api/v1/admin/payouts/batches/{id}/items", requireFinance(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		items, err := s.Items(r.Context(), &id, nil, 500)
		respond(w, r, map[string]any{"items": items}, err)
	})))
	mux.Handle("GET /api/v1/admin/payouts/batches/{id}/export.csv", requireFinance(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		data, err := s.ExportCSV(r.Context(), a, id)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="krejt-payouts-`+id.String()[:8]+`.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	})))
	mux.Handle("PATCH /api/v1/admin/payouts/items/{id}", requireFinance(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		it, err := s.SettleItem(r.Context(), a, id, in.Status, in.Reason)
		respond(w, r, it, err)
	})))
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
