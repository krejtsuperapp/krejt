// Package pricing — çmimi server-side i udhëtimeve (§18, §51): tarifa për zonë × kategori,
// çmim i paracaktuar (upfront) nga distanca/koha e MapProvider-it, oferta (quote) me afat.
// Klienti nuk dërgon kurrë çmim: dërgon quote_id. Para vetëm si numra të plotë në cent.
package pricing

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/providers/maps"
)

var (
	ErrOutsideServiceArea = &httpx.APIError{Code: "OUTSIDE_SERVICE_AREA", MessageKey: "errors.rides.outside_service_area", HTTPStatus: http.StatusUnprocessableEntity}
	ErrQuoteExpired       = &httpx.APIError{Code: "QUOTE_EXPIRED", MessageKey: "errors.rides.quote_expired", HTTPStatus: http.StatusUnprocessableEntity}
	ErrNoPricing          = &httpx.APIError{Code: "NO_PRICING", MessageKey: "errors.rides.no_pricing", HTTPStatus: http.StatusServiceUnavailable, Retryable: true}
)

// QuoteTTL — sa kohë vlen një ofertë çmimi.
const QuoteTTL = 2 * time.Minute

type Rule struct {
	ID                       uuid.UUID
	AreaID                   string
	CategoryID               string
	Currency                 string
	BaseMinor                int64
	PerKmMinor               int64
	PerMinMinor              int64
	MinimumMinor             int64
	CancellationFeeMinor     int64
	CancellationGraceSeconds int
	SurgeBP                  int
	CommissionBP             int
	Seats                    int
	Sort                     int
}

// Compute — çmimi i paracaktuar në cent, vetëm me numra të plotë:
// (bazë + km×per_km + min×per_min) × surge, rrumbullakim lart në 10 cent, jo nën minimum.
func Compute(r Rule, distanceM, durationS int) int64 {
	if distanceM < 0 {
		distanceM = 0
	}
	if durationS < 0 {
		durationS = 0
	}
	km := (r.PerKmMinor*int64(distanceM) + 500) / 1000
	mins := (r.PerMinMinor*int64(durationS) + 30) / 60
	p := r.BaseMinor + km + mins
	surge := r.SurgeBP
	if surge < 10000 {
		surge = 10000
	}
	p = (p*int64(surge) + 5000) / 10000
	p = (p + 9) / 10 * 10
	if p < r.MinimumMinor {
		p = r.MinimumMinor
	}
	return p
}

// Commission — pjesa e platformës (pikë bazë) nga çmimi, rrumbullakim gjysmë-lart.
func Commission(priceMinor int64, commissionBP int) int64 {
	if commissionBP < 0 {
		return 0
	}
	return (priceMinor*int64(commissionBP) + 5000) / 10000
}

type Area struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Center   geo.Point `json:"center"`
	RadiusKm float64   `json:"radius_km"`
}

// Nearby — ETA e shoferit më të afërt (nga moduli location); opsionale në ofertë.
type Nearby interface {
	NearestETA(ctx context.Context, category string, p geo.Point) (etaSeconds int, found bool, err error)
}

type Service struct {
	pool   *pgxpool.Pool
	maps   maps.Provider
	nearby Nearby
}

func New(pool *pgxpool.Pool, m maps.Provider, nearby Nearby) *Service {
	return &Service{pool: pool, maps: m, nearby: nearby}
}

