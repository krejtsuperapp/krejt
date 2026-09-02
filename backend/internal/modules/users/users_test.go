package users

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
)

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestUsersModuleEndToEnd(t *testing.T) {
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
	svc := New(pool, ledger.New(pool))

	newUser := func(phone string) Actor {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, phone).Scan(&id); err != nil {
			t.Fatal(err)
		}
		var sid uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO sessions (user_id, device_id, platform, refresh_token_hash, refresh_expires_at, ip)
			VALUES ($1, 'dev-1', 'android', '\x00', now() + interval '30 days', '203.0.113.7'::inet) RETURNING id`, id).Scan(&sid); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return Actor{UserID: id, SessionID: sid, IP: "203.0.113.7"}
	}
	suffix := uuid.NewString()[:8]
	a := newUser("+38344" + suffix[:6])
	b := newUser("+38345" + suffix[:6])

	// profili
	name, email := "  liridon   osmani ", "Doni+"+suffix+"@Krejt.app"
	p, err := svc.UpdateProfile(ctx, a, ProfileUpdate{FullName: &name, Email: &email})
	if err != nil {
		t.Fatal(err)
	}
	if *p.FullName != "liridon osmani" || *p.Email != "doni+"+suffix+"@krejt.app" {
		t.Fatalf("profili: %+v", p)
	}
	if _, err := svc.UpdateProfile(ctx, b, ProfileUpdate{Email: &email}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("email i zënë: err = %v", err)
	}
	badLocale := "sr"
	if _, err := svc.UpdateProfile(ctx, a, ProfileUpdate{Locale: &badLocale}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("locale sr duhej refuzuar (§2): %v", err)
	}

	// adresat: e para bëhet default, e dyta me is_default e zhvendos, jashtë Kosovës refuzohet
	home, err := svc.CreateAddress(ctx, a, AddressInput{Label: "home", Line1: "Rr. Agim Ramadani 12", City: "Prishtinë", Lat: 42.6629, Lng: 21.1655})
	if err != nil {
		t.Fatal(err)
	}
	if !home.IsDefault {
		t.Fatal("adresa e parë duhet të jetë default")
	}
	work, err := svc.CreateAddress(ctx, a, AddressInput{Label: "work", Line1: "Sheshi Nëna Terezë 1", City: "Prishtinë", Lat: 42.6600, Lng: 21.1620, IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListAddresses(ctx, a.UserID)
	if err != nil || len(list) != 2 || list[0].ID != work.ID || !list[0].IsDefault || list[1].IsDefault {
		t.Fatalf("lista: %+v err=%v", list, err)
	}
	if _, err := svc.CreateAddress(ctx, a, AddressInput{Label: "other", Line1: "Rruga e Durrësit 1", City: "Tiranë", Lat: 41.3275, Lng: 19.8187}); !errors.Is(err, ErrOutsideKosovo) {
		t.Fatalf("Tirana duhej refuzuar (§1): %v", err)
	}
	if _, err := svc.UpdateAddress(ctx, b, home.ID, AddressInput{Label: "home", Line1: "Hajde", City: "Pejë", Lat: 42.66, Lng: 20.29}); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("adresa e tjetërkujt duhet të jetë NOT_FOUND: %v", err)
	}
	if err := svc.DeleteAddress(ctx, a, home.ID); err != nil {
		t.Fatal(err)
	}

	// preferencat
	prefs, err := svc.Preferences(ctx, a.UserID)
	if err != nil || len(prefs) != len(Categories) || prefs[6].Category != "promotions" || prefs[6].Email {
		t.Fatalf("parazgjedhjet: %+v err=%v", prefs, err)
	}
	if _, err := svc.SetPreferences(ctx, a, []Preference{{Category: "security", Push: false}}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("security.push=false duhej refuzuar: %v", err)
	}
	prefs, err = svc.SetPreferences(ctx, a, []Preference{{Category: "promotions", Push: false, Email: true, SMS: true}})
	if err != nil || prefs[6].Push || !prefs[6].SMS {
		t.Fatalf("pas ruajtjes: %+v err=%v", prefs, err)
	}

	// sesionet
	sess, err := svc.Sessions(ctx, a)
	if err != nil || len(sess) != 1 || !sess[0].Current || sess[0].IP == nil {
		t.Fatalf("sesionet: %+v err=%v", sess, err)
	}
	if err := svc.RevokeSession(ctx, b, a.SessionID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("shkyçja e sesionit të tjetërkujt: %v", err)
	}

	// fshirja e llogarisë: anonimizim + shkyçje; pas saj asgjë nuk punon
	if err := svc.DeleteAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	var status string
	var phone *string
	if err := pool.QueryRow(ctx, `SELECT status, phone_e164 FROM users WHERE id = $1`, a.UserID).Scan(&status, &phone); err != nil {
		t.Fatal(err)
	}
	if status != "deleted" || phone != nil {
		t.Fatalf("pas fshirjes: status=%s phone=%v", status, phone)
	}
	if sess, _ := svc.Sessions(ctx, a); len(sess) != 0 {
		t.Fatalf("sesionet duhej të ishin shkyçur: %+v", sess)
	}
	if _, err := svc.UpdateProfile(ctx, a, ProfileUpdate{Locale: &[]string{"de"}[0]}); !errors.Is(err, httpx.ErrUnauthorized) {
		t.Fatalf("përdoruesi i fshirë: %v", err)
	}

	// audit + outbox u shkruan në të njëjtat transaksione
	var audits, evs int
	pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_id = $1`, a.UserID).Scan(&audits)
	pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1`, a.UserID.String()).Scan(&evs)
	// profile_updated, address_added ×2, address_deleted, preferences_updated, user.deleted = 6
	if audits < 6 || evs < 4 {
		t.Fatalf("audit=%d outbox=%d", audits, evs)
	}
}
