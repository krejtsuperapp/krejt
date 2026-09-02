package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/platform/httpx"
)

// Categories — kategoritë e njoftimeve (§29), në rendin që i shfaq aplikacioni.
var Categories = []string{"security", "rides", "orders", "payments", "wallet", "loyalty", "promotions", "support"}

type Preference struct {
	Category string `json:"category"`
	Push     bool   `json:"push"`
	Email    bool   `json:"email"`
	SMS      bool   `json:"sms"`
}

// defaultPreference — push për gjithçka, email për gjithçka përveç promocioneve (opt-in), SMS vetëm me zgjedhje.
func defaultPreference(cat string) Preference {
	return Preference{Category: cat, Push: true, Email: cat != "promotions", SMS: false}
}

// Preferences kthen të 8 kategoritë: të ruajturat + parazgjedhjet për ato që s'janë prekur.
func (s *Service) Preferences(ctx context.Context, userID uuid.UUID) ([]Preference, error) {
	rows, err := s.pool.Query(ctx, `SELECT category, push, email, sms FROM notification_preferences WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stored := map[string]Preference{}
	for rows.Next() {
		var p Preference
		if err := rows.Scan(&p.Category, &p.Push, &p.Email, &p.SMS); err != nil {
			return nil, err
		}
		stored[p.Category] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Preference, 0, len(Categories))
	for _, c := range Categories {
		if p, ok := stored[c]; ok {
			out = append(out, p)
		} else {
			out = append(out, defaultPreference(c))
		}
	}
	return out, nil
}

// SetPreferences ruan kategoritë e dërguara (të tjerat mbeten siç janë) dhe kthen gjendjen e plotë.
func (s *Service) SetPreferences(ctx context.Context, a Actor, in []Preference) ([]Preference, error) {
	if f := validatePreferences(in); len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		changed := make([]string, 0, len(in))
		for _, p := range in {
			if _, err := tx.Exec(ctx, `
				INSERT INTO notification_preferences (user_id, category, push, email, sms)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (user_id, category) DO UPDATE
				SET push = EXCLUDED.push, email = EXCLUDED.email, sms = EXCLUDED.sms, updated_at = now()`,
				a.UserID, p.Category, p.Push, p.Email, p.SMS); err != nil {
				return err
			}
			changed = append(changed, p.Category)
		}
		return audit(ctx, tx, a, "user.notification_preferences_updated", "user", a.UserID.String(), map[string]any{"categories": changed})
	})
	if err != nil {
		return nil, err
	}
	return s.Preferences(ctx, a.UserID)
}
