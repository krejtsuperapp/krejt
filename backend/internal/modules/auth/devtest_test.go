package auth

import (
	"context"
	"errors"
	"testing"
)

// Kodi fiks i provës nuk prek fare bazën: rruga e shkurtër duhet të vendosë para çdo pyetjeje.
// Prandaj transaksioni këtu është nil — nëse dikush e zhvendos kontrollin pas pyetjes, testi bie.
func TestDevTestPhoneFixedCode(t *testing.T) {
	s := (&Service{}).WithDevTestPhones([]string{"+38344100200"}, "111111")
	ctx := context.Background()

	t.Run("numri i provës me kodin fiks kalon", func(t *testing.T) {
		if err := s.verifyChallenge(ctx, nil, "+38344100200", "111111"); err != nil {
			t.Fatalf("prisja kalim, mora %v", err)
		}
	})

	t.Run("numri i provës me kod të gabuar refuzohet", func(t *testing.T) {
		err := s.verifyChallenge(ctx, nil, "+38344100200", "000000")
		if !errors.Is(err, ErrOTPInvalid) {
			t.Fatalf("prisja ErrOTPInvalid, mora %v", err)
		}
	})

	t.Run("lista bosh nuk aktivizon asgjë", func(t *testing.T) {
		plain := (&Service{}).WithDevTestPhones(nil, "111111")
		if plain.isTestPhone("+38344100200") {
			t.Fatal("pa listë, asnjë numër nuk duhet të jetë i provës")
		}
	})

	t.Run("kodi bosh nuk aktivizon asgjë", func(t *testing.T) {
		plain := (&Service{}).WithDevTestPhones([]string{"+38344100200"}, "")
		if plain.isTestPhone("+38344100200") {
			t.Fatal("pa kod, lista nuk duhet të ketë efekt")
		}
	})

	t.Run("numër tjetër nuk është i provës", func(t *testing.T) {
		if s.isTestPhone("+38344123456") {
			t.Fatal("vetëm numrat e listuar janë të provës")
		}
	})
}
