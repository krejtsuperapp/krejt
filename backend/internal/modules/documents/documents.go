// Package documents — dokumentet e shoferit (§18 eligibility/documents, §51 file validation):
// ngarkim direkt në S3 me URL të nënshkruar, verifikim pas ngarkimit (lloji/madhësia), shqyrtim nga
// Operacionet, skadim automatik (patentë, sigurim, certifikatë) → pezullim derisa të rinovohet.
package documents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/storage"
)

var (
	ErrNotDriver      = &httpx.APIError{Code: "NOT_A_DRIVER", MessageKey: "errors.drivers.not_a_driver", HTTPStatus: http.StatusForbidden}
	ErrObjectMissing  = &httpx.APIError{Code: "UPLOAD_NOT_FOUND", MessageKey: "errors.documents.upload_not_found", HTTPStatus: http.StatusUnprocessableEntity}
	ErrObjectMismatch = &httpx.APIError{Code: "UPLOAD_MISMATCH", MessageKey: "errors.documents.upload_mismatch", HTTPStatus: http.StatusUnprocessableEntity}
)

type TypeRule struct {
	Required  bool // për çdo shofer
	TaxiOnly  bool // i detyrueshëm vetëm për kategorinë taxi
	Expires   bool // ka datë skadimi
	ImageOnly bool
}

// Types — llojet e dokumenteve dhe rregullat e tyre.
var Types = map[string]TypeRule{
	"driving_license":      {Required: true, Expires: true},
	"id_card":              {Required: true, Expires: true},
	"vehicle_registration": {Required: true, Expires: true},
	"insurance":            {Required: true, Expires: true},
	"criminal_record":      {Required: true, Expires: true}, // e vlefshme 6 muaj sipas praktikës; data vendoset nga shoferi/ops
	"profile_photo":        {Required: true, ImageOnly: true},
	"taxi_permit":          {TaxiOnly: true, Expires: true},
}

const (
	MaxSizeBytes = 10 << 20
	UploadTTL    = 10 * time.Minute
	DownloadTTL  = 5 * time.Minute
)

var contentExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "application/pdf": "pdf"}

type Document struct {
	ID              uuid.UUID  `json:"id"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	ContentType     string     `json:"content_type"`
	SizeBytes       int64      `json:"size_bytes"`
	ExpiresOn       *time.Time `json:"expires_on"`
	RejectionReason *string    `json:"rejection_reason"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	DownloadURL     string     `json:"download_url,omitempty"`
	DriverID        uuid.UUID  `json:"driver_id,omitempty"`
}

type Service struct {
	pool  *pgxpool.Pool
	store storage.Provider
	now   func() time.Time
}

func New(pool *pgxpool.Pool, store storage.Provider) *Service {
	return &Service{pool: pool, store: store, now: time.Now}
}

// --- ngarkimi -------------------------------------------------------------------

