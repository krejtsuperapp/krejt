// KREJT Worker — proceset në sfond (§41, §43): releja e outbox-it → SNS `domain-events`;
// më vonë konsumatorët e SQS (dispatch, notifications, payouts). Asgjë e simuluar.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/workers/outbox"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New("worker", cfg.Env)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseDSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis connect", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	publisher, err := events.NewPublisherFromEnv(ctx, cfg.Env, cfg.EventsPublisher, cfg.Region, cfg.DomainEventsTopicARN, log)
	if err != nil {
		log.Error("events publisher", "err", err)
		os.Exit(1)
	}

	log.Info("worker started", "env", cfg.Env, "events_publisher", cfg.EventsPublisher, "queues", cfg.QueueURLs)

	relay := outbox.New(pool, publisher, log)
	done := make(chan struct{})
	go func() {
		defer close(done)
		relay.Run(ctx)
	}()

	// heartbeat + kontroll i lidhjeve (§50): humbja e DB/Redis duket në log dhe alarm
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			<-done
			log.Info("worker stopped")
			return
		case <-t.C:
			if err := pool.Ping(ctx); err != nil {
				log.Warn("db ping", "err", err)
			}
			if err := rdb.Ping(ctx).Err(); err != nil {
				log.Warn("redis ping", "err", err)
			}
		}
	}
}
