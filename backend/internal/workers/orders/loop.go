// Package orders (worker) — cikli i dorëzimeve: skadon ofertat e korrierëve, bën raundet e reja dhe
// riprovon shlyerjet e mbetura të porosive.
package orders

import (
	"context"
	"log/slog"
	"time"

	"krejt.app/backend/internal/modules/orders"
)

type Settler interface {
	SettlePending(ctx context.Context) (int, error)
}

type Loop struct {
	d        *orders.Dispatcher
	settler  Settler
	log      *slog.Logger
	Interval time.Duration
}

func New(d *orders.Dispatcher, settler Settler, log *slog.Logger) *Loop {
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
				l.log.Error("orders dispatch sweep", "err", err)
			} else if st.Expired > 0 || st.Offered > 0 {
				l.log.Info("orders dispatch sweep", "expired", st.Expired, "rounds", st.Rounds, "offered", st.Offered)
			}
			if time.Since(lastSettle) >= 10*time.Second {
				lastSettle = time.Now()
				if n, err := l.settler.SettlePending(ctx); err != nil && ctx.Err() == nil {
					l.log.Error("orders settle pending", "err", err)
				} else if n > 0 {
					l.log.Info("settled pending orders", "count", n)
				}
			}
		}
	}
}
