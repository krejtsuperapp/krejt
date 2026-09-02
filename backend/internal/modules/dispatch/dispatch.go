// Package dispatch — motori i centralizuar i caktimit (§26), v1 për udhëtimet: kandidatët nga
// Redis GEO (afërsi, disponueshmëri, kategori), pranueshmëria nga DB (i miratuar, pa udhëtim aktiv),
// një ofertë në radhë me TTL, skadim → tjetri, ricaktim automatik kur shoferi anulon, no_driver pas afatit.
// Faktorët e tjerë (workload, prioritet, siguri, rregulla biznesi) shtohen mbi këtë skelet.
package dispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/platform/events"
)

type Dispatcher struct {
	pool *pgxpool.Pool
	loc  *location.Service
	log  *slog.Logger
	now  func() time.Time

	OfferTTL        time.Duration
	SearchRadiusKm  float64
	MatchingTimeout time.Duration
}

func New(pool *pgxpool.Pool, loc *location.Service, log *slog.Logger) *Dispatcher {
	return &Dispatcher{pool: pool, loc: loc, log: log, now: time.Now,
		OfferTTL: 20 * time.Second, SearchRadiusKm: 6, MatchingTimeout: 3 * time.Minute}
}

const (
	urbanSpeed   = 25000.0 / 3600.0
	detourFactor = 1.3
)

// Round — një raund oferte për udhëtimin: nëse është në kërkim dhe pa ofertë të hapur, i ofrohet
// kandidatit më të afërt të pranueshëm që s'e ka marrë ende. Idempotent (FOR UPDATE mbi udhëtimin).
func (d *Dispatcher) Round(ctx context.Context, rideID uuid.UUID) (offered bool, err error) {
	err = pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		var state, category string
		var pickup geo.Point
		var requestedAt time.Time
		var attempts int
		if err := tx.QueryRow(ctx, `SELECT state, category_id, pickup_lat, pickup_lng, requested_at, matching_attempts
			FROM rides WHERE id = $1 FOR UPDATE`, rideID).Scan(&state, &category, &pickup.Lat, &pickup.Lng, &requestedAt, &attempts); err != nil {
			return err
		}
		if state != "matching" {
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM ride_offers WHERE ride_id = $1 AND state = 'offered' AND expires_at > now()`, rideID).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return nil
		}
		if d.now().Sub(requestedAt) > d.MatchingTimeout {
			if _, err := tx.Exec(ctx, `UPDATE rides SET state = 'no_driver', updated_at = now() WHERE id = $1`, rideID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO ride_events (ride_id, from_state, to_state, actor_type, metadata)
				VALUES ($1, 'matching', 'no_driver', 'system', jsonb_build_object('attempts', $2::int))`, rideID, attempts); err != nil {
				return err
			}
			d.log.Info("dispatch: no driver", "ride_id", rideID, "attempts", attempts)
			return events.Emit(ctx, tx, "ride", rideID.String(), "RideNoDriver", map[string]any{"ride_id": rideID})
		}

		excluded := map[uuid.UUID]bool{}
		rows, err := tx.Query(ctx, `SELECT driver_id FROM ride_offers WHERE ride_id = $1`, rideID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			excluded[id] = true
		}
		rows.Close()
		var round int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM ride_offers WHERE ride_id = $1`, rideID).Scan(&round); err != nil {
			return err
		}

		cands, err := d.loc.Nearest(ctx, category, pickup, d.SearchRadiusKm, 10)
		if err != nil {
			return err
		}
		for _, c := range cands {
			if excluded[c.DriverID] {
				continue
			}
			var eligible bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
				  SELECT 1 FROM drivers dr WHERE dr.user_id = $1 AND dr.status = 'approved' AND $2 = ANY(dr.categories)
				    AND NOT EXISTS (SELECT 1 FROM rides r WHERE r.driver_id = dr.user_id AND r.state IN ('assigned','arrived','in_progress'))
				    AND NOT EXISTS (SELECT 1 FROM ride_offers o WHERE o.driver_id = dr.user_id AND o.state = 'offered' AND o.expires_at > now()))`,
				c.DriverID, category).Scan(&eligible); err != nil {
				return err
			}
			if !eligible {
				continue
			}
			eta := int(c.DistanceM * detourFactor / urbanSpeed)
			if eta < 60 {
				eta = 60
			}
			expires := d.now().Add(d.OfferTTL)
			var offerID uuid.UUID
			if err := tx.QueryRow(ctx, `
				INSERT INTO ride_offers (ride_id, driver_id, round, distance_m, eta_s, expires_at)
				VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, rideID, c.DriverID, round+1, int(c.DistanceM), eta, expires).Scan(&offerID); err != nil {
				return err
			}
			offered = true
			d.log.Info("dispatch: offer", "ride_id", rideID, "driver_id", c.DriverID, "round", round+1, "distance_m", int(c.DistanceM))
			return events.Emit(ctx, tx, "ride", rideID.String(), "RideOffered", map[string]any{
				"ride_id": rideID, "driver_id": c.DriverID, "offer_id": offerID, "expires_at": expires, "round": round + 1})
		}
		return nil
	})
	return offered, err
}

type SweepStats struct {
	Expired int
	Rounds  int
	Offered int
}

// Sweep — skadon ofertat e vjetruara dhe bën një raund për çdo udhëtim në kërkim pa ofertë të hapur.
// Thirret çdo sekondë nga worker-i; e sigurt me disa worker-a (FOR UPDATE / SKIP LOCKED).
func (d *Dispatcher) Sweep(ctx context.Context) (SweepStats, error) {
	var st SweepStats
	rows, err := d.pool.Query(ctx, `
		UPDATE ride_offers SET state = 'expired', responded_at = now()
		WHERE state = 'offered' AND expires_at <= now() RETURNING id, ride_id, driver_id`)
	if err != nil {
		return st, err
	}
	type exp struct{ id, rideID, driverID uuid.UUID }
	var expired []exp
	for rows.Next() {
		var e exp
		if err := rows.Scan(&e.id, &e.rideID, &e.driverID); err != nil {
			rows.Close()
			return st, err
		}
		expired = append(expired, e)
	}
	rows.Close()
	for _, e := range expired {
		st.Expired++
		if err := events.Emit(ctx, d.pool, "ride", e.rideID.String(), "RideOfferExpired", map[string]any{"ride_id": e.rideID, "driver_id": e.driverID, "offer_id": e.id}); err != nil {
			return st, err
		}
	}

	rows, err = d.pool.Query(ctx, `
		SELECT r.id FROM rides r
		WHERE r.state = 'matching'
		  AND NOT EXISTS (SELECT 1 FROM ride_offers o WHERE o.ride_id = r.id AND o.state = 'offered' AND o.expires_at > now())
		ORDER BY r.requested_at LIMIT 200`)
	if err != nil {
		return st, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return st, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		st.Rounds++
		offered, err := d.Round(ctx, id)
		if err != nil {
			return st, err
		}
		if offered {
			st.Offered++
		}
	}
	return st, nil
}
