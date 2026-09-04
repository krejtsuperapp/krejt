// Package location — lokacioni i gjallë i shoferëve (§27): Redis GEO për kërkim, hash për gjendjen
// (available/busy, koha e fundit), TTL/heartbeat për shoferët e heshtur, mostra të renditura sipas
// kohës (dublikatat dhe ato jashtë rendit hidhen). Në PostgreSQL ruhen VETËM mostra gjatë udhëtimit,
// ~çdo 30 s — kurrë çdo GPS update.
package location

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
)

var ErrDriverOffline = &httpx.APIError{Code: "DRIVER_OFFLINE", MessageKey: "errors.location.driver_offline", HTTPStatus: http.StatusConflict}

const (
	StaleAfter   = 60 * time.Second // pa mostër për kaq kohë → nuk merr oferta
	hashTTL      = 3 * time.Minute  // pa heartbeat fare → hiqet vetë
	PersistEvery = 30 * time.Second // persistencë selektive gjatë udhëtimit
	MaxBatch     = 50
	urbanSpeed   = 25000.0 / 3600.0 // m/s (25 km/h) për ETA pa rrugë
	detourFactor = 1.3
)

type Sample struct {
	Lat          float64  `json:"lat"`
	Lng          float64  `json:"lng"`
	Heading      *float64 `json:"heading"`
	SpeedMPS     *float64 `json:"speed_mps"`
	RecordedAtMs int64    `json:"ts"` // koha e pajisjes, ms Unix
}

type DriverState struct {
	DriverID   uuid.UUID `json:"driver_id"`
	Status     string    `json:"status"` // available | busy
	Categories []string  `json:"categories"`
	Point      geo.Point `json:"point"`
	RecordedAt time.Time `json:"recorded_at"`
	RideID     string    `json:"ride_id,omitempty"`
}

type Candidate struct {
	DriverID   uuid.UUID
	DistanceM  float64
	Point      geo.Point
	RecordedAt time.Time
}

// Realtime — kanali i gjallë (§42): pozicioni i shoferit gjatë udhëtimit shkon te klienti pa polling.
type Realtime interface {
	Publish(ctx context.Context, channel string, data any) error
}

type Service struct {
	rdb      redis.UniversalClient
	pool     *pgxpool.Pool
	now      func() time.Time
	realtime Realtime
}

func New(rdb redis.UniversalClient, pool *pgxpool.Pool) *Service {
	return &Service{rdb: rdb, pool: pool, now: time.Now}
}

// WithRealtime — aktivizon publikimin e pozicionit në kanalin `ride:{id}` gjatë udhëtimit.
func (s *Service) WithRealtime(r Realtime) *Service {
	s.realtime = r
	return s
}

func geoKey(category string) string { return "geo:drivers:" + category }
func drvKey(id uuid.UUID) string    { return "driver:" + id.String() }

// SetOnline — shoferi hyn në punë me kategoritë e lejuara; në GEO futet me mostrën e parë.
func (s *Service) SetOnline(ctx context.Context, driverID uuid.UUID, categories []string) error {
	old, _ := s.rdb.HGet(ctx, drvKey(driverID), "cats").Result()
	pipe := s.rdb.Pipeline()
	for _, c := range splitCats(old) { // kategori që s'i ka më
		pipe.ZRem(ctx, geoKey(c), driverID.String())
	}
	pipe.HSet(ctx, drvKey(driverID), "status", "available", "cats", strings.Join(categories, ","))
	pipe.HDel(ctx, drvKey(driverID), "ride_id")
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Service) SetOffline(ctx context.Context, driverID uuid.UUID) error {
	cats, _ := s.rdb.HGet(ctx, drvKey(driverID), "cats").Result()
	pipe := s.rdb.Pipeline()
	for _, c := range splitCats(cats) {
		pipe.ZRem(ctx, geoKey(c), driverID.String())
	}
	pipe.Del(ctx, drvKey(driverID))
	_, err := pipe.Exec(ctx)
	return err
}

