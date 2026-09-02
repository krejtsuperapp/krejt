package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
)

// Address — adresë e ruajtur (shtëpi/punë/tjetër) me koordinata; përdoret nga rides/orders (§18, §19).
type Address struct {
	ID           uuid.UUID `json:"id"`
	Label        string    `json:"label"`
	Name         *string   `json:"name"`
	Line1        string    `json:"line1"`
	Line2        *string   `json:"line2"`
	City         string    `json:"city"`
	PostalCode   *string   `json:"postal_code"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	PlaceID      *string   `json:"place_id"`
	Instructions *string   `json:"instructions"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AddressInput struct {
	Label        string  `json:"label"`
	Name         *string `json:"name"`
	Line1        string  `json:"line1"`
	Line2        *string `json:"line2"`
	City         string  `json:"city"`
	PostalCode   *string `json:"postal_code"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	PlaceID      *string `json:"place_id"`
	Instructions *string `json:"instructions"`
	IsDefault    bool    `json:"is_default"`
}

const addressCols = `id, label, name, line1, line2, city, postal_code, lat, lng, place_id, instructions, is_default, created_at, updated_at`

func scanAddress(row pgx.Row) (*Address, error) {
	var x Address
	err := row.Scan(&x.ID, &x.Label, &x.Name, &x.Line1, &x.Line2, &x.City, &x.PostalCode, &x.Lat, &x.Lng,
		&x.PlaceID, &x.Instructions, &x.IsDefault, &x.CreatedAt, &x.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &x, nil
}

func (s *Service) ListAddresses(ctx context.Context, userID uuid.UUID) ([]Address, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+addressCols+` FROM user_addresses
		WHERE user_id = $1 AND deleted_at IS NULL ORDER BY is_default DESC, created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Address{}
	for rows.Next() {
		x, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

func checkAddress(in *AddressInput) error {
	if f := validateAddress(in); len(f) > 0 {
		return httpx.ErrValidation.WithFields(f)
	}
	if !InKosovo(in.Lat, in.Lng) {
		return ErrOutsideKosovo
	}
	return nil
}

func (s *Service) CreateAddress(ctx context.Context, a Actor, in AddressInput) (*Address, error) {
	if err := checkAddress(&in); err != nil {
		return nil, err
	}
	var out *Address
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// kyçim mbi rreshtin e përdoruesit: dy kërkesa paralele nuk e kalojnë limitin dhe nuk krijojnë dy default
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, a.UserID); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM user_addresses WHERE user_id = $1 AND deleted_at IS NULL`, a.UserID).Scan(&n); err != nil {
			return err
		}
		if n >= MaxAddresses {
			return ErrAddressLimit
		}
		if n == 0 {
			in.IsDefault = true // adresa e parë bëhet parazgjedhje
		}
		if in.IsDefault {
			if err := clearDefault(ctx, tx, a.UserID); err != nil {
				return err
			}
		}
		x, err := scanAddress(tx.QueryRow(ctx, `
			INSERT INTO user_addresses (user_id, label, name, line1, line2, city, postal_code, lat, lng, place_id, instructions, is_default)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING `+addressCols,
			a.UserID, in.Label, in.Name, in.Line1, in.Line2, in.City, in.PostalCode, in.Lat, in.Lng, in.PlaceID, in.Instructions, in.IsDefault))
		if err != nil {
			return err
		}
		out = x
		if err := audit(ctx, tx, a, "user.address_added", "address", x.ID.String(), map[string]any{"label": x.Label}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "user", a.UserID.String(), "UserAddressAdded", map[string]any{"user_id": a.UserID, "address_id": x.ID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) UpdateAddress(ctx context.Context, a Actor, id uuid.UUID, in AddressInput) (*Address, error) {
	if err := checkAddress(&in); err != nil {
		return nil, err
	}
	var out *Address
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if in.IsDefault {
			if err := clearDefault(ctx, tx, a.UserID); err != nil {
				return err
			}
		}
		x, err := scanAddress(tx.QueryRow(ctx, `
			UPDATE user_addresses SET label=$3, name=$4, line1=$5, line2=$6, city=$7, postal_code=$8, lat=$9, lng=$10,
			       place_id=$11, instructions=$12, is_default=$13, updated_at=now()
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL RETURNING `+addressCols,
			id, a.UserID, in.Label, in.Name, in.Line1, in.Line2, in.City, in.PostalCode, in.Lat, in.Lng, in.PlaceID, in.Instructions, in.IsDefault))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = x
		return audit(ctx, tx, a, "user.address_updated", "address", x.ID.String(), map[string]any{"label": x.Label})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) DeleteAddress(ctx context.Context, a Actor, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE user_addresses SET deleted_at = now(), is_default = false, updated_at = now()
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, a.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.ErrNotFound
		}
		return audit(ctx, tx, a, "user.address_deleted", "address", id.String(), nil)
	})
}

func clearDefault(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE user_addresses SET is_default = false, updated_at = now()
		WHERE user_id = $1 AND is_default AND deleted_at IS NULL`, userID)
	return err
}
