package documents

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
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

func TestEligibility(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	soon := now.AddDate(0, 0, 10)
	late := now.AddDate(2, 0, 0)
	past := now.AddDate(0, 0, -1)
	approved := map[string]*time.Time{
		"profile_photo": nil, "id_card": &late, "driving_license": &soon, "vehicle_registration": &late, "insurance": &past, "criminal_record": &late,
	}
	missing, expiring := eligibility([]string{"economy"}, approved, now)
	if strings.Join(missing, ",") != "insurance" || strings.Join(expiring, ",") != "driving_license" {
		t.Fatalf("missing=%v expiring=%v", missing, expiring)
	}
	// taxi kërkon edhe taxi_permit
	missing, _ = eligibility([]string{"economy", "taxi"}, approved, now)
	if strings.Join(missing, ",") != "insurance,taxi_permit" {
		t.Fatalf("taxi missing=%v", missing)
	}
}

// --- test integrimi (Postgres + DevFS në disk) ---------------------------------

func TestDocumentsFlow(t *testing.T) {
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
	svc := New(pool, fs)

	newUser := func() uuid.UUID {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, 'sq') RETURNING id`, "+38346"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id
	}
	driverID, opsID, otherID := newUser(), newUser(), newUser()
	if _, err := pool.Exec(ctx, `INSERT INTO drivers (user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
		VALUES ($1, 'approved', 'Opel', 'Astra', '03-111-AA', 'e zezë', '{economy}')`, driverID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_capabilities (user_id, capability) VALUES ($1, 'RIDE_DRIVER')`, driverID); err != nil {
		t.Fatal(err)
	}
	d, ops, other := principal.Actor{UserID: driverID}, principal.Actor{UserID: opsID}, principal.Actor{UserID: otherID}

	// jo shofer → refuzohet
	if _, err := svc.UploadURL(ctx, other, UploadRequest{Type: "id_card", ContentType: "image/jpeg", SizeBytes: 100}); !errors.Is(err, ErrNotDriver) {
		t.Fatalf("jo shofer: %v", err)
	}
	// validim: lloj/lloj përmbajtjeje/madhësi
	if _, err := svc.UploadURL(ctx, d, UploadRequest{Type: "passport", ContentType: "image/gif", SizeBytes: MaxSizeBytes + 1}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("validimi: %v", err)
	}
	if _, err := svc.UploadURL(ctx, d, UploadRequest{Type: "profile_photo", ContentType: "application/pdf", SizeBytes: 10}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("foto si pdf: %v", err)
	}

	content := bytes.Repeat([]byte("x"), 1234)
	up, err := svc.UploadURL(ctx, d, UploadRequest{Type: "driving_license", ContentType: "image/jpeg", SizeBytes: int64(len(content))})
	if err != nil || !strings.HasPrefix(up.ObjectKey, "drivers/"+driverID.String()+"/driving_license/") || up.Upload.Method != "PUT" {
		t.Fatalf("upload url: %+v err=%v", up, err)
	}
	// konfirmim para ngarkimit → objekti mungon
	exp := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if _, err := svc.Confirm(ctx, d, ConfirmRequest{Type: "driving_license", ObjectKey: up.ObjectKey, ExpiresOn: exp}); !errors.Is(err, ErrObjectMissing) {
		t.Fatalf("pa ngarkim: %v", err)
	}
	// ngarkim me madhësi tjetër → refuzohet nga devfs
	if err := fs.Put(up.ObjectKey, "image/jpeg", bytes.NewReader(content[:100])); err == nil {
		t.Fatal("madhësi tjetër duhej refuzuar")
	}
	if err := fs.Put(up.ObjectKey, "image/jpeg", bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	// çelës i tjetërkujt / datë e kaluar
	if _, err := svc.Confirm(ctx, other, ConfirmRequest{Type: "driving_license", ObjectKey: up.ObjectKey, ExpiresOn: exp}); !errors.Is(err, ErrNotDriver) {
		t.Fatalf("çelësi i tjetërkujt: %v", err)
	}
	if _, err := svc.Confirm(ctx, d, ConfirmRequest{Type: "driving_license", ObjectKey: up.ObjectKey, ExpiresOn: "2020-01-01"}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("datë e kaluar: %v", err)
	}
	doc, err := svc.Confirm(ctx, d, ConfirmRequest{Type: "driving_license", ObjectKey: up.ObjectKey, ExpiresOn: exp})
	if err != nil || doc.Status != "pending" || doc.SizeBytes != 1234 {
		t.Fatalf("confirm: %+v err=%v", doc, err)
	}
	// ridërgim i të njëjtit lloj → i pari 'replaced'
	up2, _ := svc.UploadURL(ctx, d, UploadRequest{Type: "driving_license", ContentType: "application/pdf", SizeBytes: int64(len(content))})
	_ = fs.Put(up2.ObjectKey, "application/pdf", bytes.NewReader(content))
	doc2, err := svc.Confirm(ctx, d, ConfirmRequest{Type: "driving_license", ObjectKey: up2.ObjectKey, ExpiresOn: exp})
	if err != nil {
		t.Fatal(err)
	}
	var st string
	pool.QueryRow(ctx, `SELECT status FROM driver_documents WHERE id = $1`, doc.ID).Scan(&st)
	if st != "replaced" {
		t.Fatalf("i pari duhej replaced: %s", st)
	}
	// shqyrtimi
	pend, _ := svc.Pending(ctx, 50)
	found := false
	for _, p := range pend {
		if p.ID == doc2.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("dokumenti i ri duhej në pritje")
	}
	if _, err := svc.Review(ctx, ops, doc2.ID, "reject", ""); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("refuzim pa arsye: %v", err)
	}
	rev, err := svc.Review(ctx, ops, doc2.ID, "approve", "")
	if err != nil || rev.Status != "approved" || rev.ReviewedAt == nil {
		t.Fatalf("approve: %+v err=%v", rev, err)
	}
	if _, err := svc.Review(ctx, ops, doc2.ID, "approve", ""); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("shqyrtim i dytë: %v", err)
	}
	ov, err := svc.List(ctx, driverID, true)
	if err != nil || ov.Eligible || len(ov.Documents) != 1 || ov.Documents[0].DownloadURL == "" {
		t.Fatalf("overview: %+v err=%v", ov, err)
	}
	if len(ov.Missing) != 5 { // profile_photo, id_card, vehicle_registration, insurance, criminal_record
		t.Fatalf("missing = %v", ov.Missing)
	}

	// skadimi: patenta skadon → dokumenti 'expired', shoferi i miratuar pezullohet dhe humb kapacitetin
	if _, err := pool.Exec(ctx, `UPDATE driver_documents SET expires_on = current_date - 1 WHERE id = $1`, doc2.ID); err != nil {
		t.Fatal(err)
	}
	expired, suspended, err := svc.ExpireSweep(ctx)
	if err != nil || expired < 1 || suspended < 1 {
		t.Fatalf("sweep: expired=%d suspended=%d err=%v", expired, suspended, err)
	}
	var dst string
	var revoked *time.Time
	pool.QueryRow(ctx, `SELECT status FROM drivers WHERE user_id = $1`, driverID).Scan(&dst)
	pool.QueryRow(ctx, `SELECT revoked_at FROM user_capabilities WHERE user_id = $1 AND capability = 'RIDE_DRIVER'`, driverID).Scan(&revoked)
	if dst != "suspended" || revoked == nil {
		t.Fatalf("pas skadimit: status=%s revoked=%v", dst, revoked)
	}
}
