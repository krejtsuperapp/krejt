package media

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/media"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

func TestKeyRoundTrip(t *testing.T) {
	owner := uuid.New()
	key := media.NewKey(media.KindMerchantLogo, owner, "jpg")
	if !strings.HasPrefix(key, "media/merchant_logo/"+owner.String()+"/") || !strings.HasSuffix(key, ".jpg") {
		t.Fatalf("çelës i papritur: %s", key)
	}
	kind, got, ok := media.Parse(key)
	if !ok || kind != media.KindMerchantLogo || got != owner {
		t.Fatalf("parse: kind=%s owner=%s ok=%v", kind, got, ok)
	}
	for _, bad := range []string{
		"drivers/" + owner.String() + "/id_card/x.jpg", // dokument shoferi: jo media publike
		"media/merchant_logo/jo-uuid/x.jpg",
		"media/unknown/" + owner.String() + "/x.jpg",
		"media/merchant_logo/" + owner.String() + "/pa-prapashtesë",
		"../media/merchant_logo/" + owner.String() + "/x.jpg",
	} {
		if _, _, ok := media.Parse(bad); ok {
			t.Fatalf("u pranua çelës i keq: %s", bad)
		}
	}
}

func TestPublicURL(t *testing.T) {
	media.SetBaseURL("")
	k := "media/user_photo/x/y.jpg"
	if media.URL(&k) != nil {
		t.Fatal("pa bazë nuk duhet të ketë URL")
	}
	media.SetBaseURL("https://media.example/")
	if u := media.URL(&k); u == nil || *u != "https://media.example/media/user_photo/x/y.jpg" {
		t.Fatalf("url: %v", u)
	}
	if media.URL(nil) != nil {
		t.Fatal("çelës nil → URL nil")
	}
}

// --- membership i rremë -----------------------------------------------------------

type fakeMembers struct{ roles map[[2]uuid.UUID]string }

func (f fakeMembers) Membership(_ context.Context, userID, merchantID uuid.UUID) (string, error) {
	if r, ok := f.roles[[2]uuid.UUID{userID, merchantID}]; ok {
		return r, nil
	}
	return "", httpx.ErrForbidden
}

// --- test integrimi (Postgres + DevFS) --------------------------------------------

func TestMediaFlow(t *testing.T) {
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
	fs, err := storage.NewDevFS(t.TempDir(), "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	media.SetBaseURL("http://localhost:8080/api/v1/dev/uploads")

	newUser := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38346"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	ownerID, staffID, strangerID := newUser(), newUser(), newUser()

	var merchantID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO merchants (owner_user_id, type, name, slug, address_line1, city, lat, lng, status)
		VALUES ($2, 'restaurant', 'Prova Media', $1, 'Rr. 1', 'Prishtinë', 42.66, 21.16, 'active') RETURNING id`, "prova-media-"+uuid.NewString()[:8], ownerID).Scan(&merchantID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM merchants WHERE id = $1`, merchantID) })
	var productID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO products (merchant_id, name, price_minor) VALUES ($1, 'Qebapa', 350) RETURNING id`, merchantID).Scan(&productID); err != nil {
		t.Fatal(err)
	}

	members := fakeMembers{roles: map[[2]uuid.UUID]string{
		{ownerID, merchantID}: "owner",
		{staffID, merchantID}: "staff",
	}}
	svc := New(pool, fs, members)
	owner := principal.Actor{UserID: ownerID}
	staff := principal.Actor{UserID: staffID}
	stranger := principal.Actor{UserID: strangerID}

	upload := func(a principal.Actor, kind string, target *uuid.UUID, ct string, body []byte) (*UploadResponse, error) {
		out, err := svc.UploadURL(ctx, a, UploadRequest{Kind: kind, TargetID: target, ContentType: ct, SizeBytes: int64(len(body))})
		if err != nil {
			return nil, err
		}
		if err := fs.Put(out.ObjectKey, ct, bytes.NewReader(body)); err != nil {
			t.Fatal(err)
		}
		return out, nil
	}
	png := []byte("\x89PNG\r\n\x1a\nprova")

	// 1) logoja: vetëm owner/manager
	if _, err := svc.UploadURL(ctx, staff, UploadRequest{Kind: "merchant_logo", TargetID: &merchantID, ContentType: "image/png", SizeBytes: 10}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("stafi nuk duhet të ndryshojë logon: %v", err)
	}
	if _, err := svc.UploadURL(ctx, stranger, UploadRequest{Kind: "merchant_logo", TargetID: &merchantID, ContentType: "image/png", SizeBytes: 10}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("i huaji: %v", err)
	}
	up, err := upload(owner, "merchant_logo", &merchantID, "image/png", png)
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.Confirm(ctx, owner, ConfirmRequest{ObjectKey: up.ObjectKey})
	if err != nil {
		t.Fatal(err)
	}
	if c.URL == nil || !strings.HasSuffix(*c.URL, up.ObjectKey) || c.TargetID != merchantID {
		t.Fatalf("konfirmimi: %+v", c)
	}
	var logoKey *string
	if err := pool.QueryRow(ctx, `SELECT logo_key FROM merchants WHERE id = $1`, merchantID).Scan(&logoKey); err != nil || logoKey == nil || *logoKey != up.ObjectKey {
		t.Fatalf("logo_key nuk u vendos: %v %v", logoKey, err)
	}

	// 2) zëvendësimi fshin objektin e vjetër
	up2, err := upload(owner, "merchant_logo", &merchantID, "image/jpeg", []byte("jpegprova"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(ctx, owner, ConfirmRequest{ObjectKey: up2.ObjectKey}); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Head(ctx, up.ObjectKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("objekti i vjetër duhej fshirë: %v", err)
	}

	// 3) çelësi i dikujt tjetër nuk konfirmohet dot
	if _, err := svc.Confirm(ctx, stranger, ConfirmRequest{ObjectKey: up2.ObjectKey}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("i huaji konfirmoi: %v", err)
	}

	// 4) imazhi i produktit: edhe stafi; lloji i gabuar refuzohet
	if _, err := svc.UploadURL(ctx, staff, UploadRequest{Kind: "product_image", TargetID: &productID, ContentType: "application/pdf", SizeBytes: 10}); err == nil {
		t.Fatal("pdf duhej refuzuar")
	}
	pu, err := upload(staff, "product_image", &productID, "image/webp", []byte("webpprova"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(ctx, staff, ConfirmRequest{ObjectKey: pu.ObjectKey}); err != nil {
		t.Fatal(err)
	}

	// 5) objekt i munguar → UPLOAD_NOT_FOUND
	missing, _ := svc.UploadURL(ctx, owner, UploadRequest{Kind: "user_photo", ContentType: "image/png", SizeBytes: 10})
	if _, err := svc.Confirm(ctx, owner, ConfirmRequest{ObjectKey: missing.ObjectKey}); !errors.Is(err, ErrObjectMissing) {
		t.Fatalf("mungesa: %v", err)
	}

	// 6) fotoja e profilit dhe heqja e saj
	ph, err := upload(stranger, "user_photo", nil, "image/png", png)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(ctx, stranger, ConfirmRequest{ObjectKey: ph.ObjectKey}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(ctx, stranger, "user_photo", nil); err != nil {
		t.Fatal(err)
	}
	var photo *string
	if err := pool.QueryRow(ctx, `SELECT photo_key FROM users WHERE id = $1`, strangerID).Scan(&photo); err != nil || photo != nil {
		t.Fatalf("photo_key duhej NULL: %v %v", photo, err)
	}
}
