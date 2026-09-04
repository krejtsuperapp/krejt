package notifications

import (
	"testing"

	"github.com/google/uuid"
)

// Shërbimet ndjekin të njëjtën rregull: çdo hap që e pret dikush bëhet njoftim, dhe kurrë nuk
// njoftohet ai që sapo e bëri hapin vetë.
func TestMapServices(t *testing.T) {
	cust, prov, req := uuid.New(), uuid.New(), uuid.New()

	// Oferta: marrësi është klienti, por ngjarja mban vetëm mjeshtrin — id-ja plotësohet nga baza.
	got, err := Map(ev(t, "ServiceOffered", map[string]any{"request_id": req, "provider_id": prov, "price_minor": 2500}))
	if err != nil || len(got) != 1 || got[0].UserID != uuid.Nil || got[0].TextKey != "notif.service.offer" {
		t.Fatalf("ofertë: %+v %v", got, err)
	}
	if got[0].Params["price_minor"] != "2500" {
		t.Fatalf("çmimi: %+v", got[0].Params)
	}

	got, _ = Map(ev(t, "ServiceBooked", map[string]any{"request_id": req, "code": "K7F3QA", "customer_id": cust, "provider_id": prov, "price_minor": 2500}))
	if len(got) != 1 || got[0].UserID != prov || got[0].TextKey != "notif.service.booked" {
		t.Fatalf("rezervimi → mjeshtri: %+v", got)
	}

	for typ, key := range map[string]string{
		"ServiceStarted":   "notif.service.started",
		"ServiceCompleted": "notif.service.completed",
		"ServiceReleased":  "notif.service.released",
	} {
		got, _ = Map(ev(t, typ, map[string]any{"request_id": req, "code": "K7F3QA", "customer_id": cust, "provider_id": prov}))
		if len(got) != 1 || got[0].UserID != cust || got[0].TextKey != key {
			t.Fatalf("%s: %+v", typ, got)
		}
	}

	// Anulimi nga klienti njofton mjeshtrin; klienti jo, se e bëri vetë.
	got, _ = Map(ev(t, "ServiceCancelled", map[string]any{"request_id": req, "code": "K7F3QA", "customer_id": cust, "provider_id": prov, "by": "customer"}))
	if len(got) != 1 || got[0].UserID != prov {
		t.Fatalf("anulimi: %+v", got)
	}

	got, _ = Map(ev(t, "ServiceProviderStatusChanged", map[string]any{"provider_id": prov, "status": "approved"}))
	if len(got) != 1 || got[0].UserID != prov || got[0].TextKey != "notif.provider.approved" {
		t.Fatalf("miratimi: %+v", got)
	}
	got, _ = Map(ev(t, "ServiceProviderStatusChanged", map[string]any{"provider_id": prov, "status": "pending"}))
	if len(got) != 0 {
		t.Fatalf("kthimi në pending nuk njofton: %+v", got)
	}
}

func TestServiceTextsExist(t *testing.T) {
	for _, key := range []string{
		"notif.service.offer", "notif.service.booked", "notif.service.started", "notif.service.completed",
		"notif.service.released", "notif.service.cancelled.provider",
		"notif.provider.approved", "notif.provider.suspended",
	} {
		if _, ok := texts[key]; !ok {
			t.Errorf("mungon teksti për %s", key)
		}
	}
}
