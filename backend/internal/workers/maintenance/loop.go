// Package maintenance — punët periodike të worker-it (skadimi i dokumenteve, pastrimi i chat-it…):
// secila me intervalin e vet, ekzekutohet edhe në nisje (worker-i mund të ketë qenë poshtë).
package maintenance

import (
	"context"
	"log/slog"
	"time"
)

type Job struct {
	Name  string
	Every time.Duration
	Run   func(ctx context.Context) (summary string, err error)
}

type Loop struct {
	jobs []Job
	log  *slog.Logger
}

func New(log *slog.Logger, jobs ...Job) *Loop {
	return &Loop{jobs: jobs, log: log}
}

func (l *Loop) Run(ctx context.Context) {
	next := make([]time.Time, len(l.jobs))
	for i := range l.jobs {
		l.tick(ctx, i)
		next[i] = time.Now().Add(l.jobs[i].Every)
	}
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			for i := range l.jobs {
				if now.After(next[i]) {
					l.tick(ctx, i)
					next[i] = now.Add(l.jobs[i].Every)
				}
			}
		}
	}
}

func (l *Loop) tick(ctx context.Context, i int) {
	j := l.jobs[i]
	summary, err := j.Run(ctx)
	if err != nil && ctx.Err() == nil {
		l.log.Error("maintenance job failed", "job", j.Name, "err", err)
		return
	}
	if summary != "" && summary != "expired=0 suspended=0" && summary != "deleted=0" {
		l.log.Info("maintenance job", "job", j.Name, "result", summary)
	}
}
