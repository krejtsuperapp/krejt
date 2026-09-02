package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
)

type ctxKey int

const claimsKey ctxKey = 1

// ClaimsFrom kthen pretendimet e verifikuara të kërkesës (vendosen nga RequireAuth).
func ClaimsFrom(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok
}

// RequireAuth — verifikon JWT-në DHE që sesioni s'është shkyçur; kërkon (opsionalisht) një kapacitet.
func (s *Service) RequireAuth(capability string) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				httpx.WriteError(w, r, httpx.ErrUnauthorized)
				return
			}
			claims, err := s.signer.Verify(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrUnauthorized.With(err))
				return
			}
			sid, err := uuid.Parse(claims.SessionID)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrUnauthorized)
				return
			}
			active, err := s.SessionActive(r.Context(), sid)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrInternal.With(err))
				return
			}
			if !active {
				httpx.WriteError(w, r, ErrSessionInvalid)
				return
			}
			if capability != "" && !hasCap(claims.Capabilities, capability) {
				httpx.WriteError(w, r, httpx.ErrForbidden)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			ctx = logx.WithUserID(ctx, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func hasCap(caps []string, want string) bool {
	for _, c := range caps {
		if c == want || c == "SUPER_ADMIN" {
			return true
		}
	}
	return false
}

// --- handlers -----------------------------------------------------------------

type otpRequestBody struct {
	Phone  string `json:"phone"`
	Locale string `json:"locale"`
}

type otpVerifyBody struct {
	Phone  string `json:"phone"`
	Code   string `json:"code"`
	Locale string `json:"locale"`
	Device struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
	} `json:"device"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// Routes regjistron /api/v1/auth/* dhe /api/v1/users/me.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/otp/request", func(w http.ResponseWriter, r *http.Request) {
		var b otpRequestBody
		if err := decode(r, &b); err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.With(err))
			return
		}
		if err := s.RequestOTP(r.Context(), strings.TrimSpace(b.Phone), clientIP(r), b.Locale); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		// e njëjta përgjigje gjithmonë — pa enumerim numrash
		httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"status": "sent", "expires_in": int(otpTTL.Seconds())})
	})

	mux.HandleFunc("POST /api/v1/auth/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		var b otpVerifyBody
		if err := decode(r, &b); err != nil {
			httpx.WriteError(w, r, httpx.ErrValidation.With(err))
			return
		}
		pair, err := s.VerifyOTP(r.Context(), strings.TrimSpace(b.Phone), strings.TrimSpace(b.Code), b.Locale,
			Device{ID: b.Device.ID, Name: b.Device.Name, Platform: b.Device.Platform, IP: clientIP(r)})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, pair)
	})

	mux.HandleFunc("POST /api/v1/auth/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		var b refreshBody
		if err := decode(r, &b); err != nil || b.RefreshToken == "" {
			httpx.WriteError(w, r, httpx.ErrValidation)
			return
		}
		pair, err := s.Refresh(r.Context(), b.RefreshToken)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, pair)
	})

	mux.Handle("POST /api/v1/auth/logout", s.RequireAuth("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFrom(r.Context())
		sid, _ := uuid.Parse(claims.SessionID)
		if err := s.Logout(r.Context(), sid); err != nil {
			httpx.WriteError(w, r, httpx.ErrInternal.With(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))

	mux.Handle("POST /api/v1/auth/logout-all", s.RequireAuth("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFrom(r.Context())
		uid, _ := uuid.Parse(claims.Subject)
		if err := s.RevokeAll(r.Context(), uid); err != nil {
			httpx.WriteError(w, r, httpx.ErrInternal.With(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))

	mux.Handle("GET /api/v1/users/me", s.RequireAuth("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFrom(r.Context())
		uid, _ := uuid.Parse(claims.Subject)
		me, err := s.Me(r.Context(), uid)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, me)
	})))
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}
