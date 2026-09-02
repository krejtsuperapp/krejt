package notifications

import (
	"fmt"
	"strings"
)

// Tekstet e push-it sipas gjuhës (§2: sq/en/de). Kutia në aplikacion merr çelësat + parametrat dhe
// i rendit vetë; push-i duhet të mbajë tekst të gatshëm sepse sistemi operativ e shfaq direkt.
var texts = map[string]map[string][2]string{
	"notif.ride.offer": {
		"sq": {"Kërkesë e re për udhëtim", "{distance_km} km larg · {price} · përgjigju brenda {ttl_s} s"},
		"en": {"New ride request", "{distance_km} km away · {price} · respond within {ttl_s} s"},
		"de": {"Neue Fahrtanfrage", "{distance_km} km entfernt · {price} · innerhalb von {ttl_s} s antworten"},
	},
	"notif.ride.assigned": {
		"sq": {"Shoferi u caktua", "{driver} me {vehicle} ({plate}) po vjen te ti."},
		"en": {"Driver assigned", "{driver} in a {vehicle} ({plate}) is on the way."},
		"de": {"Fahrer zugewiesen", "{driver} mit {vehicle} ({plate}) ist unterwegs."},
	},
	"notif.ride.arrived": {
		"sq": {"Shoferi mbërriti", "{driver} po të pret te pika e marrjes."},
		"en": {"Your driver has arrived", "{driver} is waiting at the pickup point."},
		"de": {"Fahrer ist da", "{driver} wartet am Abholpunkt."},
	},
	"notif.ride.started": {
		"sq": {"Udhëtimi filloi", "Rrugë të mbarë! Destinacioni: {dropoff}."},
		"en": {"Ride started", "Have a good trip! Destination: {dropoff}."},
		"de": {"Fahrt gestartet", "Gute Fahrt! Ziel: {dropoff}."},
	},
	"notif.ride.completed": {
		"sq": {"Udhëtimi përfundoi", "Gjithsej {price}. Faleminderit që udhëtove me KREJT."},
		"en": {"Ride completed", "Total {price}. Thanks for riding with KREJT."},
		"de": {"Fahrt beendet", "Gesamt {price}. Danke, dass du mit KREJT gefahren bist."},
	},
	"notif.ride.completed.driver": {
		"sq": {"Udhëtimi u mbyll", "Fitimi yt: {earnings}."},
		"en": {"Ride closed", "Your earnings: {earnings}."},
		"de": {"Fahrt abgeschlossen", "Dein Verdienst: {earnings}."},
	},
	"notif.ride.cancelled.customer": {
		"sq": {"Udhëtimi u anulua", "Klienti e anuloi udhëtimin."},
		"en": {"Ride cancelled", "The customer cancelled the ride."},
		"de": {"Fahrt storniert", "Der Kunde hat die Fahrt storniert."},
	},
	"notif.ride.cancelled.fee": {
		"sq": {"Udhëtimi u anulua", "U aplikua tarifa e anulimit prej {fee}."},
		"en": {"Ride cancelled", "A cancellation fee of {fee} was applied."},
		"de": {"Fahrt storniert", "Eine Stornogebühr von {fee} wurde berechnet."},
	},
	"notif.ride.reassigning": {
		"sq": {"Po kërkojmë shofer tjetër", "Shoferi nuk mund të vijë. Po të gjejmë një tjetër."},
		"en": {"Finding another driver", "Your driver can't make it. We're finding another one."},
		"de": {"Wir suchen einen anderen Fahrer", "Dein Fahrer kann nicht kommen. Wir suchen einen anderen."},
	},
	"notif.ride.no_driver": {
		"sq": {"Nuk u gjet shofer", "Na vjen keq — provo sërish pas pak."},
		"en": {"No driver found", "Sorry — please try again in a moment."},
		"de": {"Kein Fahrer gefunden", "Leider nicht — bitte versuche es gleich noch einmal."},
	},
	"notif.payment.paid": {
		"sq": {"Pagesa u konfirmua", "{price} u pagua nga KREJT Wallet."},
		"en": {"Payment confirmed", "{price} was paid from your KREJT Wallet."},
		"de": {"Zahlung bestätigt", "{price} wurde aus deinem KREJT Wallet bezahlt."},
	},
	"notif.payment.failed": {
		"sq": {"Pagesa dështoi", "Wallet-i nuk kishte mjaftueshëm. Mbushe wallet-in që të vazhdosh."},
		"en": {"Payment failed", "Your wallet balance was too low. Top up to continue."},
		"de": {"Zahlung fehlgeschlagen", "Dein Wallet-Guthaben reichte nicht. Lade auf, um fortzufahren."},
	},
	"notif.wallet.topup": {
		"sq": {"Wallet-i u mbush", "{amount} u shtua në KREJT Wallet."},
		"en": {"Wallet topped up", "{amount} was added to your KREJT Wallet."},
		"de": {"Wallet aufgeladen", "{amount} wurde deinem KREJT Wallet gutgeschrieben."},
	},
	"notif.driver.document_rejected": {
		"sq": {"Dokument i refuzuar", "{doc_type}: {reason}. Ngarkoje sërish."},
		"en": {"Document rejected", "{doc_type}: {reason}. Please upload it again."},
		"de": {"Dokument abgelehnt", "{doc_type}: {reason}. Bitte lade es erneut hoch."},
	},
	"notif.chat.message": {
		"sq": {"Mesazh i ri", "{preview}"},
		"en": {"New message", "{preview}"},
		"de": {"Neue Nachricht", "{preview}"},
	},
	"notif.support.reply": {
		"sq": {"Përgjigje nga Mbështetja", "{subject}: ke një përgjigje të re."},
		"en": {"Reply from Support", "{subject}: you have a new reply."},
		"de": {"Antwort vom Support", "{subject}: du hast eine neue Antwort."},
	},
	"notif.driver.approved": {
		"sq": {"Je miratuar si shofer", "Mund të dalësh online dhe të marrësh udhëtime."},
		"en": {"You're approved as a driver", "You can go online and start taking rides."},
		"de": {"Du bist als Fahrer freigeschaltet", "Du kannst online gehen und Fahrten annehmen."},
	},
	"notif.driver.suspended": {
		"sq": {"Llogaria e shoferit u pezullua", "Kontakto mbështetjen për detaje."},
		"en": {"Driver account suspended", "Contact support for details."},
		"de": {"Fahrerkonto gesperrt", "Kontaktiere den Support für Details."},
	},
	"notif.security.profile_changed": {
		"sq": {"Profili u ndryshua", "U ndryshua: {changed}. Nëse s'ke qenë ti, siguro llogarinë."},
		"en": {"Profile changed", "Changed: {changed}. If this wasn't you, secure your account."},
		"de": {"Profil geändert", "Geändert: {changed}. Falls du das nicht warst, sichere dein Konto."},
	},
}

// Render — titulli dhe trupi në gjuhën e dhënë (sq si rezervë), me parametrat e zëvendësuar.
func Render(key, locale string, params map[string]string) (title, body string, ok bool) {
	t, ok := texts[key]
	if !ok {
		return "", "", false
	}
	pair, found := t[locale]
	if !found {
		pair = t["sq"]
	}
	return substitute(pair[0], params), substitute(pair[1], params), true
}

func substitute(s string, params map[string]string) string {
	for k, v := range params {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// FormatMoney — "12,40 €" (sq/de) ose "€12.40" (en).
func FormatMoney(minor int64, currency, locale string) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	whole, cents := minor/100, minor%100
	sym := currency
	if currency == "EUR" {
		sym = "€"
	}
	var s string
	if locale == "en" {
		s = fmt.Sprintf("%s%d.%02d", sym, whole, cents)
	} else {
		s = fmt.Sprintf("%d,%02d %s", whole, cents, sym)
	}
	if neg {
		return "-" + s
	}
	return s
}
