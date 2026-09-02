package analytics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/providers/analytics"
)

type fake struct{ got []analytics.Event }

func (f *fake) Capture(ev analytics.Event)  { f.got = append(f.got, ev) }
func (f *fake) Close(context.Context) error { return nil }

func ev(t *testing.T, typ string, payload map[string]any) events.Event {
	t.Helper()
	raw, _ := json.Marshal(payload)
	return events.Event{ID: uuid.New(), EventType: typ, OccurredAt: time.Now(), Payload: raw}
}

func TestMapAndHandle(t *testing.T) {
	f := &fake{}
	s := New(f)
	cust := uuid.New()
	_ = s.Handle(context.Background(), ev(t, "RideRequested", map[string]any{"customer_id": cust, "category": "economy", "area": "prishtina", "attempt": 1}))
	_ = s.Handle(context.Background(), ev(t, "UserProfileUpdated", map[string]any{"user_id": cust}))
	_ = s.Handle(context.Background(), ev(t, "RidePaymentFailed", map[string]any{"customer_id": cust, "ride_id": uuid.New(), "method": "wallet"}))
	if len(f.got) != 2 || f.got[0].Event != "ride_requested" || f.got[0].DistinctID != cust.String() || f.got[0].Properties["category"] != "economy" {
		t.Fatalf("got %+v", f.got)
	}
	if f.got[1].Event != "payment_failure" || f.got[1].Properties["method"] != "wallet" {
		t.Fatalf("payment: %+v", f.got[1])
	}
	// privatësia: asnjë telefon/email/emër nuk kalon në veti edhe nëse payload-i i ka
	_ = s.Handle(context.Background(), ev(t, "UserCreated", map[string]any{"user_id": cust, "locale": "sq", "phone": "+38344000000"}))
	last := f.got[len(f.got)-1]
	if last.Event != "signup" || last.Properties["phone"] != nil || last.Properties["locale"] != "sq" {
		t.Fatalf("signup: %+v", last)
	}
}
