// Package auth — identiteti dhe autentikimi (§37, §53): një llogari, shumë kapacitete;
// OTP → sesion me refresh token të rrotullueshëm → access token JWT. Serveri është autoritar.
package auth

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/providers/sms"
)

type Service struct {
	pool   *pgxpool.Pool
	rdb    redis.UniversalClient
	sms    sms.Provider
	signer *Signer
	ledger *ledger.Service
	pepper []byte

	// Numra prove me kod fiks (vetëm development). Bosh në çdo mjedis tjetër.
	testPhones map[string]struct{}
	testAdmins map[string]struct{}
	testOTP    string
}

// WithDevTestPhones aktivizon kyçjen me kod fiks për numrat e dhënë. Konfigurimi e ka
// refuzuar tashmë këtë jashtë development-it; këtu vetëm regjistrohet lista.
func (s *Service) WithDevTestPhones(phones, admins []string, code string) *Service {
	if len(phones) == 0 || code == "" {
		return s
	}
	s.testPhones = make(map[string]struct{}, len(phones))
	for _, p := range phones {
		s.testPhones[p] = struct{}{}
	}
	s.testAdmins = make(map[string]struct{}, len(admins))
	for _, p := range admins {
		s.testAdmins[p] = struct{}{}
	}
	s.testOTP = code
	return s
}

// isTestAdmin thotë nëse numri i provës merr SUPER_ADMIN në kyçje.
func (s *Service) isTestAdmin(phone string) bool {
	_, ok := s.testAdmins[phone]
	return ok
}

// isTestPhone thotë nëse numri kyçet me kodin fiks të provës.
func (s *Service) isTestPhone(phone string) bool {
	_, ok := s.testPhones[phone]
	return ok
}

func New(pool *pgxpool.Pool, rdb redis.UniversalClient, smsp sms.Provider, signer *Signer, led *ledger.Service, pepper []byte) *Service {
	return &Service{pool: pool, rdb: rdb, sms: smsp, signer: signer, ledger: led, pepper: pepper}
}

type Device struct {
	ID       string
	Name     string
	Platform string // ios | android | web
	IP       string
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       uuid.UUID `json:"user_id"`
	IsNewUser    bool      `json:"is_new_user"`
}

// VerifyOTP: kodi i saktë → përdoruesi ekzistues ose i ri (CUSTOMER + wallet i mbyllur) → sesion + tokena.
func (s *Service) VerifyOTP(ctx context.Context, phone, code, locale string, dev Device) (*TokenPair, error) {
	if !ValidPhone(phone) {
		return nil, ErrPhoneInvalid
	}
	if dev.ID == "" || dev.Platform == "" {
		return nil, httpx.ErrValidation
	}
	var out *TokenPair
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.verifyChallenge(ctx, tx, phone, code); err != nil {
			return err
		}
		// përdoruesi
		var userID uuid.UUID
		var status string
		isNew := false
		err := tx.QueryRow(ctx, `SELECT id, status FROM users WHERE phone_e164 = $1`, phone).Scan(&userID, &status)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if locale == "" {
				locale = "sq"
			}
			if err := tx.QueryRow(ctx, `INSERT INTO users (phone_e164, locale) VALUES ($1, $2) RETURNING id`, phone, locale).Scan(&userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_capabilities (user_id, capability) VALUES ($1, 'CUSTOMER')`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO ledger_accounts (code, owner_type, owner_id, kind, currency)
				VALUES ($1, 'user', $2, 'liability', 'EUR') ON CONFLICT (code) DO NOTHING`, "user:"+userID.String()+":wallet", userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
				VALUES ('user', $1, 'UserCreated', jsonb_build_object('user_id', $1::text, 'locale', $2::text))`, userID, locale); err != nil {
				return err
			}
			isNew = true
		case err != nil:
			return err
		case status != "active":
			return httpx.ErrForbidden
		}

		// Administratori i provës e merr të drejtën në kyçje: paneli provohet pa ndezje të dytë.
		if s.isTestAdmin(phone) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_capabilities (user_id, capability) VALUES ($1, 'SUPER_ADMIN')
				ON CONFLICT DO NOTHING`, userID); err != nil {
				return err
			}
		}

		// sesioni + refresh token
		refresh, hash, err := NewRefreshToken()
		if err != nil {
			return err
		}
		var ip *net.IP
		if p := net.ParseIP(dev.IP); p != nil {
			ip = &p
		}
		var sessionID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO sessions (user_id, device_id, device_name, platform, refresh_token_hash, refresh_expires_at, ip)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			userID, dev.ID, nullable(dev.Name), dev.Platform, hash, time.Now().Add(RefreshTokenTTL), ip).Scan(&sessionID); err != nil {
			return err
		}
		caps, err := loadCapabilities(ctx, tx, userID)
		if err != nil {
			return err
		}
		now := time.Now()
		access, err := s.signer.IssueAccess(userID, sessionID, caps, now)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, metadata)
			VALUES ($1, 'auth.login', 'session', $2, $3, jsonb_build_object('platform', $4::text, 'new_user', $5::boolean))`,
			userID, sessionID.String(), ip, dev.Platform, isNew); err != nil {
			return err
		}
		out = &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: now.Add(AccessTokenTTL), UserID: userID, IsNewUser: isNew}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Refresh rrotullon refresh token-in: i vjetri bëhet i pavlefshëm menjëherë (§53 rotation).
// Ripërdorimi i një token-i të rrotulluar shkyç gjithë sesionin (sinjal vjedhjeje).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	hash := HashToken(refreshToken)
	var out *TokenPair
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var sessionID, userID uuid.UUID
		var expires time.Time
		var revoked *time.Time
		err := tx.QueryRow(ctx, `
			SELECT id, user_id, refresh_expires_at, revoked_at FROM sessions WHERE refresh_token_hash = $1 FOR UPDATE`, hash).
			Scan(&sessionID, &userID, &expires, &revoked)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionInvalid
		}
		if err != nil {
			return err
		}
		if revoked != nil || time.Now().After(expires) {
			return ErrSessionInvalid
		}
		newRefresh, newHash, err := NewRefreshToken()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE sessions SET refresh_token_hash = $2, refresh_expires_at = $3, last_seen_at = now() WHERE id = $1`,
			sessionID, newHash, time.Now().Add(RefreshTokenTTL)); err != nil {
			return err
		}
		caps, err := loadCapabilities(ctx, tx, userID)
		if err != nil {
			return err
		}
		now := time.Now()
		access, err := s.signer.IssueAccess(userID, sessionID, caps, now)
		if err != nil {
			return err
		}
		out = &TokenPair{AccessToken: access, RefreshToken: newRefresh, ExpiresAt: now.Add(AccessTokenTTL), UserID: userID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Logout shkyç sesionin aktual; access token-i vdes vetë brenda 15 min.
func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	return err
}

// RevokeAll shkyç të gjitha sesionet e përdoruesit (siguri: "dil nga të gjitha pajisjet").
func (s *Service) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// SessionActive verifikohet në çdo kërkesë të autentikuar: shkyçja vlen menjëherë, jo pas 15 min.
func (s *Service) SessionActive(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.revoked_at IS NULL AND s.refresh_expires_at > now() AND u.status = 'active')`, sessionID).Scan(&active)
	return active, err
}

func loadCapabilities(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, userID uuid.UUID) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT capability FROM user_capabilities WHERE user_id = $1 AND revoked_at IS NULL ORDER BY capability`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var caps []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		caps = append(caps, c)
	}
	return caps, rows.Err()
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
