package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrBootstrapUserMissing — numri i dhënë nuk ka hyrë ende asnjëherë.
var ErrBootstrapUserMissing = errors.New("auth: përdoruesi i ndezjes fillestare nuk ekziston")

// BootstrapAdmin i jep SUPER_ADMIN numrit të dhënë, por vetëm nëse sistemi ende nuk ka asnjë.
//
// Pa këtë, të drejtat e stafit nuk lindin kurrë: jepen vetëm nga një administrator, dhe
// administratori i parë nuk ekziston. Baza rri në rrjet privat, ndaj as me dorë nuk arrihet.
//
// Kushti "vetëm nëse nuk ka asnjë" është ajo që e mban të sigurt. Pas të parit, cilësimi nuk
// bën më asgjë edhe nëse mbetet i ndezur: nuk shton, nuk ngre, nuk rikthen. Numri duhet të ketë
// hyrë një herë që llogaria të ekzistojë — kështu një variabël mjedisi nuk krijon dot llogari.
//
// Kthen true vetëm kur e drejta u dha vërtet.
func (s *Service) BootstrapAdmin(ctx context.Context, phone string) (bool, error) {
	if phone == "" {
		return false, nil
	}
	if !phoneRe.MatchString(phone) {
		return false, fmt.Errorf("auth: numri i ndezjes fillestare i pavlefshëm")
	}

	granted := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Bllokim këshillues: dy detyra që nisen njëkohësisht nuk e japin dot dy herë.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('krejt:bootstrap-admin'))`); err != nil {
			return err
		}

		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM user_capabilities
				WHERE capability = 'SUPER_ADMIN' AND revoked_at IS NULL
			)`).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}

		var userID string
		err := tx.QueryRow(ctx, `SELECT id FROM users WHERE phone_e164 = $1`, phone).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBootstrapUserMissing
		}
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO user_capabilities (user_id, capability) VALUES ($1, 'SUPER_ADMIN')
			ON CONFLICT DO NOTHING`, userID); err != nil {
			return err
		}
		granted = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return granted, nil
}
