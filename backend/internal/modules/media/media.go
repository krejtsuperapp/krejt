// Package media — imazhet publike (§43): logo/kopertinë vendi, imazh produkti, foto profili.
// Ngarkimi shkon drejt në bucket me URL të nënshkruar (PUT); pas tij serveri verifikon objektin
// (lloji dhe madhësia si u premtuan) dhe e lidh me pronarin në një hap të vetëm, që një çelës
// të mos mund të vendoset kurrë "me dorë" te një vend apo produkt që s'të takon.
package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/media"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

var (
	ErrObjectMissing  = &httpx.APIError{Code: "UPLOAD_NOT_FOUND", MessageKey: "errors.documents.upload_not_found", HTTPStatus: http.StatusUnprocessableEntity}
	ErrObjectMismatch = &httpx.APIError{Code: "UPLOAD_MISMATCH", MessageKey: "errors.documents.upload_mismatch", HTTPStatus: http.StatusUnprocessableEntity}
)

const (
	MaxSizeBytes = 5 << 20
	UploadTTL    = 10 * time.Minute
)

var contentExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// Membership — stafi i vendit (nga moduli merchants); injektohet për të shmangur varësi ciklike.
type Membership interface {
	Membership(ctx context.Context, userID, merchantID uuid.UUID) (string, error)
}

// MenuInvalidator — menuja publike ka cache të shkurtër; imazhi i ri duhet të duket menjëherë.
type MenuInvalidator interface {
	InvalidateMenu(merchantID uuid.UUID)
}

type Service struct {
	pool   *pgxpool.Pool
	store  storage.Provider
	member Membership
	menu   MenuInvalidator
}

func New(pool *pgxpool.Pool, store storage.Provider, member Membership) *Service {
	return &Service{pool: pool, store: store, member: member}
}

func (s *Service) WithMenuInvalidator(m MenuInvalidator) *Service {
	s.menu = m
	return s
}

// --- ngarkimi ---------------------------------------------------------------------

type UploadRequest struct {
	Kind        string     `json:"kind"`
	TargetID    *uuid.UUID `json:"target_id"` // vendi ose produkti; për user_photo lihet bosh
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
}

type UploadResponse struct {
	ObjectKey string               `json:"object_key"`
	Upload    storage.UploadTarget `json:"upload"`
}

// owner — pronari i objektit sipas llojit, pasi është verifikuar se aktori ka të drejtë mbi të.
// Brendimi i vendit (logo/kopertinë) është vendim i pronarit/menaxherit; imazhi i produktit
// ndryshohet nga kushdo që ndryshon menunë, si vetë produkti.
func (s *Service) owner(ctx context.Context, a principal.Actor, kind media.Kind, target *uuid.UUID) (uuid.UUID, error) {
	switch kind {
	case media.KindUserPhoto:
		if target != nil && *target != a.UserID {
			return uuid.Nil, httpx.ErrForbidden
		}
		return a.UserID, nil
	case media.KindMerchantLogo, media.KindMerchantCover:
		if target == nil {
			return uuid.Nil, httpx.ErrValidation.WithFields(map[string]string{"target_id": "required"})
		}
		role, err := s.member.Membership(ctx, a.UserID, *target)
		if err != nil {
			return uuid.Nil, err
		}
		if role != "owner" && role != "manager" {
			return uuid.Nil, httpx.ErrForbidden
		}
		return *target, nil
	case media.KindProductImage:
		if target == nil {
			return uuid.Nil, httpx.ErrValidation.WithFields(map[string]string{"target_id": "required"})
		}
		var merchantID uuid.UUID
		err := s.pool.QueryRow(ctx, `SELECT merchant_id FROM products WHERE id = $1 AND deleted_at IS NULL`, *target).Scan(&merchantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, httpx.ErrNotFound
		}
		if err != nil {
			return uuid.Nil, err
		}
		if _, err := s.member.Membership(ctx, a.UserID, merchantID); err != nil {
			return uuid.Nil, err
		}
		return *target, nil
	}
	return uuid.Nil, httpx.ErrValidation.WithFields(map[string]string{"kind": "invalid"})
}

