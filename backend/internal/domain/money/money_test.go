package money

import "testing"

func TestPercentRoundsHalfUp(t *testing.T) {
	cases := []struct {
		amount int64
		bps    int64
		want   int64
	}{
		{1240, 1800, 223}, // 18 % komision mbi 12,40 € = 2,232 → 223
		{280, 1500, 42},   // 15 % mbi 2,80 € = 0,42
		{1, 5000, 1},      // 0,5 cent → 1 (half up)
		{0, 1800, 0},
	}
	for _, c := range cases {
		got := EUR(c.amount).Percent(c.bps).Minor
		if int64(got) != c.want {
			t.Fatalf("Percent(%d, %d) = %d, want %d", c.amount, c.bps, got, c.want)
		}
	}
}

func TestAddRejectsCurrencyMismatch(t *testing.T) {
	_, err := EUR(100).Add(Amount{Minor: 100, Currency: "ALL"})
	if err == nil {
		t.Fatal("expected currency mismatch error")
	}
}

func TestString(t *testing.T) {
	if s := EUR(-1240).String(); s != "-12.40 EUR" {
		t.Fatalf("got %q", s)
	}
}
