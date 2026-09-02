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
