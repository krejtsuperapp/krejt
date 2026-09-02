package openapi

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var routeRe = regexp.MustCompile(`"(GET|POST|PUT|PATCH|DELETE) (/[^"]+)"`)

// TestSpecCoversAllRoutes — çdo rrugë e regjistruar në kod (mux.Handle*("METHOD /path")) duhet të jetë
// në openapi.yaml dhe anasjelltas. Rrugët dev (vetëm development) dhe webhook-u me ofrues dinamik trajtohen veçmas.
func TestSpecCoversAllRoutes(t *testing.T) {
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(Spec, &doc); err != nil {
		t.Fatalf("openapi.yaml nuk lexohet: %v", err)
	}
	documented := map[string]bool{}
	for p, ops := range doc.Paths {
		for m := range ops {
			switch m {
			case "get", "post", "put", "patch", "delete":
				documented[strings.ToUpper(m)+" "+p] = true
			}
		}
	}

	root := filepath.Join("..", "..", "..", "..") // backend/
	registered := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range routeRe.FindAllStringSubmatch(string(b), -1) {
			route := m[1] + " " + m[2]
			if strings.Contains(route, "/api/v1/dev/") {
				continue
			}
			registered[route] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// webhook-u regjistrohet me emrin e ofruesit në runtime
	delete(registered, "POST /api/v1/payments/webhook/")
	registered["POST /api/v1/payments/webhook/{provider}"] = true
	registered["GET /api/v1/openapi.yaml"] = true

	var missing, extra []string
	for r := range registered {
		if !documented[r] {
			missing = append(missing, r)
		}
	}
	for d := range documented {
		if !registered[d] {
			extra = append(extra, d)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("rrugë pa dokument në openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("dokumente pa rrugë në kod:\n  %s", strings.Join(extra, "\n  "))
	}
	if len(registered) < 80 {
		t.Errorf("u gjetën vetëm %d rrugë — skanimi i kodit duket i thyer", len(registered))
	}
}
