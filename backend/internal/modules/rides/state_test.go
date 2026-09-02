package rides

import "testing"

func TestTransitions(t *testing.T) {
	allowed := [][2]string{
		{StateMatching, StateAssigned}, {StateMatching, StateCancelled}, {StateMatching, StateNoDriver},
		{StateAssigned, StateArrived}, {StateAssigned, StateCancelled}, {StateAssigned, StateMatching},
		{StateArrived, StateInProgress}, {StateArrived, StateCancelled}, {StateArrived, StateMatching},
		{StateInProgress, StateCompleted},
	}
	for _, a := range allowed {
		if !CanTransition(a[0], a[1]) {
			t.Errorf("%s → %s duhej lejuar", a[0], a[1])
		}
	}
	forbidden := [][2]string{
		{StateMatching, StateCompleted}, {StateMatching, StateInProgress}, {StateAssigned, StateCompleted},
		{StateInProgress, StateCancelled}, {StateInProgress, StateMatching}, {StateCompleted, StateMatching},
		{StateCancelled, StateAssigned}, {StateNoDriver, StateAssigned}, {"", StateMatching},
	}
	for _, f := range forbidden {
		if CanTransition(f[0], f[1]) {
			t.Errorf("%s → %s duhej ndaluar", f[0], f[1])
		}
	}
}

func TestActiveTerminal(t *testing.T) {
	for _, s := range []string{StateMatching, StateAssigned, StateArrived, StateInProgress} {
		if !IsActive(s) || IsTerminal(s) {
			t.Errorf("%s: aktiv", s)
		}
	}
	for _, s := range []string{StateCompleted, StateCancelled, StateNoDriver} {
		if IsActive(s) || !IsTerminal(s) {
			t.Errorf("%s: terminal", s)
		}
	}
}
