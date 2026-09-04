package parcels

import "testing"

func TestPriceRoundsToTenCentsAndNeverBelowBase(t *testing.T) {
	p := Pricing{Size: "s", BaseMinor: 150, PerKmMinor: 40, CommissionBP: 2000, Currency: "EUR"}
	cases := map[int]int64{0: 150, 100: 150, 1000: 190, 2500: 250, 3333: 280, 10000: 550}
	for dist, want := range cases {
		if got := Price(p, dist); got != want {
			t.Errorf("distance %d: got %d, want %d", dist, got, want)
		}
	}
}

func TestCommission(t *testing.T) {
	if got := Commission(250, 2000); got != 50 {
		t.Fatalf("got %d", got)
	}
	if got := Commission(255, 2000); got != 51 {
		t.Fatalf("got %d", got)
	}
	if got := Commission(100, 0); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestTransitions(t *testing.T) {
	ok := [][2]string{
		{StateRequested, StateCourierAssigned}, {StateRequested, StateCancelled}, {StateRequested, StateNoCourier},
		{StateCourierAssigned, StatePickedUp}, {StateCourierAssigned, StateRequested}, {StateCourierAssigned, StateCancelled},
		{StatePickedUp, StateDelivered},
	}
	for _, c := range ok {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s duhet të lejohet", c[0], c[1])
		}
	}
	bad := [][2]string{
		{StatePickedUp, StateCancelled}, {StatePickedUp, StateRequested}, {StateDelivered, StateRequested}, {StateCancelled, StateRequested},
	}
	for _, c := range bad {
		if CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s nuk duhet të lejohet", c[0], c[1])
		}
	}
	if !CustomerCanCancel(StateCourierAssigned) || CustomerCanCancel(StatePickedUp) {
		t.Fatal("anulimi lejohet vetëm para marrjes")
	}
	if !ValidSize("m") || ValidSize("xl") {
		t.Fatal("madhësitë: s, m, l")
	}
}

func TestForCourierHidesCodes(t *testing.T) {
	p := &Parcel{PickupCode: "1234", DeliveryCode: "5678", Courier: &CourierCard{Name: "X"}}
	c := forCourier(p)
	if c.PickupCode != "" || c.DeliveryCode != "" || c.Courier != nil {
		t.Fatalf("korrieri nuk duhet t'i shohë kodet: %+v", c)
	}
	if p.PickupCode != "1234" {
		t.Fatal("origjinali nuk duhet të ndryshojë")
	}
	if len(newDigits()) != 4 || len(newCode()) != 6 {
		t.Fatal("gjatësitë e kodeve")
	}
}
