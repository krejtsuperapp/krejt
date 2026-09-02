package notifications

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{[a-z_]+\}`)

func TestTextsCompleteAndConsistent(t *testing.T) {
	for key, langs := range texts {
		for _, l := range []string{"sq", "en", "de"} {
			if _, ok := langs[l]; !ok {
				t.Errorf("%s: mungon gjuha %s", key, l)
			}
			if langs[l][0] == "" || langs[l][1] == "" {
				t.Errorf("%s/%s: titull ose trup bosh", key, l)
			}
		}
		ph := func(l string) []string {
			m := placeholder.FindAllString(langs[l][0]+" "+langs[l][1], -1)
			sort.Strings(m)
			return m
		}
		sq := strings.Join(ph("sq"), ",")
		for _, l := range []string{"en", "de"} {
			if got := strings.Join(ph(l), ","); got != sq {
				t.Errorf("%s: parametrat ndryshojnë sq=[%s] %s=[%s]", key, sq, l, got)
			}
		}
	}
}

func TestRenderAndMoney(t *testing.T) {
	title, body, ok := Render("notif.ride.completed", "de", map[string]string{"price": "12,40 €"})
	if !ok || title != "Fahrt beendet" || !strings.Contains(body, "12,40 €") {
		t.Fatalf("de: %q %q %v", title, body, ok)
	}
	if _, _, ok := Render("notif.nuk.ekziston", "sq", nil); ok {
		t.Fatal("çelës i panjohur duhej të kthente ok=false")
	}
	// gjuhë e panjohur → shqip
	if title, _, _ := Render("notif.ride.arrived", "sr", nil); title != "Shoferi mbërriti" {
		t.Fatalf("fallback: %q", title)
	}
	cases := map[string]string{
		FormatMoney(1240, "EUR", "sq"): "12,40 €",
		FormatMoney(1240, "EUR", "de"): "12,40 €",
		FormatMoney(1240, "EUR", "en"): "€12.40",
		FormatMoney(5, "EUR", "sq"):    "0,05 €",
		FormatMoney(-65, "EUR", "en"):  "-€0.65",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("FormatMoney = %q, want %q", got, want)
		}
	}
}