// Ingest — grup mostrash nga pajisja; pranohen vetëm më të rejat se e fundit e njohur (rendit, dublikata).
func (s *Service) Ingest(ctx context.Context, driverID uuid.UUID, samples []Sample) (accepted int, err error) {
	if len(samples) == 0 {
		return 0, httpx.ErrValidation.WithFields(map[string]string{"samples": "empty"})
	}
	if len(samples) > MaxBatch {
		return 0, httpx.ErrValidation.WithFields(map[string]string{"samples": "too_many"})
	}
	h, err := s.rdb.HGetAll(ctx, drvKey(driverID)).Result()
	if err != nil {
		return 0, err
	}
	if h["status"] == "" {
		return 0, ErrDriverOffline
	}
	lastMs, _ := strconv.ParseInt(h["ts"], 10, 64)
	nowMs := s.now().UnixMilli()

	sort.Slice(samples, func(i, j int) bool { return samples[i].RecordedAtMs < samples[j].RecordedAtMs })
	var latest *Sample
	for i := range samples {
		sm := samples[i]
		p := geo.Point{Lat: sm.Lat, Lng: sm.Lng}
		if !p.Valid() || !geo.InKosovo(p) || sm.RecordedAtMs <= lastMs || sm.RecordedAtMs > nowMs+60_000 {
			continue
		}
		lastMs = sm.RecordedAtMs
		latest = &samples[i]
		accepted++
	}
	if latest == nil {
		return 0, nil
	}

	pipe := s.rdb.Pipeline()
	for _, c := range splitCats(h["cats"]) {
		pipe.GeoAdd(ctx, geoKey(c), &redis.GeoLocation{Name: driverID.String(), Longitude: latest.Lng, Latitude: latest.Lat})
	}
	fields := []any{"lat", latest.Lat, "lng", latest.Lng, "ts", latest.RecordedAtMs}
	if latest.Heading != nil {
		fields = append(fields, "heading", *latest.Heading)
	}
	if latest.SpeedMPS != nil {
		fields = append(fields, "speed", *latest.SpeedMPS)
	}
	pipe.HSet(ctx, drvKey(driverID), fields...)
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}

	// gjatë udhëtimit: pozicioni te klienti në kohë reale (best-effort; gabimi s'e ndal marrjen)
	if rideID := h["ride_id"]; rideID != "" && s.realtime != nil {
		pos := map[string]any{"type": "driver_location", "ride_id": rideID, "lat": latest.Lat, "lng": latest.Lng, "ts": latest.RecordedAtMs}
		if latest.Heading != nil {
			pos["heading"] = *latest.Heading
		}
		if latest.SpeedMPS != nil {
			pos["speed_mps"] = *latest.SpeedMPS
		}
		_ = s.realtime.Publish(ctx, "ride:"+rideID, pos)
	}

	// gjatë dorëzimit: pozicioni i korrierit te klienti i porosisë (best-effort, pa persistencë)
	if orderID := h["order_id"]; orderID != "" && s.realtime != nil {
		pos := map[string]any{"type": "courier_location", "order_id": orderID, "lat": latest.Lat, "lng": latest.Lng, "ts": latest.RecordedAtMs}
		if latest.Heading != nil {
			pos["heading"] = *latest.Heading
		}
		_ = s.realtime.Publish(ctx, "order:"+orderID, pos)
	}

	if parcelID := h["parcel_id"]; parcelID != "" && s.realtime != nil {
		pos := map[string]any{"type": "courier_location", "parcel_id": parcelID, "lat": latest.Lat, "lng": latest.Lng, "ts": latest.RecordedAtMs}
		_ = s.realtime.Publish(ctx, "parcel:"+parcelID, pos)
	}

	// persistencë selektive: vetëm gjatë udhëtimit, ~çdo 30 s
	if rideID := h["ride_id"]; rideID != "" {
		persistedMs, _ := strconv.ParseInt(h["persisted_ts"], 10, 64)
		if latest.RecordedAtMs-persistedMs >= PersistEvery.Milliseconds() {
			rid, err := uuid.Parse(rideID)
			if err == nil {
				if _, err := s.pool.Exec(ctx, `
					INSERT INTO driver_location_samples (driver_id, ride_id, lat, lng, heading, speed_mps, recorded_at)
					VALUES ($1, $2, $3, $4, $5, $6, to_timestamp($7::double precision / 1000))`,
					driverID, rid, latest.Lat, latest.Lng, latest.Heading, latest.SpeedMPS, latest.RecordedAtMs); err != nil {
					return accepted, err
				}
				_ = s.rdb.HSet(ctx, drvKey(driverID), "persisted_ts", latest.RecordedAtMs).Err()
			}
		}
	}
	return accepted, nil
}

