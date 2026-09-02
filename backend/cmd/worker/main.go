// KREJT Worker — proceset në sfond (§41, §43): releja e outbox-it → SNS `domain-events`, cikli i
// dispatch-it, konsumatori i radhës `notifications` (SQS) → push + kanalet e gjalla (Centrifugo).
// Asgjë e simuluar: në development pa AWS, ngjarjet kalojnë në proces te të njëjtët përpunues.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"krejt.app/backend/internal/modules/chat"
	"krejt.app/backend/internal/modules/dispatch"
	"krejt.app/backend/internal/modules/documents"
	"krejt.app/backend/internal/modules/drivers"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/modules/notifications"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/modules/realtime"
	"krejt.app/backend/internal/modules/rides"
	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/providers/maps"
	"krejt.app/backend/internal/platform/providers/push"
	rtprovider "krejt.app/backend/internal/platform/providers/realtime"
	"krejt.app/backend/internal/platform/providers/storage"
	dispatchworker "krejt.app/backend/internal/workers/dispatch"
	"krejt.app/backend/internal/workers/maintenance"
	"krejt.app/backend/internal/workers/outbox"
	"krejt.app/backend/internal/workers/queue"
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

	// --- ofruesit -----------------------------------------------------------------
	mapsProvider, err := maps.NewFromEnv(cfg.Env, cfg.MapsProvider, cfg.GoogleMapsKey, log)
	fatal(log, "maps provider", err)
	pushProvider, err := push.NewFromEnv(cfg.Env, cfg.PushProvider, cfg.FCMServiceAccountJSON, log)
	fatal(log, "push provider", err)
	rtPub, err := rtprovider.NewFromEnv(cfg.Env, cfg.RealtimeProvider, cfg.CentrifugoAPIURL, cfg.CentrifugoAPIKey, log)
	fatal(log, "realtime provider", err)
	store, err := storage.NewFromEnv(ctx, cfg.Env, cfg.StorageProvider, cfg.Region, cfg.AssetsBucket, cfg.DevFSDir, cfg.PublicBaseURL, log)
	fatal(log, "storage provider", err)

	// --- modulet ------------------------------------------------------------------
	locSvc := location.New(rdb, pool).WithRealtime(rtPub)
	ledgerSvc := ledger.New(pool)
	driversSvc := drivers.New(pool, locSvc)
	ridesSvc := rides.New(pool, locSvc, ledgerSvc, driversSvc, pricing.New(pool, mapsProvider, locSvc))
	dispatcher := dispatch.New(pool, locSvc, log)
	notifSvc := notifications.New(pool, pushProvider)
	rtSvc := realtime.New(pool, rtPub, nil) // worker-i vetëm publikon; token-at i lëshon API-ja

	// përpunuesit e ngjarjeve (të njëjtët në AWS përmes SQS dhe lokalisht në proces)
	handle := func(ctx context.Context, ev events.Event) error {
		if err := rtSvc.Handle(ctx, ev); err != nil {
			return err
		}
		return notifSvc.Handle(ctx, ev)
	}

	// --- ngjarjet: outbox → SNS (AWS) ose → përpunuesit në proces (development) ------
	publisher, err := events.NewPublisherFromEnv(ctx, cfg.Env, cfg.EventsPublisher, cfg.Region, cfg.DomainEventsTopicARN, log)
	fatal(log, "events publisher", err)
	if cfg.EventsPublisher == "devlog" {
		publisher = events.Fanout{publisher, events.HandlerPublisher{Name: "handlers", Fn: handle}}
	}

	log.Info("worker started", "env", cfg.Env, "events_publisher", cfg.EventsPublisher,
		"push_provider", cfg.PushProvider, "realtime_provider", cfg.RealtimeProvider, "queues", cfg.QueueURLs)

	var wg sync.WaitGroup
	run := func(name string, fn func(context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(ctx)
			log.Info("stopped", "loop", name)
		}()
	}
	run("outbox", outbox.New(pool, publisher, log).Run)
	run("dispatch", dispatchworker.New(dispatcher, ridesSvc, log).Run)
	run("maintenance", maintenance.New(log,
		maintenance.Job{Name: "documents.expire", Every: time.Hour, Run: func(ctx context.Context) (string, error) {
			e, s, err := documents.New(pool, store).ExpireSweep(ctx)
			return fmt.Sprintf("expired=%d suspended=%d", e, s), err
		}},
		maintenance.Job{Name: "chat.retention", Every: 6 * time.Hour, Run: func(ctx context.Context) (string, error) {
			n, err := chat.New(pool).RetentionSweep(ctx)
			return fmt.Sprintf("deleted=%d", n), err
		}},
	).Run)
	if url := cfg.QueueURLs["notifications"]; url != "" && cfg.EventsPublisher == "sns" {
		consumer, err := queue.New(ctx, cfg.Region, url, "notifications", handle, log)
		fatal(log, "sqs consumer", err)
		run("notifications", consumer.Run)
	} else if cfg.EventsPublisher == "sns" {
		log.Warn("SQS_NOTIFICATIONS_QUEUE_URL mungon: njoftimet/kanalet e gjalla nuk konsumohen nga ky worker")
	}

	// heartbeat + kontroll i lidhjeve (§50): humbja e DB/Redis duket në log dhe alarm
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
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

func fatal(log *slog.Logger, what string, err error) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
