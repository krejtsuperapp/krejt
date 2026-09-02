// Package admin — leximet e Admin-it (§35): përdoruesit (kërkim, detaje), udhëtimet (listë, detaje me
// ngjarjet/ofertat), audit log, dispatch live (udhëtimet aktive + shoferët online nga Redis).
// Veprimet me peshë jetojnë në modulet e tyre (drivers, documents, support, payments, appconfig);
// këtu vetëm lexim + audit i qasjes në të dhëna personale.
package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

type Service struct {
	pool   *pgxpool.Pool
	rdb    redis.UniversalClient
	ledger *ledger.Service
}

func New(pool *pgxpool.Pool, rdb redis.UniversalClient, led *ledger.Service) *Service {
	return &Service{pool: pool, rdb: rdb, ledger: led}
}

// --- përdoruesit ------------------------------------------------------------------------

type UserRow struct {
	ID           uuid.UUID `json:"id"`
	Phone        *string   `json:"phone"`
	Email        *string   `json:"email"`
	FullName     *string   `json:"full_name"`
	Locale       string    `json:"locale"`
	Status       string    `json:"status"`
	Capabilities []string  `json:"capabilities"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Service) SearchUsers(ctx context.Context, q string, limit int) ([]UserRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.phone_e164, u.email, u.full_name, u.locale, u.status, u.created_at,
		       COALESCE((SELECT array_agg(capability ORDER BY capability) FROM user_capabilities c WHERE c.user_id = u.id AND c.revoked_at IS NULL), '{}')
		FROM users u
		WHERE $1 = '' OR u.phone_e164 LIKE $1 || '%' OR u.email ILIKE '%' || $1 || '%' OR unaccent(COALESCE(u.full_name,'')) ILIKE '%' || unaccent($1) || '%'
		   OR u.id::text = $1
		ORDER BY u.created_at DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserRow{}
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Phone, &u.Email, &u.FullName, &u.Locale, &u.Status, &u.CreatedAt, &u.Capabilities); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type UserDetail struct {
	UserRow
	WalletBalanceMinor int64        `json:"wallet_balance_minor"`
	RidesTotal         int          `json:"rides_total"`
	RidesCompleted     int          `json:"rides_completed"`
	ActiveSessions     int          `json:"active_sessions"`
	Driver             *DriverBrief `json:"driver,omitempty"`
	RecentAudit        []AuditRow   `json:"recent_audit"`
}

type DriverBrief struct {
	Status       string   `json:"status"`
	Categories   []string `json:"categories"`
	VehiclePlate string   `json:"vehicle_plate"`
	Rating       *float64 `json:"rating"`
}

func (s *Service) User(ctx context.Context, viewer principal.Actor, id uuid.UUID) (*UserDetail, error) {
	rows, err := s.SearchUsers(ctx, id.String(), 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, httpx.ErrNotFound
	}
	d := &UserDetail{UserRow: rows[0], RecentAudit: []AuditRow{}}
	if bal, err := s.ledger.Balance(ctx, ledger.UserWalletCode(id)); err == nil {
		d.WalletBalanceMinor = int64(bal.Minor)
	} else if !errors.Is(err, ledger.ErrAccountMissing) {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE state = 'completed') FROM rides WHERE customer_id = $1 OR driver_id = $1`, id).Scan(&d.RidesTotal, &d.RidesCompleted); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL AND refresh_expires_at > now()`, id).Scan(&d.ActiveSessions); err != nil {
		return nil, err
	}
	var db DriverBrief
	var sum, cnt int
	err = s.pool.QueryRow(ctx, `SELECT status, categories, vehicle_plate, rating_sum, rating_count FROM drivers WHERE user_id = $1`, id).Scan(&db.Status, &db.Categories, &db.VehiclePlate, &sum, &cnt)
	if err == nil {
		if cnt > 0 {
			r := float64(int(float64(sum)/float64(cnt)*100+0.5)) / 100
			db.Rating = &r
		}
		d.Driver = &db
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if d.RecentAudit, err = s.Audit(ctx, AuditFilter{ActorID: &id, Limit: 20}); err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id) VALUES ($1, 'admin.user_viewed', 'user', $2)`, viewer.UserID, id.String())
	return d, nil
}

// --- udhëtimet ---------------------------------------------------------------------------

