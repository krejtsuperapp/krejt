// Package outbox — releja e outbox-it (§41): lexon ngjarjet e papublikuara me
// FOR UPDATE SKIP LOCKED (disa worker-a paralelisht pa dyfishim), i publikon, i shënon.
// Dështimi i një ngjarjeje bllokon ngjarjet PASUESE të të njëjtit agregat në atë grup
// (renditja për agregat ruhet) dhe riprovohet me backoff eksponencial.
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

// Tick trajton një grup ngjarjesh; kthen sa u trajtuan (sukses + dështim).
func (r *Relay) Tick(ctx context.Context) (int, error) {
	handled := 0
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, attempts
			FROM outbox_events
			WHERE published_at IS NULL AND next_attempt_at <= now()
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, r.BatchSize)
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

		blocked := map[string]bool{} // agregatet me dështim në këtë grup
		for _, p := range batch {
			key := p.ev.AggregateType + ":" + p.ev.AggregateID
			if blocked[key] {
				continue
			}
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
			blocked[key] = true
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
