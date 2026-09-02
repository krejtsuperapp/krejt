// Package maintenance — punët periodike të worker-it: skadimi i dokumenteve të shoferëve (çdo orë).
package maintenance

import (
	"context"
	"log/slog"
	"time"
)

type DocumentExpirer interface {
	ExpireSweep(ctx context.Context) (expired, suspended int, err error)
}

type Loop struct {
	docs     DocumentExpirer
	log      *slog.Logger
	Interval time.Duration
}

func New(docs DocumentExpirer, log *slog.Logger) *Loop {
	return &Loop{docs: docs, log: log, Interval: time.Hour}
}

func (l *Loop) Run(ctx context.Context) {
	l.tick(ctx) // edhe në nisje: worker-i mund të ketë qenë poshtë kur skadoi diçka
	t := time.NewTicker(l.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.tick(ctx)
		}
	}
}

func (l *Loop) tick(ctx context.Context) {
	expired, suspended, err := l.docs.ExpireSweep(ctx)
	if err != nil && ctx.Err() == nil {
		l.log.Error("documents expire sweep", "err", err)
		return
	}
	if expired > 0 || suspended > 0 {
		l.log.Info("documents expire sweep", "expired", expired, "drivers_suspended", suspended)
	}
}
