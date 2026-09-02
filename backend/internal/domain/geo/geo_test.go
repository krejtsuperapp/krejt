package geo

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	prishtina := Point{42.6629, 21.1655}
	prizren := Point{42.2139, 20.7397}
	d := Haversine(prishtina, prizren)
	// ~61 km në vijë ajrore
	if math.Abs(d-61000) > 2000 {
		t.Fatalf("Prishtinë–Prizren = %.0f m, pritej ~61 km", d)
	}
	if Haversine(prishtina, prishtina) != 0 {
		t.Fatal("distanca me veten duhet 0")
	}
}

func TestValidAndInKosovo(t *testing.T) {
	if !(Point{42.66, 21.16}).Valid() || (Point{}).Valid() || (Point{91, 0}).Valid() {
		t.Fatal("Valid")
	}
	if !InKosovo(Point{42.66, 21.16}) || InKosovo(Point{41.33, 19.82}) {
		t.Fatal("InKosovo")
	}
}
