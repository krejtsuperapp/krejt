package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPhoneValidation(t *testing.T) {
	ok := []string{"+38344123456", "+355691234567", "+4915112345678", "+41791234567"}
	bad := []string{"044123456", "+383 44 123 456", "+0123", "38344123456", "+383441234567890123"}
	for _, p := range ok {
		if !ValidPhone(p) {
			t.Errorf("expected valid: %s", p)
		}
	}
	for _, p := range bad {
		if ValidPhone(p) {
			t.Errorf("expected invalid: %s", p)
		}
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	signer, err := GenerateEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	uid, sid := uuid.New(), uuid.New()
	tok, err := signer.IssueAccess(uid, sid, []string{"CUSTOMER"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != uid.String() || claims.SessionID != sid.String() || len(claims.Capabilities) != 1 {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	// çelës tjetër → i pavlefshëm
	other, _ := GenerateEphemeralSigner()
	if _, err := other.Verify(tok); err == nil {
		t.Fatal("token verified with wrong key")
	}
	// i skaduar → i pavlefshëm
	old, _ := signer.IssueAccess(uid, sid, nil, time.Now().Add(-2*AccessTokenTTL))
	if _, err := signer.Verify(old); err == nil {
		t.Fatal("expired token verified")
	}
}

func TestRefreshTokenHash(t *testing.T) {
	tok, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) < 40 || len(hash) != 32 {
		t.Fatalf("unexpected sizes: %d %d", len(tok), len(hash))
	}
	if string(HashToken(tok)) != string(hash) {
		t.Fatal("hash mismatch")
	}
}

func TestHasCap(t *testing.T) {
	if !hasCap([]string{"CUSTOMER", "RIDE_DRIVER"}, "RIDE_DRIVER") || hasCap([]string{"CUSTOMER"}, "ADMIN") || !hasCap([]string{"SUPER_ADMIN"}, "FINANCE") {
		t.Fatal("capability check wrong")
	}
}
