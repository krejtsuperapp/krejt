// Package parcels (worker) — cikli i pakove: skadon ofertat e korrierëve, bën raundet e reja dhe
// riprovon shlyerjet e mbetura.
package parcels

import (
	"context"
	"log/slog"
	"time"

	"krejt.app/backend/internal/modules/parcels"
)

type Settler interface {
	SettlePending(ctx context.Context) (int, error)
}

type Loop struct {
	d        *parcels.Dispatcher
	settler  Settler
	log      *slog.Logger
	Interval time.Duration
}

func New(d *parcels.Dispatcher, settler Settler, log *slog.Logger) *Loop {
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
				l.log.Error("parcels dispatch sweep", "err", err)
			} else if st.Expired > 0 || st.Offered > 0 {
				l.log.Info("parcels dispatch sweep", "expired", st.Expired, "rounds", st.Rounds, "offered", st.Offered)
			}
			if time.Since(lastSettle) >= 10*time.Second {
				lastSettle = time.Now()
				if n, err := l.settler.SettlePending(ctx); err != nil && ctx.Err() == nil {
					l.log.Error("parcels settle pending", "err", err)
				} else if n > 0 {
					l.log.Info("settled pending parcels", "count", n)
				}
			}
		}
	}
}