// Nearest — shoferët e disponueshëm (jo busy, jo të ndenjur) të kategorisë, sipas distancës.
func (s *Service) Nearest(ctx context.Context, category string, p geo.Point, radiusKm float64, limit int) ([]Candidate, error) {
	if limit <= 0 {
		limit = 10
	}
	locs, err := s.rdb.GeoSearchLocation(ctx, geoKey(category), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{Longitude: p.Lng, Latitude: p.Lat, Radius: radiusKm, RadiusUnit: "km", Sort: "ASC", Count: limit * 3},
		WithCoord:      true,
		WithDist:       true,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	out := make([]Candidate, 0, limit)
	cutoff := s.now().Add(-StaleAfter).UnixMilli()
	for _, l := range locs {
		id, err := uuid.Parse(l.Name)
		if err != nil {
			continue
		}
		vals, err := s.rdb.HMGet(ctx, drvKey(id), "status", "ts").Result()
		if err != nil {
			return nil, err
		}
		status, _ := vals[0].(string)
		tsStr, _ := vals[1].(string)
		ts, _ := strconv.ParseInt(tsStr, 10, 64)
		if status == "" || ts < cutoff {
			// i ndenjur ose i dalë: pastrohet nga GEO që të mos kthehet më
			_ = s.rdb.ZRem(ctx, geoKey(category), l.Name).Err()
			continue
		}
		if status != "available" {
			continue
		}
		out = append(out, Candidate{DriverID: id, DistanceM: l.Dist * 1000, Point: geo.Point{Lat: l.Latitude, Lng: l.Longitude}, RecordedAt: time.UnixMilli(ts)})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// NearestETA — ETA e shoferit më të afërt (vlerësim urban pa rrugë), për ofertën e çmimit.
func (s *Service) NearestETA(ctx context.Context, category string, p geo.Point) (int, bool, error) {
	c, err := s.Nearest(ctx, category, p, 8, 1)
	if err != nil || len(c) == 0 {
		return 0, false, err
	}
	eta := int(c[0].DistanceM * detourFactor / urbanSpeed)
	if eta < 60 {
		eta = 60
	}
	return eta, true, nil
}

// SetBusy / SetAvailable — gjendja e punës (udhëtim aktiv apo jo).
func (s *Service) SetBusy(ctx context.Context, driverID uuid.UUID, rideID uuid.UUID) error {
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, drvKey(driverID), "status", "busy", "ride_id", rideID.String())
	pipe.HDel(ctx, drvKey(driverID), "persisted_ts")
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// SetBusyOrder — korrieri mori një porosi: pozicioni i tij shkon te kanali `order:{id}` (jo te rides).
func (s *Service) SetBusyOrder(ctx context.Context, driverID uuid.UUID, orderID uuid.UUID) error {
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, drvKey(driverID), "status", "busy", "order_id", orderID.String())
	pipe.HDel(ctx, drvKey(driverID), "ride_id")
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// SetBusyParcel — korrieri mori një pako: pozicioni shkon te kanali `parcel:{id}`.
func (s *Service) SetBusyParcel(ctx context.Context, driverID uuid.UUID, parcelID uuid.UUID) error {
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, drvKey(driverID), "status", "busy", "parcel_id", parcelID.String())
	pipe.HDel(ctx, drvKey(driverID), "ride_id", "order_id")
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Service) SetAvailable(ctx context.Context, driverID uuid.UUID) error {
	exists, err := s.rdb.Exists(ctx, drvKey(driverID)).Result()
	if err != nil || exists == 0 {
		return err // offline: asgjë për të bërë
	}
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, drvKey(driverID), "status", "available")
	pipe.HDel(ctx, drvKey(driverID), "ride_id", "order_id", "parcel_id", "persisted_ts")
	pipe.Expire(ctx, drvKey(driverID), hashTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// State — gjendja e gjallë e shoferit; nil kur është offline.
func (s *Service) State(ctx context.Context, driverID uuid.UUID) (*DriverState, error) {
	h, err := s.rdb.HGetAll(ctx, drvKey(driverID)).Result()
	if err != nil {
		return nil, err
	}
	if h["status"] == "" {
		return nil, nil
	}
	st := &DriverState{DriverID: driverID, Status: h["status"], Categories: splitCats(h["cats"]), RideID: h["ride_id"]}
	st.Point.Lat, _ = strconv.ParseFloat(h["lat"], 64)
	st.Point.Lng, _ = strconv.ParseFloat(h["lng"], 64)
	if ts, err := strconv.ParseInt(h["ts"], 10, 64); err == nil {
		st.RecordedAt = time.UnixMilli(ts)
	}
	return st, nil
}

func splitCats(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
