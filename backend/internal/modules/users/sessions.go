package users

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/platform/httpx"
)

// Session — pajisje e kyçur (§53 device/session management), siç e sheh pronari i llogarisë.
type Session struct {
	ID         uuid.UUID `json:"id"`
	DeviceName *string   `json:"device_name"`
	Platform   *string   `json:"platform"`
	IP         *string   `json:"ip"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	Current    bool      `json:"current"`
}

func (s *Service) Sessions(ctx context.Context, a Actor) ([]Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, device_name, platform, host(ip), last_seen_at, created_at FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND refresh_expires_at > now()
		ORDER BY last_seen_at DESC`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		if err := rows.Scan(&x.ID, &x.DeviceName, &x.Platform, &x.IP, &x.LastSeenAt, &x.CreatedAt); err != nil {
			return nil, err
		}
		x.Current = x.ID == a.SessionID
		out = append(out, x)
	}
	return out, rows.Err()
}

// RevokeSession shkyç një pajisje (edhe atë aktuale = logout). Refresh token-i i saj vdes menjëherë,
// access token-i refuzohet nga RequireAuth (kontrollon sesionin në çdo kërkesë).
func (s *Service) RevokeSession(ctx context.Context, a Actor, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, a.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.ErrNotFound
		}
		return audit(ctx, tx, a, "session.revoked", "session", id.String(), map[string]any{"self": id == a.SessionID})
	})
}
