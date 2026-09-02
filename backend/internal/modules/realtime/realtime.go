// Package realtime — kanalet e gjalla (§42) mbi Centrifugo: token lidhjeje, autorizim abonimi
// server-side për kanal (BOLA §52), dhe publikimi i ngjarjeve të domenit në kanale:
//
//	ride:{id}    — gjendja e udhëtimit + pozicioni i shoferit (klienti dhe shoferi i udhëtimit)
//	driver:{id}  — ofertat e dispatch-it (vetëm shoferi)
//	user:{id}    — sinjale personale (kutia e njoftimeve u ndryshua)
package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	rt "krejt.app/backend/internal/platform/providers/realtime"
)

const (
	ConnectionTTL   = time.Hour
	SubscriptionTTL = 2 * time.Hour
)

type Service struct {
	pool   *pgxpool.Pool
	pub    rt.Provider
	tokens *rt.TokenIssuer
}

func New(pool *pgxpool.Pool, pub rt.Provider, tokens *rt.TokenIssuer) *Service {
	return &Service{pool: pool, pub: pub, tokens: tokens}
}

func RideChannel(rideID uuid.UUID) string     { return "ride:" + rideID.String() }
func DriverChannel(driverID uuid.UUID) string { return "driver:" + driverID.String() }
func UserChannel(userID uuid.UUID) string     { return "user:" + userID.String() }

// Authorize — a mund të abonohet aktori në kanal? (kurrë sipas fjalës së klientit)
func (s *Service) Authorize(ctx context.Context, a principal.Actor, channel string) (bool, error) {
	kind, rest, ok := strings.Cut(channel, ":")
	if !ok {
		return false, nil
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return false, nil
	}
	switch kind {
	case "user", "driver":
		return id == a.UserID, nil
	case "ops":
		// ops:{çdo id} — vetëm stafi (dispatch live, SOS); id-ja është emri i panelit, jo përdorues
		return a.Has("OPERATIONS") || a.Has("ADMIN") || a.Has("SUPPORT"), nil
	case "ride":
		var allowed bool
		err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2))`, id, a.UserID).Scan(&allowed)
		return allowed, err
	}
	return false, nil
}

// Handle — ngjarje e domenit → publikim në kanalin përkatës (thirret nga worker-i, si notifications.Handle).
func (s *Service) Handle(ctx context.Context, ev events.Event) error {
	var p map[string]any
	if len(ev.Payload) > 0 {
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
	}
	str := func(k string) string {
		if v, ok := p[k].(string); ok {
			return v
		}
		return ""
	}
	msg := map[string]any{"type": ev.EventType, "event_id": ev.ID, "at": ev.OccurredAt, "data": p}
	rideID, hasRide := uuid.Parse(str("ride_id"))
	driverID, hasDriver := uuid.Parse(str("driver_id"))

	switch ev.EventType {
	case "RideOffered", "RideOfferExpired":
		if hasDriver == nil {
			return s.pub.Publish(ctx, DriverChannel(driverID), msg)
		}
	case "RideAssigned", "RideDriverArrived", "RideStarted", "RideCompleted", "RideNoDriver", "RidePaymentSettled", "RidePaymentFailed", "RideChatMessage":
		if hasRide == nil {
			return s.pub.Publish(ctx, RideChannel(rideID), msg)
		}
	case "RideCancelled":
		if hasRide == nil {
			if err := s.pub.Publish(ctx, RideChannel(rideID), msg); err != nil {
				return err
			}
		}
		if hasDriver == nil && str("by") == "customer" {
			return s.pub.Publish(ctx, DriverChannel(driverID), msg)
		}
	case "RideRequested":
		if hasRide == nil && p["reassign"] == true {
			return s.pub.Publish(ctx, RideChannel(rideID), msg)
		}
	case "SafetyReportCreated", "SupportTicketCreated":
		return s.pub.Publish(ctx, OpsChannel, msg)
	}
	return nil
}

// OpsChannel — paneli i Operacioneve (SOS, tiketa të reja); vetëm stafi abonohet.
const OpsChannel = "ops:" + "00000000-0000-0000-0000-000000000001"

// --- HTTP -------------------------------------------------------------------------

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/realtime/token", requireAuth(principal.Handler(s.handleToken)))
	mux.Handle("POST /api/v1/realtime/subscribe", requireAuth(principal.Handler(s.handleSubscribe)))
}

type tokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Channel   string    `json:"channel,omitempty"`
}

func (s *Service) handleToken(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	tok, exp, err := s.tokens.ConnectionToken(a.UserID.String(), ConnectionTTL)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{Token: tok, ExpiresAt: exp})
}

func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	var in struct {
		Channel string `json:"channel"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	in.Channel = strings.TrimSpace(in.Channel)
	ok, err := s.Authorize(r.Context(), a, in.Channel)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !ok {
		httpx.WriteError(w, r, httpx.ErrForbidden)
		return
	}
	tok, exp, err := s.tokens.SubscriptionToken(a.UserID.String(), in.Channel, SubscriptionTTL)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{Token: tok, ExpiresAt: exp, Channel: in.Channel})
}
