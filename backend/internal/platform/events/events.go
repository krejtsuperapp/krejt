// Package events — outbox-i transaksional (§41). Ngjarjet e domenit shkruhen në të NJËJTIN
// transaksion me ndryshimin e biznesit (pa dual-write) dhe publikohen nga worker-i në SNS
// (`domain-events`), prej ku radhët SQS (notifications, …) i marrin me fan-out (§43).
// Të qëndrueshme (tabelë), të gjurmueshme (id + agregat), të riprovueshme (backoff),
// idempotente te konsumatorët (id-ja e ngjarjes udhëton me mesazhin).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer — pgx.Tx ose *pgxpool.Pool.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Event — një ngjarje domeni ashtu siç del nga outbox-i dhe siç udhëton në SNS.
type Event struct {
	ID            uuid.UUID       `json:"id"`
	EventType     string          `json:"type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

// Emit shkruan ngjarjen në outbox BRENDA transaksionit të thirrësit. Nëse transaksioni
// rrëzohet, rrëzohet edhe ngjarja: asnjë ngjarje pa ndryshimin e saj, asnjë ndryshim pa ngjarjen.
func Emit(ctx context.Context, tx Execer, aggregateType, aggregateID, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal %s: %w", eventType, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)`, aggregateType, aggregateID, eventType, body); err != nil {
		return fmt.Errorf("events: emit %s: %w", eventType, err)
	}
	return nil
}

// Publisher — kanali i daljes: SNS në AWS; devlog VETËM në development.
type Publisher interface {
	Publish(ctx context.Context, ev Event) error
}
