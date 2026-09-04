// Package maps — MapProvider (§46): rrugë, distancë, kohëzgjatje. Google Routes dhe Mapbox Directions
// janë dy zbatime të njëjtëvlefshme; logjika e biznesit sheh vetëm ndërfaqen Provider, ndaj ofruesi
// ndërrohet me MAPS_PROVIDER pa prekur asnjë modul.
package maps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"krejt.app/backend/internal/domain/geo"
)

type Route struct {
	DistanceM int `json:"distance_m"`
	DurationS int `json:"duration_s"`
	// Path — gjeometria e rrugës (lat/lng me radhë), vetëm kur kërkohet me Directions; Route e lë bosh.
	Path []geo.Point `json:"path,omitempty"`
}

type Provider interface {
	// Route — rruga me makinë nga from te to (distancë + kohë me trafik ku ofruesi e mbështet).
	Route(ctx context.Context, from, to geo.Point) (Route, error)
	// Directions — si Route, por me gjeometrinë e plotë (për vizatim në hartë).
	Directions(ctx context.Context, from, to geo.Point) (Route, error)
	// Search — vende/adresa sipas tekstit, afër një pike (kur jepet); vetëm brenda Kosovës.
	Search(ctx context.Context, q string, near *geo.Point, limit int) ([]Place, error)
	// Reverse — adresa më e afërt e një pike (për pikën e marrjes nga GPS-i).
	Reverse(ctx context.Context, p geo.Point) (*Place, error)
}

// Place — një vend ose adresë e gjetur nga ofruesi.
type Place struct {
	Name    string    `json:"name"`
	Address string    `json:"address"`
	Kind    string    `json:"kind"` // address | street | poi | place | locality
	Point   geo.Point `json:"point"`
}

var ErrNoRoute = errors.New("maps: no route")

// --- Google Routes API ---------------------------------------------------------

type Google struct {
	key      string
	endpoint string
	http     *http.Client
}

func NewGoogle(key string) *Google {
	return &Google{
		key:      key,
		endpoint: "https://routes.googleapis.com/directions/v2:computeRoutes",
		http:     &http.Client{Timeout: 6 * time.Second},
	}
}

type latLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (g *Google) Route(ctx context.Context, from, to geo.Point) (Route, error) {
	return g.route(ctx, from, to, false)
}

func (g *Google) route(ctx context.Context, from, to geo.Point, geometry bool) (Route, error) {
	body := map[string]any{
		"origin":            map[string]any{"location": map[string]any{"latLng": latLng{from.Lat, from.Lng}}},
		"destination":       map[string]any{"location": map[string]any{"latLng": latLng{to.Lat, to.Lng}}},
		"travelMode":        "DRIVE",
		"routingPreference": "TRAFFIC_AWARE",
		"units":             "METRIC",
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(raw))
	if err != nil {
		return Route{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", g.key)
	mask := "routes.duration,routes.distanceMeters"
	if geometry {
		mask += ",routes.polyline.encodedPolyline"
	}
	req.Header.Set("X-Goog-FieldMask", mask)
	resp, err := g.http.Do(req)
	if err != nil {
		return Route{}, fmt.Errorf("maps: google routes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// trupi i përgjigjes nuk logohet (mund të përmbajë detaje llogarie); statusi mjafton
		return Route{}, fmt.Errorf("maps: google routes: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Routes []struct {
			DistanceMeters int    `json:"distanceMeters"`
			Duration       string `json:"duration"` // "1234s"
			Polyline       struct {
				Encoded string `json:"encodedPolyline"`
			} `json:"polyline"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Route{}, fmt.Errorf("maps: google routes: decode: %w", err)
	}
	if len(out.Routes) == 0 {
		return Route{}, ErrNoRoute
	}
	secs, err := strconv.ParseFloat(strings.TrimSuffix(out.Routes[0].Duration, "s"), 64)
	if err != nil {
		return Route{}, fmt.Errorf("maps: google routes: duration %q", out.Routes[0].Duration)
	}
	r := Route{DistanceM: out.Routes[0].DistanceMeters, DurationS: int(math.Round(secs))}
	if geometry {
		r.Path = decodePolyline(out.Routes[0].Polyline.Encoded)
	}
	return r, nil
}

// --- Mapbox Directions API -----------------------------------------------------

type Mapbox struct {
	token    string
	endpoint string
	geocode  string // override në teste; bosh = mapboxGeocode
	http     *http.Client
}

// NewMapbox — profili `driving-traffic` përdor trafikun e çastit, njësoj si TRAFFIC_AWARE te Google.
func NewMapbox(token string) *Mapbox {
	return &Mapbox{
		token:    token,
		endpoint: "https://api.mapbox.com/directions/v5/mapbox/driving-traffic",
		http:     &http.Client{Timeout: 6 * time.Second},
	}
}

func (m *Mapbox) Route(ctx context.Context, from, to geo.Point) (Route, error) {
	return m.route(ctx, from, to, false)
}

// --- DevEstimate (VETËM development) -------------------------------------------

// DevEstimate — vlerësim pa ofrues: vijë ajrore × 1.3 (rrugë urbane) me 25 km/h. Refuzohet jashtë dev.
type DevEstimate struct{}

func (DevEstimate) Route(_ context.Context, from, to geo.Point) (Route, error) {
	d := geo.Haversine(from, to) * 1.3
	return Route{DistanceM: int(math.Round(d)), DurationS: int(math.Round(d / (25000.0 / 3600.0)))}, nil
}

// NewFromEnv — MAPS_PROVIDER: google (parazgjedhje) | mapbox | devestimate (vetëm development).
func NewFromEnv(env, provider, googleKey, mapboxToken string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "google", "":
		if googleKey == "" {
			return nil, errors.New("maps: GOOGLE_MAPS_KEY mungon")
		}
		return NewGoogle(googleKey), nil
	case "mapbox":
		if mapboxToken == "" {
			return nil, errors.New("maps: MAPBOX_TOKEN mungon")
		}
		return NewMapbox(mapboxToken), nil
	case "devestimate":
		if env != "development" {
			return nil, fmt.Errorf("maps: devestimate lejohet vetëm në development (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — MAPS_PROVIDER=devestimate: distanca/koha vlerësohen, jo nga Google Routes")
		return DevEstimate{}, nil
	default:
		return nil, fmt.Errorf("maps: ofrues i panjohur %q", provider)
	}
}
