package pricing

import "testing"

func TestCompute(t *testing.T) {
	economy := Rule{BaseMinor: 100, PerKmMinor: 45, PerMinMinor: 8, MinimumMinor: 200, SurgeBP: 10000}
	cases := []struct {
		name        string
		distM, durS int
		want        int64
	}{
		// 5 km, 12 min: 100 + 225 + 96 = 421 → 430 (rrumbullakim lart në 10 cent)
		{"5 km / 12 min", 5000, 720, 430},
		// 0.6 km, 2 min: 100 + 27 + 16 = 143 → 150 → nën minimum → 200
		{"minimum", 600, 120, 200},
		{"zero", 0, 0, 200},
		{"negativ mbrohet", -10, -10, 200},
	}
	for _, c := range cases {
		if got := Compute(economy, c.distM, c.durS); got != c.want {
			t.Errorf("%s: Compute = %d, want %d", c.name, got, c.want)
		}
	}
	// surge 1.5×: 421 × 1.5 = 631.5 → 632 → 640
	surged := economy
	surged.SurgeBP = 15000
	if got := Compute(surged, 5000, 720); got != 640 {
		t.Errorf("surge: %d, want 640", got)
	}
	// surge nën 1.0 nuk lejohet (mbrojtje: trajtohet si 1.0)
	under := economy
	under.SurgeBP = 5000
	if got := Compute(under, 5000, 720); got != 430 {
		t.Errorf("surge < 1: %d, want 430", got)
	}
}

func TestCommission(t *testing.T) {
	if got := Commission(430, 1500); got != 65 { // 64.5 → 65
		t.Errorf("Commission(430, 15%%) = %d, want 65", got)
	}
	if got := Commission(1000, 0); got != 0 {
		t.Errorf("0%%: %d", got)
	}
	if got := Commission(1000, -5); got != 0 {
		t.Errorf("negativ: %d", got)
	}
}
