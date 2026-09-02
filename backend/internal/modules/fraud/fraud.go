// Package fraud — fraud / risk v1 (§67): rregulla mbi ngjarjet e domenit që ngrenë flamuj risku për
// Operacionet (anulime të shpeshta të shoferit, anulime në seri të klientit, vlerësime në seri, mbushje
// të shpeshta, rimbursime të përsëritura) dhe kufij shpejtësie në Redis që zbatohen menjëherë
// (kërkesa udhëtimesh për orë). Flamujt nuk bllokojnë vetë: bllokimi është veprim i njeriut, me audit.
package fraud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var ErrVelocity = &httpx.APIError{Code: "TOO_MANY_REQUESTS", MessageKey: "errors.risk.velocity", HTTPStatus: http.StatusTooManyRequests, Retryable: true}

type Flag struct {
	ID          uuid.UUID      `json:"id"`
	UserID      uuid.UUID      `json:"user_id"`
	Kind        string         `json:"kind"`
	Severity    string         `json:"severity"`
	Score       int            `json:"score"`
	Details     map[string]any `json:"details"`
	SourceEvent *uuid.UUID     `json:"source_event"`
	Status      string         `json:"status"`
	Note        *string        `json:"note"`
	CreatedAt   time.Time      `json:"created_at"`
	ResolvedAt  *time.Time     `json:"resolved_at"`
}

type Service struct {
	pool *pgxpool.Pool
	rdb  redis.UniversalClient
	now  func() time.Time
}

func New(pool *pgxpool.Pool, rdb redis.UniversalClient) *Service {
	return &Service{pool: pool, rdb: rdb, now: time.Now}
}

// --- kufij shpejtësie (zbatim i menjëhershëm) ------------------------------------------

// Allow — numërues me dritare fikse për veprim/përdorues (p.sh. ride_request 10/orë). Fail-open pa Redis.
func (s *Service) Allow(ctx context.Context, userID uuid.UUID, action string, limit int, window time.Duration) error {
	if s.rdb == nil {
		return nil
	}
	slot := s.now().Unix() / int64(window.Seconds())
	key := fmt.Sprintf("risk:%s:%s:%d", action, userID, slot)
	pipe := s.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window+time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil
	}
	if incr.Val() > int64(limit) {
		return ErrVelocity
	}
	return nil
}

// --- rregullat mbi ngjarjet ---------------------------------------------------------------

func (s *Service) raise(ctx context.Context, userID uuid.UUID, kind, severity string, score int, details map[string]any, source uuid.UUID) error {
	raw, _ := json.Marshal(details)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO risk_flags (user_id, kind, severity, score, details, source_event)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (user_id, kind, source_event) DO NOTHING`, userID, kind, severity, score, raw, source)
	return err
}

// Handle — përpunues i outbox-it (si notifications/realtime/analytics): vlerëson rregullat për ngjarjen.
func (s *Service) Handle(ctx context.Context, ev events.Event) error {
	var p map[string]any
	if len(ev.Payload) > 0 {
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
	}
	id := func(k string) (uuid.UUID, bool) {
		v, _ := p[k].(string)
		u, err := uuid.Parse(v)
		return u, err == nil
	}
	switch ev.EventType {
	case "RideRequested":
		// shoferi që anulon shpesh: 7 ditët e fundit, ≥ 5 udhëtime të caktuara, ≥ 30 % anulime nga ai
		if p["reassign"] == true {
			if did, ok := id("previous_driver_id"); ok {
				return s.driverCancelRate(ctx, did, ev.ID)
			}
		}
	case "RideCancelled":
		if p["by"] == "customer" {
			if cid, ok := id("customer_id"); ok {
				return s.customerCancelBurst(ctx, cid, ev.ID)
			}
		}
	case "RideReviewed":
		if rid, ok := id("reviewer_id"); ok {
			if rating, _ := p["rating"].(float64); rating <= 1 {
				return s.reviewBurst(ctx, rid, ev.ID)
			}
		}
	case "WalletToppedUp":
		if uid, ok := id("user_id"); ok {
			return s.topupVelocity(ctx, uid, ev.ID)
		}
	}
	return nil
}

func (s *Service) driverCancelRate(ctx context.Context, driverID, source uuid.UUID) error {
	var assigned, cancelled int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE to_state = 'assigned'), count(*) FILTER (WHERE to_state = 'matching' AND from_state IN ('assigned','arrived') AND actor_type = 'driver')
		FROM ride_events WHERE actor_id = $1 AND created_at > now() - interval '7 days'`, driverID).Scan(&assigned, &cancelled); err != nil {
		return err
	}
	if assigned >= 5 && cancelled*100 >= assigned*30 {
		return s.raise(ctx, driverID, "driver_cancel_rate", "medium", cancelled*100/assigned,
			map[string]any{"assigned_7d": assigned, "cancelled_7d": cancelled}, source)
	}
	return nil
}

