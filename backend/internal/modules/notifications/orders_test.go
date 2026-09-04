package notifications

import (
	"testing"

	"github.com/google/uuid"
)

// Porositë dhe pakot ndjekin të njëjtën rregull si udhëtimet: çdo hap që e pret dikush
// bëhet njoftim, dhe asnjë njoftim nuk shkon te ai që sapo e shkaktoi vetë hapin.
func TestMapOrders(t *testing.T) {
	cust, owner, courier, order, offer := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()

	got, err := Map(ev(t, "OrderCreated", map[string]any{"order_id": order, "code": "K7F3QA", "customer_id": cust, "owner_id": owner, "total_minor": 950}))
	if err != nil || len(got) != 1 || got[0].UserID != owner || got[0].TextKey != "notif.order.new" {
		t.Fatalf("porosi e re → partneri: %+v %v", got, err)
	}
	if got[0].Params["total_minor"] != "950" {
		t.Fatalf("totali: %+v", got[0].Params)
	}

	got, _ = Map(ev(t, "OrderAccepted", map[string]any{"order_id": order, "code": "K7F3QA", "customer_id": cust}))
	if len(got) != 1 || got[0].UserID != cust || got[0].TextKey != "notif.order.accepted" {
		t.Fatalf("pranimi → klienti: %+v", got)
	}

	// "Gati" me korrier nuk i thotë asgjë klientit: ai pret dorëzimin, jo marrjen.
	if got, _ = Map(ev(t, "OrderReady", map[string]any{"order_id": order, "customer_id": cust, "fulfillment": "courier"})); len(got) != 0 {
		t.Fatalf("gati me korrier: %+v", got)
	}
	if got, _ = Map(ev(t, "OrderReady", map[string]any{"order_id": order, "customer_id": cust, "fulfillment": "pickup"})); len(got) != 1 {
		t.Fatalf("gati për marrje në vend: %+v", got)
	}

	got, _ = Map(ev(t, "OrderOffered", map[string]any{"order_id": order, "courier_id": courier, "offer_id": offer}))
	if len(got) != 1 || got[0].UserID != courier || got[0].TextKey != "notif.order.offer" || got[0].TTL == 0 {
		t.Fatalf("ofertë dorëzimi → korrieri: %+v", got)
	}

	got, _ = Map(ev(t, "OrderPickedUp", map[string]any{"order_id": order, "code": "K7F3QA", "customer_id": cust, "courier_id": courier}))
	if len(got) != 1 || got[0].UserID != cust || got[0].TextKey != "notif.order.on_the_way" {
		t.Fatalf("u mor → klienti: %+v", got)
	}

	// Anulimi nga klienti: korrieri njoftohet, klienti jo (e bëri vetë).
	got, _ = Map(ev(t, "OrderCancelled", map[string]any{"order_id": order, "code": "K7F3QA", "customer_id": cust, "courier_id": courier, "by": "customer"}))
	if len(got) != 1 || got[0].UserID != courier {
		t.Fatalf("anulim nga klienti: %+v", got)
	}
	got, _ = Map(ev(t, "OrderCancelled", map[string]any{"order_id": order, "code": "K7F3QA", "customer_id": cust, "by": "merchant"}))
	if len(got) != 1 || got[0].UserID != cust {
		t.Fatalf("anulim nga partneri: %+v", got)
	}
}

func TestMapParcels(t *testing.T) {
	cust, courier, parcel, offer := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	got, err := Map(ev(t, "ParcelOffered", map[string]any{"parcel_id": parcel, "courier_id": courier, "offer_id": offer}))
	if err != nil || len(got) != 1 || got[0].UserID != courier || got[0].TextKey != "notif.parcel.offer" {
		t.Fatalf("ofertë pakoje: %+v %v", got, err)
	}

	for typ, key := range map[string]string{
		"ParcelCourierAssigned": "notif.parcel.courier",
		"ParcelPickedUp":        "notif.parcel.picked_up",
		"ParcelDelivered":       "notif.parcel.delivered",
	} {
		got, _ = Map(ev(t, typ, map[string]any{"parcel_id": parcel, "code": "K7F3QA", "customer_id": cust}))
		if len(got) != 1 || got[0].UserID != cust || got[0].TextKey != key {
			t.Fatalf("%s: %+v", typ, got)
		}
	}

	// Pa korrier: marrësi plotësohet nga baza, ndaj Map e lë id-në bosh.
	got, _ = Map(ev(t, "ParcelNoCourier", map[string]any{"parcel_id": parcel}))
	if len(got) != 1 || got[0].UserID != uuid.Nil || got[0].TextKey != "notif.parcel.no_courier" {
		t.Fatalf("pa korrier: %+v", got)
	}

	got, _ = Map(ev(t, "ParcelCancelled", map[string]any{"parcel_id": parcel, "code": "K7F3QA", "courier_id": courier, "by": "customer"}))
	if len(got) != 1 || got[0].UserID != courier {
		t.Fatalf("anulim pakoje: %+v", got)
	}
}

// Çdo çelës që përdor Map duhet të ketë tekst në të tria gjuhët (texts_test e kontrollon plotësinë).
func TestOrderAndParcelTextsExist(t *testing.T) {
	for _, key := range []string{
		"notif.order.new", "notif.order.accepted", "notif.order.preparing", "notif.order.ready", "notif.order.rejected",
		"notif.order.offer", "notif.order.courier", "notif.order.on_the_way", "notif.order.delivered",
		"notif.order.cancelled", "notif.order.cancelled.courier",
		"notif.parcel.offer", "notif.parcel.courier", "notif.parcel.picked_up", "notif.parcel.delivered",
		"notif.parcel.no_courier", "notif.parcel.cancelled.courier",
	} {
		if _, ok := texts[key]; !ok {
			t.Errorf("mungon teksti për %s", key)
		}
	}
}
