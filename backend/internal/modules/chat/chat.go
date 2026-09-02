// Package chat — chat klient ↔ shofer brenda udhëtimit (§28): tekst me kohë dhe gjendje leximi,
// vetëm mes palëve të udhëtimit, gjatë udhëtimit dhe deri 24 h pas përfundimit; dorëzimi në kohë
// reale përmes kanalit `ride:{id}` dhe push te marrësi (ngjarje `RideChatMessage`). Raportimi bëhet
// me tiketë mbështetjeje (ride_id); mesazhet pastrohen pas 90 ditësh.
package chat

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var ErrChatClosed = &httpx.APIError{Code: "CHAT_CLOSED", MessageKey: "errors.chat.closed", HTTPStatus: http.StatusConflict}

const (
	Window    = 24 * time.Hour      // pas përfundimit
	Retention = 90 * 24 * time.Hour // §28 retention
	MaxBody   = 500
)

type Message struct {
	ID         uuid.UUID  `json:"id"`
	RideID     uuid.UUID  `json:"ride_id"`
	SenderID   uuid.UUID  `json:"sender_id"`
	SenderRole string     `json:"sender_role"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at"`
	Mine       bool       `json:"mine"`
}

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

type participant struct {
	role        string
	recipientID uuid.UUID
	open        bool
}

func (s *Service) participant(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, a principal.Actor, rideID uuid.UUID) (*participant, error) {
	var customerID uuid.UUID
	var driverID *uuid.UUID
	var state string
	var completedAt, cancelledAt *time.Time
	err := q.QueryRow(ctx, `SELECT customer_id, driver_id, state, completed_at, cancelled_at FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2)`, rideID, a.UserID).
		Scan(&customerID, &driverID, &state, &completedAt, &cancelledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if driverID == nil {
		return nil, ErrChatClosed // ende pa shofer: s'ka me kë të flasë
	}
	p := &participant{role: "customer", recipientID: *driverID}
	if a.UserID == *driverID {
		p.role, p.recipientID = "driver", customerID
	}
	switch state {
	case "assigned", "arrived", "in_progress":
		p.open = true
	case "completed":
		p.open = completedAt != nil && s.now().Sub(*completedAt) <= Window
	case "cancelled":
		p.open = cancelledAt != nil && s.now().Sub(*cancelledAt) <= time.Hour
	}
	return p, nil
}

// Send — mesazh nga njëra palë te tjetra; ngjarja mbart parapamjen (push) dhe id-në (kanali live).
func (s *Service) Send(ctx context.Context, a principal.Actor, rideID uuid.UUID, body string) (*Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > MaxBody {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"body": "invalid"})
	}
	var out Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := s.participant(ctx, tx, a, rideID)
		if err != nil {
			return err
		}
		if !p.open {
			return ErrChatClosed
		}
		if err := tx.QueryRow(ctx, `INSERT INTO ride_chat_messages (ride_id, sender_id, sender_role, body) VALUES ($1, $2, $3, $4)
			RETURNING id, ride_id, sender_id, sender_role, body, created_at, read_at`, rideID, a.UserID, p.role, body).
			Scan(&out.ID, &out.RideID, &out.SenderID, &out.SenderRole, &out.Body, &out.CreatedAt, &out.ReadAt); err != nil {
			return err
		}
		out.Mine = true
		preview := body
		if r := []rune(preview); len(r) > 80 {
			preview = string(r[:80]) + "…"
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideChatMessage", map[string]any{
			"ride_id": rideID, "message_id": out.ID, "sender_id": a.UserID, "sender_role": p.role, "recipient_id": p.recipientID, "preview": preview, "created_at": out.CreatedAt})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// List — mesazhet e udhëtimit (pas `after` nëse jepet); mesazhet e palës tjetër shënohen si të lexuara.
func (s *Service) List(ctx context.Context, a principal.Actor, rideID uuid.UUID, after *time.Time, limit int) ([]Message, error) {
	if _, err := s.participant(ctx, s.pool, a, rideID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if after == nil {
		t := time.Time{}
		after = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT id, ride_id, sender_id, sender_role, body, created_at, read_at FROM ride_chat_messages
		WHERE ride_id = $1 AND created_at > $2 ORDER BY created_at LIMIT $3`, rideID, *after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RideID, &m.SenderID, &m.SenderRole, &m.Body, &m.CreatedAt, &m.ReadAt); err != nil {
			return nil, err
		}
		m.Mine = m.SenderID == a.UserID
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE ride_chat_messages SET read_at = now() WHERE ride_id = $1 AND sender_id <> $2 AND read_at IS NULL`, rideID, a.UserID); err != nil {
		return nil, err
	}
	return out, nil
}

// Unread — sa mesazhe të palëxuara ka aktori në këtë udhëtim (për shenjën në ekran).
func (s *Service) Unread(ctx context.Context, a principal.Actor, rideID uuid.UUID) (int, error) {
	if _, err := s.participant(ctx, s.pool, a, rideID); err != nil {
		return 0, err
	}
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ride_chat_messages WHERE ride_id = $1 AND sender_id <> $2 AND read_at IS NULL`, rideID, a.UserID).Scan(&n)
	return n, err
}

// RetentionSweep — fshin mesazhet më të vjetra se 90 ditë (§28). Thirret nga worker-i.
func (s *Service) RetentionSweep(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ride_chat_messages WHERE created_at < $1`, s.now().Add(-Retention))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// --- HTTP ---------------------------------------------------------------------------

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("GET /api/v1/rides/{id}/chat", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var after *time.Time
		if v := r.URL.Query().Get("after"); v != "" {
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"after": "invalid"}))
				return
			}
			after = &t
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := s.List(r.Context(), a, id, after, limit)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "max_body": MaxBody})
	})))
	mux.Handle("POST /api/v1/rides/{id}/chat", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		m, err := s.Send(r.Context(), a, id, in.Body)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, m)
	})))
}
