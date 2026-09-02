package rides

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/dispatch"
	"krejt.app/backend/internal/modules/drivers"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/cache"
	"krejt.app/backend/internal/platform/config"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/maps"
)

// Zona e testit: Fushë Kosovë (brenda rrezes së Prishtinës, larg testeve të tjera që përdorin qendrën).
var (
	tPickup  = geo.Point{Lat: 42.6414, Lng: 21.0975}
	tDropoff = geo.Point{Lat: 42.6629, Lng: 21.1655}
)

type env struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	rdb      redis.UniversalClient
	loc      *location.Service
	led      *ledger.Service
	drv      *drivers.Service
	pr       *pricing.Service
	rides    *Service
	dispatch *dispatch.Dispatcher
}

func setup(t *testing.T) *env {
	t.Helper()
	dsn, raddr := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_REDIS_ADDR")
	if dsn == "" || raddr == "" {
		t.Skip("TEST_DATABASE_URL / TEST_REDIS_ADDR not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	rdb, err := cache.Connect(ctx, config.Redis{Host: raddr})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rdb.Close() })
	log := logx.New("test", "development")
	e := &env{ctx: ctx, pool: pool, rdb: rdb}
	e.loc = location.New(rdb, pool)
	e.led = ledger.New(pool)
	e.drv = drivers.New(pool, e.loc)
	e.pr = pricing.New(pool, maps.DevEstimate{}, e.loc)
	e.rides = New(pool, e.loc, e.led, e.drv, e.pr)
	e.dispatch = dispatch.New(pool, e.loc, log)
	return e
}

func (e *env) newUser(t *testing.T, caps ...string) principal.Actor {
	t.Helper()
	var id uuid.UUID
	phone := "+38349" + uuid.NewString()[:6]
	if err := e.pool.QueryRow(e.ctx, `INSERT INTO users (phone_e164, full_name, locale) VALUES ($1, 'Test Përdorues', 'sq') RETURNING id`, phone).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, c := range caps {
		if _, err := e.pool.Exec(e.ctx, `INSERT INTO user_capabilities (user_id, capability) VALUES ($1, $2)`, id, c); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = e.loc.SetOffline(context.Background(), id)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM ride_offers WHERE driver_id = $1`, id)
		_, _ = e.pool.Exec(context.Background(), `UPDATE rides SET driver_id = NULL WHERE driver_id = $1`, id)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM rides WHERE customer_id = $1`, id)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM ride_quotes WHERE customer_id = $1`, id)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return principal.Actor{UserID: id, SessionID: uuid.New(), IP: "203.0.113.9", Capabilities: caps}
}

func (e *env) newDriver(t *testing.T, ops principal.Actor, at geo.Point) principal.Actor {
	t.Helper()
	d := e.newUser(t)
	if _, err := e.drv.Apply(e.ctx, d, drivers.ApplyInput{VehicleMake: "Škoda", VehicleModel: "Octavia", VehiclePlate: "01-123-AB", VehicleColor: "e bardhë", Categories: []string{"economy"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.drv.Approve(e.ctx, ops, d.UserID, []string{"economy"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.drv.GoOnline(e.ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := e.loc.Ingest(e.ctx, d.UserID, []location.Sample{{Lat: at.Lat, Lng: at.Lng, RecordedAtMs: time.Now().UnixMilli()}}); err != nil {
		t.Fatal(err)
	}
	return d
}

func (e *env) fund(t *testing.T, userID uuid.UUID, minor int64) {
	t.Helper()
	code := ledger.UserWalletCode(userID)
	uid := userID
	if err := e.led.EnsureAccount(e.ctx, code, "user", &uid, "liability", "EUR"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.led.Post(e.ctx, ledger.Transaction{Kind: "topup_cash", Reference: "test:" + uuid.NewString(), IdempotencyKey: "test:" + uuid.NewString(), Currency: "EUR",
		Postings: []ledger.Posting{{AccountCode: "krejt:cash_clearing", Debit: money.Minor(minor)}, {AccountCode: code, Credit: money.Minor(minor)}}}); err != nil {
		t.Fatal(err)
	}
}

func (e *env) quote(t *testing.T, c principal.Actor) uuid.UUID {
	t.Helper()
	q, err := e.pr.Quote(e.ctx, c.UserID, pricing.QuoteInput{Pickup: tPickup, Dropoff: tDropoff, PickupAddress: "Fushë Kosovë", DropoffAddress: "Prishtinë"})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range q.Quotes {
		if x.CategoryID == "economy" {
			if x.PriceMinor < 200 {
				t.Fatalf("çmimi nën minimum: %d", x.PriceMinor)
			}
			return x.ID
		}
	}
	t.Fatal("s'ka ofertë economy")
	return uuid.Nil
}

func (e *env) balance(t *testing.T, code string) int64 {
	t.Helper()
	b, err := e.led.Balance(e.ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	return int64(b.Minor)
}

func TestRideLifecycleWalletHappyPath(t *testing.T) {
	e := setup(t)
	ops := e.newUser(t, "OPERATIONS")
	customer := e.newUser(t, "CUSTOMER")
	e.fund(t, customer.UserID, 5000)
	driver := e.newDriver(t, ops, geo.Point{Lat: tPickup.Lat + 0.005, Lng: tPickup.Lng})
	before := e.balance(t, "krejt:commission")

	qid := e.quote(t, customer)
	ride, err := e.rides.Request(e.ctx, customer, "idem-"+uuid.NewString(), RequestInput{QuoteID: qid, PaymentMethod: "wallet", Note: "te hyrja"})
	if err != nil {
		t.Fatal(err)
	}
	if ride.State != StateMatching || ride.PriceQuotedMinor < 200 {
		t.Fatalf("kërkesa: %+v", ride)
	}
	// idempotencë: i njëjti çelës → i njëjti udhëtim; udhëtim i dytë aktiv → refuzohet
	again, err := e.rides.Request(e.ctx, customer, "idem-2-"+uuid.NewString(), RequestInput{QuoteID: qid, PaymentMethod: "wallet"})
	if !errors.Is(err, ErrActiveRide) {
		t.Fatalf("udhëtim i dytë aktiv: %v %+v", err, again)
	}

	// dispatch → ofertë → pranim
	offered, err := e.dispatch.Round(e.ctx, ride.ID)
	if err != nil || !offered {
		t.Fatalf("round: offered=%v err=%v", offered, err)
	}
	if offered, err := e.dispatch.Round(e.ctx, ride.ID); err != nil || offered {
		t.Fatalf("round i dytë me ofertë të hapur duhej të mos ofronte: %v %v", offered, err)
	}
	offers, err := e.rides.Offers(e.ctx, driver)
	if err != nil || len(offers) != 1 || offers[0].RideID != ride.ID || offers[0].EarningsMinor >= offers[0].PriceMinor {
		t.Fatalf("ofertat: %+v err=%v", offers, err)
	}
	ride, err = e.rides.AcceptOffer(e.ctx, driver, offers[0].ID)
	if err != nil || ride.State != StateAssigned || ride.DriverID == nil || *ride.DriverID != driver.UserID || ride.Driver == nil || ride.Driver.VehiclePlate != "01-123-AB" {
		t.Fatalf("pranimi: %+v err=%v", ride, err)
	}
	if st, _ := e.loc.State(e.ctx, driver.UserID); st == nil || st.Status != "busy" {
		t.Fatalf("shoferi duhej busy: %+v", st)
	}
	// klienti e sheh udhëtimin me shoferin; një tjetër jo
	if got, err := e.rides.Get(e.ctx, customer, ride.ID); err != nil || got.Driver == nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := e.rides.Get(e.ctx, ops, ride.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("BOLA: %v", err)
	}

	// hapat e shoferit; kalim i gabuar refuzohet
	if _, err := e.rides.Start(e.ctx, driver, ride.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("start para arrived: %v", err)
	}
	if ride, err = e.rides.Arrived(e.ctx, driver, ride.ID); err != nil || ride.State != StateArrived {
		t.Fatalf("arrived: %v", err)
	}
	if ride, err = e.rides.Start(e.ctx, driver, ride.ID); err != nil || ride.State != StateInProgress {
		t.Fatalf("start: %v", err)
	}
	if _, err := e.rides.CancelByCustomer(e.ctx, customer, ride.ID, "u pendova"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("anulim gjatë udhëtimit: %v", err)
	}
	ride, err = e.rides.Complete(e.ctx, driver, ride.ID)
	if err != nil || ride.State != StateCompleted || ride.PaymentStatus != "paid" || ride.PriceFinalMinor == nil {
		t.Fatalf("complete: %+v err=%v", ride, err)
	}
	price := *ride.PriceFinalMinor
	commission := pricing.Commission(price, 1500)
	if got := e.balance(t, ledger.UserWalletCode(customer.UserID)); got != 5000-price {
		t.Fatalf("wallet-i i klientit = %d, pritej %d", got, 5000-price)
	}
	if got := e.balance(t, "driver:"+driver.UserID.String()+":wallet"); got != price-commission {
		t.Fatalf("wallet-i i shoferit = %d, pritej %d", got, price-commission)
	}
	if got := e.balance(t, "krejt:commission"); got-before != commission {
		t.Fatalf("komisioni = %d, pritej %d", got-before, commission)
	}
	if st, _ := e.loc.State(e.ctx, driver.UserID); st == nil || st.Status != "available" {
		t.Fatalf("shoferi duhej available: %+v", st)
	}
	hist, err := e.rides.History(e.ctx, customer, nil, 10)
	if err != nil || len(hist) != 1 || hist[0].ID != ride.ID {
		t.Fatalf("historiku: %+v err=%v", hist, err)
	}
}

func TestRideCashDriverCancelAndNoDriver(t *testing.T) {
	e := setup(t)
	ops := e.newUser(t, "OPERATIONS")
	customer := e.newUser(t, "CUSTOMER")
	driver := e.newDriver(t, ops, geo.Point{Lat: tPickup.Lat, Lng: tPickup.Lng + 0.004})

	// kartë: ende pa modul pagesash → refuzohet hapur, jo e simuluar
	if _, err := e.rides.Request(e.ctx, customer, "k-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "card"}); !errors.Is(err, ErrPaymentMethodUnavailable) {
		t.Fatalf("card: %v", err)
	}
	// wallet pa para → refuzohet para kërkimit
	if _, err := e.rides.Request(e.ctx, customer, "w-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "wallet"}); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("wallet bosh: %v", err)
	}

	ride, err := e.rides.Request(e.ctx, customer, "c-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "cash"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.dispatch.Round(e.ctx, ride.ID); err != nil {
		t.Fatal(err)
	}
	offers, _ := e.rides.Offers(e.ctx, driver)
	if len(offers) != 1 {
		t.Fatalf("ofertat: %+v", offers)
	}
	if _, err := e.rides.AcceptOffer(e.ctx, driver, offers[0].ID); err != nil {
		t.Fatal(err)
	}
	// shoferi anulon → ricaktim: kthehet në kërkim, shoferi i mëparshëm nuk merr më ofertë
	ride, err = e.rides.CancelByDriver(e.ctx, driver, ride.ID, "defekt")
	if err != nil || ride.State != StateMatching || ride.DriverID != nil || ride.MatchingAttempts != 1 {
		t.Fatalf("anulimi i shoferit: %+v err=%v", ride, err)
	}
	if offered, err := e.dispatch.Round(e.ctx, ride.ID); err != nil || offered {
		t.Fatalf("shoferi që anuloi mori sërish ofertë: %v %v", offered, err)
	}
	// skadimi i kërkimit → no_driver
	if _, err := e.pool.Exec(e.ctx, `UPDATE rides SET requested_at = now() - interval '10 minutes' WHERE id = $1`, ride.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.dispatch.Round(e.ctx, ride.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.rides.Get(e.ctx, customer, ride.ID); got.State != StateNoDriver {
		t.Fatalf("no_driver: %s", got.State)
	}

	// cash deri në fund: komisioni i shoferit regjistrohet si borxh
	before := e.balance(t, "krejt:commission")
	ride, err = e.rides.Request(e.ctx, customer, "c2-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "cash"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.dispatch.Round(e.ctx, ride.ID); err != nil {
		t.Fatal(err)
	}
	offers, _ = e.rides.Offers(e.ctx, driver)
	if len(offers) != 1 {
		t.Fatalf("ofertat 2: %+v", offers)
	}
	if _, err := e.rides.AcceptOffer(e.ctx, driver, offers[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.rides.Arrived(e.ctx, driver, ride.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.rides.Start(e.ctx, driver, ride.ID); err != nil {
		t.Fatal(err)
	}
	ride, err = e.rides.Complete(e.ctx, driver, ride.ID)
	if err != nil || ride.PaymentStatus != "cash" {
		t.Fatalf("cash complete: %+v err=%v", ride, err)
	}
	commission := pricing.Commission(*ride.PriceFinalMinor, 1500)
	if got := e.balance(t, "driver:"+driver.UserID.String()+":wallet"); got != -commission {
		t.Fatalf("borxhi i shoferit = %d, pritej %d", got, -commission)
	}
	if got := e.balance(t, "krejt:commission"); got-before != commission {
		t.Fatalf("komisioni = %d, pritej %d", got-before, commission)
	}
}

func TestRideCustomerCancelFeeAndOfferExpiry(t *testing.T) {
	e := setup(t)
	ops := e.newUser(t, "OPERATIONS")
	customer := e.newUser(t, "CUSTOMER")
	e.fund(t, customer.UserID, 2000)
	driver := e.newDriver(t, ops, geo.Point{Lat: tPickup.Lat - 0.004, Lng: tPickup.Lng})

	ride, err := e.rides.Request(e.ctx, customer, "f-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "wallet"})
	if err != nil {
		t.Fatal(err)
	}
	// oferta skadon → sweep e shënon dhe bën raund të ri (shoferi i njëjti është i përjashtuar → asgjë)
	e.dispatch.OfferTTL = -time.Second
	if _, err := e.dispatch.Round(e.ctx, ride.ID); err != nil {
		t.Fatal(err)
	}
	st, err := e.dispatch.Sweep(e.ctx)
	if err != nil || st.Expired < 1 {
		t.Fatalf("sweep: %+v err=%v", st, err)
	}
	var state string
	if err := e.pool.QueryRow(e.ctx, `SELECT state FROM ride_offers WHERE ride_id = $1 AND driver_id = $2`, ride.ID, driver.UserID).Scan(&state); err != nil || state != "expired" {
		t.Fatalf("oferta: %s %v", state, err)
	}
	// anulim gjatë kërkimit: pa tarifë
	ride, err = e.rides.CancelByCustomer(e.ctx, customer, ride.ID, "")
	if err != nil || ride.State != StateCancelled || ride.CancellationFeeMinor != 0 || ride.PaymentStatus != "none" {
		t.Fatalf("anulim falas: %+v err=%v", ride, err)
	}

	// anulim pas caktimit dhe pas periudhës së hirit: tarifë nga wallet-i
	e.dispatch.OfferTTL = 20 * time.Second
	ride, err = e.rides.Request(e.ctx, customer, "f2-"+uuid.NewString(), RequestInput{QuoteID: e.quote(t, customer), PaymentMethod: "wallet"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.dispatch.Round(e.ctx, ride.ID); err != nil {
		t.Fatal(err)
	}
	offers, _ := e.rides.Offers(e.ctx, driver)
	if len(offers) != 1 {
		t.Fatalf("ofertat: %+v", offers)
	}
	if _, err := e.rides.AcceptOffer(e.ctx, driver, offers[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(e.ctx, `UPDATE rides SET assigned_at = now() - interval '5 minutes' WHERE id = $1`, ride.ID); err != nil {
		t.Fatal(err)
	}
	before := e.balance(t, ledger.UserWalletCode(customer.UserID))
	ride, err = e.rides.CancelByCustomer(e.ctx, customer, ride.ID, "s'më duhet më")
	if err != nil || ride.State != StateCancelled || ride.CancellationFeeMinor != 100 || ride.PaymentStatus != "paid" {
		t.Fatalf("anulim me tarifë: %+v err=%v", ride, err)
	}
	if got := e.balance(t, ledger.UserWalletCode(customer.UserID)); got != before-100 {
		t.Fatalf("tarifa nuk u arkëtua: %d → %d", before, got)
	}
	if st, _ := e.loc.State(e.ctx, driver.UserID); st == nil || st.Status != "available" {
		t.Fatalf("shoferi pas anulimit: %+v", st)
	}
}
