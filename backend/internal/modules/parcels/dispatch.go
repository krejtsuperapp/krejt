package parcels

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/location"
	"krejt.app/backend/internal/platform/events"
)

// Nearby — kandidatët nga Redis GEO (moduli location); korrierët janë shoferë me kategori 'economy'.
type Nearby interface {
	Nearest(ctx context.Context, category string, p geo.Point, radiusKm float64, limit int) ([]NearbyCandidate, error)
}

type NearbyCandidate struct {
	DriverID  uuid.UUID
	DistanceM float64
}

// LocationNearby — adapter nga moduli location.
type LocationNearby struct{ Loc *location.Service }

func (l LocationNearby) Nearest(ctx context.Context, category string, p geo.Point, radiusKm float64, limit int) ([]NearbyCandidate, error) {
	cands, err := l.Loc.Nearest(ctx, category, p, radiusKm, limit)
	if err != nil {
		return nil, err
	}
	out := make([]NearbyCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, NearbyCandidate{DriverID: c.DriverID, DistanceM: c.DistanceM})
	}
	return out, nil
}

type Dispatcher struct {
	svc    *Service
	nearby Nearby
	log    *slog.Logger

	OfferTTL       time.Duration
	SearchRadiusKm float64
	Category       string
	Timeout        time.Duration
}

func NewDispatcher(s *Service, n Nearby, log *slog.Logger) *Dispatcher {
	return &Dispatcher{svc: s, nearby: n, log: log, OfferTTL: 25 * time.Second, SearchRadiusKm: 6, Category: "economy", Timeout: 15 * time.Minute}
}

const (
	urbanSpeed   = 25000.0 / 3600.0
	detourFactor = 1.3
)

// Round — një raund oferte për një pako 'requested' pa ofertë të hapur.
func (d *Dispatcher) Round(ctx context.Context, parcelID uuid.UUID) (offered bool, err error) {
	err = pgx.BeginFunc(ctx, d.svc.pool, func(tx pgx.Tx) error {
		var state string
		var createdAt time.Time
		var pickup geo.Point
		if err := tx.QueryRow(ctx, `SELECT state, created_at, pickup_lat, pickup_lng FROM parcels WHERE id = $1 FOR UPDATE`, parcelID).
			Scan(&state, &createdAt, &pickup.Lat, &pickup.Lng); err != nil {
			return err
		}
		if state != StateRequested {
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM parcel_offers WHERE parcel_id = $1 AND state = 'offered' AND expires_at > now()`, parcelID).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return nil
		}
		if d.svc.now().Sub(createdAt) > d.Timeout {
			// asnjë korrier brenda afatit: pakoja mbyllet hapur, klienti e sheh dhe mund ta rikërkojë
			if _, err := tx.Exec(ctx, `UPDATE parcels SET state = 'no_courier', payment_status = 'none', updated_at = now() WHERE id = $1`, parcelID); err != nil {
				return err
			}
			from := StateRequested
			if err := parcelEvent(ctx, tx, parcelID, &from, StateNoCourier, "system", nil, nil); err != nil {
				return err
			}
			return events.Emit(ctx, tx, "parcel", parcelID.String(), "ParcelNoCourier", map[string]any{"parcel_id": parcelID})
		}
		excluded := map[uuid.UUID]bool{}
		rows, err := tx.Query(ctx, `SELECT courier_id FROM parcel_offers WHERE parcel_id = $1`, parcelID)
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
		round := len(excluded)

		cands, err := d.nearby.Nearest(ctx, d.Category, pickup, d.SearchRadiusKm, 10)
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
				  SELECT 1 FROM drivers dr WHERE dr.user_id = $1 AND dr.status = 'approved'
				    AND NOT EXISTS (SELECT 1 FROM rides r WHERE r.driver_id = dr.user_id AND r.state IN ('assigned','arrived','in_progress'))
				    AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.courier_id = dr.user_id AND o.state IN ('courier_assigned','picked_up'))
				    AND NOT EXISTS (SELECT 1 FROM parcels p WHERE p.courier_id = dr.user_id AND p.state IN ('courier_assigned','picked_up'))
				    AND NOT EXISTS (SELECT 1 FROM order_offers oo WHERE oo.courier_id = dr.user_id AND oo.state = 'offered' AND oo.expires_at > now())
				    AND NOT EXISTS (SELECT 1 FROM parcel_offers po WHERE po.courier_id = dr.user_id AND po.state = 'offered' AND po.expires_at > now()))`,
				c.DriverID).Scan(&eligible); err != nil {
				return err
			}
			if !eligible {
				continue
			}
			eta := int(c.DistanceM * detourFactor / urbanSpeed)
			if eta < 60 {
				eta = 60
			}
			expires := d.svc.now().Add(d.OfferTTL)
			var offerID uuid.UUID
			if err := tx.QueryRow(ctx, `INSERT INTO parcel_offers (parcel_id, courier_id, round, distance_m, eta_s, expires_at)
				VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, parcelID, c.DriverID, round+1, int(c.DistanceM), eta, expires).Scan(&offerID); err != nil {
				return err
			}
			offered = true
			d.log.Info("parcels dispatch: offer", "parcel_id", parcelID, "courier_id", c.DriverID, "round", round+1)
			return events.Emit(ctx, tx, "parcel", parcelID.String(), "ParcelOffered", map[string]any{
				"parcel_id": parcelID, "courier_id": c.DriverID, "offer_id": offerID, "expires_at": expires, "round": round + 1})
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

// Sweep — skadon ofertat e vjetruara dhe bën një raund për çdo pako 'requested' pa ofertë të hapur.
func (d *Dispatcher) Sweep(ctx context.Context) (SweepStats, error) {
	var st SweepStats
	rows, err := d.svc.pool.Query(ctx, `UPDATE parcel_offers SET state = 'expired', responded_at = now()
		WHERE state = 'offered' AND expires_at <= now() RETURNING id, parcel_id, courier_id`)
	if err != nil {
		return st, err
	}
	type exp struct{ id, parcelID, courierID uuid.UUID }
	var expired []exp
	for rows.Next() {
		var e exp
		if err := rows.Scan(&e.id, &e.parcelID, &e.courierID); err != nil {
			rows.Close()
			return st, err
		}
		expired = append(expired, e)
	}
	rows.Close()
	for _, e := range expired {
		st.Expired++
		if err := events.Emit(ctx, d.svc.pool, "parcel", e.parcelID.String(), "ParcelOfferExpired", map[string]any{"parcel_id": e.parcelID, "courier_id": e.courierID, "offer_id": e.id}); err != nil {
			return st, err
		}
	}
	rows, err = d.svc.pool.Query(ctx, `SELECT p.id FROM parcels p
		WHERE p.state = 'requested'
		  AND NOT EXISTS (SELECT 1 FROM parcel_offers po WHERE po.parcel_id = p.id AND po.state = 'offered' AND po.expires_at > now())
		ORDER BY p.created_at LIMIT 200`)
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
