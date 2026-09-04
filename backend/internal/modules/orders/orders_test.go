package orders

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/catalog"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
)

func TestStateMachine(t *testing.T) {
	ok := [][2]string{
		{StatePendingMerchant, StateAccepted}, {StatePendingMerchant, StateRejected}, {StatePendingMerchant, StateCancelled},
		{StateAccepted, StatePreparing}, {StatePreparing, StateReady}, {StateReady, StateCourierAssigned},
		{StateReady, StateDelivered}, {StateCourierAssigned, StatePickedUp}, {StateCourierAssigned, StateReady}, {StatePickedUp, StateDelivered},
	}
	for _, c := range ok {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s duhej lejuar", c[0], c[1])
		}
	}
	bad := [][2]string{
		{StatePendingMerchant, StateReady}, {StateAccepted, StateDelivered}, {StatePickedUp, StateCancelled},
		{StateDelivered, StateReady}, {StateCancelled, StateAccepted}, {StatePreparing, StatePickedUp},
	}
	for _, c := range bad {
		if CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s duhej ndaluar", c[0], c[1])
		}
	}
	if !CustomerCanCancel(StateAccepted) || CustomerCanCancel(StatePreparing) {
		t.Fatal("dritarja e anulimit të klientit")
	}
	if len(newCode()) != 6 {
		t.Fatal("kodi 6 shkronja")
	}
}

// members — anëtarësi e thjeshtë për testin (pa modulin merchants).
type members map[uuid.UUID]uuid.UUID

func (m members) Membership(_ context.Context, userID, merchantID uuid.UUID) (string, error) {
	if m[userID] == merchantID {
		return "owner", nil
	}
	return "", httpx.ErrForbidden
}

type env struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	led      *ledger.Service
	cat      *catalog.Service
	svc      *Service
	merchant uuid.UUID
	owner    principal.Actor
	product  uuid.UUID
	optSmall uuid.UUID
}

func setup(t *testing.T) *env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
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
	e := &env{ctx: ctx, pool: pool, led: ledger.New(pool)}
	var ownerID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38334"+uuid.NewString()[:6]).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID) })
	if err := pool.QueryRow(ctx, `INSERT INTO merchants (owner_user_id, type, name, slug, address_line1, city, lat, lng, status, min_order_minor, delivery_fee_minor, commission_bp)
		VALUES ($1,'restaurant','Test Ushqim',$2,'Rr. Test','Prishtinë',42.6629,21.1655,'active',500,150,1500) RETURNING id`, ownerID, "o-"+uuid.NewString()[:8]).Scan(&e.merchant); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM merchants WHERE id = $1`, e.merchant) })
	// hapur gjithë ditën sot
	if _, err := pool.Exec(ctx, `INSERT INTO merchant_hours (merchant_id, weekday, opens, closes) VALUES ($1, EXTRACT(DOW FROM now())::int, '00:00', '23:59')`, e.merchant); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO merchant_staff (merchant_id, user_id, role) VALUES ($1, $2, 'owner')`, e.merchant, ownerID); err != nil {
		t.Fatal(err)
	}
	e.owner = principal.Actor{UserID: ownerID}
	e.cat = catalog.New(pool, members{ownerID: e.merchant})
	p, err := e.cat.UpsertProduct(ctx, e.owner, e.merchant, nil, catalog.ProductInput{Name: "Pite", PriceMinor: 300,
		Modifiers: []catalog.ModifierGroupInput{{Name: "Madhësia", MinSelect: 1, MaxSelect: 1, Options: []catalog.OptionInput{{Name: "E vogël"}, {Name: "E madhe", PriceDeltaMinor: 200}}}}})
	if err != nil {
		t.Fatal(err)
	}
	e.product, e.optSmall = p.ID, p.Modifiers[0].Options[0].ID
	e.svc = New(pool, e.led, e.cat, members{ownerID: e.merchant})
	return e
}

func (e *env) newUser(t *testing.T) principal.Actor {
	t.Helper()
	var id uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38333"+uuid.NewString()[:6]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		e.pool.Exec(context.Background(), `DELETE FROM order_offers WHERE courier_id = $1`, id)
		e.pool.Exec(context.Background(), `UPDATE orders SET courier_id = NULL WHERE courier_id = $1`, id)
		e.pool.Exec(context.Background(), `DELETE FROM orders WHERE customer_id = $1`, id)
		e.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return principal.Actor{UserID: id}
}

