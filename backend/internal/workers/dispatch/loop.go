// Package dispatch (worker) — cikli i dispatch-it: çdo sekondë skadon ofertat, bën raundet e reja
// dhe riprovon shlyerjet e mbetura. Ngjarjet e udhëtimeve dalin përmes outbox-it si gjithçka tjetër.
package dispatch

import (
	"context"
	"log/slog"
	"time"

	"krejt.app/backend/internal/modules/dispatch"
)

type Settler interface {
	SettlePending(ctx context.Context) (int, error)
}

type Loop struct {
	d        *dispatch.Dispatcher
	settler  Settler
	log      *slog.Logger
	Interval time.Duration
}

func New(d *dispatch.Dispatcher, settler Settler, log *slog.Logger) *Loop {
	return &Loop{d: d, settler: settler, log: log, Interval: time.Second}
}

func (l *Loop) Run(ctx context.Context) {
	t := time.NewTicker(l.Interval)
	defer t.Stop()
	lastSettle := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st, err := l.d.Sweep(ctx)
			if err != nil && ctx.Err() == nil {
				l.log.Error("dispatch sweep", "err", err)
			} else if st.Expired > 0 || st.Offered > 0 {
				l.log.Info("dispatch sweep", "expired", st.Expired, "rounds", st.Rounds, "offered", st.Offered)
			}
			if time.Since(lastSettle) >= 10*time.Second {
				lastSettle = time.Now()
				if n, err := l.settler.SettlePending(ctx); err != nil && ctx.Err() == nil {
					l.log.Error("settle pending", "err", err)
				} else if n > 0 {
					l.log.Info("settled pending rides", "count", n)
				}
			}
		}
	}
}
