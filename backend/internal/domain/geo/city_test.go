package geo

import "testing"

func TestCityOf(t *testing.T) {
	cases := map[string]Point{
		"Prishtinë": {42.6629, 21.1655},
		"Prizren":   {42.2139, 20.7397},
		"Ferizaj":   {42.3706, 21.1483},
	}
	for want, p := range cases {
		if got := CityOf(p); got != want {
			t.Errorf("%v: mora %q, prisja %q", p, got, want)
		}
	}
	// Një pikë mes qyteteve merr më të afërtin, jo të parin e listës.
	if got := CityOf(Point{42.64, 21.10}); got != "Fushë Kosovë" {
		t.Errorf("mes qytetesh: %q", got)
	}
	if got := CityOf(Point{48.2, 16.3}); got != "" {
		t.Errorf("jashtë Kosovës: %q", got)
	}
}
