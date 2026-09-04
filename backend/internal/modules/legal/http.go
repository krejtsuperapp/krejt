package legal

import (
	"html"
	"net/http"
	"strings"

	"krejt.app/backend/internal/platform/httpx"
)

// Routes — të dyja publike, pa kyçje: dyqanet e aplikacioneve i hapin pa llogari, dhe ekrani i
// hyrjes i lidh para se përdoruesi të ketë sesion.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /legal/{doc}", s.handlePage)
	mux.HandleFunc("GET /api/v1/legal/{doc}", s.handleJSON)
}

// lang — gjuha nga ?lang=, ndryshe nga Accept-Language, ndryshe shqip.
func lang(r *http.Request) string {
	if v := r.URL.Query().Get("lang"); langs[v] {
		return v
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		code := strings.TrimSpace(strings.SplitN(part, "-", 2)[0])
		if idx := strings.Index(code, ";"); idx >= 0 {
			code = code[:idx]
		}
		if langs[code] {
			return code
		}
	}
	return "sq"
}

func (s *Service) handleJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := s.Load(r.PathValue("doc"), lang(r))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	if !s.op.valid() {
		httpx.WriteError(w, r, errUnavailable)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, doc)
}

func (s *Service) handlePage(w http.ResponseWriter, r *http.Request) {
	doc, err := s.Load(r.PathValue("doc"), lang(r))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !s.op.valid() {
		http.Error(w, "Dokumenti nuk është i disponueshëm.", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Një orë: teksti ndryshon rrallë, por një ndreqje nuk duhet të presë një ditë.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(render(doc)))
}

// render — faqe e vetme, pa skedarë të jashtëm dhe pa JavaScript: hapet edhe në një shfletues të
// vjetër e në një lidhje të dobët, që është pikërisht ku njerëzit i lexojnë këto dokumente.
func render(d Document) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="` + html.EscapeString(d.Lang) + `"><head>`)
	b.WriteString(`<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">`)
	b.WriteString(`<title>` + html.EscapeString(d.Title) + ` — KREJT</title><style>` + css + `</style>`)
	b.WriteString(`</head><body><main><h1>` + html.EscapeString(d.Title) + `</h1>`)
	b.WriteString(`<p class="updated">` + html.EscapeString(formatDate(d.Updated)) + `</p>`)
	for _, sec := range d.Sections {
		b.WriteString(`<h2>` + html.EscapeString(sec.Heading) + `</h2>`)
		for _, p := range sec.Paragraphs {
			cls := ""
			if strings.HasPrefix(p, "• ") {
				cls = ` class="item"`
			}
			b.WriteString(`<p` + cls + `>` + html.EscapeString(p) + `</p>`)
		}
	}
	b.WriteString(`</main></body></html>`)
	return b.String()
}

const css = `:root{color-scheme:light dark}
*{box-sizing:border-box}
body{margin:0;background:#0b0d10;color:#e6e9ee;font:16px/1.65 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:44rem;margin:0 auto;padding:2.5rem 1.25rem 4rem}
h1{font-size:1.75rem;line-height:1.25;margin:0 0 .25rem;text-wrap:balance}
h2{font-size:1.075rem;margin:2rem 0 .5rem;color:#fff}
p{margin:0 0 .85rem;color:#c3cad6}
p.item{padding-left:1rem}
.updated{color:#8b95a5;font-size:.85rem;margin-bottom:2rem}
@media (prefers-color-scheme: light){
  body{background:#fff;color:#14181f}
  h2{color:#000}
  p{color:#39414f}
  .updated{color:#6b7280}
}`

var errUnavailable = &httpx.APIError{
	Code:       "LEGAL_UNAVAILABLE",
	MessageKey: "errors.legal.unavailable",
	HTTPStatus: http.StatusServiceUnavailable,
}
