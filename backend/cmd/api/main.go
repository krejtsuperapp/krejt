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

	"krejt.app/backend/internal/modules/admin"
	"krejt.app/backend/internal/modules/appconfig"
	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/modules/catalog"
	"krejt.app/backend/internal/modules/chat"
	"krejt.app/backend/internal/modules/dataexport"
	"krejt.app/backend/internal/modules/documents"
	"krejt.app/backend/internal/modules/drivers"
	"krejt.app/backend/internal/modules/fraud"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/modules/media"
	"krejt.app/backend/internal/modules/merchants"
	"krejt.app/backend/internal/modules/notifications"
	"krejt.app/backend/internal/modules/orders"
	"krejt.app/backend/internal/modules/payments"
	"krejt.app/backend/internal/modules/payouts"
	"krejt.app/backend/internal/modules/places"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/modules/realtime"
	"krejt.app/backend/internal/modules/reviews"
	"krejt.app/backend/internal/modules/rides"
	"krejt.app/backend/internal/modules/support"
	"krejt.app/backend/internal/modules/users"
	"krejt.app/backend/internal/modules/wallet"
	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/errtrack"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/httpx/openapi"
	"krejt.app/backend/internal/platform/logx"
	mediaurl "krejt.app/backend/internal/platform/media"
	otelx "krejt.app/backend/internal/platform/otel"
	"krejt.app/backend/internal/platform/providers/maps"
	"krejt.app/backend/internal/platform/providers/payment"
	rtprovider "krejt.app/backend/internal/platform/providers/realtime"
	"krejt.app/backend/internal/platform/providers/sms"
	"krejt.app/backend/internal/platform/providers/storage"
	"krejt.app/backend/internal/platform/ratelimit"

	"github.com/redis/go-redis/extra/redisotel/v9"
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

	// --- observability (§50): OpenTelemetry + Sentry ---------------------------------
	otelShutdown, err := otelx.Init(ctx, "krejt-api", cfg.Env, cfg.Version, log)
	if err != nil {
		log.Error("otel", "err", err)
		os.Exit(1)
	}
	defer func() { _ = otelShutdown(context.Background()) }()
	sentryFlush, err := errtrack.Init(cfg.SentryDSN, cfg.Env, cfg.Version, "krejt-api", log)
	if err != nil {
		log.Error("sentry", "err", err)
		os.Exit(1)
	}
	defer sentryFlush()
	httpx.SetErrorReporter(func(ctx context.Context, err error) { errtrack.Report(ctx, err, nil) })
	db.QueryTracer = otelx.PgxTracer{}

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
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		log.Warn("redis otel", "err", err)
	}

	// --- ofruesit (pas abstraksioneve) -------------------------------------------
	smsProvider, err := sms.NewFromEnv(cfg.Env, cfg.SMSProvider, cfg.InfobipBaseURL, cfg.InfobipAPIKey, cfg.InfobipSender, log)
	if err != nil {
		log.Error("sms provider", "err", err)
		os.Exit(1)
	}

	mapsProvider, err := maps.NewFromEnv(cfg.Env, cfg.MapsProvider, cfg.GoogleMapsKey, cfg.MapboxToken, log)
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
	// Imazhet publike rrinë në bucket të veçantë (lexim përmes CloudFront-it); me devfs ndajnë
	// të njëjtën dosje lokale, që rrugët e dev-it të mbeten një.
	mediaStore := store
	if cfg.StorageProvider != "devfs" && cfg.MediaBucket != "" {
		mediaStore, err = storage.NewS3(ctx, cfg.Region, cfg.MediaBucket)
		if err != nil {
			log.Error("media storage", "err", err)
			os.Exit(1)
		}
	}
	mediaurl.SetBaseURL(cfg.MediaBaseURL)
	if cfg.MediaBaseURL == "" {
		log.Warn("MEDIA_BASE_URL mungon: imazhet publike nuk do të kenë URL")
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
	authSvc := auth.New(pool, rdb, smsProvider, signer, ledgerSvc, pepper).
		WithDevTestPhones(cfg.DevTestPhones, cfg.DevTestAdminPhones, cfg.DevTestOTP)
	if len(cfg.DevTestPhones) > 0 {
		log.Warn("VETËM DEV — numra prove me kod fiks", "phones", cfg.DevTestPhones)
	}
	// Ndezja fillestare e stafit: pa të, asnjë administrator nuk lind kurrë, sepse të drejtat
	// jepen vetëm nga një administrator ekzistues. Vepron një herë të vetme dhe hesht më pas.
	if granted, err := authSvc.BootstrapAdmin(ctx, cfg.BootstrapAdminPhone); err != nil {
		// Nuk e ndal serverin: një numër i shkruar gabim nuk duhet ta lërë API-në pa u ngritur.
		log.Warn("bootstrap admin", "err", err)
	} else if granted {
		log.Info("bootstrap admin: SUPER_ADMIN u dha", "phone", cfg.BootstrapAdminPhone)
	}

	locSvc := location.New(rdb, pool).WithRealtime(rtPub)
	pricingSvc := pricing.New(pool, mapsProvider, locSvc)
	docsSvc := documents.New(pool, store)
	driversSvc := drivers.New(pool, locSvc)
	if cfg.DocumentsRequired {
		driversSvc = driversSvc.WithEligibility(docsSvc)
	} else {
		log.Warn("VETËM DEV — dokumentet nuk kërkohen për aprovimin e shoferit")
	}
	appCfg := appconfig.New(pool)
	fraudSvc := fraud.New(pool, rdb)
	ridesSvc := rides.New(pool, locSvc, ledgerSvc, driversSvc, pricingSvc).WithFlags(appCfg).WithVelocity(fraudSvc).WithQR(rtTokens)
	limiter := ratelimit.New(rdb, log)
	perUser := limiter.PerUser(600, time.Minute) // §51: kufi për përdorues të kyçur
	requireAuth := func(next http.Handler) http.Handler { return authSvc.RequireAuth("")(perUser(next)) }
	requireDriver := func(next http.Handler) http.Handler {
		return authSvc.RequireAnyCapability("RIDE_DRIVER", "TAXI_DRIVER")(perUser(next))
	}
	requireOps := authSvc.RequireAuth("OPERATIONS")
	requireFinance := authSvc.RequireAuth("FINANCE")
	requireSupport := authSvc.RequireAnyCapability("SUPPORT", "ADMIN")
	requireStaff := authSvc.RequireAnyCapability("ADMIN", "SUPPORT", "OPERATIONS", "FINANCE")
	requireAdmin := authSvc.RequireAuth("ADMIN")

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
	mux.Handle("GET /api/v1/openapi.yaml", openapi.Handler())
	authSvc.Routes(mux)
	appCfg.Routes(mux, authSvc.OptionalAuth(), requireOps)
	users.New(pool, ledgerSvc).Routes(mux, requireAuth)
	dataexport.New(pool, store).Routes(mux, requireAuth)
	driversSvc.Routes(mux, requireAuth, requireDriver, requireOps)
	ridesSvc.Routes(mux, requireAuth, requireDriver)
	notifications.New(pool, nil).Routes(mux, requireAuth) // API-ja vetëm kutinë + token-at; push-in e dërgon worker-i
	realtime.New(pool, rtPub, rtTokens).Routes(mux, requireAuth)
	reviews.New(pool).Routes(mux, requireAuth)
	docsSvc.Routes(mux, requireAuth, requireOps)
	paymentsSvc := payments.New(pool, ledgerSvc, payProvider).WithFlags(appCfg).WithRisk(fraudSvc)
	paymentsSvc.Routes(mux, requireAuth, requireFinance)
	wallet.New(pool, ledgerSvc, wallet.Limits{MinTopUpMinor: payments.MinTopUpMinor, MaxTopUpMinor: payments.MaxTopUpMinor, DailyTopUpMinor: payments.DailyTopUpMinor}).Routes(mux, requireAuth)
	if dev, ok := payProvider.(*payment.DevLog); ok {
		paymentsSvc.DevRoutes(mux, dev) // vetëm development (devlog)
	}
	support.New(pool).Routes(mux, requireAuth, requireSupport)
	chat.New(pool).Routes(mux, requireAuth)
	fraudSvc.Routes(mux, requireOps)
	payouts.New(pool, ledgerSvc).Routes(mux, requireDriver, requireFinance)
	merchantsSvc := merchants.New(pool, pricingSvc)
	merchantsSvc.Routes(mux, authSvc.OptionalAuth(), requireAuth, requireOps)
	// Një instancë e vetme e katalogut: cache-i i menusë jeton në të, dhe imazhet e produkteve
	// duhet ta zbrazin pikërisht atë që shërben GET /menu.
	catalogSvc := catalog.New(pool, merchantsSvc)
	catalogSvc.Routes(mux, authSvc.OptionalAuth(), requireAuth)
	orders.New(pool, ledgerSvc, catalogSvc, merchantsSvc).WithLocation(locSvc).Routes(mux, requireAuth, requireDriver)
	mediaSvc := media.New(pool, mediaStore, merchantsSvc).WithMenuInvalidator(catalogSvc)
	mediaSvc.Routes(mux, requireAuth)
	places.New(mapsProvider, rdb).Routes(mux, requireAuth)
	admin.New(pool, rdb, ledgerSvc).Routes(mux, requireStaff, requireAdmin)
	if fs, ok := store.(*storage.DevFS); ok {
		documents.DevRoutes(mux, fs) // vetëm development (devfs)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.ErrNotFound)
	})

	handler := httpx.Chain(mux,
		httpx.Recover(log),
		otelx.HTTPMiddleware("krejt-api"), // §50: span për çdo kërkesë, trace_id në log dhe në zarfin e gabimit
		httpx.RequestID(),
		httpx.SecureHeaders(),
		httpx.Timeout(30*time.Second),
		httpx.AccessLog(log),
		limiter.PerIP(300, time.Minute), // §51: kufi publik për IP (para autentikimit)
		appCfg.Gate(),                   // §64: update i detyrueshëm / mirëmbajtje
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
