// Package notifications — njoftimet (§29, §47): kutia në aplikacion, token-at e push-it (regjistrim,
// rifreskim, të pavlefshëm), dorëzimi përmes PushProvider sipas preferencave dhe gjuhës së pajisjes.
// Burimi i ngjarjeve: outbox → SNS → SQS `notifications` (worker) — kurrë thirrje direkte nga modulet.
package notifications

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/push"
)

type Service struct {
	pool *pgxpool.Pool
	push push.Provider
}

func New(pool *pgxpool.Pool, p push.Provider) *Service {
	return &Service{pool: pool, push: p}
}

// --- token-at e push-it ---------------------------------------------------------

type RegisterTokenInput struct {
	Platform string `json:"platform"` // ios | android | web
	Token    string `json:"token"`
	Locale   string `json:"locale"`
}

// RegisterToken — regjistron/rifreskon token-in e pajisjes; i njëjti token kalon te përdoruesi aktual
// (pajisje e ndarë ose ri-kyçje) dhe rikthehet i vlefshëm.
func (s *Service) RegisterToken(ctx context.Context, a principal.Actor, in RegisterTokenInput) error {
	fields := map[string]string{}
	in.Token = strings.TrimSpace(in.Token)
	if len(in.Token) < 20 || len(in.Token) > 4096 {
		fields["token"] = "invalid"
	}
	switch in.Platform {
	case "ios", "android", "web":
	default:
		fields["platform"] = "invalid"
	}
	if in.Locale == "" {
		in.Locale = "sq"
	}
	if in.Locale != "sq" && in.Locale != "en" && in.Locale != "de" {
		fields["locale"] = "invalid"
	}
	if len(fields) > 0 {
		return httpx.ErrValidation.WithFields(fields)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_tokens (user_id, session_id, platform, token, locale)
		VALUES ($1, (SELECT id FROM sessions WHERE id = $2), $3, $4, $5)
		ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, session_id = EXCLUDED.session_id,
		  platform = EXCLUDED.platform, locale = EXCLUDED.locale, invalid_at = NULL, updated_at = now()`,
		a.UserID, a.SessionID, in.Platform, in.Token, in.Locale)
	return err
}

// RemoveToken — çregjistrim (dalje nga pajisja).
func (s *Service) RemoveToken(ctx context.Context, a principal.Actor, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM push_tokens WHERE user_id = $1 AND token = $2`, a.UserID, strings.TrimSpace(token))
	return err
}

// --- kutia e njoftimeve ---------------------------------------------------------

type Notification struct {
	ID        uuid.UUID         `json:"id"`
	Category  string            `json:"category"`
	TitleKey  string            `json:"title_key"`
	BodyKey   string            `json:"body_key"`
	Params    map[string]string `json:"params"`
	DeepLink  *string           `json:"deep_link"`
	ReadAt    *time.Time        `json:"read_at"`
	CreatedAt time.Time         `json:"created_at"`
}

type Inbox struct {
	Items  []Notification `json:"items"`
	Unread int            `json:"unread"`
}

func (s *Service) List(ctx context.Context, a principal.Actor, before *time.Time, limit int) (*Inbox, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if before == nil {
		t := time.Now().Add(time.Hour)
		before = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT id, category, title_key, body_key, params, deep_link, read_at, created_at
		FROM notifications WHERE user_id = $1 AND created_at < $2 ORDER BY created_at DESC LIMIT $3`, a.UserID, *before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &Inbox{Items: []Notification{}}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Category, &n.TitleKey, &n.BodyKey, &n.Params, &n.DeepLink, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, a.UserID).Scan(&out.Unread); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) MarkRead(ctx context.Context, a principal.Actor, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, id, a.UserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2)`, id, a.UserID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return httpx.ErrNotFound
		}
	}
	return nil
}

func (s *Service) MarkAllRead(ctx context.Context, a principal.Actor) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, a.UserID)
	return err
}

// --- HTTP ------------------------------------------------------------------------

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/notifications/push-token", requireAuth(principal.Handler(s.handleRegisterToken)))
	mux.Handle("DELETE /api/v1/notifications/push-token", requireAuth(principal.Handler(s.handleRemoveToken)))
	mux.Handle("GET /api/v1/notifications", requireAuth(principal.Handler(s.handleList)))
	mux.Handle("POST /api/v1/notifications/read-all", requireAuth(principal.Handler(s.handleReadAll)))
	mux.Handle("POST /api/v1/notifications/{id}/read", requireAuth(principal.Handler(s.handleRead)))
}

func (s *Service) handleRegisterToken(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in RegisterTokenInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.RegisterToken(r.Context(), a, in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleRemoveToken(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in struct {
		Token string `json:"token"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.RemoveToken(r.Context(), a, in.Token); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var before *time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"before": "invalid"}))
			return
		}
		before = &t
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		for _, c := range v {
			if c < '0' || c > '9' {
				limit = -1
				break
			}
			limit = limit*10 + int(c-'0')
		}
	}
	inbox, err := s.List(r.Context(), a, before, limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, inbox)
}

func (s *Service) handleRead(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	if err := s.MarkRead(r.Context(), a, id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleReadAll(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	if err := s.MarkAllRead(r.Context(), a); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
