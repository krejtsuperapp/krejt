package payouts

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func TestIBAN(t *testing.T) {
	good := []string{"DE89 3704 0044 0532 0130 00", "GB82 WEST 1234 5698 7654 32", "XK05 1212 0123 4567 8906", "AL47 2121 1009 0000 0002 3569 8741"}
	for _, s := range good {
		if _, ok := ValidIBAN(s); !ok {
			t.Errorf("%q duhej i vlefshëm", s)
		}
	}
	for _, s := range []string{"", "XK05 1212 0123 4567 8907", "DE89370400440532013", "XX00 1234", "de89-3704"} {
		if _, ok := ValidIBAN(s); ok {
			t.Errorf("%q duhej i pavlefshëm", s)
		}
	}
	if got := MaskIBAN("XK051212012345678906"); got != "XK05************8906" {
		t.Fatalf("mask: %s", got)
	}
}

func TestPayoutFlow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	led := ledger.New(pool)
	svc := New(pool, led)
	newUser := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38337"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM payout_items WHERE driver_id = $1`, id)
			pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		})
		return id
	}
	driver, poor, finance := newUser(), newUser(), newUser()
	for _, d := range []uuid.UUID{driver, poor} {
		if _, err := pool.Exec(ctx, `INSERT INTO drivers (user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories) VALUES ($1, 'approved', 'A', 'B', '05-1-X', 'c', '{economy}')`, d); err != nil {
			t.Fatal(err)
		}
	}
	fin, drv := principal.Actor{UserID: finance}, principal.Actor{UserID: driver}

	// IBAN: i pavlefshëm → refuzohet; i vlefshëm → ruhet i maskuar
	if _, err := svc.SetBankAccount(ctx, drv, BankAccountInput{HolderName: "Fatmir Berisha", IBAN: "XK05 1212 0123 4567 8907"}); !errors.Is(err, ErrIBAN) {
		t.Fatalf("iban i gabuar: %v", err)
	}
	ba, err := svc.SetBankAccount(ctx, drv, BankAccountInput{HolderName: "Fatmir Berisha", IBAN: "xk05 1212 0123 4567 8906", BankName: "Raiffeisen"})
	if err != nil || ba.IBANMasked != "XK05************8906" {
		t.Fatalf("bank account: %+v err=%v", ba, err)
	}
	if _, err := svc.SetBankAccount(ctx, principal.Actor{UserID: poor}, BankAccountInput{HolderName: "Arta Krasniqi", IBAN: "DE89 3704 0044 0532 0130 00"}); err != nil {
		t.Fatal(err)
	}
	// fitime: shoferi 42,00 € nga udhëtime me wallet; poor 3,00 € (nën minimum)
	credit := func(d uuid.UUID, minor int64) {
		code := DriverWalletCode(d)
		if err := led.EnsureAccount(ctx, code, "driver", &d, "liability", "EUR"); err != nil {
			t.Fatal(err)
		}
		if _, err := led.Post(ctx, ledger.Transaction{Kind: "ride_fare", Reference: "ride:test-" + uuid.NewString(), IdempotencyKey: "t:" + uuid.NewString(), Currency: "EUR",
			Postings: []ledger.Posting{{AccountCode: "krejt:cash_clearing", Debit: money.Minor(minor)}, {AccountCode: code, Credit: money.Minor(minor)}}}); err != nil {
			t.Fatal(err)
		}
	}
	credit(driver, 4200)
	credit(poor, 300)
	e, err := svc.Earnings(ctx, driver)
	if err != nil || e.BalanceMinor != 4200 || e.TodayMinor != 4200 || e.WeekMinor != 4200 {
		t.Fatalf("earnings: %+v err=%v", e, err)
	}

	// grupi: vetëm shoferi hyn; wallet-i i tij bie në 0; poor mbetet 3,00
	b, err := svc.CreateBatch(ctx, fin, time.Now().AddDate(0, 0, -7), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM payout_batches WHERE id = $1`, b.ID) })
	items, _ := svc.Items(ctx, &b.ID, nil, 10)
	mine := 0
	for _, it := range items {
		if it.DriverID == driver && it.AmountMinor == 4200 {
			mine++
		}
		if it.DriverID == poor {
			t.Fatal("shoferi nën minimum nuk duhej në grup")
		}
	}
	if mine != 1 {
		t.Fatalf("zërat: %+v", items)
	}
	if bal, _ := led.Balance(ctx, DriverWalletCode(driver)); bal.Minor != 0 {
		t.Fatalf("wallet-i pas grupit: %d", bal.Minor)
	}
	// eksporti CSV me IBAN të plotë; grupi → exported
	csvData, err := svc.ExportCSV(ctx, fin, b.ID)
	if err != nil || !strings.Contains(string(csvData), "XK051212012345678906") || !strings.Contains(string(csvData), "42.00") {
		t.Fatalf("csv: %s err=%v", csvData, err)
	}
	// banka dështon → paratë kthehen; pastaj grupi mbyllet kur s'ka pending
	var itemID uuid.UUID
	for _, it := range items {
		if it.DriverID == driver {
			itemID = it.ID
		}
	}
	it, err := svc.SettleItem(ctx, fin, itemID, "failed", "IBAN i mbyllur")
	if err != nil || it.Status != "failed" {
		t.Fatalf("settle: %+v err=%v", it, err)
	}
	if bal, _ := led.Balance(ctx, DriverWalletCode(driver)); bal.Minor != 4200 {
		t.Fatalf("wallet-i pas dështimit: %d", bal.Minor)
	}
	if _, err := svc.SettleItem(ctx, fin, itemID, "paid", ""); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("settle i dytë: %v", err)
	}
	var status string
	pool.QueryRow(ctx, `SELECT status FROM payout_batches WHERE id = $1`, b.ID).Scan(&status)
	if status != "completed" {
		t.Fatalf("grupi: %s", status)
	}
	// shoferi sheh payout-in me IBAN të maskuar
	mineItems, _ := svc.Items(ctx, nil, &driver, 10)
	if len(mineItems) != 1 || mineItems[0].IBANMasked != "XK05************8906" {
		t.Fatalf("items e shoferit: %+v", mineItems)
	}
}
