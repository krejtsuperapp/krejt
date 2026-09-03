package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
)

// Ndezja fillestare është e vetmja rrugë për administratorin e parë, ndaj sjellja e saj matet
// kundrejt një baze të vërtetë: kushti mbrojtës është një kërkesë SQL, jo një degë në Go.
func TestBootstrapAdmin(t *testing.T) {
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

	// Testet ndajnë të njëjtën bazë; pastrohet vetëm ajo që prek ky test.
	if _, err := pool.Exec(ctx, `DELETE FROM user_capabilities WHERE capability = 'SUPER_ADMIN'`); err != nil {
		t.Fatal(err)
	}

	svc := &Service{pool: pool}
	// Vetëm shifra: identifikuesit e rastësishëm përmbajnë shkronja dhe do të binin te vetë
	// kontrolli i formatit, duke e fshehur atë që testi kërkon të masë.
	phone := uniquePhone()

	t.Run("numri pa llogari nuk kalon", func(t *testing.T) {
		if _, err := svc.BootstrapAdmin(ctx, phone); !errors.Is(err, ErrBootstrapUserMissing) {
			t.Fatalf("prisja ErrBootstrapUserMissing, mora %v", err)
		}
	})

	var userID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, phone).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	t.Run("i pari e merr", func(t *testing.T) {
		granted, err := svc.BootstrapAdmin(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		if !granted {
			t.Fatal("prisja që e drejta të jepej")
		}
	})

	t.Run("nuk jepet dy herë", func(t *testing.T) {
		granted, err := svc.BootstrapAdmin(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Fatal("u dha sërish — kushti mbrojtës nuk punoi")
		}
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM user_capabilities WHERE user_id = $1 AND capability = 'SUPER_ADMIN'`,
			userID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("prisja një të drejtë, gjeta %d", n)
		}
	})

	t.Run("një numër tjetër nuk ngrihet kur administratori ekziston", func(t *testing.T) {
		other := uniquePhone()
		if _, err := pool.Exec(ctx,
			`INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq')`, other); err != nil {
			t.Fatal(err)
		}
		granted, err := svc.BootstrapAdmin(ctx, other)
		if err != nil {
			t.Fatal(err)
		}
		if granted {
			t.Fatal("një numër i dytë mori të drejtën — kjo e bën cilësimin një derë të hapur")
		}
	})

	t.Run("numri bosh nuk bën asgjë", func(t *testing.T) {
		granted, err := svc.BootstrapAdmin(ctx, "")
		if err != nil || granted {
			t.Fatalf("prisja heshtje, mora granted=%v err=%v", granted, err)
		}
	})

	t.Run("numri i keqshkruar refuzohet, nuk kalon në heshtje", func(t *testing.T) {
		if _, err := svc.BootstrapAdmin(ctx, "044123456"); err == nil {
			t.Fatal("prisja gabim për numër të pavlefshëm")
		}
	})
}

// uniquePhone jep një numër E.164 të vlefshëm dhe të papërsëritur brenda bazës së përbashkët.
func uniquePhone() string {
	return fmt.Sprintf("+383%09d", time.Now().UnixNano()%1_000_000_000)
}