func (s *Service) customerCancelBurst(ctx context.Context, customerID, source uuid.UUID) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM rides WHERE customer_id = $1 AND cancelled_by = 'customer' AND cancelled_at > now() - interval '24 hours'`, customerID).Scan(&n); err != nil {
		return err
	}
	if n >= 5 {
		sev := "low"
		if n >= 10 {
			sev = "high"
		}
		return s.raise(ctx, customerID, "customer_cancel_burst", sev, n, map[string]any{"cancelled_24h": n}, source)
	}
	return nil
}

func (s *Service) reviewBurst(ctx context.Context, reviewerID, source uuid.UUID) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ride_reviews WHERE reviewer_id = $1 AND rating = 1 AND created_at > now() - interval '24 hours'`, reviewerID).Scan(&n); err != nil {
		return err
	}
	if n >= 3 {
		return s.raise(ctx, reviewerID, "review_burst", "low", n, map[string]any{"one_star_24h": n}, source)
	}
	return nil
}

func (s *Service) topupVelocity(ctx context.Context, userID, source uuid.UUID) error {
	var n int
	var sum int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*), COALESCE(SUM(amount_minor),0) FROM payment_intents
		WHERE user_id = $1 AND status = 'succeeded' AND succeeded_at > now() - interval '1 hour'`, userID).Scan(&n, &sum); err != nil {
		return err
	}
	if n >= 3 || sum >= 50000 {
		return s.raise(ctx, userID, "topup_velocity", "medium", n, map[string]any{"topups_1h": n, "sum_1h_minor": sum}, source)
	}
	return nil
}

// RefundPattern — thirret nga payments pas rimbursimit: ≥ 3 rimbursime në 30 ditë.
func (s *Service) RefundPattern(ctx context.Context, userID uuid.UUID, refundID uuid.UUID) error {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refunds r JOIN payment_intents i ON i.id = r.intent_id
		WHERE i.user_id = $1 AND r.status <> 'failed' AND r.created_at > now() - interval '30 days'`, userID).Scan(&n); err != nil {
		return err
	}
	if n >= 3 {
		return s.raise(ctx, userID, "refund_pattern", "medium", n, map[string]any{"refunds_30d": n}, refundID)
	}
	return nil
}

// --- Operacionet -------------------------------------------------------------------------------

const flagCols = `id, user_id, kind, severity, score, details, source_event, status, note, created_at, resolved_at`

