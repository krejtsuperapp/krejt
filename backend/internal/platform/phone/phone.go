// Package phone mban rregullin e vetëm për numrat E.164.
//
// Ishte i shkruar dy herë — te kyçja dhe te regjistrimi i shoferëve — dhe dy kopje të një
// rregulli ndahen herët a vonë: njëra pranon çka tjetra refuzon, dhe dallimi del si defekt.
package phone

import "regexp"

// E.164: shenja plus, shifra e parë jo zero, gjithsej 7–15 shifra.
var re = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// Valid thotë nëse numri është i shkruar në formën E.164.
func Valid(s string) bool { return re.MatchString(s) }
