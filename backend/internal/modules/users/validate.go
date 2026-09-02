package users

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kufijtë e Kosovës (kuti kufizuese, V1). §1: asnjë adresë jashtë Kosovës.
// Poligoni i saktë i kufirit vjen me modulin `maps` (zonat e shërbimit, H3).
const (
	KosovoMinLat = 41.85
	KosovoMaxLat = 43.30
	KosovoMinLng = 19.95
	KosovoMaxLng = 21.80
)

func InKosovo(lat, lng float64) bool {
	return lat >= KosovoMinLat && lat <= KosovoMaxLat && lng >= KosovoMinLng && lng <= KosovoMaxLng
}

// NormalizeName — hapësira të bashkuara, 2–80 shkronja, pa karaktere kontrolli, të paktën një shkronjë.
func NormalizeName(s string) (string, bool) {
	s = strings.Join(strings.Fields(s), " ")
	n := utf8.RuneCountInString(s)
	if n < 2 || n > 80 {
		return "", false
	}
	hasLetter := false
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", false
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return s, hasLetter
}

// NormalizeEmail — shkronja të vogla, adresë e vetme pa emër shfaqjeje, domain me pikë.
func NormalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || len(s) > 254 || strings.ContainsAny(s, " <>\"(),;:\\") {
		return "", false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return "", false
	}
	at := strings.LastIndex(s, "@")
	if at < 1 || !strings.Contains(s[at+1:], ".") {
		return "", false
	}
	return s, true
}

func ValidLocale(s string) bool { return s == "sq" || s == "en" || s == "de" }

// trimOpt — fushë opsionale: hapësirat hiqen, e zbrazëta bëhet nil, gjatësia kufizohet.
func trimOpt(p *string, max int, name string, fields map[string]string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	if utf8.RuneCountInString(v) > max {
		fields[name] = "too_long"
	}
	return &v
}

func trimReq(v string, min, max int, name string, fields map[string]string) string {
	v = strings.Join(strings.Fields(v), " ")
	n := utf8.RuneCountInString(v)
	switch {
	case n < min:
		fields[name] = "required"
	case n > max:
		fields[name] = "too_long"
	}
	return v
}

// validateAddress normalizon dhe validon në vend; kthen gabimet për fushë (bosh = OK).
func validateAddress(in *AddressInput) map[string]string {
	fields := map[string]string{}
	in.Label = strings.ToLower(strings.TrimSpace(in.Label))
	if in.Label != "home" && in.Label != "work" && in.Label != "other" {
		fields["label"] = "invalid"
	}
	in.Name = trimOpt(in.Name, 40, "name", fields)
	in.Line1 = trimReq(in.Line1, 3, 120, "line1", fields)
	in.Line2 = trimOpt(in.Line2, 120, "line2", fields)
	in.City = trimReq(in.City, 2, 60, "city", fields)
	in.PostalCode = trimOpt(in.PostalCode, 10, "postal_code", fields)
	in.PlaceID = trimOpt(in.PlaceID, 300, "place_id", fields)
	in.Instructions = trimOpt(in.Instructions, 200, "instructions", fields)
	if in.Lat < -90 || in.Lat > 90 || in.Lng < -180 || in.Lng > 180 || (in.Lat == 0 && in.Lng == 0) {
		fields["location"] = "invalid"
	}
	return fields
}

// validatePreferences — kategori të njohura, pa dyfishim, siguria gjithmonë me push (§51).
func validatePreferences(in []Preference) map[string]string {
	fields := map[string]string{}
	if len(in) == 0 {
		fields["body"] = "empty"
		return fields
	}
	seen := map[string]bool{}
	for i := range in {
		in[i].Category = strings.ToLower(strings.TrimSpace(in[i].Category))
		c := in[i].Category
		if !knownCategory(c) {
			fields["category"] = "invalid"
			continue
		}
		if seen[c] {
			fields[c] = "duplicate"
		}
		seen[c] = true
		if c == "security" && !in[i].Push {
			fields["security.push"] = "required"
		}
	}
	return fields
}

func knownCategory(c string) bool {
	for _, k := range Categories {
		if k == c {
			return true
		}
	}
	return false
}
