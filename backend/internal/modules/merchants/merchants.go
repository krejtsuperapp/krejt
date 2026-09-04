// Package merchants — merchant-ët e Fazës 2 (§19 Merchant, §21): aplikim nga pronari, miratim nga
// Operacionet (kapaciteti MERCHANT), profil, orare (me kalim mesnate), staf me role, "pauzë" e shpejtë,
// zbulim publik (afër klientit, hapur tani, kërkim pa theksa). Vetëm Kosovë; zona e shërbimit nga koordinatat.
package merchants

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/pricing"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/media"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrNotMember    = &httpx.APIError{Code: "NOT_MERCHANT_STAFF", MessageKey: "errors.merchants.not_staff", HTTPStatus: http.StatusForbidden}
	ErrSlugTaken    = &httpx.APIError{Code: "MERCHANT_SLUG_TAKEN", MessageKey: "errors.merchants.slug_taken", HTTPStatus: http.StatusConflict}
	ErrNotActive    = &httpx.APIError{Code: "MERCHANT_NOT_ACTIVE", MessageKey: "errors.merchants.not_active", HTTPStatus: http.StatusConflict}
	ErrStaffMissing = &httpx.APIError{Code: "STAFF_USER_NOT_FOUND", MessageKey: "errors.merchants.staff_user_not_found", HTTPStatus: http.StatusUnprocessableEntity}
)

var Types = []string{"restaurant", "store", "grocery", "pharmacy"}

type Hours struct {
	Weekday int    `json:"weekday"` // 0 = e diel … 6 = e shtunë (time.Weekday)
	Opens   string `json:"opens"`   // "08:00"
	Closes  string `json:"closes"`  // "23:30"; nëse <= opens → mbyllet pas mesnate
}

type Merchant struct {
	ID               uuid.UUID `json:"id"`
	Type             string    `json:"type"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Description      *string   `json:"description"`
	Phone            *string   `json:"phone,omitempty"`
	AddressLine1     string    `json:"address_line1"`
	City             string    `json:"city"`
	Location         geo.Point `json:"location"`
	ServiceAreaID    *string   `json:"service_area_id"`
	Status           string    `json:"status"`
	Cuisines         []string  `json:"cuisines"`
	Tags             []string  `json:"tags"`
	FulfillmentMode  string    `json:"fulfillment_mode"`
	MinOrderMinor    int64     `json:"min_order_minor"`
	DeliveryFeeMinor int64     `json:"delivery_fee_minor"`
	PrepTimeMin      int       `json:"prep_time_min"`
	Rating           *float64  `json:"rating"`
	RatingCount      int       `json:"rating_count"`
	LogoKey          *string   `json:"logo_key"`
	CoverKey         *string   `json:"cover_key"`
	LogoURL          *string   `json:"logo_url"`  // publike (CloudFront); null pa logo
	CoverURL         *string   `json:"cover_url"` // publike (CloudFront); null pa kopertinë
	AcceptingOrders  bool      `json:"accepting_orders"`
	OpenNow          bool      `json:"open_now"`
	DistanceM        *int      `json:"distance_m,omitempty"`
	Favourite        bool      `json:"favourite"` // e ruajtur nga ky përdorues; false kur s'ka sesion
	Hours            []Hours   `json:"hours,omitempty"`
	CommissionBP     int       `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
}

type Service struct {
	pool    *pgxpool.Pool
	pricing *pricing.Service
	now     func() time.Time
}

func New(pool *pgxpool.Pool, pr *pricing.Service) *Service {
	return &Service{pool: pool, pricing: pr, now: time.Now}
}

const merchantCols = `id, type, name, slug, description, phone, address_line1, city, lat, lng, service_area_id, status, cuisines, tags,
	fulfillment_mode, min_order_minor, delivery_fee_minor, prep_time_min, rating_sum, rating_count, logo_key, cover_key, accepting_orders, commission_bp, created_at`

