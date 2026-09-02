package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/logx"
)

// OptionalAuth — si RequireAuth, por kërkesa pa token (ose me token të pavlefshëm) vazhdon si anonime.
// Për endpoint-e publike që personalizohen kur përdoruesi është i kyçur (p.sh. GET /config).
func (s *Service) OptionalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := s.signer.Verify(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if sid, err := uuid.Parse(claims.SessionID); err == nil {
				if active, err := s.SessionActive(r.Context(), sid); err == nil && active {
					ctx := context.WithValue(r.Context(), claimsKey, claims)
					ctx = logx.WithUserID(ctx, claims.Subject)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
