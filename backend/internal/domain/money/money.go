// Package money — paraja gjithmonë si numër i plotë në njësi minore (cent). Asnjë float (§5, §23).
package money

import (
	"errors"
	"fmt"
)

// Minor është shuma në cent (EUR: 1240 = 12,40 €).
type Minor int64

type Amount struct {
	Minor    Minor
	Currency string // "EUR" në V1; kolona ekziston për të ardhmen (§84)
}

var ErrCurrencyMismatch = errors.New("money: currency mismatch")

func EUR(minor int64) Amount { return Amount{Minor: Minor(minor), Currency: "EUR"} }

func (a Amount) Add(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, ErrCurrencyMismatch
	}
	return Amount{Minor: a.Minor + b.Minor, Currency: a.Currency}, nil
}

func (a Amount) Sub(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, ErrCurrencyMismatch
	}
	return Amount{Minor: a.Minor - b.Minor, Currency: a.Currency}, nil
}

// Percent llogarit pjesën në pikë bazë (bps): 1800 bps = 18 %. Rrumbullakim "half up".
func (a Amount) Percent(bps int64) Amount {
	v := int64(a.Minor)*bps + 5000
	if v < 0 {
		v -= 10000 - 1
	}
	return Amount{Minor: Minor(v / 10000), Currency: a.Currency}
}

func (a Amount) IsNegative() bool { return a.Minor < 0 }
func (a Amount) IsZero() bool     { return a.Minor == 0 }

// String formaton për log dhe teste (jo për UI — UI-ja formaton sipas locale, §2).
func (a Amount) String() string {
	sign := ""
	m := int64(a.Minor)
	if m < 0 {
		sign = "-"
		m = -m
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, m/100, m%100, a.Currency)
}
