package documents

import (
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

func (s *Service) Routes(mux *http.ServeMux, requireAuth, requireOps httpx.Middleware) {
	mux.Handle("POST /api/v1/driver/documents/upload-url", requireAuth(principal.Handler(s.handleUploadURL)))
	mux.Handle("POST /api/v1/driver/documents", requireAuth(principal.Handler(s.handleConfirm)))
	mux.Handle("GET /api/v1/driver/documents", requireAuth(principal.Handler(s.handleMine)))
	mux.Handle("GET /api/v1/admin/driver-documents", requireOps(principal.Handler(s.handlePending)))
	mux.Handle("GET /api/v1/admin/drivers/{id}/documents", requireOps(principal.Handler(s.handleForDriver)))
	mux.Handle("PATCH /api/v1/admin/driver-documents/{id}", requireOps(principal.Handler(s.handleReview)))
}

// DevRoutes — VETËM development me STORAGE_PROVIDER=devfs: ngarkimi/leximi lokal i objekteve.
func DevRoutes(mux *http.ServeMux, fs *storage.DevFS) {
	mux.HandleFunc("PUT /api/v1/dev/uploads/{key...}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if !storage.ValidKey(key) {
			httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"key": "invalid"}))
			return
		}
		ct := strings.Split(r.Header.Get("Content-Type"), ";")[0]
		if err := fs.Put(key, strings.TrimSpace(ct), io.LimitReader(r.Body, MaxSizeBytes+1)); err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.With(err).WithFields(map[string]string{"upload": "rejected"}))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/dev/uploads/{key...}", func(w http.ResponseWriter, r *http.Request) {
		f, ct, err := fs.Open(r.PathValue("key"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", ct)
		_, _ = io.Copy(w, f)
	})
}

func (s *Service) handleUploadURL(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in UploadRequest
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := s.UploadURL(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (s *Service) handleConfirm(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in ConfirmRequest
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	d, err := s.Confirm(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

func (s *Service) handleMine(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	ov, err := s.List(r.Context(), a.UserID, true)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ov)
}

func (s *Service) handlePending(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	items, err := s.Pending(r.Context(), 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Service) handleForDriver(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	ov, err := s.List(r.Context(), id, true)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ov)
}

func (s *Service) handleReview(w http.ResponseWriter, r *http.Request, a principal.Actor) {
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
	d, err := s.Review(r.Context(), a, id, in.Action, in.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}
