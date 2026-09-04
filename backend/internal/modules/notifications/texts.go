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
	"notif.order.new": {
		"sq": {"Porosi e re", "Kodi {code} · {total}. Prano ose refuzo te paneli."},
		"en": {"New order", "Code {code} · {total}. Accept or reject in the panel."},
		"de": {"Neue Bestellung", "Code {code} · {total}. Im Panel annehmen oder ablehnen."},
	},
	"notif.order.accepted": {
		"sq": {"Porosia u pranua", "Kuzhina e mori porosinë {code}."},
		"en": {"Order accepted", "The kitchen has your order {code}."},
		"de": {"Bestellung angenommen", "Die Küche hat deine Bestellung {code}."},
	},
	"notif.order.preparing": {
		"sq": {"Po përgatitet", "Porosia {code} është në përgatitje."},
		"en": {"Being prepared", "Order {code} is being prepared."},
		"de": {"Wird zubereitet", "Bestellung {code} wird zubereitet."},
	},
	"notif.order.ready": {
		"sq": {"Gati për marrje", "Porosia {code} të pret te vendi."},
		"en": {"Ready for pickup", "Order {code} is waiting for you."},
		"de": {"Abholbereit", "Bestellung {code} wartet auf dich."},
	},
	"notif.order.rejected": {
		"sq": {"Porosia u refuzua", "Porosia {code} nuk u pranua. {reason}"},
		"en": {"Order rejected", "Order {code} was not accepted. {reason}"},
		"de": {"Bestellung abgelehnt", "Bestellung {code} wurde nicht angenommen. {reason}"},
	},
	"notif.order.offer": {
		"sq": {"Dorëzim i ri", "Prano ose refuzo brenda pak sekondash."},
		"en": {"New delivery", "Accept or decline within seconds."},
		"de": {"Neue Lieferung", "Innerhalb von Sekunden annehmen oder ablehnen."},
	},
	"notif.order.courier": {
		"sq": {"Korrieri u caktua", "Porosia {code} do të merret së shpejti."},
		"en": {"Courier assigned", "Order {code} will be collected shortly."},
		"de": {"Kurier zugewiesen", "Bestellung {code} wird bald abgeholt."},
	},
	"notif.order.on_the_way": {
		"sq": {"Porosia është rrugës", "Korrieri e mori porosinë {code}."},
		"en": {"Order on its way", "The courier picked up order {code}."},
		"de": {"Bestellung unterwegs", "Der Kurier hat Bestellung {code} abgeholt."},
	},
	"notif.order.delivered": {
		"sq": {"Porosia u dorëzua", "Të bëftë mirë! Porosia {code}."},
		"en": {"Order delivered", "Enjoy! Order {code}."},
		"de": {"Bestellung geliefert", "Guten Appetit! Bestellung {code}."},
	},
	"notif.order.cancelled": {
		"sq": {"Porosia u anulua", "Porosia {code} u anulua. {reason}"},
		"en": {"Order cancelled", "Order {code} was cancelled. {reason}"},
		"de": {"Bestellung storniert", "Bestellung {code} wurde storniert. {reason}"},
	},
	"notif.order.cancelled.courier": {
		"sq": {"Dorëzimi u anulua", "Porosia {code} nuk dorëzohet më."},
		"en": {"Delivery cancelled", "Order {code} is no longer being delivered."},
		"de": {"Lieferung storniert", "Bestellung {code} wird nicht mehr geliefert."},
	},
	"notif.parcel.offer": {
		"sq": {"Pako e re", "Prano ose refuzo brenda pak sekondash."},
		"en": {"New parcel", "Accept or decline within seconds."},
		"de": {"Neues Paket", "Innerhalb von Sekunden annehmen oder ablehnen."},
	},
	"notif.parcel.courier": {
		"sq": {"Korrieri u caktua", "Pakoja {code} do të merret së shpejti. Kodi i marrjes të pret te aplikacioni."},
		"en": {"Courier assigned", "Parcel {code} will be collected shortly. Your pickup code is in the app."},
		"de": {"Kurier zugewiesen", "Paket {code} wird bald abgeholt. Dein Abholcode steht in der App."},
	},
	"notif.parcel.picked_up": {
		"sq": {"Pakoja është rrugës", "Marrësi e ka kodin e dorëzimit te ti — ndaje me të."},
		"en": {"Parcel on its way", "The delivery code is in your app — share it with the recipient."},
		"de": {"Paket unterwegs", "Der Zustellcode steht in deiner App — teile ihn mit dem Empfänger."},
	},
	"notif.parcel.delivered": {
		"sq": {"Pakoja u dorëzua", "Pakoja {code} arriti te marrësi."},
		"en": {"Parcel delivered", "Parcel {code} reached the recipient."},
		"de": {"Paket zugestellt", "Paket {code} ist beim Empfänger angekommen."},
	},
	"notif.parcel.no_courier": {
		"sq": {"Asnjë korrier i lirë", "Nuk gjetëm korrier për pakon tënde. Provo sërish pas pak."},
		"en": {"No courier available", "We could not find a courier for your parcel. Try again shortly."},
		"de": {"Kein Kurier verfügbar", "Wir haben keinen Kurier für dein Paket gefunden. Versuche es gleich erneut."},
	},
	"notif.parcel.cancelled.courier": {
		"sq": {"Pakoja u anulua", "Pakoja {code} nuk dërgohet më."},
		"en": {"Parcel cancelled", "Parcel {code} is no longer being sent."},
		"de": {"Paket storniert", "Paket {code} wird nicht mehr versendet."},
	},
	"notif.service.offer": {
		"sq": {"Ofertë e re", "Një mjeshtër ofroi {price}. Shihe dhe zgjidh."},
		"en": {"New offer", "A professional offered {price}. Take a look and choose."},
		"de": {"Neues Angebot", "Eine Fachkraft bietet {price}. Schau es dir an und wähle."},
	},
	"notif.service.booked": {
		"sq": {"Puna është e jotja", "Kërkesa {code} · {price}. Klienti zgjodhi ofertën tënde."},
		"en": {"The job is yours", "Request {code} · {price}. The customer chose your offer."},
		"de": {"Der Auftrag gehört dir", "Anfrage {code} · {price}. Der Kunde hat dein Angebot gewählt."},
	},
	"notif.service.started": {
		"sq": {"Puna filloi", "Mjeshtri nisi punën për {code}."},
		"en": {"Work started", "The professional started on {code}."},
		"de": {"Arbeit begonnen", "Die Fachkraft hat mit {code} begonnen."},
	},
	"notif.service.completed": {
		"sq": {"Puna përfundoi", "Kërkesa {code} u mbyll. Faleminderit!"},
		"en": {"Work finished", "Request {code} is closed. Thank you!"},
		"de": {"Arbeit erledigt", "Anfrage {code} ist abgeschlossen. Danke!"},
	},
	"notif.service.released": {
		"sq": {"Mjeshtri hoqi dorë", "Kërkesa u kthye e hapur; zgjidh një mjeshtër tjetër."},
		"en": {"The professional stepped back", "Your request is open again; choose someone else."},
		"de": {"Die Fachkraft ist abgesprungen", "Deine Anfrage ist wieder offen; wähle jemand anderen."},
	},
	"notif.service.cancelled.provider": {
		"sq": {"Puna u anulua", "Klienti anuloi kërkesën {code}."},
		"en": {"Job cancelled", "The customer cancelled request {code}."},
		"de": {"Auftrag storniert", "Der Kunde hat Anfrage {code} storniert."},
	},
	"notif.provider.approved": {
		"sq": {"Llogaria u miratua", "Tani mund të dërgosh oferta për punët e hapura."},
		"en": {"Account approved", "You can now send offers for open jobs."},
		"de": {"Konto freigegeben", "Du kannst jetzt Angebote für offene Aufträge senden."},
	},
	"notif.provider.suspended": {
		"sq": {"Llogaria u pezullua", "{reason}"},
		"en": {"Account suspended", "{reason}"},
		"de": {"Konto gesperrt", "{reason}"},
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
	"notif.merchant.active": {
		"sq": {"Merchant-i u aktivizua", "{name} tani është i dukshëm në KREJT dhe mund të marrë porosi."},
		"en": {"Merchant activated", "{name} is now live on KREJT and can receive orders."},
		"de": {"Händler aktiviert", "{name} ist jetzt auf KREJT sichtbar und kann Bestellungen annehmen."},
	},
	"notif.merchant.suspended": {
		"sq": {"Merchant-i u pezullua", "{name}: {reason}. Kontakto mbështetjen."},
		"en": {"Merchant suspended", "{name}: {reason}. Contact support."},
		"de": {"Händler gesperrt", "{name}: {reason}. Kontaktiere den Support."},
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