type UploadRequest struct {
	Type        string `json:"type"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type UploadResponse struct {
	ObjectKey string               `json:"object_key"`
	Upload    storage.UploadTarget `json:"upload"`
}

func (s *Service) requireDriver(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var cats []string
	err := s.pool.QueryRow(ctx, `SELECT categories FROM drivers WHERE user_id = $1`, userID).Scan(&cats)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotDriver
	}
	return cats, err
}

// UploadURL — URL e nënshkruar për PUT direkt në S3; çelësi lidhet me shoferin dhe llojin.
func (s *Service) UploadURL(ctx context.Context, a principal.Actor, in UploadRequest) (*UploadResponse, error) {
	if _, err := s.requireDriver(ctx, a.UserID); err != nil {
		return nil, err
	}
	fields := map[string]string{}
	rule, ok := Types[in.Type]
	if !ok {
		fields["type"] = "invalid"
	}
	ext, okCT := contentExt[in.ContentType]
	if !okCT || (rule.ImageOnly && in.ContentType == "application/pdf") {
		fields["content_type"] = "unsupported"
	}
	if in.SizeBytes <= 0 || in.SizeBytes > MaxSizeBytes {
		fields["size_bytes"] = "invalid"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	key := fmt.Sprintf("drivers/%s/%s/%s.%s", a.UserID, in.Type, uuid.NewString(), ext)
	target, err := s.store.PresignUpload(ctx, key, in.ContentType, in.SizeBytes, UploadTTL)
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	return &UploadResponse{ObjectKey: key, Upload: target}, nil
}

type ConfirmRequest struct {
	Type      string `json:"type"`
	ObjectKey string `json:"object_key"`
	ExpiresOn string `json:"expires_on"` // YYYY-MM-DD
}

// Confirm — pas ngarkimit: objekti ekziston, lloji/madhësia përputhen, data e skadimit vlen; dokumenti
// i mëparshëm i të njëjtit lloj bëhet 'replaced'; i riu pret shqyrtimin.
func (s *Service) Confirm(ctx context.Context, a principal.Actor, in ConfirmRequest) (*Document, error) {
	if _, err := s.requireDriver(ctx, a.UserID); err != nil {
		return nil, err
	}
	rule, ok := Types[in.Type]
	fields := map[string]string{}
	if !ok {
		fields["type"] = "invalid"
	}
	prefix := fmt.Sprintf("drivers/%s/%s/", a.UserID, in.Type)
	if !storage.ValidKey(in.ObjectKey) || !strings.HasPrefix(in.ObjectKey, prefix) {
		fields["object_key"] = "invalid"
	}
	var expires *time.Time
	if rule.Expires {
		t, err := time.Parse("2006-01-02", in.ExpiresOn)
		if err != nil {
			fields["expires_on"] = "invalid"
		} else if !t.After(s.now()) {
			fields["expires_on"] = "expired"
		} else if t.After(s.now().AddDate(15, 0, 0)) {
			fields["expires_on"] = "too_far"
		} else {
			expires = &t
		}
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	info, err := s.store.Head(ctx, in.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrObjectMissing
	}
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	ext := path.Ext(in.ObjectKey)
	if want, ok := contentExt[info.ContentType]; !ok || "."+want != ext || info.SizeBytes <= 0 || info.SizeBytes > MaxSizeBytes ||
		(rule.ImageOnly && info.ContentType == "application/pdf") {
		_ = s.store.Delete(ctx, in.ObjectKey)
		return nil, ErrObjectMismatch
	}

	var out Document
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE driver_documents SET status = 'replaced', updated_at = now()
			WHERE driver_id = $1 AND type = $2 AND status IN ('pending','approved','rejected','expired')`, a.UserID, in.Type); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO driver_documents (driver_id, type, object_key, content_type, size_bytes, expires_on)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, type, status, content_type, size_bytes, expires_on, rejection_reason, reviewed_at, created_at`,
			a.UserID, in.Type, in.ObjectKey, info.ContentType, info.SizeBytes, expires).
			Scan(&out.ID, &out.Type, &out.Status, &out.ContentType, &out.SizeBytes, &out.ExpiresOn, &out.RejectionReason, &out.ReviewedAt, &out.CreatedAt); err != nil {
			return err
		}
		if err := audit(ctx, tx, a, "driver.document_submitted", out.ID, map[string]any{"type": in.Type}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", a.UserID.String(), "DriverDocumentSubmitted", map[string]any{"driver_id": a.UserID, "document_id": out.ID, "type": in.Type})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// --- listimi & pranueshmëria ------------------------------------------------------

type Overview struct {
	Documents []Document `json:"documents"`
	Missing   []string   `json:"missing"`  // lloje të detyrueshme pa dokument të miratuar
	Expiring  []string   `json:"expiring"` // të miratuara që skadojnë brenda 30 ditësh
	Eligible  bool       `json:"eligible"`
}

// List — dokumentet aktuale (jo 'replaced') të shoferit; withURLs: URL leximi jetëshkurtër.
func (s *Service) List(ctx context.Context, driverID uuid.UUID, withURLs bool) (*Overview, error) {
	cats, err := s.requireDriver(ctx, driverID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, status, content_type, size_bytes, expires_on, rejection_reason, reviewed_at, created_at, object_key
		FROM driver_documents WHERE driver_id = $1 AND status <> 'replaced' ORDER BY type, created_at DESC`, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ov := &Overview{Documents: []Document{}}
	approved := map[string]*time.Time{}
	for rows.Next() {
		var d Document
		var key string
		if err := rows.Scan(&d.ID, &d.Type, &d.Status, &d.ContentType, &d.SizeBytes, &d.ExpiresOn, &d.RejectionReason, &d.ReviewedAt, &d.CreatedAt, &key); err != nil {
			return nil, err
		}
		if withURLs {
			if u, err := s.store.PresignDownload(ctx, key, DownloadTTL); err == nil {
				d.DownloadURL = u
			}
		}
		if d.Status == "approved" {
			approved[d.Type] = d.ExpiresOn
		}
		ov.Documents = append(ov.Documents, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ov.Missing, ov.Expiring = eligibility(cats, approved, s.now())
	ov.Eligible = len(ov.Missing) == 0
	return ov, nil
}

// eligibility — llojet e detyrueshme që mungojnë (ose kanë skaduar) dhe ato që skadojnë së shpejti.
func eligibility(categories []string, approved map[string]*time.Time, now time.Time) (missing, expiring []string) {
	taxi := false
	for _, c := range categories {
		if c == "taxi" {
			taxi = true
		}
	}
	for _, typ := range orderedTypes() {
		rule := Types[typ]
		if !rule.Required && !(rule.TaxiOnly && taxi) {
			continue
		}
		exp, ok := approved[typ]
		if !ok || (exp != nil && !exp.After(now)) {
			missing = append(missing, typ)
			continue
		}
		if exp != nil && exp.Before(now.AddDate(0, 0, 30)) {
			expiring = append(expiring, typ)
		}
	}
	return missing, expiring
}

func orderedTypes() []string {
	return []string{"profile_photo", "id_card", "driving_license", "vehicle_registration", "insurance", "criminal_record", "taxi_permit"}
}

// Eligible — a i ka shoferi të gjitha dokumentet e detyrueshme të miratuara dhe të paskaduara? (për drivers.Approve)
func (s *Service) Eligible(ctx context.Context, driverID uuid.UUID) (bool, []string, error) {
	ov, err := s.List(ctx, driverID, false)
	if err != nil {
		return false, nil, err
	}
	return ov.Eligible, ov.Missing, nil
}

// --- shqyrtimi nga Operacionet ------------------------------------------------------

func (s *Service) Pending(ctx context.Context, limit int) ([]Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT id, driver_id, type, status, content_type, size_bytes, expires_on, created_at
		FROM driver_documents WHERE status = 'pending' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.DriverID, &d.Type, &d.Status, &d.ContentType, &d.SizeBytes, &d.ExpiresOn, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Review — approve | reject (me arsye). Refuzimi njofton shoferin (ngjarje).
func (s *Service) Review(ctx context.Context, admin principal.Actor, docID uuid.UUID, action, reason string) (*Document, error) {
	reason = strings.Join(strings.Fields(reason), " ")
	fields := map[string]string{}
	switch action {
	case "approve":
	case "reject":
		if reason == "" || utf8.RuneCountInString(reason) > 300 {
			fields["reason"] = "required"
		}
	default:
		fields["action"] = "invalid"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	status := "approved"
	var reasonPtr *string
	if action == "reject" {
		status, reasonPtr = "rejected", &reason
	}
	var out Document
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE driver_documents SET status = $2, rejection_reason = $3, reviewed_by = $4, reviewed_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'pending'
			RETURNING id, driver_id, type, status, content_type, size_bytes, expires_on, rejection_reason, reviewed_at, created_at`,
			docID, status, reasonPtr, admin.UserID).
			Scan(&out.ID, &out.DriverID, &out.Type, &out.Status, &out.ContentType, &out.SizeBytes, &out.ExpiresOn, &out.RejectionReason, &out.ReviewedAt, &out.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := audit(ctx, tx, admin, "driver.document_"+status, out.ID, map[string]any{"type": out.Type, "driver_id": out.DriverID, "reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", out.DriverID.String(), "DriverDocumentReviewed", map[string]any{
			"driver_id": out.DriverID, "document_id": out.ID, "type": out.Type, "status": status, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ExpireSweep — dokumentet e miratuara që kanë skaduar → 'expired'; shoferët e miratuar që humbin një
// dokument të detyrueshëm pezullohen (sistemi) derisa ta rinovojnë. Thirret nga worker-i (çdo orë).
func (s *Service) ExpireSweep(ctx context.Context) (expired, suspended int, err error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE driver_documents SET status = 'expired', updated_at = now()
		WHERE status = 'approved' AND expires_on IS NOT NULL AND expires_on < $1::date
		RETURNING driver_id, type`, s.now())
	if err != nil {
		return 0, 0, err
	}
	affected := map[uuid.UUID][]string{}
	for rows.Next() {
		var did uuid.UUID
		var typ string
		if err := rows.Scan(&did, &typ); err != nil {
			rows.Close()
			return 0, 0, err
		}
		expired++
		affected[did] = append(affected[did], typ)
	}
	rows.Close()
	for did, types := range affected {
		ok, missing, err := s.Eligible(ctx, did)
		if err != nil {
			return expired, suspended, err
		}
		if ok {
			continue
		}
		err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `UPDATE drivers SET status = 'suspended', suspended_reason = $2, updated_at = now() WHERE user_id = $1 AND status = 'approved'`,
				did, "document_expired: "+strings.Join(types, ", "))
			if err != nil || tag.RowsAffected() == 0 {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE user_capabilities SET revoked_at = now() WHERE user_id = $1 AND capability IN ('RIDE_DRIVER','TAXI_DRIVER') AND revoked_at IS NULL`, did); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_type, action, target_type, target_id, metadata)
				VALUES ('system', 'driver.suspended', 'driver', $1, jsonb_build_object('reason', 'document_expired', 'types', $2::text[], 'missing', $3::text[]))`,
				did.String(), types, missing); err != nil {
				return err
			}
			suspended++
			return events.Emit(ctx, tx, "driver", did.String(), "DriverSuspended", map[string]any{"driver_id": did, "reason": "document_expired", "types": types})
		})
		if err != nil {
			return expired, suspended, err
		}
	}
	return expired, suspended, nil
}

func audit(ctx context.Context, tx events.Execer, a principal.Actor, action string, target uuid.UUID, meta map[string]any) error {
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
	_, err = tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, request_id, metadata)
		VALUES ($1, $2, 'driver_document', $3, $4, $5, $6)`, a.UserID, action, target.String(), ip, reqID, metaJSON)
	return err
}