func (e *env) fundWallet(t *testing.T, userID uuid.UUID, minor int64) {
	t.Helper()
	code := ledger.UserWalletCode(userID)
	uid := userID
	if err := e.led.EnsureAccount(e.ctx, code, "user", &uid, "liability", "EUR"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.led.Post(e.ctx, ledger.Transaction{Kind: "topup_cash", Reference: "t:" + uuid.NewString(), IdempotencyKey: "t:" + uuid.NewString(), Currency: "EUR",
		Postings: []ledger.Posting{{AccountCode: "krejt:cash_clearing", Debit: money.Minor(minor)}, {AccountCode: code, Credit: money.Minor(minor)}}}); err != nil {
		t.Fatal(err)
	}
}

func (e *env) checkout(qty int) CheckoutInput {
	return CheckoutInput{MerchantID: e.merchant, PaymentMethod: "wallet", Fulfillment: "courier",
		AddressLine1: "Rr. Klientit 5", Address: &geo.Point{Lat: 42.665, Lng: 21.17},
		Items: []catalog.Selection{{ProductID: e.product, OptionIDs: []uuid.UUID{e.optSmall}, Quantity: qty}}}
}

func TestOrderWalletFlowWithCourier(t *testing.T) {
	e := setup(t)
	customer := e.newUser(t)
	courier := e.newUser(t)
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO drivers (user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
		VALUES ($1,'approved','Fiat','Punto','06-1-CC','e kuqe','{economy}')`, courier.UserID); err != nil {
		t.Fatal(err)
	}
	e.fundWallet(t, customer.UserID, 5000)
	commissionBefore := e.balance(t, "krejt:commission")
	feesBefore := e.balance(t, "krejt:delivery_fees")

	// nën minimum (1 × 300 = 300 < 500)
	if _, err := e.svc.Create(e.ctx, customer, "o1-"+uuid.NewString(), e.checkout(1)); !errors.Is(err, ErrMinOrder) {
		t.Fatalf("minimumi: %v", err)
	}
	// quote: 2 × 300 + 150 dërgesë = 750
	q, err := e.svc.Quote(e.ctx, e.checkout(2))
	if err != nil || q.ItemsTotalMinor != 600 || q.TotalMinor != 750 || !q.OpenNow {
		t.Fatalf("quote: %+v err=%v", q, err)
	}
	idem := "o2-" + uuid.NewString()
	o, err := e.svc.Create(e.ctx, customer, idem, e.checkout(2))
	if err != nil || o.State != StatePendingMerchant || o.TotalMinor != 750 || len(o.Items) != 1 || len(o.Code) != 6 {
		t.Fatalf("create: %+v err=%v", o, err)
	}
	again, err := e.svc.Create(e.ctx, customer, idem, e.checkout(2))
	if err != nil || again.ID != o.ID {
		t.Fatalf("idempotencë: %+v err=%v", again, err)
	}
	// merchant-i: pranon, përgatit, gati
	if _, err := e.svc.MerchantTransition(e.ctx, customer, o.ID, StateAccepted, 15, ""); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("klienti s'është staf: %v", err)
	}
	acc, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateAccepted, 15, "")
	if err != nil || acc.State != StateAccepted || acc.PrepTimeMin != 15 || acc.ReadyAtEstimate == nil {
		t.Fatalf("accept: %+v err=%v", acc, err)
	}
	if _, err := e.svc.CancelByCustomer(e.ctx, customer, o.ID, "u pendova"); err != nil {
		t.Fatalf("anulim i lejuar para përgatitjes: %v", err)
	}
	// porosi e re për rrjedhën e plotë
	o, err = e.svc.Create(e.ctx, customer, "o3-"+uuid.NewString(), e.checkout(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateAccepted, 15, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StatePreparing, 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CancelByCustomer(e.ctx, customer, o.ID, "vonë"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("anulim pas përgatitjes: %v", err)
	}
	ready, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateReady, 0, "")
	if err != nil || ready.ReadyAt == nil {
		t.Fatalf("ready: %+v err=%v", ready, err)
	}
	// merchant-i s'e dorëzon vetë kur përmbushja është me korrier
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateDelivered, 0, ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("dorëzim nga merchant-i me korrier: %v", err)
	}
	// oferta te korrieri (e krijojmë direkt: dispatch-i testohet veçmas)
	var offerID uuid.UUID
	if err := e.pool.QueryRow(e.ctx, `INSERT INTO order_offers (order_id, courier_id, round, distance_m, eta_s, expires_at)
		VALUES ($1,$2,1,500,120, now() + interval '25 seconds') RETURNING id`, o.ID, courier.UserID).Scan(&offerID); err != nil {
		t.Fatal(err)
	}
	offers, err := e.svc.Offers(e.ctx, courier)
	if err != nil || len(offers) != 1 || offers[0].EarningsMinor != 150 || offers[0].TotalMinor != 750 {
		t.Fatalf("ofertat: %+v err=%v", offers, err)
	}
	assigned, err := e.svc.AcceptOffer(e.ctx, courier, offerID)
	if err != nil || assigned.State != StateCourierAssigned || assigned.CourierID == nil {
		t.Fatalf("accept offer: %+v err=%v", assigned, err)
	}
	if _, err := e.svc.PickUp(e.ctx, courier, o.ID, "XXXXXX"); !errors.Is(err, ErrPickupCode) {
		t.Fatalf("kod i gabuar: %v", err)
	}
	if _, err := e.svc.PickUp(e.ctx, courier, o.ID, assigned.Code); err != nil {
		t.Fatal(err)
	}
	delivered, err := e.svc.Deliver(e.ctx, courier, o.ID)
	if err != nil || delivered.State != StateDelivered || delivered.PaymentStatus != "paid" {
		t.Fatalf("deliver: %+v err=%v", delivered, err)
	}
	// ledger: klienti −750, merchant-i +510 (600 − 90 komision), komisioni +90, tarifa +150
	if got := e.balance(t, ledger.UserWalletCode(customer.UserID)); got != 5000-750 {
		t.Fatalf("wallet-i i klientit: %d", got)
	}
	if got := e.balance(t, "merchant:"+e.merchant.String()+":wallet"); got != 510 {
		t.Fatalf("wallet-i i merchant-it: %d", got)
	}
	if got := e.balance(t, "krejt:commission") - commissionBefore; got != 90 {
		t.Fatalf("komisioni: %d", got)
	}
	if got := e.balance(t, "krejt:delivery_fees") - feesBefore; got != 150 {
		t.Fatalf("tarifa e dërgesës: %d", got)
	}
}

func TestOrderCashAndPickup(t *testing.T) {
	e := setup(t)
	customer := e.newUser(t)
	commissionBefore := e.balance(t, "krejt:commission")

	in := e.checkout(2)
	in.PaymentMethod = "cash"
	in.Fulfillment = "pickup"
	in.Address = nil
	in.AddressLine1 = ""
	o, err := e.svc.Create(e.ctx, customer, "c1-"+uuid.NewString(), in)
	if err != nil || o.DeliveryFeeMinor != 0 || o.TotalMinor != 600 {
		t.Fatalf("pickup: %+v err=%v", o, err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateAccepted, 10, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StatePreparing, 0, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateReady, 0, ""); err != nil {
		t.Fatal(err)
	}
	done, err := e.svc.MerchantTransition(e.ctx, e.owner, o.ID, StateDelivered, 0, "")
	if err != nil || done.PaymentStatus != "cash" {
		t.Fatalf("pickup delivered: %+v err=%v", done, err)
	}
	// cash: merchant-i i detyrohet KREJT komisionin (90), pa tarifë dërgese
	if got := e.balance(t, "krejt:commission") - commissionBefore; got != 90 {
		t.Fatalf("komisioni cash: %d", got)
	}
	// wallet-i i klientit nuk preket
	if got := e.balance(t, ledger.UserWalletCode(customer.UserID)); got != 0 {
		t.Fatalf("wallet-i i klientit me cash: %d", got)
	}
	// refuzimi kërkon arsye
	o2, err := e.svc.Create(e.ctx, customer, "c2-"+uuid.NewString(), in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.MerchantTransition(e.ctx, e.owner, o2.ID, StateRejected, 0, ""); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("refuzim pa arsye: %v", err)
	}
	rej, err := e.svc.MerchantTransition(e.ctx, e.owner, o2.ID, StateRejected, 0, "s'kemi më")
	if err != nil || rej.State != StateRejected || rej.PaymentStatus != "none" {
		t.Fatalf("reject: %+v err=%v", rej, err)
	}
	// wallet pa mjaftueshëm
	poor := e.newUser(t)
	pin := e.checkout(2)
	if _, err := e.svc.Create(e.ctx, poor, "c3-"+uuid.NewString(), pin); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("wallet bosh: %v", err)
	}
	// merchant-i i mbyllur (pauzë)
	if _, err := e.pool.Exec(e.ctx, `UPDATE merchants SET accepting_orders = false WHERE id = $1`, e.merchant); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Create(e.ctx, customer, "c4-"+uuid.NewString(), in); !errors.Is(err, ErrMerchantClosed) {
		t.Fatalf("pauzë: %v", err)
	}
	_ = logx.New("test", "development")
}

func (e *env) balance(t *testing.T, code string) int64 {
	t.Helper()
	b, err := e.led.Balance(e.ctx, code)
	if err != nil {
		if errors.Is(err, ledger.ErrAccountMissing) {
			return 0
		}
		t.Fatal(err)
	}
	return int64(b.Minor)
}
