// Package media — çelësat dhe URL-të publike të imazheve (§43): logot dhe kopertinat e vendeve,
// imazhet e produkteve, fotot e profilit. Objektet rrinë në bucket-in e medias (privat), lexohen
// përmes CloudFront-it me një bazë të vetme URL-je që vendoset një herë në nisje.
//
// Çelësi mbart pronësinë: `media/<lloji>/<pronari>/<uuid>.<ext>`. Kështu serveri e di nga vetë
// çelësi kujt i takon një objekt dhe kush ka të drejtë ta vendosë, pa tabelë të veçantë.
package media

import (
	"strings"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/providers/storage"
)

// Kind — llojet e imazheve dhe kolona ku shkon çelësi i tyre.
type Kind string

const (
	KindMerchantLogo  Kind = "merchant_logo"
	KindMerchantCover Kind = "merchant_cover"
	KindProductImage  Kind = "product_image"
	KindUserPhoto     Kind = "user_photo"
)

// Kinds — rendi i qëndrueshëm për dokumentim dhe validim.
var Kinds = []Kind{KindMerchantLogo, KindMerchantCover, KindProductImage, KindUserPhoto}

func ValidKind(k string) bool {
	for _, x := range Kinds {
		if string(x) == k {
			return true
		}
	}
	return false
}

var baseURL string

// SetBaseURL — baza publike (CloudFront) e objekteve; "" do të thotë pa URL (imazhet nuk shfaqen).
func SetBaseURL(u string) { baseURL = strings.TrimRight(strings.TrimSpace(u), "/") }

func BaseURL() string { return baseURL }

// URL — adresa publike e një çelësi; nil kur s'ka çelës ose bazë. Kthehet si pointer që JSON-i
// të ketë `null` në vend të një vargu bosh, si çdo fushë tjetër opsionale e API-së.
func URL(key *string) *string {
	if key == nil || *key == "" || baseURL == "" {
		return nil
	}
	u := baseURL + "/" + *key
	return &u
}

// Prefix — dosja e një pronari për një lloj: `media/<lloji>/<pronari>/`.
func Prefix(kind Kind, owner uuid.UUID) string {
	return "media/" + string(kind) + "/" + owner.String() + "/"
}

// NewKey — çelës i ri brenda prefiksit të pronarit.
func NewKey(kind Kind, owner uuid.UUID, ext string) string {
	return Prefix(kind, owner) + uuid.NewString() + "." + ext
}

// Parse — lexon llojin dhe pronarin nga çelësi; false për çdo formë tjetër (përfshirë çelësa
// jashtë `media/`, që të mos merren kurrë dokumentet e shoferëve për imazhe publike).
func Parse(key string) (Kind, uuid.UUID, bool) {
	if !storage.ValidKey(key) {
		return "", uuid.Nil, false
	}
	parts := strings.Split(key, "/")
	if len(parts) != 4 || parts[0] != "media" || !ValidKind(parts[1]) {
		return "", uuid.Nil, false
	}
	owner, err := uuid.Parse(parts[2])
	if err != nil {
		return "", uuid.Nil, false
	}
	if !strings.Contains(parts[3], ".") {
		return "", uuid.Nil, false
	}
	return Kind(parts[1]), owner, true
}
