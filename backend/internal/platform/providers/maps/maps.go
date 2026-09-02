// Package maps — MapProvider (§46): rrugë, distancë, kohëzgjatje. Google Routes API është zbatimi
// i parë; logjika e biznesit sheh vetëm ndërfaqen Provider, që ofruesi të ndërrohet pa prekur modulet.
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
}

type Provider interface {
	// Route — rruga me makinë nga from te to (distancë + kohë me trafik ku ofruesi e mbështet).
	Route(ctx context.Context, from, to geo.Point) (Route, error)
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
	req.Header.Set("X-Goog-FieldMask", "routes.duration,routes.distanceMeters")
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
	return Route{DistanceM: out.Routes[0].DistanceMeters, DurationS: int(math.Round(secs))}, nil
}

// --- DevEstimate (VETËM development) -------------------------------------------

// DevEstimate — vlerësim pa ofrues: vijë ajrore × 1.3 (rrugë urbane) me 25 km/h. Refuzohet jashtë dev.
type DevEstimate struct{}

func (DevEstimate) Route(_ context.Context, from, to geo.Point) (Route, error) {
	d := geo.Haversine(from, to) * 1.3
	return Route{DistanceM: int(math.Round(d)), DurationS: int(math.Round(d / (25000.0 / 3600.0)))}, nil
}

// NewFromEnv — MAPS_PROVIDER: google (parazgjedhje) | devestimate (vetëm development).
func NewFromEnv(env, provider, googleKey string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "google", "":
		if googleKey == "" {
			return nil, errors.New("maps: GOOGLE_MAPS_KEY mungon")
		}
		return NewGoogle(googleKey), nil
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
