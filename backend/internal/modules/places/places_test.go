package places

import (
	"context"
	"errors"
	"testing"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/providers/maps"
)

var prishtina = geo.Point{Lat: 42.6629, Lng: 21.1655}

type fakeMaps struct {
	maps.Provider
	searches int
	routes   int
}

func (f *fakeMaps) Search(_ context.Context, q string, _ *geo.Point, limit int) ([]maps.Place, error) {
	f.searches++
	return []maps.Place{{Name: q, Address: q + ", Prishtinë", Kind: "poi", Point: prishtina}}, nil
}

func (f *fakeMaps) Reverse(_ context.Context, p geo.Point) (*maps.Place, error) {
	return &maps.Place{Name: "Rr. Test 1", Address: "Rr. Test 1, Prishtinë", Kind: "address", Point: p}, nil
}

func (f *fakeMaps) Directions(_ context.Context, from, to geo.Point) (maps.Route, error) {
	f.routes++
	if to.Lat > 42.9 {
		return maps.Route{}, maps.ErrNoRoute
	}
	return maps.Route{DistanceM: 1200, DurationS: 240, Path: []geo.Point{from, to}}, nil
}

func TestSearchValidatesQuery(t *testing.T) {
	s := New(&fakeMaps{}, nil)
	for _, q := range []string{"", "a", "  "} {
		if _, err := s.Search(context.Background(), q, nil, 5); err == nil {
			t.Fatalf("q=%q: pritej gabim validimi", q)
		}
	}
	items, err := s.Search(context.Background(), "Sheshi", &geo.Point{Lat: 0, Lng: 0}, 100)
	if err != nil || len(items) != 1 || items[0].Name != "Sheshi" {
		t.Fatalf("items=%v err=%v", items, err)
	}
}

func TestReverseOutsideKosovo(t *testing.T) {
	s := New(&fakeMaps{}, nil)
	if _, err := s.Reverse(context.Background(), geo.Point{Lat: 48.2, Lng: 16.3}); err == nil {
		t.Fatal("Vjena nuk është në zonë")
	}
	p, err := s.Reverse(context.Background(), prishtina)
	if err != nil || p == nil || p.Point != prishtina {
		t.Fatalf("p=%v err=%v", p, err)
	}
}

func TestRoutePathNoRouteIsValidation(t *testing.T) {
	f := &fakeMaps{}
	s := New(f, nil)
	_, err := s.RoutePath(context.Background(), prishtina, geo.Point{Lat: 43.0, Lng: 21.0})
	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != 422 {
		t.Fatalf("pritej 422, mora %v", err)
	}
	r, err := s.RoutePath(context.Background(), prishtina, geo.Point{Lat: 42.65, Lng: 21.15})
	if err != nil || r.DistanceM != 1200 || len(r.Path) != 2 {
		t.Fatalf("r=%+v err=%v", r, err)
	}
	if _, err := s.RoutePath(context.Background(), geo.Point{}, prishtina); err == nil {
		t.Fatal("pika bosh duhet refuzuar")
	}
}
