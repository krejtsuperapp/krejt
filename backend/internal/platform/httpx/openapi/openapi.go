// Package openapi — specifikimi OpenAPI i API-së (§39/§74: api-client i gjeneruar prej tij), i embed-uar
// dhe i shërbyer te GET /api/v1/openapi.yaml. Testi i paketës siguron që çdo rrugë e regjistruar në kod
// të jetë e dokumentuar (asnjë endpoint pa dokument, asnjë dokument pa endpoint).
package openapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var Spec []byte

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(Spec)
	})
}