// ResolveArea — zona aktive që mbulon pikën (më e afërta nëse mbivendosen).
func (s *Service) ResolveArea(ctx context.Context, p geo.Point) (*Area, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, center_lat, center_lng, radius_km FROM service_areas WHERE active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var best *Area
	bestD := 0.0
	for rows.Next() {
		var a Area
		if err := rows.Scan(&a.ID, &a.Name, &a.Center.Lat, &a.Center.Lng, &a.RadiusKm); err != nil {
			return nil, err
		}
		d := geo.Haversine(a.Center, p)
		if d <= a.RadiusKm*1000 && (best == nil || d < bestD) {
			cp := a
			best, bestD = &cp, d
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if best == nil {
		return nil, ErrOutsideServiceArea
	}
	return best, nil
}

// Rules — rregulli në fuqi për çdo kategori aktive të zonës, sipas renditjes së kategorive.
func (s *Service) Rules(ctx context.Context, areaID string) ([]Rule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (r.category_id)
		       r.id, r.service_area_id, r.category_id, r.currency, r.base_minor, r.per_km_minor, r.per_min_minor,
		       r.minimum_minor, r.cancellation_fee_minor, r.cancellation_grace_seconds, r.surge_bp, r.commission_bp,
		       c.seats, c.sort
		FROM pricing_rules r JOIN ride_categories c ON c.id = r.category_id
		WHERE r.service_area_id = $1 AND c.active
		  AND r.valid_from <= now() AND (r.valid_to IS NULL OR r.valid_to > now())
		ORDER BY r.category_id, r.valid_from DESC`, areaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.AreaID, &r.CategoryID, &r.Currency, &r.BaseMinor, &r.PerKmMinor, &r.PerMinMinor,
			&r.MinimumMinor, &r.CancellationFeeMinor, &r.CancellationGraceSeconds, &r.SurgeBP, &r.CommissionBP,
			&r.Seats, &r.Sort); err != nil {
			return nil, err
		}
		r.Currency = strings.TrimSpace(r.Currency)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sort < out[j].Sort })
	return out, nil
}

type QuoteInput struct {
	Pickup         geo.Point `json:"pickup"`
	Dropoff        geo.Point `json:"dropoff"`
	PickupAddress  string    `json:"pickup_address"`
	DropoffAddress string    `json:"dropoff_address"`
}

type Quote struct {
	ID         uuid.UUID `json:"id"`
	CategoryID string    `json:"category"`
	Seats      int       `json:"seats"`
	PriceMinor int64     `json:"price_minor"`
	Currency   string    `json:"currency"`
	SurgeBP    int       `json:"surge_bp"`
	PickupETAS *int      `json:"pickup_eta_s"` // null kur s'ka shofer afër
	ExpiresAt  time.Time `json:"expires_at"`
}

type QuoteResult struct {
	Area      Area    `json:"area"`
	DistanceM int     `json:"distance_m"`
	DurationS int     `json:"duration_s"`
	Quotes    []Quote `json:"quotes"`
}

// Quote — ofertat e çmimit për të gjitha kategoritë (një rresht ride_quotes për kategori).
func (s *Service) Quote(ctx context.Context, customerID uuid.UUID, in QuoteInput) (*QuoteResult, error) {
	fields := map[string]string{}
	if !in.Pickup.Valid() || !geo.InKosovo(in.Pickup) {
		fields["pickup"] = "invalid"
	}
	if !in.Dropoff.Valid() || !geo.InKosovo(in.Dropoff) {
		fields["dropoff"] = "invalid"
	}
	if geo.Haversine(in.Pickup, in.Dropoff) < 50 {
		fields["dropoff"] = "same_as_pickup"
	}
	in.PickupAddress = clip(in.PickupAddress, 200)
	in.DropoffAddress = clip(in.DropoffAddress, 200)
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	area, err := s.ResolveArea(ctx, in.Pickup)
	if err != nil {
		return nil, err
	}
	route, err := s.maps.Route(ctx, in.Pickup, in.Dropoff)
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	rules, err := s.Rules(ctx, area.ID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, ErrNoPricing
	}
	expires := time.Now().Add(QuoteTTL)
	res := &QuoteResult{Area: *area, DistanceM: route.DistanceM, DurationS: route.DurationS}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, r := range rules {
			q := Quote{CategoryID: r.CategoryID, Seats: r.Seats, PriceMinor: Compute(r, route.DistanceM, route.DurationS),
				Currency: r.Currency, SurgeBP: r.SurgeBP, ExpiresAt: expires}
			if err := tx.QueryRow(ctx, `
				INSERT INTO ride_quotes (customer_id, service_area_id, category_id, pricing_rule_id,
				  pickup_lat, pickup_lng, pickup_address, dropoff_lat, dropoff_lng, dropoff_address,
				  distance_m, duration_s, price_minor, currency, surge_bp, expires_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
				customerID, area.ID, r.CategoryID, r.ID,
				in.Pickup.Lat, in.Pickup.Lng, nullable(in.PickupAddress), in.Dropoff.Lat, in.Dropoff.Lng, nullable(in.DropoffAddress),
				route.DistanceM, route.DurationS, q.PriceMinor, r.Currency, r.SurgeBP, expires).Scan(&q.ID); err != nil {
				return err
			}
			if s.nearby != nil {
				if eta, ok, err := s.nearby.NearestETA(ctx, r.CategoryID, in.Pickup); err == nil && ok {
					q.PickupETAS = &eta
				}
			}
			res.Quotes = append(res.Quotes, q)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// StoredQuote — oferta e ruajtur, me rregullin e saj (për komision/anulim).
type StoredQuote struct {
	ID                       uuid.UUID
	CustomerID               uuid.UUID
	AreaID                   string
	CategoryID               string
	RuleID                   uuid.UUID
	Pickup                   geo.Point
	Dropoff                  geo.Point
	PickupAddress            *string
	DropoffAddress           *string
	DistanceM                int
	DurationS                int
	PriceMinor               int64
	Currency                 string
	SurgeBP                  int
	ExpiresAt                time.Time
	CommissionBP             int
	CancellationFeeMinor     int64
	CancellationGraceSeconds int
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LoadQuote — oferta e klientit; NOT_FOUND nëse s'është e tija, QUOTE_EXPIRED nëse ka skaduar.
func LoadQuote(ctx context.Context, q rowQuerier, quoteID, customerID uuid.UUID) (*StoredQuote, error) {
	var sq StoredQuote
	err := q.QueryRow(ctx, `
		SELECT q.id, q.customer_id, q.service_area_id, q.category_id, q.pricing_rule_id,
		       q.pickup_lat, q.pickup_lng, q.pickup_address, q.dropoff_lat, q.dropoff_lng, q.dropoff_address,
		       q.distance_m, q.duration_s, q.price_minor, q.currency, q.surge_bp, q.expires_at,
		       r.commission_bp, r.cancellation_fee_minor, r.cancellation_grace_seconds
		FROM ride_quotes q JOIN pricing_rules r ON r.id = q.pricing_rule_id
		WHERE q.id = $1 AND q.customer_id = $2`, quoteID, customerID).
		Scan(&sq.ID, &sq.CustomerID, &sq.AreaID, &sq.CategoryID, &sq.RuleID,
			&sq.Pickup.Lat, &sq.Pickup.Lng, &sq.PickupAddress, &sq.Dropoff.Lat, &sq.Dropoff.Lng, &sq.DropoffAddress,
			&sq.DistanceM, &sq.DurationS, &sq.PriceMinor, &sq.Currency, &sq.SurgeBP, &sq.ExpiresAt,
			&sq.CommissionBP, &sq.CancellationFeeMinor, &sq.CancellationGraceSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sq.Currency = strings.TrimSpace(sq.Currency)
	if time.Now().After(sq.ExpiresAt) {
		return nil, ErrQuoteExpired
	}
	return &sq, nil
}

func clip(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = string(r[:max])
	}
	return s
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
