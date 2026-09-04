package promos

import (
	"errors"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{" krejt-10 ": "KREJT10", "mirë se vjen": "MIRËSEVJEN", "": ""} {
		if got := Normalize(in); got != want {
			t.Errorf("%q → %q, pritej %q", in, got, want)
		}
	}
}

func TestDiscount(t *testing.T) {
	pct := Coupon{Kind: "percent", PercentBP: 1500}
	if got := Discount(pct, 1000); got != 150 {
		t.Fatalf("15%% e 10,00 = %d", got)
	}
	if got := Discount(pct, 0); got != 0 {
		t.Fatalf("baza 0: %d", got)
	}
	fixed := Coupon{Kind: "fixed", AmountMinor: 300}
	if got := Discount(fixed, 250); got != 250 {
		t.Fatalf("zbritja fikse nuk kalon bazën: %d", got)
	}
	if got := Discount(fixed, 1000); got != 300 {
		t.Fatalf("zbritja fikse: %d", got)
	}
}

func TestCheckRules(t *testing.T) {
	s := &Service{now: func() time.Time { return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC) }}
	past := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	one := 1
	cases := []struct {
		name string
		c    Coupon
		uses int
		want error
	}{
		{"aktiv, të gjitha", Coupon{Active: true, Scope: ScopeAll}, 0, nil},
		{"joaktiv", Coupon{Active: false, Scope: ScopeAll}, 0, ErrInvalid},
		{"fushë tjetër", Coupon{Active: true, Scope: ScopeParcels}, 0, ErrNotApplicable},
		{"s'ka nisur", Coupon{Active: true, Scope: ScopeAll, StartsAt: &future}, 0, ErrExpired},
		{"ka skaduar", Coupon{Active: true, Scope: ScopeAll, EndsAt: &past}, 0, ErrExpired},
		{"përdorur gjithsej", Coupon{Active: true, Scope: ScopeAll, MaxUses: &one, UsesCount: 1}, 0, ErrUsedUp},
		{"përdorur nga ky", Coupon{Active: true, Scope: ScopeAll, MaxUsesPerUser: &one}, 1, ErrUsedUp},
	}
	for _, tc := range cases {
		if err := s.check(&tc.c, ScopeFood, tc.uses); !errors.Is(err, tc.want) {
			t.Errorf("%s: mora %v, prisja %v", tc.name, err, tc.want)
		}
	}
}

func TestUpsertValidation(t *testing.T) {
	in := UpsertInput{Code: "ab", Kind: "percent", PercentBP: 500}
	if err := in.validate(); err == nil {
		t.Fatal("kodi 2 shkronja duhet refuzuar")
	}
	in = UpsertInput{Code: "krejt10", Kind: "percent", PercentBP: 20000}
	if err := in.validate(); err == nil {
		t.Fatal("200% duhet refuzuar")
	}
	in = UpsertInput{Code: "krejt10", Kind: "fixed", AmountMinor: 200, Scope: "rides"}
	if err := in.validate(); err == nil {
		t.Fatal("fusha rides s'ekziston")
	}
	in = UpsertInput{Code: " krejt-10 ", Kind: "fixed", AmountMinor: 200}
	if err := in.validate(); err != nil || in.Code != "KREJT10" || in.Scope != ScopeAll {
		t.Fatalf("err=%v code=%s scope=%s", err, in.Code, in.Scope)
	}
}