func scanFlag(row pgx.Row) (*Flag, error) {
	var f Flag
	if err := row.Scan(&f.ID, &f.UserID, &f.Kind, &f.Severity, &f.Score, &f.Details, &f.SourceEvent, &f.Status, &f.Note, &f.CreatedAt, &f.ResolvedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Service) Flags(ctx context.Context, status string, userID *uuid.UUID, limit int) ([]Flag, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if status == "" {
		status = "open"
	}
	rows, err := s.pool.Query(ctx, `SELECT `+flagCols+` FROM risk_flags
		WHERE ($1 = 'all' OR status = $1) AND ($2::uuid IS NULL OR user_id = $2)
		ORDER BY CASE severity WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END, created_at DESC LIMIT $3`, status, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Flag{}
	for rows.Next() {
		f, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (s *Service) Resolve(ctx context.Context, ops principal.Actor, id uuid.UUID, status, note string) (*Flag, error) {
	if status != "reviewing" && status != "dismissed" && status != "confirmed" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"status": "invalid"})
	}
	var out *Flag
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		f, err := scanFlag(tx.QueryRow(ctx, `UPDATE risk_flags SET status = $2, note = NULLIF($3, ''), resolved_by = $4,
			resolved_at = CASE WHEN $2 IN ('dismissed','confirmed') THEN now() ELSE resolved_at END WHERE id = $1 RETURNING `+flagCols, id, status, note, ops.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = f
		meta, _ := json.Marshal(map[string]any{"status": status, "kind": f.Kind, "user_id": f.UserID})
		_, err = tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'risk.flag_resolved', 'risk_flag', $2, $3)`, ops.UserID, id.String(), meta)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Block — bllokon përdoruesin (status = blocked): sesionet shkyçen, shoferi del offline; Unblock e kthen.
func (s *Service) Block(ctx context.Context, ops principal.Actor, userID uuid.UUID, reason string, block bool) error {
	if block && reason == "" {
		return httpx.ErrValidation.WithFields(map[string]string{"reason": "required"})
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var tag string
		var err error
		if block {
			_, err = tx.Exec(ctx, `UPDATE users SET status = 'blocked', blocked_reason = $2, blocked_at = now(), blocked_by = $3, updated_at = now() WHERE id = $1 AND status = 'active'`, userID, reason, ops.UserID)
			tag = "user.blocked"
		} else {
			_, err = tx.Exec(ctx, `UPDATE users SET status = 'active', blocked_reason = NULL, blocked_at = NULL, blocked_by = NULL, updated_at = now() WHERE id = $1 AND status = 'blocked'`, userID)
			tag = "user.unblocked"
		}
		if err != nil {
			return err
		}
		if block {
			if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
		}
		meta, _ := json.Marshal(map[string]any{"reason": reason})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, $2, 'user', $3, $4)`, ops.UserID, tag, userID.String(), meta); err != nil {
			return err
		}
		evt := "UserBlocked"
		if !block {
			evt = "UserUnblocked"
		}
		return events.Emit(ctx, tx, "user", userID.String(), evt, map[string]any{"user_id": userID, "reason": reason, "by": ops.UserID})
	})
}

// --- HTTP ----------------------------------------------------------------------------------------

func (s *Service) Routes(mux *http.ServeMux, requireOps httpx.Middleware) {
	mux.Handle("GET /api/v1/admin/risk/flags", requireOps(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		q := r.URL.Query()
		var uid *uuid.UUID
		if id, err := uuid.Parse(q.Get("user_id")); err == nil {
			uid = &id
		}
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, err := s.Flags(r.Context(), q.Get("status"), uid, limit)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))
	mux.Handle("PATCH /api/v1/admin/risk/flags/{id}", requireOps(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		var in struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		f, err := s.Resolve(r.Context(), a, id, in.Status, in.Note)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, f)
	})))
	block := func(block bool) http.Handler {
		return requireOps(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
			id, err := uuid.Parse(r.PathValue("id"))
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrNotFound)
				return
			}
			var in struct {
				Reason string `json:"reason"`
			}
			if r.ContentLength != 0 {
				if err := httpx.DecodeJSON(r, &in); err != nil {
					httpx.WriteError(w, r, err)
					return
				}
			}
			if err := s.Block(r.Context(), a, id, in.Reason, block); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
	}
	mux.Handle("POST /api/v1/admin/users/{id}/block", block(true))
	mux.Handle("POST /api/v1/admin/users/{id}/unblock", block(false))
}
