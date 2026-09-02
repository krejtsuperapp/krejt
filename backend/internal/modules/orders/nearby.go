package orders

import (
	"context"

	"github.com/google/uuid"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/modules/location"
)

// LocationNearby — adapter nga moduli location te ndërfaqja Nearby e dispatch-it të porosive.
type LocationNearby struct{ Loc *location.Service }

func (l LocationNearby) Nearest(ctx context.Context, category string, p geo.Point, radiusKm float64, limit int) ([]NearbyCandidate, error) {
	cands, err := l.Loc.Nearest(ctx, category, p, radiusKm, limit)
	if err != nil {
		return nil, err
	}
	out := make([]NearbyCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, NearbyCandidate{DriverID: c.DriverID, DistanceM: c.DistanceM})
	}
	return out, nil
}

var _ = uuid.Nil
