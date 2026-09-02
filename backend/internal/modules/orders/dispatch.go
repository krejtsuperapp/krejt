package orders

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/events"
)

// Nearby — kandidatët nga Redis GEO (moduli location); korrierët janë shoferë me kategori 'economy'
// derisa të vijë kategoria e dedikuar e korrierëve (Faza 2 e vonë).
type Nearby interface {
	Nearest(ctx context.Context, category string, p geo.Point, radiusKm float64, limit int) ([]NearbyCandidate, error)
}

type NearbyCandidate struct {
	DriverID  uuid.UUID
	DistanceM float64
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
	return &Dispatcher{svc: s, nearby: n, log: log, OfferTTL: 25 * time.Second, SearchRadiusKm: 5, Category: "economy", Timeout: 10 * time.Minute}
}

const (
	urbanSpeed   = 25000.0 / 3600.0
	detourFactor = 1.3
)

// Round — një raund oferte për një porosi 'ready' me përmbushje me korrier.
func (d *Dispatcher) Round(ctx context.Context, orderID uuid.UUID) (offered bool, err error) {
	err = pgx.BeginFunc(ctx, d.svc.pool, func(tx pgx.Tx) error {
		var state, fulfillment string
		var readyAt *time.Time
		var pickup geo.Point
		if err := tx.QueryRow(ctx, `SELECT o.state, o.fulfillment, o.ready_at, m.lat, m.lng FROM orders o JOIN merchants m ON m.id = o.merchant_id
			WHERE o.id = $1 FOR UPDATE OF o`, orderID).Scan(&state, &fulfillment, &readyAt, &pickup.Lat, &pickup.Lng); err != nil {
			return err
		}
		if state != StateReady || fulfillment != "courier" {
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM order_offers WHERE order_id = $1 AND state = 'offered' AND expires_at > now()`, orderID).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return nil
		}
		if readyAt != nil && d.svc.now().Sub(*readyAt) > d.Timeout {
			// asnjë korrier brenda afatit: mbetet 'ready' dhe Operacionet e shohin te paneli; njoftohet një herë
			var alerted int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM order_events WHERE order_id = $1 AND to_state = 'ready' AND actor_type = 'system'`, orderID).Scan(&alerted); err != nil {
				return err
			}
			if alerted == 0 {
				if err := orderEvent(ctx, tx, orderID, &state, StateReady, "system", nil, map[string]any{"no_courier": true}); err != nil {
					return err
				}
				return events.Emit(ctx, tx, "order", orderID.String(), "OrderNoCourier", map[string]any{"order_id": orderID})
			}
			return nil
		}

		excluded := map[uuid.UUID]bool{}
		rows, err := tx.Query(ctx, `SELECT courier_id FROM order_offers WHERE order_id = $1`, orderID)
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
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM order_offers WHERE order_id = $1`, orderID).Scan(&round); err != nil {
			return err
		}

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
				    AND NOT EXISTS (SELECT 1 FROM order_offers oo WHERE oo.courier_id = dr.user_id AND oo.state = 'offered' AND oo.expires_at > now()))`,
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
			if err := tx.QueryRow(ctx, `INSERT INTO order_offers (order_id, courier_id, round, distance_m, eta_s, expires_at)
				VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, orderID, c.DriverID, round+1, int(c.DistanceM), eta, expires).Scan(&offerID); err != nil {
				return err
			}
			offered = true
			d.log.Info("orders dispatch: offer", "order_id", orderID, "courier_id", c.DriverID, "round", round+1)
			return events.Emit(ctx, tx, "order", orderID.String(), "OrderOffered", map[string]any{
				"order_id": orderID, "courier_id": c.DriverID, "offer_id": offerID, "expires_at": expires, "round": round + 1})
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

// Sweep — skadon ofertat e vjetruara dhe bën një raund për çdo porosi 'ready' pa ofertë të hapur.
func (d *Dispatcher) Sweep(ctx context.Context) (SweepStats, error) {
	var st SweepStats
	rows, err := d.svc.pool.Query(ctx, `UPDATE order_offers SET state = 'expired', responded_at = now()
		WHERE state = 'offered' AND expires_at <= now() RETURNING id, order_id, courier_id`)
	if err != nil {
		return st, err
	}
	type exp struct{ id, orderID, courierID uuid.UUID }
	var expired []exp
	for rows.Next() {
		var e exp
		if err := rows.Scan(&e.id, &e.orderID, &e.courierID); err != nil {
			rows.Close()
			return st, err
		}
		expired = append(expired, e)
	}
	rows.Close()
	for _, e := range expired {
		st.Expired++
		if err := events.Emit(ctx, d.svc.pool, "order", e.orderID.String(), "OrderOfferExpired", map[string]any{"order_id": e.orderID, "courier_id": e.courierID, "offer_id": e.id}); err != nil {
			return st, err
		}
	}
	rows, err = d.svc.pool.Query(ctx, `SELECT o.id FROM orders o
		WHERE o.state = 'ready' AND o.fulfillment = 'courier'
		  AND NOT EXISTS (SELECT 1 FROM order_offers oo WHERE oo.order_id = o.id AND oo.state = 'offered' AND oo.expires_at > now())
		ORDER BY o.ready_at LIMIT 200`)
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