func scanMerchant(row pgx.Row) (*Merchant, error) {
	var m Merchant
	var sum int
	if err := row.Scan(&m.ID, &m.Type, &m.Name, &m.Slug, &m.Description, &m.Phone, &m.AddressLine1, &m.City, &m.Location.Lat, &m.Location.Lng,
		&m.ServiceAreaID, &m.Status, &m.Cuisines, &m.Tags, &m.FulfillmentMode, &m.MinOrderMinor, &m.DeliveryFeeMinor, &m.PrepTimeMin,
		&sum, &m.RatingCount, &m.LogoKey, &m.CoverKey, &m.AcceptingOrders, &m.CommissionBP, &m.CreatedAt); err != nil {
		return nil, err
	}
	if m.RatingCount > 0 {
		r := float64(int(float64(sum)/float64(m.RatingCount)*100+0.5)) / 100
		m.Rating = &r
	}
	m.LogoURL = media.URL(m.LogoKey)
	m.CoverURL = media.URL(m.CoverKey)
	return &m, nil
}

// --- ndihmës ------------------------------------------------------------------------------

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	r := strings.NewReplacer("ë", "e", "ç", "c", "ä", "a", "ö", "o", "ü", "u", "ß", "ss")
	s = r.Replace(s)
	s = strings.Trim(slugRe.ReplaceAllString(s, "-"), "-")
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

func parseHM(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, m := int(s[0]-'0')*10+int(s[1]-'0'), int(s[3]-'0')*10+int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' || s[3] < '0' || s[3] > '9' || s[4] < '0' || s[4] > '9' {
		return 0, false
	}
	return h*60 + m, true
}

// OpenAt — a është hapur në momentin t sipas orareve (me kalim mesnate)?
func OpenAt(hours []Hours, t time.Time) bool {
	minute := t.Hour()*60 + t.Minute()
	today, yesterday := int(t.Weekday()), (int(t.Weekday())+6)%7
	for _, h := range hours {
		o, ok1 := parseHM(h.Opens)
		c, ok2 := parseHM(h.Closes)
		if !ok1 || !ok2 {
			continue
		}
		if c > o { // brenda ditës
			if h.Weekday == today && minute >= o && minute < c {
				return true
			}
		} else { // kalon mesnatën: [o, 24h) sot ose [0, c) nesër
			if h.Weekday == today && minute >= o {
				return true
			}
			if h.Weekday == yesterday && minute < c {
				return true
			}
		}
	}
	return false
}

func (s *Service) hours(ctx context.Context, merchantID uuid.UUID) ([]Hours, error) {
	rows, err := s.pool.Query(ctx, `SELECT weekday, to_char(opens, 'HH24:MI'), to_char(closes, 'HH24:MI') FROM merchant_hours WHERE merchant_id = $1 ORDER BY weekday, opens`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Hours{}
	for rows.Next() {
		var h Hours
		if err := rows.Scan(&h.Weekday, &h.Opens, &h.Closes); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Membership — roli i aktorit te merchant-i (owner/manager/staff) ose ErrNotMember.
func (s *Service) Membership(ctx context.Context, userID, merchantID uuid.UUID) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `SELECT role FROM merchant_staff WHERE merchant_id = $1 AND user_id = $2`, merchantID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotMember
	}
	return role, err
}

func (s *Service) requireRole(ctx context.Context, a principal.Actor, merchantID uuid.UUID, roles ...string) error {
	role, err := s.Membership(ctx, a.UserID, merchantID)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r == role {
			return nil
		}
	}
	return httpx.ErrForbidden
}

// --- aplikimi dhe profili ------------------------------------------------------------------

type ApplyInput struct {
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Phone           string    `json:"phone"`
	AddressLine1    string    `json:"address_line1"`
	City            string    `json:"city"`
	Location        geo.Point `json:"location"`
	Cuisines        []string  `json:"cuisines"`
	FulfillmentMode string    `json:"fulfillment_mode"`
	MinOrderMinor   int64     `json:"min_order_minor"`
	PrepTimeMin     int       `json:"prep_time_min"`
}