// UploadURL — URL e nënshkruar për PUT direkt në bucket; çelësi mbart llojin dhe pronarin.
func (s *Service) UploadURL(ctx context.Context, a principal.Actor, in UploadRequest) (*UploadResponse, error) {
	fields := map[string]string{}
	if !media.ValidKind(in.Kind) {
		fields["kind"] = "invalid"
	}
	ext, okCT := contentExt[in.ContentType]
	if !okCT {
		fields["content_type"] = "unsupported"
	}
	if in.SizeBytes <= 0 || in.SizeBytes > MaxSizeBytes {
		fields["size_bytes"] = "invalid"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	kind := media.Kind(in.Kind)
	owner, err := s.owner(ctx, a, kind, in.TargetID)
	if err != nil {
		return nil, err
	}
	key := media.NewKey(kind, owner, ext)
	target, err := s.store.PresignUpload(ctx, key, in.ContentType, in.SizeBytes, UploadTTL)
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	return &UploadResponse{ObjectKey: key, Upload: target}, nil
}

// --- konfirmimi ----------------------------------------------------------------------

type ConfirmRequest struct {
	ObjectKey string `json:"object_key"`
}

type Confirmed struct {
	Kind      string    `json:"kind"`
	TargetID  uuid.UUID `json:"target_id"`
	ObjectKey string    `json:"object_key"`
	URL       *string   `json:"url"`
}

// Confirm — objekti ekziston dhe përputhet me premtimin; çelësi vendoset te pronari (kolona
// sipas llojit) dhe objekti i mëparshëm fshihet, që bucket-i të mos mbajë imazhe jetime.
func (s *Service) Confirm(ctx context.Context, a principal.Actor, in ConfirmRequest) (*Confirmed, error) {
	kind, owner, ok := media.Parse(in.ObjectKey)
	if !ok {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"object_key": "invalid"})
	}
	// I njëjti kontroll si te ngarkimi: çelësi mund të jetë kopjuar nga dikush tjetër.
	target := owner
	if _, err := s.owner(ctx, a, kind, &target); err != nil {
		return nil, err
	}
	info, err := s.store.Head(ctx, in.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrObjectMissing
	}
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	if want, ok := contentExt[info.ContentType]; !ok || "."+want != path.Ext(in.ObjectKey) || info.SizeBytes <= 0 || info.SizeBytes > MaxSizeBytes {
		_ = s.store.Delete(ctx, in.ObjectKey)
		return nil, ErrObjectMismatch
	}

	var previous *string
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var q string
		switch kind {
		case media.KindMerchantLogo:
			q = `UPDATE merchants SET logo_key = $2, updated_at = now() WHERE id = $1 RETURNING (SELECT logo_key FROM merchants WHERE id = $1)`
		case media.KindMerchantCover:
			q = `UPDATE merchants SET cover_key = $2, updated_at = now() WHERE id = $1 RETURNING (SELECT cover_key FROM merchants WHERE id = $1)`
		case media.KindProductImage:
			q = `UPDATE products SET image_key = $2, updated_at = now() WHERE id = $1 AND deleted_at IS NULL RETURNING (SELECT image_key FROM products WHERE id = $1)`
		case media.KindUserPhoto:
			q = `UPDATE users SET photo_key = $2, updated_at = now() WHERE id = $1 RETURNING (SELECT photo_key FROM users WHERE id = $1)`
		}
		if err := tx.QueryRow(ctx, q, owner, in.ObjectKey).Scan(&previous); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ErrNotFound
			}
			return err
		}
		return audit(ctx, tx, a, "media."+string(kind)+"_set", string(kind), owner, map[string]any{"object_key": in.ObjectKey})
	})
	if err != nil {
		return nil, err
	}
	if previous != nil && *previous != "" && *previous != in.ObjectKey {
		// Objekti i vjetër nuk i duhet askujt; nëse fshirja dështon, versionimi i bucket-it e pastron.
		_ = s.store.Delete(ctx, *previous)
	}
	if kind == media.KindProductImage && s.menu != nil {
		var merchantID uuid.UUID
		if err := s.pool.QueryRow(ctx, `SELECT merchant_id FROM products WHERE id = $1`, owner).Scan(&merchantID); err == nil {
			s.menu.InvalidateMenu(merchantID)
		}
	}
	return &Confirmed{Kind: string(kind), TargetID: owner, ObjectKey: in.ObjectKey, URL: media.URL(&in.ObjectKey)}, nil
}

