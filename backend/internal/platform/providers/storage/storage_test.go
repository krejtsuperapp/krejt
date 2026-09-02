package storage

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestValidKey(t *testing.T) {
	for _, ok := range []string{"drivers/abc/id_card/x.jpg", "a/b-c_d.pdf", "abcd"} {
		if !ValidKey(ok) {
			t.Errorf("%q duhej i vlefshëm", ok)
		}
	}
	for _, bad := range []string{"", "/abs", "../x", "a/../b", "A/Upper", "sp ace", "abc"} {
		if ValidKey(bad) {
			t.Errorf("%q duhej i pavlefshëm", bad)
		}
	}
}

func TestDevFSRoundTrip(t *testing.T) {
	fs, err := NewDevFS(t.TempDir(), "http://localhost:8080/")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	body := bytes.Repeat([]byte("k"), 500)
	target, err := fs.PresignUpload(ctx, "drivers/d1/id_card/f.png", "image/png", 500, time.Minute)
	if err != nil || target.URL != "http://localhost:8080/api/v1/dev/uploads/drivers/d1/id_card/f.png" || target.Headers["Content-Type"] != "image/png" {
		t.Fatalf("presign: %+v err=%v", target, err)
	}
	if _, err := fs.Head(ctx, "drivers/d1/id_card/f.png"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("head para ngarkimit: %v", err)
	}
	if err := fs.Put("drivers/d1/id_card/f.png", "image/jpeg", bytes.NewReader(body)); err == nil {
		t.Fatal("content-type tjetër duhej refuzuar")
	}
	if err := fs.Put("unknown/key.png", "image/png", bytes.NewReader(body)); err == nil {
		t.Fatal("çelës i panjohur duhej refuzuar")
	}
	if err := fs.Put("drivers/d1/id_card/f.png", "image/png", bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	info, err := fs.Head(ctx, "drivers/d1/id_card/f.png")
	if err != nil || info.SizeBytes != 500 || info.ContentType != "image/png" {
		t.Fatalf("head: %+v err=%v", info, err)
	}
	rc, ct, err := fs.Open("drivers/d1/id_card/f.png")
	if err != nil || ct != "image/png" {
		t.Fatalf("open: %v %s", err, ct)
	}
	rc.Close()
	if err := fs.Delete(ctx, "drivers/d1/id_card/f.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Head(ctx, "drivers/d1/id_card/f.png"); !errors.Is(err, ErrNotFound) {
		t.Fatal("pas fshirjes duhej NotFound")
	}
	if _, err := NewFromEnv(ctx, "production", "devfs", "", "", "", "", nil); err == nil {
		t.Fatal("devfs në production duhej refuzuar")
	}
}
