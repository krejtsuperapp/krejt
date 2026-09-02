package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/providers/push"
)

// Target — një marrës i një ngjarjeje: kujt, në cilën kategori, me cilin tekst dhe deep link.
type Target struct {
	UserID   uuid.UUID
	Category string
	TextKey  string
	Params   map[string]string // parametra "të papërkthyer" (shuma në cent, id) — teksti rendit sipas gjuhës
	DeepLink string
	Priority string
	TTL      time.Duration
	Collapse string
}

type payload map[string]any

func (p payload) str(k string) string {
	switch v := p[k].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (p payload) uuid(k string) (uuid.UUID, bool) {
	id, err := uuid.Parse(p.str(k))
	return id, err == nil
}

func (p payload) int64(k string) int64 {
	if v, ok := p[k].(float64); ok {
		return int64(v)
	}
	return 0
}

// Map — ngjarje e domenit → marrësit. Kthen bosh për ngjarjet që s'kanë njoftim (dhe kjo është në rregull).
func Map(ev events.Event) ([]Target, error) {
	var p payload
	if len(ev.Payload) > 0 {
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return nil, fmt.Errorf("notifications: payload %s: %w", ev.EventType, err)
		}
	}
	rideLink := func() string { return "krejt://rides/" + p.str("ride_id") }
	switch ev.EventType {
	case "RideOffered":
		did, ok := p.uuid("driver_id")
		if !ok {
			return nil, nil
		}
		return []Target{{UserID: did, Category: "rides", TextKey: "notif.ride.offer",
			Params:   map[string]string{"offer_id": p.str("offer_id"), "ride_id": p.str("ride_id"), "round": p.str("round")},
			DeepLink: "krejt://driver/offers/" + p.str("offer_id"), Priority: "high", TTL: 20 * time.Second, Collapse: "offer:" + p.str("ride_id")}}, nil
	case "RideAssigned":
		cid, ok := p.uuid("customer_id")
		if !ok {
			return nil, nil
		}
		return []Target{{UserID: cid, Category: "rides", TextKey: "notif.ride.assigned", Params: map[string]string{"ride_id": p.str("ride_id")},
			DeepLink: rideLink(), Priority: "high", Collapse: "ride:" + p.str("ride_id")}}, nil
	case "RideDriverArrived":
		cid, ok := p.uuid("customer_id")
		if !ok {
			return nil, nil
		}
		return []Target{{UserID: cid, Category: "rides", TextKey: "notif.ride.arrived", Params: map[string]string{"ride_id": p.str("ride_id")},
			DeepLink: rideLink(), Priority: "high", Collapse: "ride:" + p.str("ride_id")}}, nil
	case "RideStarted":
		cid, ok := p.uuid("customer_id")
		if !ok {
			return nil, nil
		}
		return []Target{{UserID: cid, Category: "rides", TextKey: "notif.ride.started", Params: map[string]string{"ride_id": p.str("ride_id")},
			DeepLink: rideLink(), Priority: "normal", Collapse: "ride:" + p.str("ride_id")}}, nil
	case "RideCompleted":
		var out []Target
		if cid, ok := p.uuid("customer_id"); ok {
			out = append(out, Target{UserID: cid, Category: "rides", TextKey: "notif.ride.completed",
				Params: map[string]string{"ride_id": p.str("ride_id"), "price_minor": fmt.Sprint(p.int64("price_final_minor"))}, DeepLink: rideLink(), Priority: "normal"})
		}
		if did, ok := p.uuid("driver_id"); ok {
			out = append(out, Target{UserID: did, Category: "rides", TextKey: "notif.ride.completed.driver",
				Params: map[string]string{"ride_id": p.str("ride_id"), "price_minor": fmt.Sprint(p.int64("price_final_minor"))}, DeepLink: "krejt://driver/rides/" + p.str("ride_id"), Priority: "normal"})
		}
		return out, nil
	case "RideCancelled":
		if p.str("by") == "customer" {
			if did, ok := p.uuid("driver_id"); ok {
				return []Target{{UserID: did, Category: "rides", TextKey: "notif.ride.cancelled.customer", Params: map[string]string{"ride_id": p.str("ride_id")},
					DeepLink: "krejt://driver/rides/" + p.str("ride_id"), Priority: "high", Collapse: "ride:" + p.str("ride_id")}}, nil
			}
		}
		return nil, nil
	case "RideRequested":
		if p.str("reassign") == "true" {
			if cid, ok := p.uuid("customer_id"); ok {
				return []Target{{UserID: cid, Category: "rides", TextKey: "notif.ride.reassigning", Params: map[string]string{"ride_id": p.str("ride_id")},
					DeepLink: rideLink(), Priority: "high", Collapse: "ride:" + p.str("ride_id")}}, nil
			}
		}
		return nil, nil
	case "RideNoDriver":
		// payload-i mban vetëm ride_id: klienti lexohet nga baza (Handle e plotëson)
		return []Target{{Category: "rides", TextKey: "notif.ride.no_driver", Params: map[string]string{"ride_id": p.str("ride_id")}, DeepLink: rideLink(), Priority: "high"}}, nil
	case "RidePaymentSettled", "RidePaymentFailed":
		cid, ok := p.uuid("customer_id")
		if !ok || p.str("method") != "wallet" {
			return nil, nil
		}
		key := "notif.payment.paid"
		if ev.EventType == "RidePaymentFailed" {
			key = "notif.payment.failed"
		}
		if ev.EventType == "RidePaymentSettled" && p.str("status") != "paid" {
			return nil, nil
		}
		return []Target{{UserID: cid, Category: "payments", TextKey: key, Params: map[string]string{"ride_id": p.str("ride_id")}, DeepLink: rideLink(), Priority: "normal"}}, nil
	case "WalletToppedUp":
		uid, ok := p.uuid("user_id")
		if !ok {
			return nil, nil
		}
		return []Target{{UserID: uid, Category: "wallet", TextKey: "notif.wallet.topup",
			Params:   map[string]string{"amount_minor": fmt.Sprint(p.int64("amount_minor")), "currency": p.str("currency"), "intent_id": p.str("intent_id")},
			DeepLink: "krejt://wallet", Priority: "normal"}}, nil
	case "DriverDocumentReviewed":
		did, ok := p.uuid("driver_id")
		if !ok || p.str("status") != "rejected" {
			return nil, nil
		}
		return []Target{{UserID: did, Category: "support", TextKey: "notif.driver.document_rejected",
			Params: map[string]string{"doc_type": p.str("type"), "reason": p.str("reason")}, DeepLink: "krejt://driver/documents", Priority: "normal"}}, nil
	case "DriverApproved", "DriverSuspended":
		did, ok := p.uuid("driver_id")
		if !ok {
			return nil, nil
		}
		key := "notif.driver.approved"
		if ev.EventType == "DriverSuspended" {
			key = "notif.driver.suspended"
		}
		return []Target{{UserID: did, Category: "support", TextKey: key, DeepLink: "krejt://driver/profile", Priority: "normal"}}, nil
	case "UserProfileUpdated":
		uid, ok := p.uuid("user_id")
		if !ok {
			return nil, nil
		}
		changed := ""
		if arr, ok := p["changed"].([]any); ok {
			parts := make([]string, 0, len(arr))
			for _, x := range arr {
				parts = append(parts, fmt.Sprint(x))
			}
			changed = strings.Join(parts, ", ")
		}
		if !strings.Contains(changed, "email") {
			return nil, nil // vetëm ndryshimet me peshë sigurie
		}
		return []Target{{UserID: uid, Category: "security", TextKey: "notif.security.profile_changed", Params: map[string]string{"changed": changed}, DeepLink: "krejt://account/security", Priority: "normal"}}, nil
	}
	return nil, nil
}

