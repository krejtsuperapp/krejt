package notifications

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/events"
)

func ev(t *testing.T, typ string, payload map[string]any) events.Event {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return events.Event{ID: uuid.New(), EventType: typ, AggregateType: "ride", AggregateID: "x", OccurredAt: time.Now(), Payload: raw}
}

func TestMap(t *testing.T) {
	cust, drv, ride, offer := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	got, err := Map(ev(t, "RideOffered", map[string]any{"ride_id": ride, "driver_id": drv, "offer_id": offer, "round": 1}))
	if err != nil || len(got) != 1 || got[0].UserID != drv || got[0].Category != "rides" || got[0].TTL != 20*time.Second ||
		got[0].DeepLink != "krejt://driver/offers/"+offer.String() || got[0].Priority != "high" {
		t.Fatalf("RideOffered: %+v err=%v", got, err)
	}
	got, _ = Map(ev(t, "RideAssigned", map[string]any{"ride_id": ride, "driver_id": drv, "customer_id": cust}))
	if len(got) != 1 || got[0].UserID != cust || got[0].TextKey != "notif.ride.assigned" {
		t.Fatalf("RideAssigned: %+v", got)
	}
	got, _ = Map(ev(t, "RideCompleted", map[string]any{"ride_id": ride, "driver_id": drv, "customer_id": cust, "price_final_minor": 430}))
	if len(got) != 2 || got[0].UserID != cust || got[1].UserID != drv || got[0].Params["price_minor"] != "430" {
		t.Fatalf("RideCompleted: %+v", got)
	}
	// anulim nga klienti → shoferi; anulim nga shoferi → asgjë (klienti merr "reassigning" nga RideRequested)
	got, _ = Map(ev(t, "RideCancelled", map[string]any{"ride_id": ride, "by": "customer", "driver_id": drv}))
	if len(got) != 1 || got[0].UserID != drv {
		t.Fatalf("RideCancelled by customer: %+v", got)
	}
	if got, _ = Map(ev(t, "RideCancelled", map[string]any{"ride_id": ride, "by": "customer"})); len(got) != 0 {
		t.Fatalf("RideCancelled pa shofer: %+v", got)
	}
	got, _ = Map(ev(t, "RideRequested", map[string]any{"ride_id": ride, "customer_id": cust, "reassign": true}))
	if len(got) != 1 || got[0].TextKey != "notif.ride.reassigning" {
		t.Fatalf("reassign: %+v", got)
	}
	if got, _ = Map(ev(t, "RideRequested", map[string]any{"ride_id": ride, "customer_id": cust, "attempt": 1})); len(got) != 0 {
		t.Fatalf("kërkesa e parë s'ka njoftim: %+v", got)
	}
	// no_driver: marrësi plotësohet nga baza (UserID bosh këtu)
	got, _ = Map(ev(t, "RideNoDriver", map[string]any{"ride_id": ride}))
	if len(got) != 1 || got[0].UserID != uuid.Nil || got[0].Params["ride_id"] != ride.String() {
		t.Fatalf("RideNoDriver: %+v", got)
	}
	// pagesa: vetëm wallet
	if got, _ = Map(ev(t, "RidePaymentSettled", map[string]any{"ride_id": ride, "customer_id": cust, "status": "cash", "method": "cash"})); len(got) != 0 {
		t.Fatalf("cash s'ka njoftim pagese: %+v", got)
	}
	got, _ = Map(ev(t, "RidePaymentFailed", map[string]any{"ride_id": ride, "customer_id": cust, "status": "failed", "method": "wallet"}))
	if len(got) != 1 || got[0].Category != "payments" || got[0].TextKey != "notif.payment.failed" {
		t.Fatalf("payment failed: %+v", got)
	}
	// siguria: vetëm kur ndryshon emaili
	got, _ = Map(ev(t, "UserProfileUpdated", map[string]any{"user_id": cust, "changed": []string{"email", "locale"}}))
	if len(got) != 1 || got[0].Category != "security" || got[0].Params["changed"] != "email, locale" {
		t.Fatalf("profile email: %+v", got)
	}
	if got, _ = Map(ev(t, "UserProfileUpdated", map[string]any{"user_id": cust, "changed": []string{"full_name"}})); len(got) != 0 {
		t.Fatalf("profile name: %+v", got)
	}
	// wallet: mbushja njofton pronarin; dokumenti: vetëm refuzimi
	got, _ = Map(ev(t, "WalletToppedUp", map[string]any{"user_id": cust, "amount_minor": 2000, "currency": "EUR", "intent_id": uuid.New()}))
	if len(got) != 1 || got[0].Category != "wallet" || got[0].Params["amount_minor"] != "2000" || got[0].DeepLink != "krejt://wallet" {
		t.Fatalf("WalletToppedUp: %+v", got)
	}
	got, _ = Map(ev(t, "DriverDocumentReviewed", map[string]any{"driver_id": drv, "status": "rejected", "type": "insurance", "reason": "e paqartë"}))
	if len(got) != 1 || got[0].TextKey != "notif.driver.document_rejected" || got[0].Params["doc_type"] != "insurance" {
		t.Fatalf("document rejected: %+v", got)
	}
	if got, _ = Map(ev(t, "DriverDocumentReviewed", map[string]any{"driver_id": drv, "status": "approved", "type": "insurance"})); len(got) != 0 {
		t.Fatalf("document approved s'ka njoftim: %+v", got)
	}
	if got, _ = Map(ev(t, "SomethingElse", map[string]any{})); len(got) != 0 {
		t.Fatalf("e panjohur: %+v", got)
	}
	// çdo çelës i përdorur nga Map ekziston në tekste
	for _, key := range []string{"notif.ride.offer", "notif.ride.assigned", "notif.ride.arrived", "notif.ride.started", "notif.ride.completed",
		"notif.ride.completed.driver", "notif.ride.cancelled.customer", "notif.ride.reassigning", "notif.ride.no_driver",
		"notif.payment.paid", "notif.payment.failed", "notif.driver.approved", "notif.driver.suspended", "notif.security.profile_changed",
		"notif.wallet.topup", "notif.driver.document_rejected", "notif.support.reply", "notif.chat.message", "notif.merchant.active", "notif.merchant.suspended"} {
		if _, ok := texts[key]; !ok {
			t.Errorf("mungon teksti për %s", key)
		}
	}
}
