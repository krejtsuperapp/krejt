package business

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Kufiri është mujor dhe rifillon vetë. Asnjë punë e planifikuar nuk e "reseton": çdo gjë që
// duhet rivendosur nga jashtë mund të harrohet, dhe atëherë kufiri do të ishte i përjetshëm.
func TestMonthStart(t *testing.T) {
	at := time.Date(2026, 9, 17, 23, 45, 12, 0, time.UTC)
	got := monthStart(at)
	want := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("monthStart = %v, pritej %v", got, want)
	}
	// Rreshti i parë i muajit rri brenda periudhës së vet.
	first := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if monthStart(first).After(first) {
		t.Fatal("fillimi i muajit nuk duhet ta përjashtojë vetveten")
	}
}

// Kodi i llogarisë ndjek të njëjtën formë si ato ekzistueset: pa këtë, bilanci i një ndërmarrjeje
// do të kërkonte rregulla të vetat te libri.
func TestWalletCodeShape(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	if got := WalletCode(id); got != "business:11111111-2222-3333-4444-555555555555:wallet" {
		t.Fatalf("kodi: %s", got)
	}
}

func TestCleanTrimsAndCuts(t *testing.T) {
	if got := clean("  Ndërmarrja    ime  ", 120); got != "Ndërmarrja ime" {
		t.Fatalf("clean: %q", got)
	}
	if got := clean("abcdef", 3); got != "abc" {
		t.Fatalf("prerja: %q", got)
	}
	// Prerja numëron shkronja, jo bajte: një emër shqip nuk duhet të thyhet në mes të një shkronje.
	if got := clean("ëëëë", 2); got != "ëë" {
		t.Fatalf("shkronjat: %q", got)
	}
}

func TestNullableKeepsEmptyOut(t *testing.T) {
	if nullable("") != nil {
		t.Fatal("bosh duhet të ruhet si NULL, jo si tekst bosh")
	}
	if v := nullable("x"); v == nil || *v != "x" {
		t.Fatal("vlera duhet të kalojë e paprekur")
	}
}
