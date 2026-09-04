package services

import "testing"

func TestTransitions(t *testing.T) {
	ok := [][2]string{
		{StateOpen, StateBooked}, {StateOpen, StateCancelled}, {StateOpen, StateNoOffers},
		{StateBooked, StateInProgress}, {StateBooked, StateOpen}, {StateBooked, StateCancelled},
		{StateInProgress, StateCompleted}, {StateInProgress, StateCancelled},
	}
	for _, c := range ok {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s duhet të lejohet", c[0], c[1])
		}
	}
	bad := [][2]string{
		{StateOpen, StateInProgress}, {StateOpen, StateCompleted}, {StateInProgress, StateOpen},
		{StateCompleted, StateInProgress}, {StateCancelled, StateOpen}, {StateNoOffers, StateBooked},
	}
	for _, c := range bad {
		if CanTransition(c[0], c[1]) {
			t.Errorf("%s → %s nuk duhet të lejohet", c[0], c[1])
		}
	}
}

func TestCustomerCanCancel(t *testing.T) {
	if !CustomerCanCancel(StateOpen) || !CustomerCanCancel(StateBooked) {
		t.Fatal("anulimi lejohet para nisjes së punës")
	}
	if CustomerCanCancel(StateInProgress) || CustomerCanCancel(StateCompleted) {
		t.Fatal("pas nisjes rruga është mbështetja, jo anulimi")
	}
}

func TestCommission(t *testing.T) {
	// 15% e 40,00 € = 6,00 €
	if got := Commission(4000, 1500); got != 600 {
		t.Fatalf("got %d", got)
	}
	if got := Commission(4001, 1500); got != 600 {
		t.Fatalf("rrumbullakim: %d", got)
	}
	if got := Commission(1000, 0); got != 0 {
		t.Fatalf("pa komision: %d", got)
	}
}

func TestActiveAndTerminal(t *testing.T) {
	for _, s := range []string{StateOpen, StateBooked, StateInProgress} {
		if !IsActive(s) || IsTerminal(s) {
			t.Errorf("%s duhet aktive", s)
		}
	}
	for _, s := range []string{StateCompleted, StateCancelled, StateNoOffers} {
		if IsActive(s) || !IsTerminal(s) {
			t.Errorf("%s duhet përfundimtare", s)
		}
	}
}