// Open — leximi publik i një objekti media nga vetë API-ja, për mjediset pa CDN përpara
// (CloudFront-i kërkon verifikim të llogarisë). Vetëm çelësa `media/…`: dokumentet e shoferëve
// dhe eksportet e të dhënave nuk kalojnë kurrë këtej.
func (s *Service) Open(ctx context.Context, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	if _, _, ok := media.Parse(key); !ok {
		return nil, storage.ObjectInfo{}, httpx.ErrNotFound
	}
	body, info, err := s.store.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, storage.ObjectInfo{}, httpx.ErrNotFound
	}
	if err != nil {
		return nil, storage.ObjectInfo{}, httpx.ErrUnavailable.With(err)
	}
	return body, info, nil
}

// Remove — heq imazhin nga pronari dhe fshin objektin.
func (s *Service) Remove(ctx context.Context, a principal.Actor, kindStr string, target *uuid.UUID) error {
	if !media.ValidKind(kindStr) {
		return httpx.ErrValidation.WithFields(map[string]string{"kind": "invalid"})
	}
	kind := media.Kind(kindStr)
	owner, err := s.owner(ctx, a, kind, target)
	if err != nil {
		return err
	}
	var previous *string
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var q string
		switch kind {
		case media.KindMerchantLogo:
			q = `UPDATE merchants SET logo_key = NULL, updated_at = now() WHERE id = $1 RETURNING (SELECT logo_key FROM merchants WHERE id = $1)`
		case media.KindMerchantCover:
			q = `UPDATE merchants SET cover_key = NULL, updated_at = now() WHERE id = $1 RETURNING (SELECT cover_key FROM merchants WHERE id = $1)`
		case media.KindProductImage:
			q = `UPDATE products SET image_key = NULL, updated_at = now() WHERE id = $1 AND deleted_at IS NULL RETURNING (SELECT image_key FROM products WHERE id = $1)`
		case media.KindUserPhoto:
			q = `UPDATE users SET photo_key = NULL, updated_at = now() WHERE id = $1 RETURNING (SELECT photo_key FROM users WHERE id = $1)`
		}
		if err := tx.QueryRow(ctx, q, owner).Scan(&previous); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ErrNotFound
			}
			return err
		}
		return audit(ctx, tx, a, "media."+string(kind)+"_removed", string(kind), owner, nil)
	})
	if err != nil {
		return err
	}
	if previous != nil && *previous != "" {
		_ = s.store.Delete(ctx, *previous)
	}
	if kind == media.KindProductImage && s.menu != nil {
		var merchantID uuid.UUID
		if err := s.pool.QueryRow(ctx, `SELECT merchant_id FROM products WHERE id = $1`, owner).Scan(&merchantID); err == nil {
			s.menu.InvalidateMenu(merchantID)
		}
	}
	return nil
}

func audit(ctx context.Context, tx pgx.Tx, a principal.Actor, action, targetType string, target uuid.UUID, meta map[string]any) error {
	var ip *net.IP
	if p := net.ParseIP(a.IP); p != nil {
		ip = &p
	}
	if meta == nil {
		meta = map[string]any{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var reqID *string
	if v := logx.RequestID(ctx); v != "" {
		reqID = &v
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, request_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, a.UserID, action, targetType, target.String(), ip, reqID, metaJSON)
	if err != nil {
		return fmt.Errorf("media: audit: %w", err)
	}
	return nil
}
