// Package services — shërbimet me mjeshtër (§22): klienti përshkruan punën, mjeshtrit e miratuar
// dërgojnë oferta me çmimin e tyre, klienti zgjedh një. Platforma nuk shpik tarifa: çmimi është ai i
// ofertës së pranuar, dhe komisioni mbahet mbi të, si te udhëtimet.
//
//	open ─► booked ─► in_progress ─► completed
//	  │        │            │
//	  ├────────┴────────────┴──► cancelled
//	  └──► no_offers (asnjë ofertë brenda afatit)
package services

const (
	StateOpen       = "open"
	StateBooked     = "booked"
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
	StateCancelled  = "cancelled"
	StateNoOffers   = "no_offers"
)

var transitions = map[string][]string{
	StateOpen:       {StateBooked, StateCancelled, StateNoOffers},
	StateBooked:     {StateInProgress, StateOpen, StateCancelled}, // kthimi në open = mjeshtri hoqi dorë
	StateInProgress: {StateCompleted, StateCancelled},
}

func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

func IsActive(state string) bool {
	return state == StateOpen || state == StateBooked || state == StateInProgress
}

func IsTerminal(state string) bool {
	return state == StateCompleted || state == StateCancelled || state == StateNoOffers
}

// CustomerCanCancel — derisa puna të ketë nisur; pas nisjes rruga është mbështetja.
func CustomerCanCancel(state string) bool {
	return state == StateOpen || state == StateBooked
}

// Commission — pjesa e platformës nga çmimi i ofertës, në cent (rrumbullakim komercial).
func Commission(priceMinor int64, bp int) int64 {
	return (priceMinor*int64(bp) + 5000) / 10000
}
