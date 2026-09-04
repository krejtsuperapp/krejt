package merchants

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/httpx"
)

// Të preferuarat e klientit (§21).
//
// Ndryshe nga Zbulimi, kjo listë nuk pritet te 15 km dhe nuk renditet sipas afërsisë: një lokal i
// preferuar mbetet i preferuar edhe kur je në qytet tjetër. Renditja është koha kur u ruajt, nga
// më e reja — ajo është e vetmja renditje që përdoruesi e njeh si të vetën.

// Favourites — lokalet e ruajtura nga një përdorues. Distanca llogaritet kur dihet pozicioni, por
// nuk filtron asgjë.
func (s *Service) Favourites(ctx context.Context, userID uuid.UUID, at *geo.Point) ([]Merchant, error) {
	var lat, lng *float64
	if at != nil && at.Valid() {
		lat, lng = &at.Lat, &at.Lng
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+merchantCols+`,
		       CASE WHEN $1::float8 IS NULL THEN NULL ELSE
		         2 * 6371000 * asin(sqrt(power(sin(radians(lat - $1) / 2), 2) + cos(radians($1)) * cos(radians(lat)) * power(sin(radians(lng - $2) / 2), 2))) END AS dist
		FROM merchants
		JOIN merchant_favourites f ON f.merchant_id = merchants.id AND f.user_id = $3
		WHERE status = 'active'
		ORDER BY f.created_at DESC`, lat, lng, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Merchant{}
	now := s.now()
	for rows.Next() {
		m, err := scanDiscovered(rows)
		if err != nil {
			return nil, err
		}
		m.Favourite = true
		m.Hours, _ = s.hours(ctx, m.ID)
		m.OpenNow = OpenAt(m.Hours, now) && m.AcceptingOrders
		out = append(out, *m)
	}
	return out, rows.Err()
}

// AddFavourite — e njëjta ruajtje dy herë nuk është gabim: përdoruesi e do lokalin te lista, dhe
// aty është.
func (s *Service) AddFavourite(ctx context.Context, userID, merchantID uuid.UUID) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM merchants WHERE id = $1 AND status = 'active')`, merchantID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return httpx.ErrNotFound
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO merchant_favourites (user_id, merchant_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, merchantID)
	return err
}

// RemoveFavourite — heqja e diçkaje që nuk është aty është po ashtu sukses: gjendja e kërkuar
// arrihet, dhe një gabim këtu do të ishte zhurmë.
func (s *Service) RemoveFavourite(ctx context.Context, userID, merchantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM merchant_favourites WHERE user_id = $1 AND merchant_id = $2`, userID, merchantID)
	return err
}

// IsFavourite — për profilin e një lokali të vetëm.
func (s *Service) IsFavourite(ctx context.Context, userID, merchantID uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM merchant_favourites WHERE user_id = $1 AND merchant_id = $2)`,
		userID, merchantID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

// favouriteIDs — cilat nga lokalet e dhëna i ka ruajtur ky përdorues. Një pyetje e vetme për të
// gjithë listën: një për rresht do të ishte N+1 te ekrani më i përdorur i aplikacionit.
func (s *Service) favouriteIDs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if userID == uuid.Nil || len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT merchant_id FROM merchant_favourites WHERE user_id = $1 AND merchant_id = ANY($2)`, userID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
