// Package orders — porositë e ushqimit/marketit (§19, §21): checkout me çmim server-side (catalog.Price),
// makinë gjendjesh e dokumentuar, dispatch i korrierit pas "gati", shlyerje në ledger si te udhëtimet.
//
//	pending_merchant ─► accepted ─► preparing ─► ready ─► courier_assigned ─► picked_up ─► delivered
//	       │               │            │          │
//	       ├──► rejected   └────────────┴──────────┴──► cancelled (klienti para pranimit; merchant/ops më vonë)
//	       └──► cancelled (klienti, brenda dritares)
//
// Për `pickup` dhe `merchant_delivers`: nga `ready` kalohet direkt në `delivered` (merchant-i e konfirmon).
package orders

const (
	StatePendingMerchant = "pending_merchant"
	StateAccepted        = "accepted"
	StatePreparing       = "preparing"
	StateReady           = "ready"
	StateCourierAssigned = "courier_assigned"
	StatePickedUp        = "picked_up"
	StateDelivered       = "delivered"
	StateCancelled       = "cancelled"
	StateRejected        = "rejected"
)

var transitions = map[string][]string{
	StatePendingMerchant: {StateAccepted, StateRejected, StateCancelled},
	StateAccepted:        {StatePreparing, StateCancelled},
	StatePreparing:       {StateReady, StateCancelled},
	StateReady:           {StateCourierAssigned, StateDelivered, StateCancelled},
	StateCourierAssigned: {StatePickedUp, StateReady, StateCancelled}, // kthimi në ready = ricaktim korrieri
	StatePickedUp:        {StateDelivered},
}

func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// IsActive — porosi që zë merchant-in/korrierin.
func IsActive(state string) bool {
	switch state {
	case StatePendingMerchant, StateAccepted, StatePreparing, StateReady, StateCourierAssigned, StatePickedUp:
		return true
	}
	return false
}

func IsTerminal(state string) bool {
	return state == StateDelivered || state == StateCancelled || state == StateRejected
}

// CustomerCanCancel — klienti anulon vetëm derisa merchant-i të fillojë përgatitjen.
func CustomerCanCancel(state string) bool {
	return state == StatePendingMerchant || state == StateAccepted
}
