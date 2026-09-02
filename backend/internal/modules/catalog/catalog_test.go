package catalog

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

type members map[uuid.UUID]uuid.UUID // user → merchant

func (m members) Membership(_ context.Context, userID, merchantID uuid.UUID) (string, error) {
	if m[userID] == merchantID {
		return "owner", nil
	}
	return "", httpx.ErrForbidden
}

func TestCatalogFlow(t *testing.T) {
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
	var ownerID, merchantID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38335"+uuid.NewString()[:6]).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
	if err := pool.QueryRow(ctx, `INSERT INTO merchants (owner_user_id, type, name, slug, address_line1, city, lat, lng, status)
		VALUES ($1, 'restaurant', 'Test', $2, 'Rr', 'Prishtinë', 42.66, 21.16, 'active') RETURNING id`, ownerID, "t-"+uuid.NewString()[:8]).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM merchants WHERE id = $1`, merchantID)
	svc := New(pool, members{ownerID: merchantID})
	owner, stranger := principal.Actor{UserID: ownerID}, principal.Actor{UserID: uuid.New()}

	if _, err := svc.UpsertCategory(ctx, stranger, merchantID, nil, CategoryInput{Name: "Pica"}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("i huaji: %v", err)
	}
	cat, err := svc.UpsertCategory(ctx, owner, merchantID, nil, CategoryInput{Name: "Pica", Sort: 1})
	if err != nil {
		t.Fatal(err)
	}
	avFalse := false
	p, err := svc.UpsertProduct(ctx, owner, merchantID, nil, ProductInput{CategoryID: &cat.ID, Name: "Margarita", PriceMinor: 450, Tags: []string{"Vegetarian", "vegetarian"},
		Modifiers: []ModifierGroupInput{
			{Name: "Madhësia", MinSelect: 1, MaxSelect: 1, Options: []OptionInput{{Name: "E vogël"}, {Name: "E madhe", PriceDeltaMinor: 200}}},
			{Name: "Shtesa", MinSelect: 0, MaxSelect: 2, Options: []OptionInput{{Name: "Djathë", PriceDeltaMinor: 50}, {Name: "Ullinj", PriceDeltaMinor: 50}, {Name: "Proshutë", PriceDeltaMinor: 100, Available: &avFalse}}},
		}})
	if err != nil || len(p.Modifiers) != 2 || len(p.Modifiers[1].Options) != 3 || len(p.Tags) != 1 {
		t.Fatalf("product: %+v err=%v", p, err)
	}
	// menuja publike: kategori + produkt me modifikues
	menu, err := svc.Menu(ctx, merchantID, false)
	if err != nil || len(menu.Categories) != 1 || len(menu.Products) != 1 || len(menu.Products[0].Modifiers) != 2 {
		t.Fatalf("menu: %+v err=%v", menu, err)
	}
	size, extras := p.Modifiers[0], p.Modifiers[1]
	// çmimi server-side: bazë 450 + e madhe 200 + djathë 50 = 700 × 2 = 1400
	line, err := svc.Price(ctx, merchantID, Selection{ProductID: p.ID, OptionIDs: []uuid.UUID{size.Options[1].ID, extras.Options[0].ID}, Quantity: 2})
	if err != nil || line.UnitMinor != 700 || line.TotalMinor != 1400 || len(line.Options) != 2 {
		t.Fatalf("price: %+v err=%v", line, err)
	}
	// rregullat: pa madhësi (min 1) → gabim; 3 shtesa (max 2) → gabim; opsion i padisponueshëm → gabim
	if _, err := svc.Price(ctx, merchantID, Selection{ProductID: p.ID, Quantity: 1}); !errors.Is(err, ErrModifiers) {
		t.Fatalf("pa madhësi: %v", err)
	}
	if _, err := svc.Price(ctx, merchantID, Selection{ProductID: p.ID, OptionIDs: []uuid.UUID{size.Options[0].ID, extras.Options[0].ID, extras.Options[1].ID, extras.Options[2].ID}, Quantity: 1}); !errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrModifiers) {
		t.Fatalf("shtesa të tepërta / e padisponueshme: %v", err)
	}
	if _, err := svc.Price(ctx, merchantID, Selection{ProductID: p.ID, OptionIDs: []uuid.UUID{size.Options[0].ID, uuid.New()}, Quantity: 1}); !errors.Is(err, ErrModifiers) {
		t.Fatalf("opsion i huaj: %v", err)
	}
	if _, err := svc.Price(ctx, uuid.New(), Selection{ProductID: p.ID, OptionIDs: []uuid.UUID{size.Options[0].ID}, Quantity: 1}); !errors.Is(err, ErrWrongMerchant) {
		t.Fatalf("merchant tjetër: %v", err)
	}
	// disponueshmëria dhe cache-i (invalidohet)
	if err := svc.SetAvailability(ctx, owner, merchantID, p.ID, false); err != nil {
		t.Fatal(err)
	}
	if menu, _ = svc.Menu(ctx, merchantID, false); len(menu.Products) != 0 {
		t.Fatalf("produkti i padisponueshëm në menunë publike: %+v", menu.Products)
	}
	if menu, _ = svc.Menu(ctx, merchantID, true); len(menu.Products) != 1 {
		t.Fatal("stafi duhet ta shohë edhe të padisponueshmin")
	}
	if _, err := svc.Price(ctx, merchantID, Selection{ProductID: p.ID, OptionIDs: []uuid.UUID{size.Options[0].ID}, Quantity: 1}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("i padisponueshëm: %v", err)
	}
	if err := svc.DeleteProduct(ctx, owner, merchantID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteCategory(ctx, owner, merchantID, cat.ID); err != nil {
		t.Fatal(err)
	}
}
