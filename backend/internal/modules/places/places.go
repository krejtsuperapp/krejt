// Package places — kërkimi i adresave dhe rruga me gjeometri për aplikacionet (§17, §46).
// Klienti nuk flet kurrë drejtpërdrejt me ofruesin e hartave: token-i i serverit mbetet në server,
// kërkesat kufizohen për përdorues dhe përgjigjet ruhen në Redis që të njëjtat pyetje të mos paguhen dy herë.
package places

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/providers/maps"
)

const (
	searchTTL = 24 * time.Hour
	routeTTL  = 10 * time.Minute
	maxQuery  = 80
	maxLimit  = 8
)

type Service struct {
	maps maps.Provider
	rdb  redis.UniversalClient
}

func New(m maps.Provider, rdb redis.UniversalClient) *Service {
	return &Service{maps: m, rdb: rdb}
}

// Search — vende sipas tekstit; afërsia (kur jepet dhe është brenda Kosovës) i rendit më afër.
func (s *Service) Search(ctx context.Context, q string, near *geo.Point, limit int) ([]maps.Place, error) {
	q = strings.TrimSpace(q)
	if n := len([]rune(q)); n < 2 || n > maxQuery {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"q": "length"})
	}
	if limit <= 0 || limit > maxLimit {
		limit = maxLimit
	}
	if near != nil && (!near.Valid() || !geo.InKosovo(*near)) {
		near = nil
	}
	key := "places:search:" + hash(strings.ToLower(q), roundPoint(near, 2), limit)
	var out []maps.Place
	if s.cached(ctx, key, &out) {
		return out, nil
	}
	out, err := s.maps.Search(ctx, q, near, limit)
	if err != nil {
		if errors.Is(err, maps.ErrUnsupported) {
			return nil, httpx.ErrUnavailable.With(err)
		}
		return nil, httpx.ErrUnavailable.With(err)
	}
	if out == nil {
		out = []maps.Place{}
	}
	s.store(ctx, key, out, searchTTL)
	return out, nil
}

// Reverse — adresa e një pike (pika e marrjes nga GPS-i). Pa adresë kthehet nil, jo gabim.
func (s *Service) Reverse(ctx context.Context, p geo.Point) (*maps.Place, error) {
	if !p.Valid() || !geo.InKosovo(p) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"point": "outside_area"})
	}
	key := "places:reverse:" + hash(roundPoint(&p, 4))
	var out *maps.Place
	if s.cached(ctx, key, &out) {
		return out, nil
	}
	out, err := s.maps.Reverse(ctx, p)
	if err != nil {
		return nil, httpx.ErrUnavailable.With(err)
	}
	s.store(ctx, key, out, searchTTL)
	return out, nil
}

// RoutePath — rruga me gjeometri mes dy pikave, për vizatim (jo për çmim: çmimin e jep vetëm quote-i).
func (s *Service) RoutePath(ctx context.Context, from, to geo.Point) (*maps.Route, error) {
	fields := map[string]string{}
	if !from.Valid() || !geo.InKosovo(from) {
		fields["from"] = "invalid"
	}
	if !to.Valid() || !geo.InKosovo(to) {
		fields["to"] = "invalid"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	key := "places:route:" + hash(roundPoint(&from, 4), roundPoint(&to, 4))
	var out *maps.Route
	if s.cached(ctx, key, &out) && out != nil {
		return out, nil
	}
	r, err := s.maps.Directions(ctx, from, to)
	if err != nil {
		if errors.Is(err, maps.ErrNoRoute) {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"to": "no_route"})
		}
		return nil, httpx.ErrUnavailable.With(err)
	}
	if len(r.Path) == 0 {
		r.Path = []geo.Point{from, to}
	}
	s.store(ctx, key, &r, routeTTL)
	return &r, nil
}

// --- cache ---------------------------------------------------------------------------------

func (s *Service) cached(ctx context.Context, key string, v any) bool {
	if s.rdb == nil {
		return false
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

func (s *Service) store(ctx context.Context, key string, v any, ttl time.Duration) {
	if s.rdb == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, key, raw, ttl).Err()
}

func hash(parts ...any) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%v|", p)
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}

// roundPoint — çelës i qëndrueshëm: 2 dhjetore (~1 km) për afërsinë e kërkimit, 4 (~10 m) për rrugët.
func roundPoint(p *geo.Point, decimals int) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.*f,%.*f", decimals, p.Lat, decimals, p.Lng)
}
