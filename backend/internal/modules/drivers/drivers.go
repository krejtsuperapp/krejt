// Package drivers — profili i shoferit (§18 Driver: automjeti, kategoritë, pranueshmëria),
// miratimi nga Operacionet, online/offline dhe marrja e lokacionit. Dokumentet (patentë, ID,
// sigurim…) me skadim vijnë në hapin tjetër — deri atëherë miratimi bëhet me dorë nga OPERATIONS.
package drivers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	phoneutil "krejt.app/backend/internal/platform/phone"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrNotDriver     = &httpx.APIError{Code: "NOT_A_DRIVER", MessageKey: "errors.drivers.not_a_driver", HTTPStatus: http.StatusForbidden}
	ErrNotApproved   = &httpx.APIError{Code: "DRIVER_NOT_APPROVED", MessageKey: "errors.drivers.not_approved", HTTPStatus: http.StatusForbidden}
	ErrProfileLocked = &httpx.APIError{Code: "DRIVER_PROFILE_LOCKED", MessageKey: "errors.drivers.profile_locked", HTTPStatus: http.StatusConflict}
)

var KnownCategories = []string{"economy", "comfort", "xl", "taxi"}

type Profile struct {
	UserID          uuid.UUID  `json:"user_id"`
	Status          string     `json:"status"`
	VehicleMake     string     `json:"vehicle_make"`
	VehicleModel    string     `json:"vehicle_model"`
	VehiclePlate    string     `json:"vehicle_plate"`
	VehicleColor    string     `json:"vehicle_color"`
	Categories      []string   `json:"categories"`
	Rating          *float64   `json:"rating"`
	RatingCount     int        `json:"rating_count"`
	ApprovedAt      *time.Time `json:"approved_at"`
	SuspendedReason *string    `json:"suspended_reason"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ApplyInput struct {
	VehicleMake  string   `json:"vehicle_make"`
	VehicleModel string   `json:"vehicle_model"`
	VehiclePlate string   `json:"vehicle_plate"`
	VehicleColor string   `json:"vehicle_color"`
	Categories   []string `json:"categories"`
}

// Eligibility — a i ka shoferi dokumentet e detyrueshme të miratuara (moduli documents).
type Eligibility interface {
	Eligible(ctx context.Context, driverID uuid.UUID) (ok bool, missing []string, err error)
}

type Service struct {
	pool        *pgxpool.Pool
	loc         *location.Service
	eligibility Eligibility
}

func New(pool *pgxpool.Pool, loc *location.Service) *Service {
	return &Service{pool: pool, loc: loc}
}

// WithEligibility — miratimi kërkon dokumentet e detyrueshme (kur është vendosur).
func (s *Service) WithEligibility(e Eligibility) *Service {
	s.eligibility = e
	return s
}

var ErrDocumentsIncomplete = &httpx.APIError{Code: "DRIVER_DOCUMENTS_INCOMPLETE", MessageKey: "errors.drivers.documents_incomplete", HTTPStatus: http.StatusUnprocessableEntity}

const profileCols = `user_id, status, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories,
	rating_sum, rating_count, approved_at, suspended_reason, created_at, updated_at`

func scanProfile(row pgx.Row) (*Profile, error) {
	var p Profile
	var sum int
	if err := row.Scan(&p.UserID, &p.Status, &p.VehicleMake, &p.VehicleModel, &p.VehiclePlate, &p.VehicleColor, &p.Categories,
		&sum, &p.RatingCount, &p.ApprovedAt, &p.SuspendedReason, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	if p.RatingCount > 0 {
		r := float64(sum) / float64(p.RatingCount)
		r = float64(int(r*100+0.5)) / 100
		p.Rating = &r
	}
	return &p, nil
}

func validateApply(in *ApplyInput) map[string]string {
	f := map[string]string{}
	norm := func(v *string, name string, min, max int) {
		*v = strings.Join(strings.Fields(*v), " ")
		n := utf8.RuneCountInString(*v)
		if n < min {
			f[name] = "required"
		} else if n > max {
			f[name] = "too_long"
		}
	}
	norm(&in.VehicleMake, "vehicle_make", 2, 40)
	norm(&in.VehicleModel, "vehicle_model", 1, 40)
	norm(&in.VehicleColor, "vehicle_color", 2, 30)
	in.VehiclePlate = strings.ToUpper(strings.Join(strings.Fields(in.VehiclePlate), "-"))
	if n := len(in.VehiclePlate); n < 4 || n > 12 {
		f["vehicle_plate"] = "invalid"
	}
	if cats, ok := NormalizeCategories(in.Categories); ok {
		in.Categories = cats
	} else {
		f["categories"] = "invalid"
	}
	return f
}

// NormalizeCategories — nëngrup i kategorive të njohura, pa dyfishim, në rendin kanonik.
func NormalizeCategories(in []string) ([]string, bool) {
	if len(in) == 0 {
		return nil, false
	}
	want := map[string]bool{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		known := false
		for _, k := range KnownCategories {
			if k == c {
				known = true
			}
		}
		if !known {
			return nil, false
		}
		want[c] = true
	}
	out := make([]string, 0, len(want))
	for _, k := range KnownCategories {
		if want[k] {
			out = append(out, k)
		}
	}
	return out, true
}

// Apply — aplikim ose ndryshim i profilit (vetëm sa është 'pending').
func (s *Service) Apply(ctx context.Context, a principal.Actor, in ApplyInput) (*Profile, error) {
	if f := validateApply(&in); len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}
	var out *Profile
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM drivers WHERE user_id = $1 FOR UPDATE`, a.UserID).Scan(&status)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && status != "pending" {
			return ErrProfileLocked
		}
		p, err := scanProfile(tx.QueryRow(ctx, `
			INSERT INTO drivers (user_id, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (user_id) DO UPDATE SET vehicle_make = EXCLUDED.vehicle_make, vehicle_model = EXCLUDED.vehicle_model,
			  vehicle_plate = EXCLUDED.vehicle_plate, vehicle_color = EXCLUDED.vehicle_color, categories = EXCLUDED.categories,
			  updated_at = now()
			RETURNING `+profileCols, a.UserID, in.VehicleMake, in.VehicleModel, in.VehiclePlate, in.VehicleColor, in.Categories))
		if err != nil {
			return err
		}
		out = p
		if err := audit(ctx, tx, a, "driver.applied", a.UserID, map[string]any{"categories": in.Categories}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", a.UserID.String(), "DriverApplied", map[string]any{"driver_id": a.UserID, "categories": in.Categories})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	p, err := scanProfile(s.pool.QueryRow(ctx, `SELECT `+profileCols+` FROM drivers WHERE user_id = $1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotDriver
	}
	return p, err
}

