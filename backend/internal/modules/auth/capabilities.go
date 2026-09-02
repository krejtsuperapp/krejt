package auth

import (
	"net/http"

	"krejt.app/backend/internal/platform/httpx"
)

// RequireAnyCapability — si RequireAuth, por mjafton njëri nga kapacitetet
// (p.sh. RIDE_DRIVER ose TAXI_DRIVER për endpoint-et e shoferit). SUPER_ADMIN kalon gjithmonë.
func (s *Service) RequireAnyCapability(caps ...string) httpx.Middleware {
	base := s.RequireAuth("")
	return func(next http.Handler) http.Handler {
		return base(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := ClaimsFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, httpx.ErrUnauthorized)
				return
			}
			for _, want := range caps {
				if hasCap(c.Capabilities, want) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, r, httpx.ErrForbidden)
		}))
	}
}
