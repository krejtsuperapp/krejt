// Package legal — Kushtet e përdorimit dhe Politika e privatësisë, në sq/en/de.
//
// Dy arsye pse jetojnë te serveri e jo brenda aplikacionit: dyqanet (Play, App Store) kërkojnë një
// adresë publike që hapet pa aplikacion, dhe një ndryshim i tekstit nuk duhet të presë një version
// të ri të aplikacionit. Teksti është një burim i vetëm për të dyja: faqja HTML dhe ekrani brenda
// aplikacionit lexojnë të njëjtin dokument.
//
// Identiteti i operatorit (emri ligjor, adresa, email-i) vjen nga konfigurimi, kurrë nga kodi: një
// politikë privatësie me emër të shpikur është më keq se asnjë politikë. Në prodhim serveri nuk niset
// pa to (shih config.validate).
package legal

import (
	"embed"
	"fmt"
	"strings"
	"time"
)

//go:embed docs/*.md
var docs embed.FS

// Updated — data e versionit aktual të dokumenteve. Ndryshohet me dorë kur teksti ndryshon;
// përdoruesi ka të drejtë ta dijë se cilën redaktim po lexon.
const Updated = "2026-09-04"

// Operator — kush e ofron shërbimin. Pa këto tri fusha dokumentet nuk kanë kuptim ligjor.
type Operator struct {
	Entity  string // emri i regjistruar, p.sh. "KREJT SH.P.K."
	Address string // adresa e regjistruar
	Email   string // adresa e kontaktit për çështje ligjore dhe privatësie
}

func (o Operator) valid() bool {
	return o.Entity != "" && o.Address != "" && o.Email != ""
}

type Service struct {
	op Operator
}

func New(op Operator) *Service { return &Service{op: op} }

// Section — një krye i dokumentit. Aplikacioni i vizaton vetë me stilin e vet, ndaj merr strukturë
// dhe jo HTML: një ekran i KREJT-it nuk duhet të duket si faqe interneti.
type Section struct {
	Heading    string   `json:"heading"`
	Paragraphs []string `json:"paragraphs"`
}

type Document struct {
	Doc      string    `json:"doc"`
	Lang     string    `json:"lang"`
	Title    string    `json:"title"`
	Updated  string    `json:"updated"`
	Sections []Section `json:"sections"`
}

var (
	kinds = map[string]bool{"terms": true, "privacy": true}
	langs = map[string]bool{"sq": true, "en": true, "de": true}
)

// Load — dokumenti i kërkuar. Gjuha e panjohur bie te shqipja, sepse shërbimi është vetëm për
// Kosovën dhe një faqe bosh do të ishte më e keqe se një faqe në gjuhën e vendit.
func (s *Service) Load(doc, lang string) (Document, error) {
	if !kinds[doc] {
		return Document{}, fmt.Errorf("legal: dokument i panjohur %q", doc)
	}
	if !langs[lang] {
		lang = "sq"
	}
	raw, err := docs.ReadFile("docs/" + doc + "." + lang + ".md")
	if err != nil {
		return Document{}, fmt.Errorf("legal: %s.%s: %w", doc, lang, err)
	}
	out := parse(string(raw))
	out.Doc, out.Lang, out.Updated = doc, lang, Updated
	return s.fill(out), nil
}

// fill — zëvendëson vendmbajtësit me identitetin e operatorit dhe datën e redaktimit.
func (s *Service) fill(d Document) Document {
	r := strings.NewReplacer(
		"{{entity}}", s.op.Entity,
		"{{address}}", s.op.Address,
		"{{email}}", s.op.Email,
		"{{updated}}", formatDate(Updated),
	)
	d.Title = r.Replace(d.Title)
	for i, sec := range d.Sections {
		d.Sections[i].Heading = r.Replace(sec.Heading)
		for j, p := range sec.Paragraphs {
			d.Sections[i].Paragraphs[j] = r.Replace(p)
		}
	}
	return d
}

// parse — Markdown i kufizuar me qëllim: `# titull`, `## krye`, paragrafë dhe rreshta me `- `.
// Asgjë më shumë nuk i duhet një dokumenti ligjor, dhe asgjë më shumë nuk duhet interpretuar.
func parse(src string) Document {
	var d Document
	var cur *Section
	var buf []string

	flush := func() {
		if len(buf) > 0 && cur != nil {
			cur.Paragraphs = append(cur.Paragraphs, strings.Join(buf, " "))
		}
		buf = nil
	}
	closeSection := func() {
		flush()
		if cur != nil {
			d.Sections = append(d.Sections, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimRight(line, "\r \t")
		switch {
		case strings.HasPrefix(line, "## "):
			closeSection()
			cur = &Section{Heading: strings.TrimSpace(line[3:])}
		case strings.HasPrefix(line, "# "):
			closeSection()
			d.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "- "):
			flush()
			if cur != nil {
				cur.Paragraphs = append(cur.Paragraphs, "• "+strings.TrimSpace(line[2:]))
			}
		case line == "":
			flush()
		default:
			buf = append(buf, strings.TrimSpace(line))
		}
	}
	closeSection()
	return d
}

// formatDate — 2026-09-04 → 4.9.2026, si i shkruhen datat në Kosovë.
func formatDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d.%d.%d", t.Day(), t.Month(), t.Year())
}
