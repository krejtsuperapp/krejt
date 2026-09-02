package ledger

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
)

// --- teste njësie (pa DB) ------------------------------------------------------

func TestValidateBalanced(t *testing.T) {
	// porosi 12,40 €: klienti paguan me wallet; merchant-i merr 10,17 €, KREJT 2,23 € komision
	p := []Posting{
		{AccountCode: "user:1:wallet", Debit: 1240},
		{AccountCode: "merchant:2:payable", Credit: 1017},
		{AccountCode: "krejt:commission", Credit: 223},
	}
	if err := Validate(p); err != nil {
		t.Fatalf("expected balanced, got %v", err)
	}
}

func TestValidateUnbalanced(t *testing.T) {
	p := []Posting{
		{AccountCode: "user:1:wallet", Debit: 1240},
		{AccountCode: "merchant:2:payable", Credit: 1000},
	}
	if err := Validate(p); !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("expected ErrUnbalanced, got %v", err)
	}
}

func TestValidateRejectsBothSides(t *testing.T) {
	p := []Posting{{AccountCode: "a", Debit: 1, Credit: 1}, {AccountCode: "b", Credit: 0, Debit: 0}}
	if err := Validate(p); !errors.Is(err, ErrInvalidPosting) {
		t.Fatalf("expected ErrInvalidPosting, got %v", err)
	}
}

func TestValidateEmpty(t *testing.T) {
	if err := Validate(nil); !errors.Is(err, ErrEmpty) {
		t.Fatalf("expected ErrEmpty, got %v", err)
	}
}

// --- test integrimi (kërkon TEST_DATABASE_URL; docker-compose up postgres) ----

func TestPostIsAtomicIdempotentAndImmutable(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	svc := New(pool)
	uid := uuid.New()
	mid := uuid.New()
	wallet := "user:" + uid.String() + ":wallet"
	payable := "merchant:" + mid.String() + ":payable"
	if err := svc.EnsureAccount(ctx, wallet, "user", &uid, "liability", "EUR"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureAccount(ctx, payable, "merchant", &mid, "liability", "EUR"); err != nil {
		t.Fatal(err)
	}

	// mbushje 50,00 € (kartë → wallet)
	topup := Transaction{Kind: "topup", Reference: "topup:" + uid.String(), IdempotencyKey: "idem-" + uuid.NewString(),
		Postings: []Posting{{AccountCode: "krejt:card_clearing", Debit: 5000}, {AccountCode: wallet, Credit: 5000}}}
	id1, err := svc.Post(ctx, topup)
	if err != nil {
		t.Fatal(err)
	}
	// e njëjta kërkesë përsëri (riprovim) → i njëjti transaksion, asnjë dyfishim
	id2, err := svc.Post(ctx, topup)
	if err != nil || id1 != id2 {
		t.Fatalf("idempotency broken: %v %v %v", id1, id2, err)
	}

	// pagesë porosie 12,40 €
	pay := Transaction{Kind: "order_payment", Reference: "order:F-1", IdempotencyKey: "idem-" + uuid.NewString(),
		Postings: []Posting{{AccountCode: wallet, Debit: 1240}, {AccountCode: payable, Credit: 1017}, {AccountCode: "krejt:commission", Credit: 223}}}
	if _, err := svc.Post(ctx, pay); err != nil {
		t.Fatal(err)
	}

	bal, err := svc.Balance(ctx, wallet)
	if err != nil || bal.Minor != 3760 {
		t.Fatalf("wallet balance = %v (%v), want 37.60", bal, err)
	}

	// e pandryshueshme: UPDATE/DELETE duhet të dështojnë
	if _, err := pool.Exec(ctx, `DELETE FROM ledger_entries WHERE tx_id = $1`, id1); err == nil {
		t.Fatal("expected immutability trigger to block DELETE")
	}
	// e pabalancuar në nivel DB: trigger-i i shtyrë duhet ta refuzojë në COMMIT
	if _, err := svc.Post(ctx, Transaction{Kind: "x", Reference: "x", IdempotencyKey: "idem-" + uuid.NewString(),
		Postings: []Posting{{AccountCode: wallet, Debit: 100}, {AccountCode: payable, Credit: 90}}}); err == nil {
		t.Fatal("expected unbalanced transaction to be rejected")
	}
}
