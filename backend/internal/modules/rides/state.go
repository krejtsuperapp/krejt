// Package rides — udhëtimet (§18): makinë gjendjesh e dokumentuar, çmim i paracaktuar nga quote,
// anulim me politikë tarifash, ricaktim kur shoferi anulon, pagesë cash/wallet në përfundim.
//
//	matching ──► assigned ──► arrived ──► in_progress ──► completed
//	   │  ▲          │            │
//	   │  └──────────┴────────────┘   (shoferi anulon → ricaktim)
//	   ├──► no_driver                 (skadon kërkimi)
//	   └──► cancelled  ◄── assigned/arrived (klienti anulon; tarifë pas periudhës së hirit)
package rides

const (
	StateMatching   = "matching"
	StateAssigned   = "assigned"
	StateArrived    = "arrived"
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
	StateCancelled  = "cancelled"
	StateNoDriver   = "no_driver"
)

var transitions = map[string][]string{
	StateMatching:   {StateAssigned, StateCancelled, StateNoDriver},
	StateAssigned:   {StateArrived, StateCancelled, StateMatching},
	StateArrived:    {StateInProgress, StateCancelled, StateMatching},
	StateInProgress: {StateCompleted},
}

// CanTransition — kalimi lejohet vetëm sipas tabelës (kurrë "completed" nga "matching", etj.).
func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// IsActive — udhëtim që zë klientin (dhe shoferin kur ka).
func IsActive(state string) bool {
	switch state {
	case StateMatching, StateAssigned, StateArrived, StateInProgress:
		return true
	}
	return false
}

// IsTerminal — nuk ndryshon më.
func IsTerminal(state string) bool {
	return state == StateCompleted || state == StateCancelled || state == StateNoDriver
}
