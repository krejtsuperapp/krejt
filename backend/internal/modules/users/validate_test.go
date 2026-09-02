package users

import "testing"

func TestInKosovo(t *testing.T) {
	cases := []struct {
		name     string
		lat, lng float64
		want     bool
	}{
		{"Prishtinë", 42.6629, 21.1655, true},
		{"Prizren", 42.2139, 20.7397, true},
		{"Pejë", 42.6600, 20.2883, true},
		{"Mitrovicë", 42.8914, 20.8660, true},
		{"Tiranë (jashtë §1)", 41.3275, 19.8187, false},
		{"Beograd", 44.7866, 20.4489, false},
		{"Berlin (diaspora)", 52.5200, 13.4050, false},
		{"0,0", 0, 0, false},
	}
	for _, c := range cases {
		if got := InKosovo(c.lat, c.lng); got != c.want {
			t.Errorf("%s: InKosovo = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	ok := map[string]string{"  Liridon   Osmani ": "Liridon Osmani", "Ë": "", "Shkëlqim Ç.": "Shkëlqim Ç."}
	for in, want := range ok {
		got, valid := NormalizeName(in)
		if want == "" {
			if valid {
				t.Errorf("%q duhej të refuzohej", in)
			}
			continue
		}
		if !valid || got != want {
			t.Errorf("NormalizeName(%q) = %q,%v; want %q", in, got, valid, want)
		}
	}
	for _, bad := range []string{"", " ", "123", "a\x00b", string(make([]rune, 81))} {
		if _, valid := NormalizeName(bad); valid {
			t.Errorf("%q duhej të refuzohej", bad)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	good := map[string]string{" Doni@Krejt.APP ": "doni@krejt.app", "a.b+c@sub.example.de": "a.b+c@sub.example.de"}
	for in, want := range good {
		got, ok := NormalizeEmail(in)
		if !ok || got != want {
			t.Errorf("NormalizeEmail(%q) = %q,%v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "doni", "doni@localhost", "Doni <doni@krejt.app>", "a b@krejt.app", "@krejt.app"} {
		if _, ok := NormalizeEmail(bad); ok {
			t.Errorf("%q duhej të refuzohej", bad)
		}
	}
}

func TestValidateAddress(t *testing.T) {
	name := "  Te gjyshja "
	in := AddressInput{Label: " Home ", Name: &name, Line1: "  Rr. Agim Ramadani  12 ", City: "Prishtinë", Lat: 42.66, Lng: 21.16}
	if f := validateAddress(&in); len(f) != 0 {
		t.Fatalf("fields = %v", f)
	}
	if in.Label != "home" || in.Line1 != "Rr. Agim Ramadani 12" || *in.Name != "Te gjyshja" {
		t.Fatalf("normalizimi: %+v", in)
	}
	empty := ""
	bad := AddressInput{Label: "villa", Name: &empty, Line1: "x", City: "", Lat: 0, Lng: 0}
	f := validateAddress(&bad)
	for _, k := range []string{"label", "line1", "city", "location"} {
		if f[k] == "" {
			t.Errorf("mungon gabimi për %s: %v", k, f)
		}
	}
	if bad.Name != nil {
		t.Error("name i zbrazët duhej të bëhej nil")
	}
}

func TestValidatePreferences(t *testing.T) {
	if f := validatePreferences(nil); f["body"] != "empty" {
		t.Errorf("bosh: %v", f)
	}
	f := validatePreferences([]Preference{
		{Category: "Security", Push: false},
		{Category: "rides", Push: true},
		{Category: "rides", Push: false},
		{Category: "weather"},
	})
	if f["security.push"] != "required" || f["rides"] != "duplicate" || f["category"] != "invalid" {
		t.Errorf("fields = %v", f)
	}
	if f := validatePreferences([]Preference{{Category: "promotions", Push: false, Email: false, SMS: false}}); len(f) != 0 {
		t.Errorf("promocionet duhet të çaktivizohen lirshëm: %v", f)
	}
}
