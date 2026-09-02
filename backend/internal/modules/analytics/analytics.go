// Package analytics — ngjarje domeni → ngjarje produkti (§66): signup, ride_requested/accepted/completed/
// cancelled, payment_success/failure, wallet_topup, refund, review, support_ticket … me respekt për
// privatësinë (vetëm id dhe vlera biznesi). Thirret nga worker-i si përpunues i outbox-it.
package analytics

import (
	"context"
	"encoding/json"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/providers/analytics"
)

type Service struct {
	p analytics.Provider
}

func New(p analytics.Provider) *Service { return &Service{p: p} }

// Map — ngjarja e domenit → (distinct_id, emri, vetitë); "" = nuk gjurmohet.
func Map(ev events.Event) (distinctID, name string, props map[string]any) {
	var p map[string]any
	if len(ev.Payload) > 0 {
		_ = json.Unmarshal(ev.Payload, &p)
	}
	str := func(k string) string {
		if v, ok := p[k].(string); ok {
			return v
		}
		return ""
	}
	pick := func(keys ...string) map[string]any {
		out := map[string]any{"event_id": ev.ID.String()}
		for _, k := range keys {
			if v, ok := p[k]; ok && v != nil {
				out[k] = v
			}
		}
		return out
	}
	switch ev.EventType {
	case "UserCreated":
		return str("user_id"), "signup", pick("locale")
	case "RideRequested":
		return str("customer_id"), "ride_requested", pick("category", "area", "attempt", "reassign")
	case "RideAssigned":
		return str("customer_id"), "ride_accepted", pick("ride_id", "driver_id")
	case "RideCompleted":
		return str("customer_id"), "ride_completed", pick("ride_id", "price_final_minor")
	case "RideCancelled":
		return str("customer_id"), "ride_cancelled", pick("ride_id", "by", "fee_minor")
	case "RideNoDriver":
		return str("ride_id"), "ride_no_driver", pick("ride_id") // klienti lexohet nga ride_id në panel
	case "RidePaymentSettled":
		return str("customer_id"), "payment_success", pick("ride_id", "method", "status")
	case "RidePaymentFailed":
		return str("customer_id"), "payment_failure", pick("ride_id", "method")
	case "WalletToppedUp":
		return str("user_id"), "wallet_topup", pick("amount_minor", "currency")
	case "RideReviewed":
		return str("reviewer_id"), "review", pick("ride_id", "role", "rating")
	case "SupportTicketCreated":
		return str("user_id"), "support_ticket", pick("category", "priority")
	case "SafetyReportCreated":
		return str("reporter_id"), "safety_report", pick("kind")
	case "DriverApplied":
		return str("driver_id"), "driver_applied", pick("categories")
	case "DriverApproved":
		return str("driver_id"), "driver_approved", pick("categories")
	}
	return "", "", nil
}

func (s *Service) Handle(_ context.Context, ev events.Event) error {
	id, name, props := Map(ev)
	if id == "" || name == "" {
		return nil
	}
	s.p.Capture(analytics.Event{DistinctID: id, Event: name, Properties: props, Timestamp: ev.OccurredAt})
	return nil
}
