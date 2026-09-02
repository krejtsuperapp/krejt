package maps

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"krejt.app/backend/internal/domain/geo"
)

var (
	prishtina = geo.Point{Lat: 42.6629, Lng: 21.1655}
	prizren   = geo.Point{Lat: 42.2139, Lng: 20.7397}
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// mapboxAt ndërton ofruesin kundrejt një serveri testi.
func mapboxAt(url string) *Mapbox {
	m := NewMapbox("test-token")
	m.endpoint = url
	return m
}

func TestMapboxRoute(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"distance":12345.6,"duration":1234.4}]}`))
	}))
	defer srv.Close()

	route, err := mapboxAt(srv.URL).Route(context.Background(), prishtina, prizren)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.DistanceM != 12346 || route.DurationS != 1234 {
		t.Fatalf("distanca/koha e rrumbullakosur gabim: %+v", route)
	}

	// Mapbox i pret koordinatat lng,lat — nëse renditja kthehet, rruga del në vendin e gabuar.
	if !strings.Contains(gotPath, "21.165500,42.662900;20.739700,42.213900") {
		t.Fatalf("renditja e koordinatave: %s", gotPath)
	}
	if !strings.Contains(gotQuery, "access_token=test-token") {
		t.Fatalf("token-i mungon te kërkesa: %s", gotQuery)
	}
	if !strings.Contains(gotQuery, "overview=false") {
		t.Fatalf("gjeometria nuk duhet kërkuar: %s", gotQuery)
	}
}

func TestMapboxNoRoute(t *testing.T) {
	// Mapbox e kthen 200 edhe kur nuk ka rrugë; arsyeja rri te fusha code.
	for _, code := range []string{"NoRoute", "NoSegment"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"code":"` + code + `","routes":[]}`))
		}))
		if _, err := mapboxAt(srv.URL).Route(context.Background(), prishtina, prizren); !errors.Is(err, ErrNoRoute) {
			t.Fatalf("%s: pritej ErrNoRoute, u kthye %v", code, err)
		}
		srv.Close()
	}
}

func TestMapboxErrorsDoNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Not Authorized - Invalid Token"}`))
	}))
	defer srv.Close()

	_, err := mapboxAt(srv.URL).Route(context.Background(), prishtina, prizren)
	if err == nil {
		t.Fatal("pritej gabim për HTTP 401")
	}
	if strings.Contains(err.Error(), "test-token") {
		t.Fatalf("token-i doli te teksti i gabimit: %s", err)
	}
}

func TestMapboxEmptyRoutes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[]}`))
	}))
	defer srv.Close()

	if _, err := mapboxAt(srv.URL).Route(context.Background(), prishtina, prizren); !errors.Is(err, ErrNoRoute) {
		t.Fatalf("pritej ErrNoRoute, u kthye %v", err)
	}
}

func TestNewFromEnvChoosesProvider(t *testing.T) {
	log := discard()

	p, err := NewFromEnv("production", "mapbox", "", "mapbox-token", log)
	if err != nil {
		t.Fatalf("mapbox: %v", err)
	}
	if _, ok := p.(*Mapbox); !ok {
		t.Fatalf("pritej *Mapbox, u kthye %T", p)
	}

	p, err = NewFromEnv("production", "google", "google-key", "", log)
	if err != nil {
		t.Fatalf("google: %v", err)
	}
	if _, ok := p.(*Google); !ok {
		t.Fatalf("pritej *Google, u kthye %T", p)
	}

	// Parazgjedhja mbetet Google, që ndryshimi i ofruesit të jetë gjithmonë i vetëdijshëm.
	p, err = NewFromEnv("production", "", "google-key", "mapbox-token", log)
	if err != nil {
		t.Fatalf("parazgjedhja: %v", err)
	}
	if _, ok := p.(*Google); !ok {
		t.Fatalf("parazgjedhja duhet Google, u kthye %T", p)
	}
}

func TestNewFromEnvRequiresCredentials(t *testing.T) {
	log := discard()

	if _, err := NewFromEnv("production", "mapbox", "google-key", "", log); err == nil {
		t.Fatal("mapbox pa token duhet të refuzohet")
	}
	if _, err := NewFromEnv("production", "google", "", "mapbox-token", log); err == nil {
		t.Fatal("google pa çelës duhet të refuzohet")
	}
	if _, err := NewFromEnv("production", "openstreetmap", "k", "t", log); err == nil {
		t.Fatal("ofruesi i panjohur duhet të refuzohet")
	}
}

func TestDevEstimateOnlyInDevelopment(t *testing.T) {
	log := discard()

	if _, err := NewFromEnv("production", "devestimate", "", "", log); err == nil {
		t.Fatal("devestimate nuk lejohet jashtë development")
	}
	p, err := NewFromEnv("development", "devestimate", "", "", log)
	if err != nil {
		t.Fatalf("development: %v", err)
	}
	route, err := p.Route(context.Background(), prishtina, prizren)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.DistanceM <= 0 || route.DurationS <= 0 {
		t.Fatalf("vlerësimi duhet pozitiv: %+v", route)
	}
}