// CreateFor — OPERATIONS regjistron një shofer në emër të tij: onboarding në zyrë, ku të dhënat
// e automjetit i lexon dikush nga letrat dhe jo shoferi nga telefoni.
//
// Numri duhet të ketë hyrë një herë. Kjo është e qëllimshme: paneli regjistron shoferë, nuk
// krijon llogari. Një llogari e krijuar nga paneli nuk do të kishte kaluar kurrë nga verifikimi
// i numrit, dhe askush nuk do ta dinte se kujt i përket.
func (s *Service) CreateFor(ctx context.Context, admin principal.Actor, phone string, in ApplyInput) (*Profile, error) {
	f := validateApply(&in)
	if f == nil {
		f = map[string]string{}
	}
	if !phoneutil.Valid(phone) {
		f["phone"] = "invalid"
	}
	if len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}

	var out *Profile
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var driverID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM users WHERE phone_e164 = $1`, phone).Scan(&driverID)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound.WithFields(map[string]string{"phone": "unknown"})
		}
		if err != nil {
			return err
		}

		var status string
		err = tx.QueryRow(ctx, `SELECT status FROM drivers WHERE user_id = $1 FOR UPDATE`, driverID).Scan(&status)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// I njëjti kufi si te vetë-aplikimi: një profil i aprovuar nuk rishkruhet pa vendim.
		if err == nil && status != "pending" {
			return ErrProfileLocked
		}

		p, err := scanProfile(tx.QueryRow(ctx, `
			INSERT INTO drivers (user_id, vehicle_make, vehicle_model, vehicle_plate, vehicle_color, categories)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (user_id) DO UPDATE SET vehicle_make = EXCLUDED.vehicle_make, vehicle_model = EXCLUDED.vehicle_model,
			  vehicle_plate = EXCLUDED.vehicle_plate, vehicle_color = EXCLUDED.vehicle_color, categories = EXCLUDED.categories,
			  updated_at = now()
			RETURNING `+profileCols, driverID, in.VehicleMake, in.VehicleModel, in.VehiclePlate, in.VehicleColor, in.Categories))
		if err != nil {
			return err
		}
		out = p
		// Gjurma mban kush e regjistroi: ndryshe nga vetë-aplikimi, këtu vepruesi nuk është shoferi.
		if err := audit(ctx, tx, admin, "driver.created_by_ops", driverID, map[string]any{"categories": in.Categories}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", driverID.String(), "DriverApplied", map[string]any{"driver_id": driverID, "categories": in.Categories})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Approved — profili vetëm nëse është i miratuar (pranueshmëria për punë).
func (s *Service) Approved(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	p, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p.Status != "approved" {
		return nil, ErrNotApproved
	}
	return p, nil
}

// Pending — aplikimet në pritje (për Admin → Shoferët & verifikimi).
func (s *Service) Pending(ctx context.Context, limit int) ([]Profile, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT `+profileCols+` FROM drivers WHERE status = 'pending' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// Approve — OPERATIONS miraton shoferin dhe i jep kapacitetet (RIDE_DRIVER; TAXI_DRIVER nëse 'taxi').
func (s *Service) Approve(ctx context.Context, admin principal.Actor, driverID uuid.UUID, categories []string) (*Profile, error) {
	cats, ok := NormalizeCategories(categories)
	if !ok {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"categories": "invalid"})
	}
	if s.eligibility != nil {
		// kategoritë e reja (p.sh. taxi) ndikojnë në dokumentet e kërkuara — ruhen para kontrollit
		if _, err := s.pool.Exec(ctx, `UPDATE drivers SET categories = $2, updated_at = now() WHERE user_id = $1`, driverID, cats); err != nil {
			return nil, err
		}
		ok, missing, err := s.eligibility.Eligible(ctx, driverID)
		if err != nil {
			return nil, err
		}
		if !ok {
			f := map[string]string{}
			for _, m := range missing {
				f["documents."+m] = "missing"
			}
			return nil, ErrDocumentsIncomplete.WithFields(f)
		}
	}
	var out *Profile
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := scanProfile(tx.QueryRow(ctx, `
			UPDATE drivers SET status = 'approved', categories = $2, approved_at = now(), approved_by = $3,
			                   suspended_reason = NULL, updated_at = now()
			WHERE user_id = $1 RETURNING `+profileCols, driverID, cats, admin.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = p
		grant := []string{"RIDE_DRIVER"}
		for _, c := range cats {
			if c == "taxi" {
				grant = append(grant, "TAXI_DRIVER")
			}
		}
		for _, cap := range grant {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_capabilities (user_id, capability, granted_by) VALUES ($1, $2, $3)
				ON CONFLICT (user_id, capability) DO UPDATE SET revoked_at = NULL, granted_at = now(), granted_by = EXCLUDED.granted_by`,
				driverID, cap, admin.UserID); err != nil {
				return err
			}
		}
		if err := audit(ctx, tx, admin, "driver.approved", driverID, map[string]any{"categories": cats, "capabilities": grant}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", driverID.String(), "DriverApproved", map[string]any{"driver_id": driverID, "categories": cats})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Suspend — pezullim: kapacitetet hiqen, shoferi del offline. Sesionet mbeten (mund të shohë arsyen).
func (s *Service) Suspend(ctx context.Context, admin principal.Actor, driverID uuid.UUID, reason string) (*Profile, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"reason": "required"})
	}
	var out *Profile
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		p, err := scanProfile(tx.QueryRow(ctx, `
			UPDATE drivers SET status = 'suspended', suspended_reason = $2, updated_at = now()
			WHERE user_id = $1 RETURNING `+profileCols, driverID, reason))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = p
		if _, err := tx.Exec(ctx, `UPDATE user_capabilities SET revoked_at = now()
			WHERE user_id = $1 AND capability IN ('RIDE_DRIVER','TAXI_DRIVER') AND revoked_at IS NULL`, driverID); err != nil {
			return err
		}
		if err := audit(ctx, tx, admin, "driver.suspended", driverID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "driver", driverID.String(), "DriverSuspended", map[string]any{"driver_id": driverID})
	})
	if err != nil {
		return nil, err
	}
	_ = s.loc.SetOffline(ctx, driverID)
	return out, nil
}

// GoOnline / GoOffline / Ingest — puna e përditshme e shoferit.
func (s *Service) GoOnline(ctx context.Context, a principal.Actor) (*location.DriverState, error) {
	p, err := s.Approved(ctx, a.UserID)
	if err != nil {
		return nil, err
	}
	if err := s.loc.SetOnline(ctx, a.UserID, p.Categories); err != nil {
		return nil, err
	}
	return s.loc.State(ctx, a.UserID)
}

func (s *Service) GoOffline(ctx context.Context, a principal.Actor) error {
	return s.loc.SetOffline(ctx, a.UserID)
}

func (s *Service) Ingest(ctx context.Context, a principal.Actor, samples []location.Sample) (int, error) {
	return s.loc.Ingest(ctx, a.UserID, samples)
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
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, request_id, metadata)
		VALUES ($1, $2, 'driver', $3, $4, $5, $6)`, a.UserID, action, target.String(), ip, reqID, metaJSON)
	return err
}
