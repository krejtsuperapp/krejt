// Package users — profili (§16 Profile), adresat e ruajtura (§46; vetëm Kosovë §1), preferencat
// e njoftimeve (§29), pajisjet/sesionet (§53) dhe fshirja e llogarisë. Serveri është autoritar:
// klienti nuk dërgon rol, status apo bilanc. Çdo ndryshim: rresht në audit_log + ngjarje në outbox.
package users

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
)

var (
	ErrEmailTaken     = &httpx.APIError{Code: "EMAIL_TAKEN", MessageKey: "errors.users.email_taken", HTTPStatus: http.StatusConflict}
	ErrWalletNotEmpty = &httpx.APIError{Code: "WALLET_NOT_EMPTY", MessageKey: "errors.users.wallet_not_empty", HTTPStatus: http.StatusConflict}
	ErrAddressLimit   = &httpx.APIError{Code: "ADDRESS_LIMIT", MessageKey: "errors.users.address_limit", HTTPStatus: http.StatusUnprocessableEntity}
	ErrOutsideKosovo  = &httpx.APIError{Code: "ADDRESS_OUTSIDE_KOSOVO", MessageKey: "errors.users.address_outside_kosovo", HTTPStatus: http.StatusUnprocessableEntity}
)

// MaxAddresses — adresa aktive për përdorues.
const MaxAddresses = 20

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
}

func New(pool *pgxpool.Pool, led *ledger.Service) *Service {
	return &Service{pool: pool, ledger: led}
}

// Actor — kush vepron (nga JWT-ja e verifikuar), për audit.
type Actor struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	IP        string
}

// --- profili ------------------------------------------------------------------

type Profile struct {
	ID        uuid.UUID `json:"id"`
	Phone     *string   `json:"phone"`
	Email     *string   `json:"email"`
	FullName  *string   `json:"full_name"`
	Locale    string    `json:"locale"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfileUpdate — fushat e dërguara ndryshohen; "" pastron full_name / email.
type ProfileUpdate struct {
	FullName *string `json:"full_name"`
	Email    *string `json:"email"`
	Locale   *string `json:"locale"`
}

func (s *Service) UpdateProfile(ctx context.Context, a Actor, in ProfileUpdate) (*Profile, error) {
	if in.FullName == nil && in.Email == nil && in.Locale == nil {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"body": "empty"})
	}
	fields := map[string]string{}
	var fullName, email *string
	if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
		v, ok := NormalizeName(*in.FullName)
		if !ok {
			fields["full_name"] = "invalid"
		}
		fullName = &v
	}
	if in.Email != nil && strings.TrimSpace(*in.Email) != "" {
		v, ok := NormalizeEmail(*in.Email)
		if !ok {
			fields["email"] = "invalid"
		}
		email = &v
	}
	if in.Locale != nil && !ValidLocale(*in.Locale) {
		fields["locale"] = "invalid"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}

	var p Profile
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT id, phone_e164, email, full_name, locale FROM users
			WHERE id = $1 AND status = 'active' FOR UPDATE`, a.UserID).
			Scan(&p.ID, &p.Phone, &p.Email, &p.FullName, &p.Locale)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		var changed []string
		if in.FullName != nil {
			p.FullName = fullName
			changed = append(changed, "full_name")
		}
		if in.Email != nil {
			p.Email = email
			changed = append(changed, "email")
		}
		if in.Locale != nil {
			p.Locale = *in.Locale
			changed = append(changed, "locale")
		}
		err = tx.QueryRow(ctx, `
			UPDATE users SET full_name = $2, email = $3, locale = $4, updated_at = now()
			WHERE id = $1 RETURNING updated_at`, p.ID, p.FullName, p.Email, p.Locale).Scan(&p.UpdatedAt)
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		if err != nil {
			return err
		}
		meta := map[string]any{"changed": changed}
		if err := audit(ctx, tx, a, "user.profile_updated", "user", p.ID.String(), meta); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "user", p.ID.String(), "UserProfileUpdated",
			map[string]any{"user_id": p.ID, "changed": changed, "locale": p.Locale})
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteAccount — fshirje e butë + anonimizim (§51): telefoni/emaili/emri hiqen, sesionet shkyçen,
// adresat fshihen. Wallet-i i mbyllur duhet të jetë 0 (paratë nuk humbin dhe nuk tërhiqen — §5).
func (s *Service) DeleteAccount(ctx context.Context, a Actor) error {
	code := ledger.UserWalletCode(a.UserID)
	uid := a.UserID
	if err := s.ledger.EnsureAccount(ctx, code, "user", &uid, "liability", "EUR"); err != nil {
		return err
	}
	bal, err := s.ledger.Balance(ctx, code)
	if err != nil {
		return err
	}
	if bal.Minor != 0 {
		return ErrWalletNotEmpty
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var phone *string
		err := tx.QueryRow(ctx, `SELECT phone_e164 FROM users WHERE id = $1 AND status = 'active' FOR UPDATE`, a.UserID).Scan(&phone)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET status = 'deleted', deleted_at = now(), phone_e164 = NULL, email = NULL,
			                 full_name = NULL, updated_at = now() WHERE id = $1`, a.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, a.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE user_addresses SET deleted_at = now() WHERE user_id = $1 AND deleted_at IS NULL`, a.UserID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM notification_preferences WHERE user_id = $1`, a.UserID); err != nil {
			return err
		}
		meta := map[string]any{}
		if phone != nil { // hash për anti-abuzim (§67), jo teksti i telefonit
			sum := sha256.Sum256([]byte(*phone))
			meta["phone_sha256"] = hex.EncodeToString(sum[:])
		}
		if err := audit(ctx, tx, a, "user.deleted", "user", a.UserID.String(), meta); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "user", a.UserID.String(), "UserDeleted", map[string]any{"user_id": a.UserID})
	})
}

// --- ndihmës ------------------------------------------------------------------

func audit(ctx context.Context, tx events.Execer, a Actor, action, targetType, targetID string, meta map[string]any) error {
	var ip *net.IP
	if p := net.ParseIP(a.IP); p != nil {
		ip = &p
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var reqID *string
	if v := logx.RequestID(ctx); v != "" {
		reqID = &v
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, request_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, a.UserID, action, targetType, targetID, ip, reqID, metaJSON)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