type RideRow struct {
	ID               uuid.UUID  `json:"id"`
	State            string     `json:"state"`
	CategoryID       string     `json:"category"`
	CustomerID       uuid.UUID  `json:"customer_id"`
	DriverID         *uuid.UUID `json:"driver_id"`
	PaymentMethod    string     `json:"payment_method"`
	PaymentStatus    string     `json:"payment_status"`
	PriceQuotedMinor int64      `json:"price_quoted_minor"`
	PriceFinalMinor  *int64     `json:"price_final_minor"`
	PickupAddress    *string    `json:"pickup_address"`
	DropoffAddress   *string    `json:"dropoff_address"`
	RequestedAt      time.Time  `json:"requested_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	CancelledBy      *string    `json:"cancelled_by"`
}

type RideFilter struct {
	State      string
	CustomerID *uuid.UUID
	DriverID   *uuid.UUID
	Before     *time.Time
	Limit      int
}

const rideRowCols = `id, state, category_id, customer_id, driver_id, payment_method, payment_status, price_quoted_minor, price_final_minor,
	pickup_address, dropoff_address, requested_at, completed_at, cancelled_by`

func scanRideRow(row pgx.Row) (*RideRow, error) {
	var r RideRow
	if err := row.Scan(&r.ID, &r.State, &r.CategoryID, &r.CustomerID, &r.DriverID, &r.PaymentMethod, &r.PaymentStatus, &r.PriceQuotedMinor, &r.PriceFinalMinor,
		&r.PickupAddress, &r.DropoffAddress, &r.RequestedAt, &r.CompletedAt, &r.CancelledBy); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Service) Rides(ctx context.Context, f RideFilter) ([]RideRow, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	if f.Before == nil {
		t := time.Now().Add(time.Hour)
		f.Before = &t
	}
	rows, err := s.pool.Query(ctx, `SELECT `+rideRowCols+` FROM rides
		WHERE ($1 = '' OR state = $1) AND ($2::uuid IS NULL OR customer_id = $2) AND ($3::uuid IS NULL OR driver_id = $3) AND requested_at < $4
		ORDER BY requested_at DESC LIMIT $5`, f.State, f.CustomerID, f.DriverID, *f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RideRow{}
	for rows.Next() {
		r, err := scanRideRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

type RideEvent struct {
	FromState *string        `json:"from_state"`
	ToState   string         `json:"to_state"`
	ActorType string         `json:"actor_type"`
	ActorID   *uuid.UUID     `json:"actor_id"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

type OfferRow struct {
	DriverID    uuid.UUID  `json:"driver_id"`
	Round       int        `json:"round"`
	State       string     `json:"state"`
	DistanceM   int        `json:"distance_m"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RespondedAt *time.Time `json:"responded_at"`
}

type RideDetail struct {
	RideRow
	Events []RideEvent `json:"events"`
	Offers []OfferRow  `json:"offers"`
}

func (s *Service) Ride(ctx context.Context, id uuid.UUID) (*RideDetail, error) {
	r, err := scanRideRow(s.pool.QueryRow(ctx, `SELECT `+rideRowCols+` FROM rides WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d := &RideDetail{RideRow: *r, Events: []RideEvent{}, Offers: []OfferRow{}}
	rows, err := s.pool.Query(ctx, `SELECT from_state, to_state, actor_type, actor_id, metadata, created_at FROM ride_events WHERE ride_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e RideEvent
		if err := rows.Scan(&e.FromState, &e.ToState, &e.ActorType, &e.ActorID, &e.Metadata, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		d.Events = append(d.Events, e)
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT driver_id, round, state, distance_m, expires_at, responded_at FROM ride_offers WHERE ride_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var o OfferRow
		if err := rows.Scan(&o.DriverID, &o.Round, &o.State, &o.DistanceM, &o.ExpiresAt, &o.RespondedAt); err != nil {
			return nil, err
		}
		d.Offers = append(d.Offers, o)
	}
	return d, rows.Err()
}

// --- audit ---------------------------------------------------------------------------------

type AuditRow struct {
	ID         int64          `json:"id"`
	ActorID    *uuid.UUID     `json:"actor_id"`
	ActorType  string         `json:"actor_type"`
	Action     string         `json:"action"`
	TargetType *string        `json:"target_type"`
	TargetID   *string        `json:"target_id"`
	IP         *string        `json:"ip"`
	RequestID  *string        `json:"request_id"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

type AuditFilter struct {
	ActorID  *uuid.UUID
	TargetID string
	Action   string
	Before   *time.Time
	Limit    int
}

func (s *Service) Audit(ctx context.Context, f AuditFilter) ([]AuditRow, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Before == nil {
		t := time.Now().Add(time.Hour)
		f.Before = &t
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, actor_type, action, target_type, target_id, host(ip), request_id, metadata, created_at FROM audit_log
		WHERE ($1::uuid IS NULL OR actor_id = $1) AND ($2 = '' OR target_id = $2) AND ($3 = '' OR action LIKE $3 || '%') AND created_at < $4
		ORDER BY created_at DESC LIMIT $5`, f.ActorID, f.TargetID, f.Action, *f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditRow{}
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorType, &a.Action, &a.TargetType, &a.TargetID, &a.IP, &a.RequestID, &a.Metadata, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- dispatch live -------------------------------------------------------------------------

type Live struct {
	Rides         []RideRow      `json:"rides"`          // aktive (matching/assigned/arrived/in_progress)
	Counts        map[string]int `json:"counts"`         // sipas gjendjes
	OnlineDrivers map[string]int `json:"online_drivers"` // sipas kategorisë (Redis GEO)
	OpenOffers    int            `json:"open_offers"`
	SafetyOpen    int            `json:"safety_open"`
	GeneratedAt   time.Time      `json:"generated_at"`
}

func (s *Service) DispatchLive(ctx context.Context) (*Live, error) {
	l := &Live{Rides: []RideRow{}, Counts: map[string]int{}, OnlineDrivers: map[string]int{}, GeneratedAt: time.Now()}
	rows, err := s.pool.Query(ctx, `SELECT `+rideRowCols+` FROM rides WHERE state IN ('matching','assigned','arrived','in_progress') ORDER BY requested_at LIMIT 200`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		r, err := scanRideRow(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		l.Rides = append(l.Rides, *r)
		l.Counts[r.State]++
	}
	rows.Close()
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM ride_offers WHERE state = 'offered' AND expires_at > now()`).Scan(&l.OpenOffers); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM safety_reports WHERE status <> 'closed'`).Scan(&l.SafetyOpen); err != nil {
		return nil, err
	}
	for _, cat := range []string{"economy", "comfort", "xl", "taxi"} {
		n, err := s.rdb.ZCard(ctx, "geo:drivers:"+cat).Result()
		if err == nil {
			l.OnlineDrivers[cat] = int(n)
		}
	}
	return l, nil
}

// --- HTTP ------------------------------------------------------------------------------------

// Routes — requireStaff: ADMIN | SUPPORT | OPERATIONS | FINANCE (SUPER_ADMIN kalon gjithmonë); audit: ADMIN vetëm.
func (s *Service) Routes(mux *http.ServeMux, requireStaff, requireAdmin httpx.Middleware) {
	mux.Handle("GET /api/v1/admin/users", requireStaff(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := s.SearchUsers(r.Context(), r.URL.Query().Get("q"), limit)
		respond(w, r, map[string]any{"items": items}, err)
	})))
	mux.Handle("GET /api/v1/admin/users/{id}", requireStaff(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		d, err := s.User(r.Context(), a, id)
		respond(w, r, d, err)
	})))
	mux.Handle("GET /api/v1/admin/rides", requireStaff(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		q := r.URL.Query()
		f := RideFilter{State: q.Get("state")}
		if id, err := uuid.Parse(q.Get("customer_id")); err == nil {
			f.CustomerID = &id
		}
		if id, err := uuid.Parse(q.Get("driver_id")); err == nil {
			f.DriverID = &id
		}
		if t, err := time.Parse(time.RFC3339, q.Get("before")); err == nil {
			f.Before = &t
		}
		f.Limit, _ = strconv.Atoi(q.Get("limit"))
		items, err := s.Rides(r.Context(), f)
		respond(w, r, map[string]any{"items": items}, err)
	})))
	mux.Handle("GET /api/v1/admin/rides/{id}", requireStaff(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound)
			return
		}
		d, err := s.Ride(r.Context(), id)
		respond(w, r, d, err)
	})))
	mux.Handle("GET /api/v1/admin/dispatch/live", requireStaff(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		l, err := s.DispatchLive(r.Context())
		respond(w, r, l, err)
	})))
	mux.Handle("GET /api/v1/admin/audit", requireAdmin(principal.Handler(func(w http.ResponseWriter, r *http.Request, _ principal.Actor) {
		q := r.URL.Query()
		f := AuditFilter{TargetID: q.Get("target_id"), Action: q.Get("action")}
		if id, err := uuid.Parse(q.Get("actor_id")); err == nil {
			f.ActorID = &id
		}
		if t, err := time.Parse(time.RFC3339, q.Get("before")); err == nil {
			f.Before = &t
		}
		f.Limit, _ = strconv.Atoi(q.Get("limit"))
		items, err := s.Audit(r.Context(), f)
		respond(w, r, map[string]any{"items": items}, err)
	})))
}

func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}
