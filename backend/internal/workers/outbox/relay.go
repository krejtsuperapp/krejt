// Package outbox — releja e outbox-it (§41): lexon ngjarjet e papublikuara me
// FOR UPDATE SKIP LOCKED (disa worker-a paralelisht pa dyfishim), i publikon, i shënon.
// Renditja për agregat është strikte: publikohet vetëm "koka" (ngjarja më e hershme e papublikuar)
// e çdo agregati; dështimi i saj mban pas edhe ngjarjet pasuese derisa të riprovohet me backoff.
package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/events"
)

type Relay struct {
	pool *pgxpool.Pool
	pub  events.Publisher
	log  *slog.Logger

	BatchSize int
	IdleSleep time.Duration
}

func New(pool *pgxpool.Pool, pub events.Publisher, log *slog.Logger) *Relay {
	return &Relay{pool: pool, pub: pub, log: log, BatchSize: 50, IdleSleep: time.Second}
}

// Run — cikli i worker-it derisa ctx të mbyllet.
func (r *Relay) Run(ctx context.Context) {
	for {
		n, err := r.Tick(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			r.log.Error("outbox tick", "err", err)
		}
		if n == 0 || err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.IdleSleep):
			}
		}
	}
}

type pending struct {
	ev       events.Event
	attempts int
}

// Tick trajton një grup ngjarjesh (vetëm kokat e agregateve); kthen sa u trajtuan (sukses + dështim).
func (r *Relay) Tick(ctx context.Context) (int, error) {
	handled := 0
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT e.id, e.aggregate_type, e.aggregate_id, e.event_type, e.payload, e.created_at, e.attempts
			FROM outbox_events e
			WHERE e.published_at IS NULL
			  AND e.next_attempt_at <= now()
			  AND NOT EXISTS (
			        SELECT 1 FROM outbox_events p
			        WHERE p.published_at IS NULL
			          AND p.aggregate_type = e.aggregate_type AND p.aggregate_id = e.aggregate_id
			          AND p.seq < e.seq)
			ORDER BY e.seq
			LIMIT $1
			FOR UPDATE OF e SKIP LOCKED`, r.BatchSize)
		if err != nil {
			return err
		}
		var batch []pending
		for rows.Next() {
			var p pending
			var payload []byte
			if err := rows.Scan(&p.ev.ID, &p.ev.AggregateType, &p.ev.AggregateID, &p.ev.EventType,
				&payload, &p.ev.OccurredAt, &p.attempts); err != nil {
				rows.Close()
				return err
			}
			p.ev.Payload = payload
			batch = append(batch, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, p := range batch {
			handled++
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			perr := r.pub.Publish(pctx, p.ev)
			cancel()
			if perr == nil {
				if _, err := tx.Exec(ctx,
					`UPDATE outbox_events SET published_at = now(), last_error = NULL WHERE id = $1`, p.ev.ID); err != nil {
					return err
				}
				continue
			}
			attempt := p.attempts + 1
			delay := Backoff(attempt)
			lvl := slog.LevelWarn
			if attempt >= 10 { // alarmi në Grafana lexon këtë nivel (§50)
				lvl = slog.LevelError
			}
			r.log.Log(ctx, lvl, "outbox publish failed",
				"id", p.ev.ID, "type", p.ev.EventType, "attempt", attempt, "retry_in", delay, "err", perr)
			if _, err := tx.Exec(ctx, `
				UPDATE outbox_events
				SET attempts = attempts + 1,
				    next_attempt_at = now() + make_interval(secs => $2::double precision),
				    last_error = left($3, 500)
				WHERE id = $1`, p.ev.ID, delay.Seconds(), perr.Error()); err != nil {
				return err
			}
		}
		return nil
	})
	return handled, err
}

// Drain — thërret Tick derisa të mos mbetet asgjë e gatshme (për teste dhe mbyllje të butë).
func (r *Relay) Drain(ctx context.Context) error {
	for {
		n, err := r.Tick(ctx)
		if err != nil || n == 0 {
			return err
		}
	}
}

// Backoff: 2 s, 4 s, 8 s, … deri në 10 min.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		return 10 * time.Minute
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}
