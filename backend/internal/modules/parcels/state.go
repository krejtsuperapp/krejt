// Package parcels — pakot brenda qytetit (§21): kërkesë me çmim server-side, dispatch i korrierit,
// marrje dhe dorëzim me kode, shlyerje në ledger si te udhëtimet.
//
//	requested ─► courier_assigned ─► picked_up ─► delivered
//	    │               │
//	    ├──► no_courier  └──► requested (korrieri heq dorë para marrjes)
//	    └──► cancelled (klienti, para marrjes)
package parcels

import "math"

const (
	StateRequested       = "requested"
	StateCourierAssigned = "courier_assigned"
	StatePickedUp        = "picked_up"
	StateDelivered       = "delivered"
	StateCancelled       = "cancelled"
	StateNoCourier       = "no_courier"
)

var transitions = map[string][]string{
	StateRequested:       {StateCourierAssigned, StateCancelled, StateNoCourier},
	StateCourierAssigned: {StatePickedUp, StateRequested, StateCancelled},
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

func IsActive(state string) bool {
	return state == StateRequested || state == StateCourierAssigned || state == StatePickedUp
}

func IsTerminal(state string) bool {
	return state == StateDelivered || state == StateCancelled || state == StateNoCourier
}

// CustomerCanCancel — derisa korrieri ta ketë marrë pakon.
func CustomerCanCancel(state string) bool {
	return state == StateRequested || state == StateCourierAssigned
}

// Pricing — rregulli i çmimit për një madhësi; vjen nga tabela parcel_pricing.
type Pricing struct {
	Size         string
	BaseMinor    int64
	PerKmMinor   int64
	CommissionBP int
	Currency     string
}

// Price — bazë + për km (proporcional), i rrumbullakuar në 10 cent; kurrë nën bazën.
func Price(p Pricing, distanceM int) int64 {
	km := float64(distanceM) / 1000.0
	raw := float64(p.BaseMinor) + float64(p.PerKmMinor)*km
	rounded := int64(math.Round(raw/10.0)) * 10
	if rounded < p.BaseMinor {
		return p.BaseMinor
	}
	return rounded
}

// Commission — pjesa e platformës nga çmimi, në cent (rrumbullakim komercial).
func Commission(priceMinor int64, bp int) int64 {
	return (priceMinor*int64(bp) + 5000) / 10000
}

var sizes = map[string]bool{"s": true, "m": true, "l": true}

func ValidSize(s string) bool { return sizes[s] }
