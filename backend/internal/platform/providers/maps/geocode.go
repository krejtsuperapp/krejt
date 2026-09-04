package maps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"krejt.app/backend/internal/domain/geo"
)

// ErrUnsupported — ofruesi nuk e mbështet këtë funksion (p.sh. gjeokodimi te Google, që s'e kemi lidhur).
var ErrUnsupported = errors.New("maps: unsupported by provider")

// --- Mapbox: Directions me gjeometri + Geocoding v6 ------------------------------------------

func (m *Mapbox) Directions(ctx context.Context, from, to geo.Point) (Route, error) {
	return m.route(ctx, from, to, true)
}

// route — një thirrje e vetme për të dyja: Route (pa gjeometri) dhe Directions (me gjeometri GeoJSON).
func (m *Mapbox) route(ctx context.Context, from, to geo.Point, geometry bool) (Route, error) {
	coords := fmt.Sprintf("%.6f,%.6f;%.6f,%.6f", from.Lng, from.Lat, to.Lng, to.Lat)
	q := url.Values{}
	q.Set("access_token", m.token)
	q.Set("alternatives", "false")
	if geometry {
		q.Set("overview", "full")
		q.Set("geometries", "geojson")
	} else {
		q.Set("overview", "false")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.endpoint+"/"+coords+"?"+q.Encode(), nil)
	if err != nil {
		return Route{}, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return Route{}, fmt.Errorf("maps: mapbox directions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Route{}, fmt.Errorf("maps: mapbox directions: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Code   string `json:"code"`
		Routes []struct {
			DistanceM float64 `json:"distance"`
			DurationS float64 `json:"duration"`
			Geometry  struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Route{}, fmt.Errorf("maps: mapbox directions: decode: %w", err)
	}
	if out.Code != "" && out.Code != "Ok" {
		if out.Code == "NoRoute" || out.Code == "NoSegment" {
			return Route{}, ErrNoRoute
		}
		return Route{}, fmt.Errorf("maps: mapbox directions: %s", out.Code)
	}
	if len(out.Routes) == 0 {
		return Route{}, ErrNoRoute
	}
	r := Route{DistanceM: int(math.Round(out.Routes[0].DistanceM)), DurationS: int(math.Round(out.Routes[0].DurationS))}
	if geometry {
		for _, c := range out.Routes[0].Geometry.Coordinates {
			if len(c) >= 2 {
				r.Path = append(r.Path, geo.Point{Lat: c[1], Lng: c[0]})
			}
		}
	}
	return r, nil
}

const mapboxGeocode = "https://api.mapbox.com/search/geocode/v6"

// geocodeBase — endpoint-i i gjeokodimit; ndryshohet vetëm në teste.
func (m *Mapbox) geocodeBase() string {
	if m.geocode != "" {
		return m.geocode
	}
	return mapboxGeocode
}

type mapboxFeature struct {
	Properties struct {
		Name           string `json:"name"`
		FullAddress    string `json:"full_address"`
		PlaceFormatted string `json:"place_formatted"`
		FeatureType    string `json:"feature_type"`
		Coordinates    struct {
			Longitude float64 `json:"longitude"`
			Latitude  float64 `json:"latitude"`
		} `json:"coordinates"`
	} `json:"properties"`
}

func (f mapboxFeature) place() Place {
	p := f.Properties
	addr := p.FullAddress
	if addr == "" {
		addr = p.PlaceFormatted
	}
	return Place{Name: p.Name, Address: addr, Kind: p.FeatureType, Point: geo.Point{Lat: p.Coordinates.Latitude, Lng: p.Coordinates.Longitude}}
}

func (m *Mapbox) Search(ctx context.Context, q string, near *geo.Point, limit int) ([]Place, error) {
	v := url.Values{}
	v.Set("access_token", m.token)
	v.Set("q", q)
	v.Set("country", "xk")
	v.Set("language", "sq")
	v.Set("limit", fmt.Sprint(limit))
	// Geocoding v6 nuk njeh "poi" (ai jeton te Search Box API); pa këtë filtër Mapbox-i kthen 422.
	v.Set("types", "address,street,place,locality,neighborhood")
	if near != nil {
		v.Set("proximity", fmt.Sprintf("%.5f,%.5f", near.Lng, near.Lat))
	}
	feats, err := m.geocodeCall(ctx, m.geocodeBase()+"/forward?"+v.Encode())
	if err != nil {
		return nil, err
	}
	out := make([]Place, 0, len(feats))
	for _, f := range feats {
		p := f.place()
		if p.Point.Valid() && geo.InKosovo(p.Point) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *Mapbox) Reverse(ctx context.Context, p geo.Point) (*Place, error) {
	v := url.Values{}
	v.Set("access_token", m.token)
	v.Set("longitude", fmt.Sprintf("%.6f", p.Lng))
	v.Set("latitude", fmt.Sprintf("%.6f", p.Lat))
	v.Set("language", "sq")
	v.Set("types", "address,street,neighborhood,locality,place")
	v.Set("limit", "1")
	feats, err := m.geocodeCall(ctx, m.geocodeBase()+"/reverse?"+v.Encode())
	if err != nil {
		return nil, err
	}
	if len(feats) == 0 {
		return nil, nil
	}
	pl := feats[0].place()
	// Adresa e kthyer përshkruan pikën e kërkuar; koordinatat mbeten ato të përdoruesit.
	pl.Point = p
	return &pl, nil
}

func (m *Mapbox) geocodeCall(ctx context.Context, u string) ([]mapboxFeature, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maps: mapbox geocoding: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// URL-ja mban token-in: nuk logohet as ajo, as trupi.
		return nil, fmt.Errorf("maps: mapbox geocoding: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Features []mapboxFeature `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("maps: mapbox geocoding: decode: %w", err)
	}
	return out.Features, nil
}

// --- Google: Directions me polyline; gjeokodimi nuk është lidhur -------------------------------

func (g *Google) Directions(ctx context.Context, from, to geo.Point) (Route, error) {
	return g.route(ctx, from, to, true)
}

func (g *Google) Search(context.Context, string, *geo.Point, int) ([]Place, error) {
	return nil, ErrUnsupported
}

func (g *Google) Reverse(context.Context, geo.Point) (*Place, error) { return nil, ErrUnsupported }

// decodePolyline — algoritmi i Google-it (precision 1e5).
func decodePolyline(s string) []geo.Point {
	var out []geo.Point
	var lat, lng int64
	for i := 0; i < len(s); {
		var result int64
		var shift uint
		for {
			if i >= len(s) {
				return out
			}
			b := int64(s[i]) - 63
			i++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lat += ^(result >> 1)
		} else {
			lat += result >> 1
		}
		result, shift = 0, 0
		for {
			if i >= len(s) {
				return out
			}
			b := int64(s[i]) - 63
			i++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lng += ^(result >> 1)
		} else {
			lng += result >> 1
		}
		out = append(out, geo.Point{Lat: float64(lat) / 1e5, Lng: float64(lng) / 1e5})
	}
	return out
}

// --- DevEstimate: vende të njohura të Prishtinës, vetëm për zhvillim pa ofrues -----------------

var devPlaces = []Place{
	{Name: "Sheshi Nënë Tereza", Address: "Sheshi Nënë Tereza, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6629, Lng: 21.1655}},
	{Name: "Katedralja Nënë Tereza", Address: "Rr. Bill Clinton, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6598, Lng: 21.1595}},
	{Name: "Biblioteka Kombëtare", Address: "Rr. Eqrem Çabej, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6577, Lng: 21.1640}},
	{Name: "Monumenti NEWBORN", Address: "Rr. Luan Haradinaj, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6602, Lng: 21.1602}},
	{Name: "Stadiumi Fadil Vokrri", Address: "Rr. Luan Haradinaj, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6560, Lng: 21.1568}},
	{Name: "Aeroporti i Prishtinës Adem Jashari", Address: "Sllatinë e Madhe", Kind: "poi", Point: geo.Point{Lat: 42.5728, Lng: 21.0358}},
	{Name: "Stacioni i autobusëve", Address: "Rr. Lidhja e Pejës, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6533, Lng: 21.1447}},
	{Name: "Albi Mall", Address: "Magjistralja Prishtinë–Ferizaj", Kind: "poi", Point: geo.Point{Lat: 42.6190, Lng: 21.1620}},
	{Name: "Universiteti i Prishtinës", Address: "Rr. George Bush, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6558, Lng: 21.1670}},
	{Name: "QKUK", Address: "Rr. Spitalit, Prishtinë", Kind: "poi", Point: geo.Point{Lat: 42.6480, Lng: 21.1560}},
	{Name: "Lagjja Dardania", Address: "Dardania, Prishtinë", Kind: "neighborhood", Point: geo.Point{Lat: 42.6470, Lng: 21.1520}},
	{Name: "Lagjja Ulpiana", Address: "Ulpiana, Prishtinë", Kind: "neighborhood", Point: geo.Point{Lat: 42.6510, Lng: 21.1600}},
	{Name: "Lagjja Bregu i Diellit", Address: "Bregu i Diellit, Prishtinë", Kind: "neighborhood", Point: geo.Point{Lat: 42.6540, Lng: 21.1700}},
	{Name: "Fushë Kosovë", Address: "Fushë Kosovë", Kind: "place", Point: geo.Point{Lat: 42.6360, Lng: 21.0970}},
	{Name: "Prizren, qendra", Address: "Shatërvani, Prizren", Kind: "place", Point: geo.Point{Lat: 42.2130, Lng: 20.7410}},
	{Name: "Pejë, qendra", Address: "Pejë", Kind: "place", Point: geo.Point{Lat: 42.6590, Lng: 20.2880}},
}

func (DevEstimate) Directions(_ context.Context, from, to geo.Point) (Route, error) {
	r, _ := DevEstimate{}.Route(context.Background(), from, to)
	// Një kthesë e lehtë në mes, që vija të mos duket si ajrore.
	mid := geo.Point{Lat: (from.Lat+to.Lat)/2 + (to.Lng-from.Lng)*0.08, Lng: (from.Lng+to.Lng)/2 - (to.Lat-from.Lat)*0.08}
	r.Path = []geo.Point{from, mid, to}
	return r, nil
}

func (DevEstimate) Search(_ context.Context, q string, near *geo.Point, limit int) ([]Place, error) {
	needle := strings.ToLower(strings.TrimSpace(q))
	var out []Place
	for _, p := range devPlaces {
		if strings.Contains(strings.ToLower(p.Name), needle) || strings.Contains(strings.ToLower(p.Address), needle) {
			out = append(out, p)
		}
	}
	if near != nil {
		sort.SliceStable(out, func(i, j int) bool {
			return geo.Haversine(*near, out[i].Point) < geo.Haversine(*near, out[j].Point)
		})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (DevEstimate) Reverse(_ context.Context, p geo.Point) (*Place, error) {
	best := devPlaces[0]
	for _, c := range devPlaces[1:] {
		if geo.Haversine(p, c.Point) < geo.Haversine(p, best.Point) {
			best = c
		}
	}
	return &Place{Name: "Afër: " + best.Name, Address: best.Address, Kind: "address", Point: p}, nil
}