// Handle — përpunon një ngjarje: krijon njoftimin në kutinë e secilit marrës (idempotent për ngjarje+përdorues),
// zbaton preferencat (§29) dhe dërgon push në pajisjet e vlefshme. Gabimi i push-it nuk e rrëzon ngjarjen:
// regjistrohet në gjurmën e dorëzimit dhe njoftimi mbetet në kuti.
func (s *Service) Handle(ctx context.Context, ev events.Event) error {
	targets, err := Map(ev)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if t.UserID == uuid.Nil {
			if err := s.fillUser(ctx, &t); err != nil {
				return err
			}
			if t.UserID == uuid.Nil {
				continue
			}
		}
		if err := s.deliver(ctx, ev, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) fillUser(ctx context.Context, t *Target) error {
	rid, err := uuid.Parse(t.Params["ride_id"])
	if err != nil {
		return nil
	}
	err = s.pool.QueryRow(ctx, `SELECT customer_id FROM rides WHERE id = $1`, rid).Scan(&t.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (s *Service) deliver(ctx context.Context, ev events.Event, t Target) error {
	// 1) kutia në aplikacion (idempotente)
	params := s.enrich(ctx, t)
	paramsJSON, _ := json.Marshal(params)
	var notifID uuid.UUID
	var locale string
	err := s.pool.QueryRow(ctx, `
		WITH ins AS (
		  INSERT INTO notifications (user_id, event_id, event_type, category, title_key, body_key, params, deep_link)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		  ON CONFLICT (event_id, user_id) DO NOTHING
		  RETURNING id)
		SELECT ins.id, u.locale FROM ins JOIN users u ON u.id = $1`,
		t.UserID, ev.ID, ev.EventType, t.Category, t.TextKey+".title", t.TextKey+".body", paramsJSON, nullable(t.DeepLink)).Scan(&notifID, &locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // e përpunuar më parë (ridorëzim i SQS) — asnjë push i dytë
	}
	if err != nil {
		return err
	}

	// 2) preferencat: security gjithmonë push; të tjerat sipas zgjedhjes (parazgjedhja: push on)
	pushOn := true
	if t.Category != "security" {
		var v bool
		err := s.pool.QueryRow(ctx, `SELECT push FROM notification_preferences WHERE user_id = $1 AND category = $2`, t.UserID, t.Category).Scan(&v)
		if err == nil {
			pushOn = v
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if !pushOn {
		_, err := s.pool.Exec(ctx, `INSERT INTO notification_deliveries (notification_id, channel, status, error) VALUES ($1, 'push', 'skipped', 'preference_off')`, notifID)
		return err
	}

	// 3) push në çdo pajisje të vlefshme, në gjuhën e pajisjes
	rows, err := s.pool.Query(ctx, `SELECT id, token, locale FROM push_tokens WHERE user_id = $1 AND invalid_at IS NULL`, t.UserID)
	if err != nil {
		return err
	}
	type tok struct {
		id     uuid.UUID
		token  string
		locale string
	}
	var toks []tok
	for rows.Next() {
		var x tok
		if err := rows.Scan(&x.id, &x.token, &x.locale); err != nil {
			rows.Close()
			return err
		}
		toks = append(toks, x)
	}
	rows.Close()
	if len(toks) == 0 {
		_, err := s.pool.Exec(ctx, `INSERT INTO notification_deliveries (notification_id, channel, status, error) VALUES ($1, 'push', 'skipped', 'no_device')`, notifID)
		return err
	}
	data := map[string]string{"notification_id": notifID.String(), "category": t.Category, "key": t.TextKey, "event_type": ev.EventType}
	if t.DeepLink != "" {
		data["deep_link"] = t.DeepLink
	}
	for k, v := range t.Params {
		data[k] = v
	}
	for _, x := range toks {
		loc := x.locale
		if loc == "" {
			loc = locale
		}
		title, body, _ := Render(t.TextKey, loc, localize(params, loc))
		res, perr := s.push.Send(ctx, push.Message{Token: x.token, Title: title, Body: body, Data: data, Priority: t.Priority, TTL: t.TTL, Collapse: t.Collapse})
		status, errText := "sent", ""
		if perr != nil {
			status, errText = "failed", perr.Error()
			if res.InvalidToken {
				if _, err := s.pool.Exec(ctx, `UPDATE push_tokens SET invalid_at = now(), updated_at = now() WHERE id = $1`, x.id); err != nil {
					return err
				}
			}
		} else {
			_, _ = s.pool.Exec(ctx, `UPDATE push_tokens SET last_used_at = now() WHERE id = $1`, x.id)
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO notification_deliveries (notification_id, channel, status, target, provider_message_id, error)
			VALUES ($1, 'push', $2, $3, $4, $5)`, notifID, status, push.Shorten(x.token), nullable(res.ProviderMessageID), nullable(errText)); err != nil {
			return err
		}
	}
	return nil
}

// enrich — plotëson parametrat nga baza (emri i shoferit, automjeti, çmimi…) që teksti të jetë i plotë.
func (s *Service) enrich(ctx context.Context, t Target) map[string]string {
	params := map[string]string{}
	for k, v := range t.Params {
		params[k] = v
	}
	rid, err := uuid.Parse(params["ride_id"])
	if err != nil {
		return params
	}
	var driver, vehicleMake, vehicleModel, plate, dropoff *string
	var priceQuoted int64
	var priceFinal, commission *int64
	var fee int64
	var currency string
	err = s.pool.QueryRow(ctx, `
		SELECT u.full_name, d.vehicle_make, d.vehicle_model, d.vehicle_plate, r.dropoff_address,
		       r.price_quoted_minor, r.price_final_minor, r.commission_minor, r.cancellation_fee_minor, r.currency
		FROM rides r LEFT JOIN drivers d ON d.user_id = r.driver_id LEFT JOIN users u ON u.id = r.driver_id
		WHERE r.id = $1`, rid).Scan(&driver, &vehicleMake, &vehicleModel, &plate, &dropoff, &priceQuoted, &priceFinal, &commission, &fee, &currency)
	if err != nil {
		return params
	}
	if driver != nil {
		params["driver"] = *driver
	} else {
		params["driver"] = "Shoferi"
	}
	if vehicleMake != nil {
		params["vehicle"] = strings.TrimSpace(*vehicleMake + " " + deref(vehicleModel))
	}
	if plate != nil {
		params["plate"] = *plate
	}
	if dropoff != nil {
		params["dropoff"] = *dropoff
	}
	price := priceQuoted
	if priceFinal != nil {
		price = *priceFinal
	}
	params["price_minor"] = fmt.Sprint(price)
	params["currency"] = strings.TrimSpace(currency)
	if commission != nil {
		params["earnings_minor"] = fmt.Sprint(price - *commission)
	}
	if fee > 0 {
		params["fee_minor"] = fmt.Sprint(fee)
	}
	// oferta: distanca shofer→marrje
	if oid, err := uuid.Parse(params["offer_id"]); err == nil {
		var dist int
		if err := s.pool.QueryRow(ctx, `SELECT distance_m FROM ride_offers WHERE id = $1`, oid).Scan(&dist); err == nil {
			params["distance_km"] = fmt.Sprintf("%.1f", float64(dist)/1000)
		}
		params["ttl_s"] = "20"
	}
	return params
}

// localize — shumat në cent → tekst me monedhë sipas gjuhës.
func localize(params map[string]string, locale string) map[string]string {
	out := map[string]string{}
	for k, v := range params {
		out[k] = v
	}
	cur := params["currency"]
	if cur == "" {
		cur = "EUR"
	}
	for _, pair := range [][2]string{{"price_minor", "price"}, {"earnings_minor", "earnings"}, {"fee_minor", "fee"}, {"amount_minor", "amount"}} {
		var minor int64
		if _, err := fmt.Sscan(params[pair[0]], &minor); err == nil && params[pair[0]] != "" {
			out[pair[1]] = FormatMoney(minor, cur, locale)
		}
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
