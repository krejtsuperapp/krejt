package media

import (
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/media/upload-url", requireAuth(principal.Handler(s.handleUploadURL)))
	mux.Handle("POST /api/v1/media", requireAuth(principal.Handler(s.handleConfirm)))
	mux.Handle("DELETE /api/v1/media/{kind}", requireAuth(principal.Handler(s.handleRemove)))
	// Leximi publik nga vetë API-ja (MEDIA_BASE_URL = …/api/v1/media/objects) derisa mjedisi të
	// ketë CloudFront përpara. Çelësat janë uuid-e të papërsëritshme, ndaj cache-i është i gjatë.
	mux.HandleFunc("GET /api/v1/media/objects/{key...}", s.handleObject)
}

func (s *Service) handleObject(w http.ResponseWriter, r *http.Request) {
	body, info, err := s.Open(r.Context(), r.PathValue("key"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.SizeBytes, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
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
	out, err := s.Confirm(r.Context(), a, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

// DELETE /media/{kind}?target_id=… — heq logon/kopertinën/imazhin e produktit/foton e profilit.
func (s *Service) handleRemove(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var target *uuid.UUID
	if raw := r.URL.Query().Get("target_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"target_id": "invalid"}))
			return
		}
		target = &id
	}
	if err := s.Remove(r.Context(), a, r.PathValue("kind"), target); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