func validateApply(in *ApplyInput) map[string]string {
	f := map[string]string{}
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	if !contains(Types, in.Type) {
		f["type"] = "invalid"
	}
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if n := utf8.RuneCountInString(in.Name); n < 2 || n > 80 {
		f["name"] = "invalid"
	}
	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > 600 {
		f["description"] = "too_long"
	}
	in.AddressLine1 = strings.Join(strings.Fields(in.AddressLine1), " ")
	if n := utf8.RuneCountInString(in.AddressLine1); n < 3 || n > 120 {
		f["address_line1"] = "invalid"
	}
	in.City = strings.Join(strings.Fields(in.City), " ")
	if n := utf8.RuneCountInString(in.City); n < 2 || n > 60 {
		f["city"] = "invalid"
	}
	if !in.Location.Valid() || !geo.InKosovo(in.Location) {
		f["location"] = "outside_kosovo"
	}
	if in.FulfillmentMode == "" {
		in.FulfillmentMode = "courier"
	}
	if !contains([]string{"courier", "merchant_delivers", "pickup"}, in.FulfillmentMode) {
		f["fulfillment_mode"] = "invalid"
	}
	if in.MinOrderMinor < 0 || in.MinOrderMinor > 20000 {
		f["min_order_minor"] = "invalid"
	}
	if in.PrepTimeMin == 0 {
		in.PrepTimeMin = 20
	}
	if in.PrepTimeMin < 5 || in.PrepTimeMin > 180 {
		f["prep_time_min"] = "invalid"
	}
	seen := map[string]bool{}
	cuisines := []string{}
	for _, c := range in.Cuisines {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || seen[c] || len(cuisines) >= 6 {
			continue
		}
		seen[c] = true
		cuisines = append(cuisines, c)
	}
	in.Cuisines = cuisines
	in.Phone = strings.Join(strings.Fields(in.Phone), "")
	if in.Phone != "" && (len(in.Phone) < 8 || len(in.Phone) > 16) {
		f["phone"] = "invalid"
	}
	return f
}

