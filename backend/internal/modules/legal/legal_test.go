package legal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func svc() *Service {
	return New(Operator{Entity: "KREJT SH.P.K.", Address: "Rr. Test 1, Prishtinë", Email: "ligj@krejt.app"})
}

// Çdo dokument, në çdo gjuhë, duhet të ketë titull, krye dhe tekst. Një skedar i harruar ose një
// përkthim gjysmak do të shfaqej si faqe bosh te dyqani i aplikacioneve.
func TestAllDocumentsLoad(t *testing.T) {
	s := svc()
	for _, doc := range []string{"terms", "privacy"} {
		for _, lang := range []string{"sq", "en", "de"} {
			d, err := s.Load(doc, lang)
			if err != nil {
				t.Fatalf("%s.%s: %v", doc, lang, err)
			}
			if d.Title == "" || len(d.Sections) < 5 {
				t.Fatalf("%s.%s: titull %q, %d krye", doc, lang, d.Title, len(d.Sections))
			}
			for _, sec := range d.Sections {
				if sec.Heading == "" || len(sec.Paragraphs) == 0 {
					t.Errorf("%s.%s: krye bosh: %+v", doc, lang, sec)
				}
			}
		}
	}
}

// Vendmbajtësit duhet të zhduken të gjithë: një dokument që publikon {{entity}} është më keq se
// asnjë dokument.
func TestPlaceholdersAreFilled(t *testing.T) {
	s := svc()
	for _, doc := range []string{"terms", "privacy"} {
		for _, lang := range []string{"sq", "en", "de"} {
			d, _ := s.Load(doc, lang)
			var all strings.Builder
			all.WriteString(d.Title)
			for _, sec := range d.Sections {
				all.WriteString(sec.Heading)
				all.WriteString(strings.Join(sec.Paragraphs, " "))
			}
			if strings.Contains(all.String(), "{{") {
				t.Errorf("%s.%s ka mbetur me vendmbajtës", doc, lang)
			}
			if !strings.Contains(all.String(), "KREJT SH.P.K.") {
				t.Errorf("%s.%s nuk e përmend operatorin", doc, lang)
			}
		}
	}
}

func TestUnknownDocAndLanguage(t *testing.T) {
	s := svc()
	if _, err := s.Load("cookies", "sq"); err == nil {
		t.Fatal("dokument i panjohur duhet të japë gabim")
	}
	// Gjuha e panjohur bie te shqipja, jo te një faqe bosh.
	d, err := s.Load("terms", "fr")
	if err != nil || d.Lang != "sq" {
		t.Fatalf("d=%+v err=%v", d, err)
	}
}

func TestPageAndJSON(t *testing.T) {
	mux := http.NewServeMux()
	svc().Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/legal/privacy?lang=de", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Datenschutzerkl") {
		t.Fatalf("faqja: %d %s", rec.Code, rec.Body.String()[:120])
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type: %s", ct)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/legal/terms", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"sections"`) {
		t.Fatalf("json: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/legal/cookies", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("dokument i panjohur: %d", rec.Code)
	}
}

// Gjuha nga Accept-Language kur ?lang= mungon: shfletuesi i një lexuesi gjerman nuk duhet të marrë
// shqip vetëm sepse linku nuk e mban gjuhën.
func TestLanguageFromHeader(t *testing.T) {
	mux := http.NewServeMux()
	svc().Routes(mux)
	req := httptest.NewRequest(http.MethodGet, "/legal/terms", nil)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Nutzungsbedingungen") {
		t.Fatalf("gjuha nga koka nuk u zbatua")
	}
}

// Pa identitetin e operatorit dokumenti nuk shërbehet fare.
func TestRefusesWithoutOperator(t *testing.T) {
	mux := http.NewServeMux()
	New(Operator{}).Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/legal/privacy", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("pa operator: %d", rec.Code)
	}
}

func TestParseStructure(t *testing.T) {
	d := parse("# Titulli\n\n## Krye\nRreshti i parë\ni dyti.\n\n- pika një\n- pika dy\n")
	if d.Title != "Titulli" || len(d.Sections) != 1 {
		t.Fatalf("d=%+v", d)
	}
	got := d.Sections[0].Paragraphs
	want := []string{"Rreshti i parë i dyti.", "• pika një", "• pika dy"}
	if len(got) != len(want) {
		t.Fatalf("paragrafët: %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paragrafi %d: %q != %q", i, got[i], want[i])
		}
	}
}
