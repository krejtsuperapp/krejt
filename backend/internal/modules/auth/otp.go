// OTP me telefon (§53). Kodi ruhet vetëm si HMAC-SHA256 me pepper të serverit; TTL 5 min;
// maksimum 5 përpjekje; kufij: 5 kërkesa / 15 min për numër, 20 / 15 min për IP (§51 brute-force,
// mbrojtje nga "SMS pumping"). Numrat pranohen në E.164 nga çdo shtet (diaspora).
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/platform/httpx"
	phoneutil "krejt.app/backend/internal/platform/phone"
)

const (
	otpTTL         = 5 * time.Minute
	otpMaxAttempts = 5
	rlWindow       = 15 * time.Minute
	rlPerPhone     = 5
	rlPerIP        = 20
)

var phoneRe = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

var (
	ErrPhoneInvalid   = &httpx.APIError{Code: "PHONE_INVALID", MessageKey: "errors.auth.phone_invalid", HTTPStatus: 422}
	ErrOTPInvalid     = &httpx.APIError{Code: "OTP_INVALID", MessageKey: "errors.auth.otp_invalid", HTTPStatus: 401}
	ErrOTPExpired     = &httpx.APIError{Code: "OTP_EXPIRED", MessageKey: "errors.auth.otp_expired", HTTPStatus: 401}
	ErrOTPTooMany     = &httpx.APIError{Code: "OTP_TOO_MANY_ATTEMPTS", MessageKey: "errors.auth.otp_too_many", HTTPStatus: 429, Retryable: false}
	ErrRateLimited    = httpx.ErrRateLimited
	ErrSessionInvalid = &httpx.APIError{Code: "SESSION_INVALID", MessageKey: "errors.auth.session_invalid", HTTPStatus: 401}
)

func ValidPhone(p string) bool { return phoneutil.Valid(p) }

func (s *Service) hashCode(phone, code string) []byte {
	m := hmac.New(sha256.New, s.pepper)
	m.Write([]byte(phone + ":" + code))
	return m.Sum(nil)
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// rateLimit — INCR + EXPIRE në Redis; kthen ErrRateLimited kur kalohet.
func (s *Service) rateLimit(ctx context.Context, key string, limit int64) error {
	pipe := s.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rlWindow)
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return httpx.ErrUnavailable.With(err)
	}
	if incr.Val() > limit {
		return ErrRateLimited
	}
	return nil
}

// RequestOTP krijon sfidën dhe dërgon kodin. Përgjigja është e njëjtë pavarësisht nëse numri ekziston (pa enumerim).
func (s *Service) RequestOTP(ctx context.Context, phone, ip, locale string) error {
	if !ValidPhone(phone) {
		return ErrPhoneInvalid
	}
	if err := s.rateLimit(ctx, "rl:otp:phone:"+phone, rlPerPhone); err != nil {
		return err
	}
	if ip != "" {
		if err := s.rateLimit(ctx, "rl:otp:ip:"+ip, rlPerIP); err != nil {
			return err
		}
	}
	code, err := randomCode()
	if err != nil {
		return httpx.ErrInternal.With(err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO otp_challenges (phone_e164, code_hash, channel, expires_at) VALUES ($1, $2, 'sms', $3)`,
		phone, s.hashCode(phone, code), time.Now().Add(otpTTL)); err != nil {
		return httpx.ErrInternal.With(err)
	}
	if err := s.sms.SendOTP(ctx, phone, code, locale); err != nil {
		return httpx.ErrUnavailable.With(err)
	}
	return nil
}

// verifyChallenge konsumon sfidën e fundit të vlefshme për numrin; numëron përpjekjet.
func (s *Service) verifyChallenge(ctx context.Context, tx pgx.Tx, phone, code string) error {
	var id string
	var hash []byte
	var attempts int16
	var expires time.Time
	err := tx.QueryRow(ctx, `
		SELECT id, code_hash, attempts, expires_at FROM otp_challenges
		WHERE phone_e164 = $1 AND consumed_at IS NULL
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, phone).Scan(&id, &hash, &attempts, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOTPInvalid
	}
	if err != nil {
		return httpx.ErrInternal.With(err)
	}
	if time.Now().After(expires) {
		return ErrOTPExpired
	}
	if attempts >= otpMaxAttempts {
		return ErrOTPTooMany
	}
	if subtle.ConstantTimeCompare(hash, s.hashCode(phone, code)) != 1 {
		_, _ = tx.Exec(ctx, `UPDATE otp_challenges SET attempts = attempts + 1 WHERE id = $1`, id)
		return ErrOTPInvalid
	}
	_, err = tx.Exec(ctx, `UPDATE otp_challenges SET consumed_at = now() WHERE id = $1`, id)
	return err
}
