// Package geo — pika, distanca dhe kufijtë gjeografikë. Vetëm Kosovë në V1 (§1).
package geo

import "math"

type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Valid — koordinata reale (dhe jo 0,0 — "pa sinjal").
func (p Point) Valid() bool {
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180 && !(p.Lat == 0 && p.Lng == 0)
}

// Kufijtë e Kosovës (kuti kufizuese, V1). Poligoni i saktë vjen me zonat e shërbimit (H3) në modulin `maps`.
const (
	KosovoMinLat = 41.85
	KosovoMaxLat = 43.30
	KosovoMinLng = 19.95
	KosovoMaxLng = 21.80
)

func InKosovo(p Point) bool {
	return p.Lat >= KosovoMinLat && p.Lat <= KosovoMaxLat && p.Lng >= KosovoMinLng && p.Lng <= KosovoMaxLng
}

const earthRadiusM = 6371000.0

// Haversine — distanca në metra në vijë ajrore.
func Haversine(a, b Point) float64 {
	la1, la2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

// Qytetet kryesore të Kosovës, për të thënë "afër ku" pa zbuluar adresën e saktë (§57).
var cities = []struct {
	Name  string
	Point Point
}{
	{"Prishtinë", Point{42.6629, 21.1655}},
	{"Prizren", Point{42.2139, 20.7397}},
	{"Pejë", Point{42.6593, 20.2887}},
	{"Gjakovë", Point{42.3803, 20.4308}},
	{"Gjilan", Point{42.4637, 21.4694}},
	{"Mitrovicë", Point{42.8914, 20.8660}},
	{"Ferizaj", Point{42.3706, 21.1483}},
	{"Podujevë", Point{42.9106, 21.1933}},
	{"Vushtrri", Point{42.8231, 20.9675}},
	{"Suharekë", Point{42.3586, 20.8256}},
	{"Rahovec", Point{42.3994, 20.6547}},
	{"Drenas", Point{42.6242, 20.8931}},
	{"Lipjan", Point{42.5244, 21.1256}},
	{"Malishevë", Point{42.4822, 20.7458}},
	{"Fushë Kosovë", Point{42.6367, 21.0972}},
	{"Skenderaj", Point{42.7469, 20.7897}},
	{"Viti", Point{42.3222, 21.3572}},
	{"Deçan", Point{42.5406, 20.2886}},
	{"Istog", Point{42.7803, 20.4856}},
	{"Kamenicë", Point{42.5786, 21.5772}},
}

// CityOf — qyteti më i afërt me këtë pikë; bosh kur pika është jashtë Kosovës.
func CityOf(p Point) string {
	if !InKosovo(p) {
		return ""
	}
	best, dist := "", 0.0
	for i, c := range cities {
		d := Haversine(p, c.Point)
		if i == 0 || d < dist {
			best, dist = c.Name, d
		}
	}
	return best
}