// Apply — pronari aplikon; merchant-i krijohet 'pending' me pronarin si staf (owner).
func (s *Service) Apply(ctx context.Context, a principal.Actor, in ApplyInput) (*Merchant, error) {
	if f := validateApply(&in); len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}
	var areaID *string
	if area, err := s.pricing.ResolveArea(ctx, in.Location); err == nil {
		areaID = &area.ID
	}
	// slug-u zgjidhet PARA transaksionit: një përplasje brenda tij do ta anulonte gjithë transaksionin
	slug := Slugify(in.Name)
	if slug == "" {
		slug = "merchant"
	}
	for attempt := 0; attempt < 5; attempt++ {
		var taken bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM merchants WHERE slug = $1)`, slug).Scan(&taken); err != nil {
			return nil, err
		}
		if !taken {
			break
		}
		slug = Slugify(in.Name) + "-" + uuid.NewString()[:4]
	}
	var out *Merchant
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		m, err := scanMerchant(tx.QueryRow(ctx, `
			INSERT INTO merchants (owner_user_id, type, name, slug, description, phone, address_line1, city, lat, lng, service_area_id,
			  cuisines, fulfillment_mode, min_order_minor, prep_time_min)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING `+merchantCols,
			a.UserID, in.Type, in.Name, slug, nullable(in.Description), nullable(in.Phone), in.AddressLine1, in.City,
			in.Location.Lat, in.Location.Lng, areaID, in.Cuisines, in.FulfillmentMode, in.MinOrderMinor, in.PrepTimeMin))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrSlugTaken // garë e rrallë: klienti provon sërish me emër tjetër
			}
			return err
		}
		out = m
		if _, err := tx.Exec(ctx, `INSERT INTO merchant_staff (merchant_id, user_id, role) VALUES ($1, $2, 'owner')`, out.ID, a.UserID); err != nil {
			return err
		}
		if err := audit(ctx, tx, a, "merchant.applied", out.ID, map[string]any{"type": in.Type, "name": in.Name}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "merchant", out.ID.String(), "MerchantApplied", map[string]any{"merchant_id": out.ID, "owner_id": a.UserID, "type": in.Type})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Mine — merchant-ët ku aktori është staf.
func (s *Service) Mine(ctx context.Context, a principal.Actor) ([]Merchant, error) {
	// Pa JOIN: merchant_staff ka edhe ajo created_at, dhe kolonat e papërcaktuara bëheshin të
	// dykuptimta — kërkesa dështonte me 500 sapo një përdorues kishte ndonjë tregtar.
	rows, err := s.pool.Query(ctx, `SELECT `+merchantCols+` FROM merchants
		WHERE id IN (SELECT merchant_id FROM merchant_staff WHERE user_id = $1) ORDER BY created_at`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Merchant{}
	for rows.Next() {
		m, err := scanMerchant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Orari dhe "hapur tani" njësoj si te Get: paneli i partnerit e lexon nga kjo listë, dhe pa
	// to tregonte "Jashtë orarit" për një vend që ishte hapur dhe pranonte porosi.
	now := s.now()
	for k := range out {
		out[k].Hours, _ = s.hours(ctx, out[k].ID)
		out[k].OpenNow = OpenAt(out[k].Hours, now) && out[k].AcceptingOrders
	}
	return out, nil
}

type ProfileUpdate struct {
	Name             *string   `json:"name"`
	Description      *string   `json:"description"`
	Phone            *string   `json:"phone"`
	Cuisines         *[]string `json:"cuisines"`
	FulfillmentMode  *string   `json:"fulfillment_mode"`
	MinOrderMinor    *int64    `json:"min_order_minor"`
	DeliveryFeeMinor *int64    `json:"delivery_fee_minor"`
	PrepTimeMin      *int      `json:"prep_time_min"`
	AcceptingOrders  *bool     `json:"accepting_orders"`
	LogoKey          *string   `json:"logo_key"`
	CoverKey         *string   `json:"cover_key"`
}

// UpdateProfile — owner/manager; ndryshimet e adresës/lokacionit kërkojnë Operacionet (jo këtu).
func (s *Service) UpdateProfile(ctx context.Context, a principal.Actor, merchantID uuid.UUID, in ProfileUpdate) (*Merchant, error) {
	if err := s.requireRole(ctx, a, merchantID, "owner", "manager"); err != nil {
		return nil, err
	}
	f := map[string]string{}
	if in.Name != nil {
		*in.Name = strings.Join(strings.Fields(*in.Name), " ")
		if n := utf8.RuneCountInString(*in.Name); n < 2 || n > 80 {
			f["name"] = "invalid"
		}
	}
	if in.FulfillmentMode != nil && !contains([]string{"courier", "merchant_delivers", "pickup"}, *in.FulfillmentMode) {
		f["fulfillment_mode"] = "invalid"
	}
	if in.PrepTimeMin != nil && (*in.PrepTimeMin < 5 || *in.PrepTimeMin > 180) {
		f["prep_time_min"] = "invalid"
	}
	if in.MinOrderMinor != nil && (*in.MinOrderMinor < 0 || *in.MinOrderMinor > 20000) {
		f["min_order_minor"] = "invalid"
	}
	if in.DeliveryFeeMinor != nil && (*in.DeliveryFeeMinor < 0 || *in.DeliveryFeeMinor > 2000) {
		f["delivery_fee_minor"] = "invalid"
	}
	if len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}
	var out *Merchant
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		m, err := scanMerchant(tx.QueryRow(ctx, `
			UPDATE merchants SET name = COALESCE($2, name), description = COALESCE($3, description), phone = COALESCE($4, phone),
			  cuisines = COALESCE($5, cuisines), fulfillment_mode = COALESCE($6, fulfillment_mode), min_order_minor = COALESCE($7, min_order_minor),
			  delivery_fee_minor = COALESCE($8, delivery_fee_minor), prep_time_min = COALESCE($9, prep_time_min), accepting_orders = COALESCE($10, accepting_orders),
			  logo_key = COALESCE($11, logo_key), cover_key = COALESCE($12, cover_key), updated_at = now()
			WHERE id = $1 RETURNING `+merchantCols, merchantID, in.Name, in.Description, in.Phone, in.Cuisines, in.FulfillmentMode, in.MinOrderMinor,
			in.DeliveryFeeMinor, in.PrepTimeMin, in.AcceptingOrders, in.LogoKey, in.CoverKey))
		if err != nil {
			return err
		}
		out = m
		return audit(ctx, tx, a, "merchant.profile_updated", merchantID, nil)
	})
	if err != nil {
		return nil, err
	}
	out.Hours, _ = s.hours(ctx, merchantID)
	out.OpenNow = OpenAt(out.Hours, s.now())
	return out, nil
}

// SetHours — zëvendëson oraret (deri 3 intervale për ditë).
func (s *Service) SetHours(ctx context.Context, a principal.Actor, merchantID uuid.UUID, hours []Hours) ([]Hours, error) {
	if err := s.requireRole(ctx, a, merchantID, "owner", "manager"); err != nil {
		return nil, err
	}
	perDay := map[int]int{}
	for i, h := range hours {
		if h.Weekday < 0 || h.Weekday > 6 {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"hours": "weekday"})
		}
		if _, ok := parseHM(h.Opens); !ok {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"hours": "opens"})
		}
		if _, ok := parseHM(h.Closes); !ok {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"hours": "closes"})
		}
		perDay[h.Weekday]++
		if perDay[h.Weekday] > 3 {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"hours": "too_many"})
		}
		hours[i] = h
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM merchant_hours WHERE merchant_id = $1`, merchantID); err != nil {
			return err
		}
		for _, h := range hours {
			if _, err := tx.Exec(ctx, `INSERT INTO merchant_hours (merchant_id, weekday, opens, closes) VALUES ($1, $2, $3::time, $4::time) ON CONFLICT DO NOTHING`, merchantID, h.Weekday, h.Opens, h.Closes); err != nil {
				return err
			}
		}
		return audit(ctx, tx, a, "merchant.hours_updated", merchantID, map[string]any{"count": len(hours)})
	})
	if err != nil {
		return nil, err
	}
	return s.hours(ctx, merchantID)
}

