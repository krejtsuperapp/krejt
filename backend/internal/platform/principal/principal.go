// Package principal — kush vepron në një kërkesë: id-ja e përdoruesit, sesioni, IP-ja, kapacitetet
// (të gjitha nga JWT-ja e verifikuar — asnjëherë nga trupi i kërkesës).
package principal

import (
	"net/http"

	"github.com/google/uuid"

	"krejt.app/backend/internal/modules/auth"
	"krejt.app/backend/internal/platform/httpx"
)

type Actor struct {
	UserID       uuid.UUID
	SessionID    uuid.UUID
	IP           string
	Capabilities []string
}

func (a Actor) Has(capability string) bool {
	for _, c := range a.Capabilities {
		if c == capability || c == "SUPER_ADMIN" {
			return true
		}
	}
	return false
}

// FromRequest — aktori i kërkesës (kërkon RequireAuth më parë).
func FromRequest(r *http.Request) (Actor, bool) {
	c, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		return Actor{}, false
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return Actor{}, false
	}
	sid, err := uuid.Parse(c.SessionID)
	if err != nil {
		return Actor{}, false
	}
	return Actor{UserID: uid, SessionID: sid, IP: httpx.ClientIP(r), Capabilities: c.Capabilities}, true
}

// Handler — mbështjell një handler që kërkon aktor; pa aktor → UNAUTHORIZED.
func Handler(fn func(w http.ResponseWriter, r *http.Request, a Actor)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := FromRequest(r)
		if !ok {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		fn(w, r, a)
	})
}
