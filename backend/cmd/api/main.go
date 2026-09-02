// KREJT API — monolit modular në Go (§39). Hyrja HTTP e të gjitha aplikacioneve.
// Serveri është autoritar: klienti nuk dërgon kurrë çmim, bilanc, rol apo status (§51, §53).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/modules/documents"
	"krejt.app/backend/internal/modules/drivers"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/modules/notifications"
	"krejt.app/backend/internal/modules/payments"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/modules/realtime"
	"krejt.app/backend/internal/modules/reviews"
	"krejt.app/backend/internal/modules/rides"
	"krejt.app/backend/internal/modules/users"
	"krejt.app/backend/internal/modules/wallet"
	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/providers/maps"
	"krejt.app/backend/internal/platform/providers/payment"
	rtprovider "krejt.app/backend/internal/platform/providers/realtime"
	"krejt.app/backend/internal/platform/providers/sms"
	"krejt.app/backend/internal/platform/providers/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := logx.New("api", cfg.Env)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- varësitë ------------------------------------------------------------
	pool, err := db.Connect(ctx, cfg.DatabaseDSN())
	if err != nil {
		log.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Error("db migrate", "err", err)
		os.Exit(1)
	}

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		log.Error("redis connect", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// --- ofruesit (pas abstraksioneve) -------------------------------------------
	smsProvider, err := sms.NewFromEnv(cfg.Env, cfg.SMSProvider, cfg.InfobipBaseURL, cfg.InfobipAPIKey, cfg.InfobipSender, log)
	if err != nil {
		log.Error("sms provider", "err", err)
		os.Exit(1)
	}

	mapsProvider, err := maps.NewFromEnv(cfg.Env, cfg.MapsProvider, cfg.GoogleMapsKey, log)
	if err != nil {
		log.Error("maps provider", "err", err)
		os.Exit(1)
	}

	rtPub, err := rtprovider.NewFromEnv(cfg.Env, cfg.RealtimeProvider, cfg.CentrifugoAPIURL, cfg.CentrifugoAPIKey, log)
	if err != nil {
		log.Error("realtime provider", "err", err)
		os.Exit(1)
	}
	rtTokens, err := rtprovider.NewTokenIssuer(cfg.Env, cfg.CentrifugoTokenHMACSecret, log)
	if err != nil {
		log.Error("realtime tokens", "err", err)
		os.Exit(1)
	}

	store, err := storage.NewFromEnv(ctx, cfg.Env, cfg.StorageProvider, cfg.Region, cfg.AssetsBucket, cfg.DevFSDir, cfg.PublicBaseURL, log)
	if err != nil {
		log.Error("storage provider", "err", err)
		os.Exit(1)
	}

	payProvider, err := payment.NewFromEnv(cfg.Env, cfg.PaymentProvider, cfg.StripeSecretKey, cfg.StripeWebhookSecret, log)
	if err != nil {
		log.Error("payment provider", "err", err)
		os.Exit(1)
	}

	var signer *auth.Signer
	if cfg.JWTPrivateKeyPEM != "" {
		signer, err = auth.LoadSigner([]byte(cfg.JWTPrivateKeyPEM))
	} else if cfg.Env == "development" {
		log.Warn("DEV ONLY — JWT_PRIVATE_KEY missing, using ephemeral RSA key (tokens die on restart)")
		signer, err = auth.GenerateEphemeralSigner()
	} else {
		err = errors.New("JWT_PRIVATE_KEY is required outside development")
	}
	if err != nil {
		log.Error("jwt signer", "err", err)
		os.Exit(1)
	}
	pepper := []byte(cfg.OTPPepper)
	if len(pepper) < 16 {
		if cfg.Env != "development" {
			log.Error("OTP_PEPPER must be at least 16 bytes outside development")
			os.Exit(1)
		}
		pepper = []byte("dev-only-pepper-not-for-prod")
	}

	// --- modulet -----------------------------------------------------------------
	ledgerSvc := ledger.New(pool)
	authSvc := auth.New(pool, rdb, smsProvider, signer, ledgerSvc, pepper)
	locSvc := location.New(rdb, pool).WithRealtime(rtPub)
	pricingSvc := pricing.New(pool, mapsProvider, locSvc)
	docsSvc := documents.New(pool, store)
	driversSvc := drivers.New(pool, locSvc).WithEligibility(docsSvc)
	ridesSvc := rides.New(pool, locSvc, ledgerSvc, driversSvc, pricingSvc)
	requireAuth := authSvc.RequireAuth("")
	requireDriver := authSvc.RequireAnyCapability("RIDE_DRIVER", "TAXI_DRIVER")
	requireOps := authSvc.RequireAuth("OPERATIONS")
	requireFinance := authSvc.RequireAuth("FINANCE")

	// --- router ----------------------------------------------------------------
	mux := http.NewServeMux()
	health := httpx.NewHealth(pool, rdb)
	mux.HandleFunc("GET /healthz", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"service": "krejt-api", "env": cfg.Env, "version": cfg.Version, "region": cfg.Region,
		})
	})
	authSvc.Routes(mux)
	users.New(pool, ledgerSvc).Routes(mux, requireAuth)
	driversSvc.Routes(mux, requireAuth, requireDriver, requireOps)
	ridesSvc.Routes(mux, requireAuth, requireDriver)
	notifications.New(pool, nil).Routes(mux, requireAuth) // API-ja vetëm kutinë + token-at; push-in e dërgon worker-i
	realtime.New(pool, rtPub, rtTokens).Routes(mux, requireAuth)
	reviews.New(pool).Routes(mux, requireAuth)
	docsSvc.Routes(mux, requireAuth, requireOps)
	paymentsSvc := payments.New(pool, ledgerSvc, payProvider)
	paymentsSvc.Routes(mux, requireAuth, requireFinance)
	wallet.New(pool, ledgerSvc, wallet.Limits{MinTopUpMinor: payments.MinTopUpMinor, MaxTopUpMinor: payments.MaxTopUpMinor, DailyTopUpMinor: payments.DailyTopUpMinor}).Routes(mux, requireAuth)
	if dev, ok := payProvider.(*payment.DevLog); ok {
		paymentsSvc.DevRoutes(mux, dev) // vetëm development (devlog)
	}
	if fs, ok := store.(*storage.DevFS); ok {
		documents.DevRoutes(mux, fs) // vetëm development (devfs)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.ErrNotFound)
	})

	handler := httpx.Chain(mux,
		httpx.Recover(log),
		httpx.RequestID(),
		httpx.SecureHeaders(),
		httpx.Timeout(30*time.Second),
		httpx.AccessLog(log),
	)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api listening", "port", cfg.HTTPPort, "env", cfg.Env, "version", cfg.Version, "sms_provider", cfg.SMSProvider)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	// mbyllje e butë: ALB-ja ndalon dërgimin, kërkesat aktive përfundojnë (§44 graceful shutdown)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
	log.Info("api stopped")
}