// AddStaff — owner shton staf me numër telefoni (përdoruesi duhet të ketë llogari KREJT).
func (s *Service) AddStaff(ctx context.Context, a principal.Actor, merchantID uuid.UUID, phone, role string) error {
	if err := s.requireRole(ctx, a, merchantID, "owner"); err != nil {
		return err
	}
	if role != "manager" && role != "staff" {
		return httpx.ErrValidation.WithFields(map[string]string{"role": "invalid"})
	}
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE phone_e164 = $1 AND status = 'active'`, strings.Join(strings.Fields(phone), "")).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStaffMissing
	}
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO merchant_staff (merchant_id, user_id, role) VALUES ($1, $2, $3)
			ON CONFLICT (merchant_id, user_id) DO UPDATE SET role = EXCLUDED.role WHERE merchant_staff.role <> 'owner'`, merchantID, userID, role); err != nil {
			return err
		}
		return audit(ctx, tx, a, "merchant.staff_added", merchantID, map[string]any{"user_id": userID, "role": role})
	})
}

func (s *Service) RemoveStaff(ctx context.Context, a principal.Actor, merchantID, userID uuid.UUID) error {
	if err := s.requireRole(ctx, a, merchantID, "owner"); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM merchant_staff WHERE merchant_id = $1 AND user_id = $2 AND role <> 'owner'`, merchantID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.ErrNotFound
		}
		return audit(ctx, tx, a, "merchant.staff_removed", merchantID, map[string]any{"user_id": userID})
	})
}

// --- Operacionet ------------------------------------------------------------------------------

func (s *Service) Pending(ctx context.Context, limit int) ([]Merchant, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT `+merchantCols+` FROM merchants WHERE status = 'pending' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Merchant{}
	for rows.Next() {
		m, err := scanMerchant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Orari dhe "hapur tani" njësoj si te Get: paneli i partnerit e lexon nga kjo listë, dhe pa
	// to tregonte "Jashtë orarit" për një vend që ishte hapur dhe pranonte porosi.
	now := s.now()
	for k := range out {
		out[k].Hours, _ = s.hours(ctx, out[k].ID)
		out[k].OpenNow = OpenAt(out[k].Hours, now) && out[k].AcceptingOrders
	}
	return out, nil
}

// SetStatus — activate (jep MERCHANT pronarit) | pause | suspend (me arsye).
func (s *Service) SetStatus(ctx context.Context, ops principal.Actor, merchantID uuid.UUID, action, reason string) (*Merchant, error) {
	status := map[string]string{"activate": "active", "pause": "paused", "suspend": "suspended"}[action]
	if status == "" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"action": "invalid"})
	}
	if status == "suspended" && strings.TrimSpace(reason) == "" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"reason": "required"})
	}
	var out *Merchant
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		m, err := scanMerchant(tx.QueryRow(ctx, `
			UPDATE merchants SET status = $2, suspended_reason = NULLIF($3, ''), approved_at = CASE WHEN $2 = 'active' THEN COALESCE(approved_at, now()) ELSE approved_at END,
			  approved_by = CASE WHEN $2 = 'active' THEN $4 ELSE approved_by END, updated_at = now()
			WHERE id = $1 RETURNING `+merchantCols, merchantID, status, reason, ops.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = m
		var ownerID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT owner_user_id FROM merchants WHERE id = $1`, merchantID).Scan(&ownerID); err != nil {
			return err
		}
		if status == "active" {
			if _, err := tx.Exec(ctx, `INSERT INTO user_capabilities (user_id, capability, granted_by) VALUES ($1, 'MERCHANT', $2)
				ON CONFLICT (user_id, capability) DO UPDATE SET revoked_at = NULL, granted_at = now(), granted_by = EXCLUDED.granted_by`, ownerID, ops.UserID); err != nil {
				return err
			}
		}
		if err := audit(ctx, tx, ops, "merchant."+status, merchantID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "merchant", merchantID.String(), "MerchantStatusChanged", map[string]any{"merchant_id": merchantID, "owner_id": ownerID, "status": status, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- publik ---------------------------------------------------------------------------------------

type DiscoverFilter struct {
	At      *geo.Point
	Type    string
	Query   string
	Cuisine string
	Limit   int
	// UserID — kur ka sesion, çdo rresht mëson nëse është i preferuar nga ky përdorues. Bosh
	// jashtë sesionit: zbulimi mbetet publik.
	UserID uuid.UUID
}

// Discover — merchant-ët aktivë afër klientit (≤ 15 km), me distancë, hapur tani, kërkim pa theksa.
func (s *Service) Discover(ctx context.Context, f DiscoverFilter) ([]Merchant, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 30
	}
	f.Query = strings.TrimSpace(f.Query)
	var lat, lng *float64
	if f.At != nil && f.At.Valid() {
		lat, lng = &f.At.Lat, &f.At.Lng
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+merchantCols+`,
		       CASE WHEN $1::float8 IS NULL THEN NULL ELSE
		         2 * 6371000 * asin(sqrt(power(sin(radians(lat - $1) / 2), 2) + cos(radians($1)) * cos(radians(lat)) * power(sin(radians(lng - $2) / 2), 2))) END AS dist
		FROM merchants
		WHERE status = 'active'
		  AND ($3 = '' OR type = $3)
		  AND ($4 = '' OR $4 = ANY(cuisines))
		  AND ($5 = '' OR krejt_unaccent(name) ILIKE '%' || krejt_unaccent($5) || '%' OR EXISTS (
		        SELECT 1 FROM products p WHERE p.merchant_id = merchants.id AND p.deleted_at IS NULL AND p.available AND krejt_unaccent(p.name) ILIKE '%' || krejt_unaccent($5) || '%'))
		ORDER BY dist NULLS LAST, rating_count DESC, name LIMIT $6`, lat, lng, f.Type, strings.ToLower(f.Cuisine), f.Query, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Merchant{}
	now := s.now()
	for rows.Next() {
		m, err := scanDiscovered(rows)
		if err != nil {
			return nil, err
		}
		// Zbulimi e pret listën te 15 km; të preferuarat jo.
		if m.DistanceM != nil && *m.DistanceM > 15000 {
			continue
		}
		m.Hours, _ = s.hours(ctx, m.ID)
		m.OpenNow = OpenAt(m.Hours, now) && m.AcceptingOrders
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Një pyetje e vetme për gjithë listën, jo një për rresht.
	ids := make([]uuid.UUID, 0, len(out))
	for _, m := range out {
		ids = append(ids, m.ID)
	}
	fav, err := s.favouriteIDs(ctx, f.UserID, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Favourite = fav[out[i].ID]
	}
	return out, nil
}

// scanDiscovered — një rresht i `merchantCols` plus kolona `dist`. Telefoni hiqet gjithmonë:
// publikisht kontakti kalon përmes chat-it ose porosisë.
func scanDiscovered(rows pgx.Rows) (*Merchant, error) {
	var m Merchant
	var sum int
	var dist *float64
	if err := rows.Scan(&m.ID, &m.Type, &m.Name, &m.Slug, &m.Description, &m.Phone, &m.AddressLine1, &m.City, &m.Location.Lat, &m.Location.Lng,
		&m.ServiceAreaID, &m.Status, &m.Cuisines, &m.Tags, &m.FulfillmentMode, &m.MinOrderMinor, &m.DeliveryFeeMinor, &m.PrepTimeMin,
		&sum, &m.RatingCount, &m.LogoKey, &m.CoverKey, &m.AcceptingOrders, &m.CommissionBP, &m.CreatedAt, &dist); err != nil {
		return nil, err
	}
	if dist != nil {
		d := int(*dist)
		m.DistanceM = &d
	}
	if m.RatingCount > 0 {
		r := float64(int(float64(sum)/float64(m.RatingCount)*100+0.5)) / 100
		m.Rating = &r
	}
	m.Phone = nil
	return &m, nil
}

// BySlug — kërkon sipas slug-ut, ose sipas id-së kur ajo që vjen është një UUID. Porositë mbajnë
// merchant_id dhe jo slug-un; pa këtë, "porosit prapë" nuk do ta gjente dot lokalin.
func (s *Service) BySlug(ctx context.Context, slug string) (*Merchant, error) {
	key := strings.ToLower(strings.TrimSpace(slug))
	where := "slug = $1"
	if _, err := uuid.Parse(key); err == nil {
		where = "id = $1::uuid"
	}
	m, err := scanMerchant(s.pool.QueryRow(ctx, `SELECT `+merchantCols+` FROM merchants WHERE `+where+` AND status = 'active'`, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.Phone = nil
	m.Hours, _ = s.hours(ctx, m.ID)
	m.OpenNow = OpenAt(m.Hours, s.now()) && m.AcceptingOrders
	return m, nil
}

// Get — për stafin (me telefon dhe orare).
func (s *Service) Get(ctx context.Context, a principal.Actor, merchantID uuid.UUID) (*Merchant, error) {
	if _, err := s.Membership(ctx, a.UserID, merchantID); err != nil {
		return nil, err
	}
	m, err := scanMerchant(s.pool.QueryRow(ctx, `SELECT `+merchantCols+` FROM merchants WHERE id = $1`, merchantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.Hours, _ = s.hours(ctx, merchantID)
	m.OpenNow = OpenAt(m.Hours, s.now()) && m.AcceptingOrders
	return m, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func audit(ctx context.Context, tx events.Execer, a principal.Actor, action string, merchantID uuid.UUID, meta map[string]any) error {
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
		VALUES ($1, $2, 'merchant', $3, $4, $5, $6)`, a.UserID, action, merchantID.String(), ip, reqID, metaJSON)
	return err
}
