// KREJT Worker — konsumatorët e SQS: outbox → SNS, dispatch, njoftimet, payout-et (§41, §43).
// Faza 0: skelet me lidhje DB/Redis dhe cikël pune të zbrazët por real (asgjë e simuluar).
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
	"krejt.app/backend/internal/platform/logx"
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

	log.Info("worker started", "env", cfg.Env, "queues", cfg.QueueURLs)

	// Cikli i punës: në Fazën 0 vetëm heartbeat + kontroll i lidhjeve.
	// Konsumatorët realë (outbox, dispatch, notifications, payouts) regjistrohen këtu fazë pas faze.
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
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
