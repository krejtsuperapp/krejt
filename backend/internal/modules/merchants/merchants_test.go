package merchants

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/maps"
)

func TestSlugAndHours(t *testing.T) {
	if got := Slugify("  Qebaptore Çelë & Ëmbëlsira 2  "); got != "qebaptore-cele-embelsira-2" {
		t.Fatalf("slug: %q", got)
	}
	hours := []Hours{{Weekday: 1, Opens: "08:00", Closes: "23:00"}, {Weekday: 5, Opens: "18:00", Closes: "02:00"}}
	mon := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) // e hënë
	if !OpenAt(hours, mon) || OpenAt(hours, mon.Add(12*time.Hour)) {
		t.Fatal("e hënë 12:00 hapur, 00:00 (e martë) mbyllur")
	}
	fri := time.Date(2026, 9, 11, 23, 30, 0, 0, time.UTC) // e premte
	sat := time.Date(2026, 9, 12, 1, 30, 0, 0, time.UTC)  // e shtunë 01:30 (pas mesnate)
	if !OpenAt(hours, fri) || !OpenAt(hours, sat) || OpenAt(hours, sat.Add(time.Hour)) {
		t.Fatal("kalimi i mesnatës")
	}
	if _, ok := parseHM("24:00"); ok {
		t.Fatal("24:00 s'është orë")
	}
}

func TestMerchantFlow(t *testing.T) {
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
	svc := New(pool, pricing.New(pool, maps.DevEstimate{}, nil))
	newUser := func() principal.Actor {
		var id uuid.UUID
		phone := "+38336" + uuid.NewString()[:6]
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, phone).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM merchants WHERE owner_user_id = $1`, id)
			pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		})
		return principal.Actor{UserID: id, IP: "203.0.113.2"}
	}
	owner, ops, staff, stranger := newUser(), newUser(), newUser(), newUser()
	var staffPhone string
	pool.QueryRow(ctx, `SELECT phone_e164 FROM users WHERE id = $1`, staff.UserID).Scan(&staffPhone)

	if _, err := svc.Apply(ctx, owner, ApplyInput{Type: "bar", Name: "X", AddressLine1: "R", City: "P", Location: geo.Point{Lat: 41.3, Lng: 19.8}}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("validimi: %v", err)
	}
	name := "Qebaptore Testi " + uuid.NewString()[:4]
	m, err := svc.Apply(ctx, owner, ApplyInput{Type: "restaurant", Name: name, AddressLine1: "Rr. UÇK 10", City: "Prishtinë", Location: geo.Point{Lat: 42.6629, Lng: 21.1655}, Cuisines: []string{"Traditional", "grill", "traditional"}})
	if err != nil || m.Status != "pending" || m.ServiceAreaID == nil || *m.ServiceAreaID != "prishtina" || len(m.Cuisines) != 2 {
		t.Fatalf("apply: %+v err=%v", m, err)
	}
	// i njëjti emër → slug me prapashtesë
	m2, err := svc.Apply(ctx, owner, ApplyInput{Type: "restaurant", Name: name, AddressLine1: "Rr. UÇK 12", City: "Prishtinë", Location: geo.Point{Lat: 42.66, Lng: 21.16}})
	if err != nil || m2.Slug == m.Slug {
		t.Fatalf("slug i dytë: %+v err=%v", m2, err)
	}
	// i panjohuri s'e sheh, s'e ndryshon
	if _, err := svc.Get(ctx, stranger, m.ID); !errors.Is(err, ErrNotMember) {
		t.Fatalf("stranger get: %v", err)
	}
	// publikisht: jo derisa të aktivizohet
	if _, err := svc.BySlug(ctx, m.Slug); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("pending publik: %v", err)
	}
	if _, err := svc.SetStatus(ctx, ops, m.ID, "activate", ""); err != nil {
		t.Fatal(err)
	}
	var caps int
	pool.QueryRow(ctx, `SELECT count(*) FROM user_capabilities WHERE user_id = $1 AND capability = 'MERCHANT' AND revoked_at IS NULL`, owner.UserID).Scan(&caps)
	if caps != 1 {
		t.Fatal("pronari duhej të merrte MERCHANT")
	}
	// orare, staf, profil
	if _, err := svc.SetHours(ctx, owner, m.ID, []Hours{{Weekday: int(time.Now().Weekday()), Opens: "00:00", Closes: "23:59"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddStaff(ctx, owner, m.ID, staffPhone, "staff"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddStaff(ctx, owner, m.ID, "+38300000000", "staff"); !errors.Is(err, ErrStaffMissing) {
		t.Fatalf("staf pa llogari: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, staff, m.ID, ProfileUpdate{PrepTimeMin: intPtr(25)}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("stafi nuk ndryshon profilin: %v", err)
	}
	up, err := svc.UpdateProfile(ctx, owner, m.ID, ProfileUpdate{PrepTimeMin: intPtr(25), DeliveryFeeMinor: int64Ptr(200)})
	if err != nil || up.PrepTimeMin != 25 || up.DeliveryFeeMinor != 200 || !up.OpenNow {
		t.Fatalf("update: %+v err=%v", up, err)
	}
	// zbulimi: afër, me distancë, hapur; kërkim pa theksa
	list, err := svc.Discover(ctx, DiscoverFilter{At: &geo.Point{Lat: 42.66, Lng: 21.16}, Query: "qebaptore testi"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range list {
		if x.ID == m.ID {
			found = true
			if x.DistanceM == nil || !x.OpenNow || x.Phone != nil {
				t.Fatalf("discover row: %+v", x)
			}
		}
	}
	if !found {
		t.Fatal("merchant-i aktiv duhej në zbulim")
	}
	pub, err := svc.BySlug(ctx, m.Slug)
	if err != nil || pub.Phone != nil || len(pub.Hours) != 1 {
		t.Fatalf("by slug: %+v err=%v", pub, err)
	}
	// pezullimi e heq nga publiku
	if _, err := svc.SetStatus(ctx, ops, m.ID, "suspend", "dokumente të pavlefshme"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BySlug(ctx, m.Slug); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("i pezulluar publik: %v", err)
	}
}

func intPtr(i int) *int       { return &i }
func int64Ptr(i int64) *int64 { return &i }
