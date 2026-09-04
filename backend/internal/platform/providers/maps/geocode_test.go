package maps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"krejt.app/backend/internal/domain/geo"
)

func TestMapboxSearchParsesAndFiltersToKosovo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("country") != "xk" || r.URL.Query().Get("proximity") == "" {
			t.Errorf("parametrat: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"features":[
		  {"properties":{"name":"Sheshi Nënë Tereza","full_address":"Sheshi Nënë Tereza, Prishtinë","feature_type":"poi","coordinates":{"longitude":21.1655,"latitude":42.6629}}},
		  {"properties":{"name":"Beograd","place_formatted":"Serbia","feature_type":"place","coordinates":{"longitude":20.4573,"latitude":44.7872}}}
		]}`))
	}))
	defer srv.Close()
	m := NewMapbox("pk.test")
	m.geocode = srv.URL
	m.http = srv.Client()
	near := geo.Point{Lat: 42.66, Lng: 21.16}
	out, err := m.Search(context.Background(), "sheshi", &near, 5)
	if err != nil || len(out) != 1 || out[0].Name != "Sheshi Nënë Tereza" || out[0].Kind != "poi" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestMapboxDirectionsWithGeometry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("geometries") != "geojson" || r.URL.Query().Get("overview") != "full" {
			t.Errorf("parametrat: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"distance":1500.4,"duration":300.2,"geometry":{"coordinates":[[21.16,42.66],[21.17,42.67]]}}]}`))
	}))
	defer srv.Close()
	m := NewMapbox("pk.test")
	m.endpoint = srv.URL
	m.http = srv.Client()
	r, err := m.Directions(context.Background(), geo.Point{Lat: 42.66, Lng: 21.16}, geo.Point{Lat: 42.67, Lng: 21.17})
	if err != nil || r.DistanceM != 1500 || r.DurationS != 300 || len(r.Path) != 2 || r.Path[1].Lat != 42.67 {
		t.Fatalf("r=%+v err=%v", r, err)
	}
}

func TestDecodePolyline(t *testing.T) {
	pts := decodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	if len(pts) != 3 || pts[0].Lat != 38.5 || pts[0].Lng != -120.2 {
		t.Fatalf("pts=%v", pts)
	}
}

func TestDevEstimateSearchAndReverse(t *testing.T) {
	d := DevEstimate{}
	near := geo.Point{Lat: 42.66, Lng: 21.16}
	out, err := d.Search(context.Background(), "lagjja", &near, 2)
	if err != nil || len(out) != 2 {
		t.Fatalf("out=%v err=%v", out, err)
	}
	p, err := d.Reverse(context.Background(), near)
	if err != nil || p == nil || p.Point != near {
		t.Fatalf("p=%v err=%v", p, err)
	}
	r, _ := d.Directions(context.Background(), near, geo.Point{Lat: 42.65, Lng: 21.15})
	if len(r.Path) != 3 {
		t.Fatalf("path=%v", r.Path)
	}
}
