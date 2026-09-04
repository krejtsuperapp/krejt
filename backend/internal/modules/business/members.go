package business

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Punonjësit dhe kufijtë (§34).
//
// Kufiri mbahet te anëtarësia e jo te përdoruesi: i njëjti person mund të punojë për dy ndërmarrje
// me dy kufij të ndryshëm, dhe kufiri i njërës nuk ka pse ta dijë tjetra.

// monthStart — fillimi i muajit aktual. Kufiri është mujor dhe rifillon vetë; asnjë punë e
// planifikuar nuk e "reseton" atë, sepse çdo gjë që duhet rivendosur mund të harrohet.
func monthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

// Members — anëtarët me shpenzimin e këtij muaji. Shpenzimi llogaritet nga rreshtat e vërtetë e jo
// nga një numërator i ruajtur: një numërator mund të dalë jashtë sinkronit dhe askush nuk e sheh.
func (s *Service) Members(ctx context.Context, a principal.Actor, businessID uuid.UUID) ([]Member, error) {
	if _, err := s.role(ctx, businessID, a.UserID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.full_name, u.phone_e164, m.role, m.monthly_limit_minor, m.active,
		       COALESCE((SELECT SUM(c.amount_minor) FROM business_charges c
		                 WHERE c.business_id = m.business_id AND c.user_id = m.user_id
		                   AND c.created_at >= $2), 0)
		FROM business_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.business_id = $1
		ORDER BY m.role, u.full_name NULLS LAST`, businessID, monthStart(s.now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Name, &m.Phone, &m.Role, &m.MonthlyLimit, &m.Active,
			&m.SpentThisMonth); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type MemberInput struct {
	Phone        string `json:"phone"`
	Role         string `json:"role"`
	MonthlyLimit *int64 `json:"monthly_limit_minor"`
}

// AddMember — ftesa bëhet me numrin e telefonit, sepse ai është identiteti i vetëm që një
// ndërmarrje e di për punonjësin e vet. Përdoruesi duhet të ekzistojë: ne nuk krijojmë llogari në
// emër të askujt.
func (s *Service) AddMember(ctx context.Context, a principal.Actor, businessID uuid.UUID, in MemberInput) (*Member, error) {
	if err := s.requireAdmin(ctx, businessID, a.UserID); err != nil {
		return nil, err
	}
	if in.Role != "admin" && in.Role != "member" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"role": "invalid"})
	}
	if in.MonthlyLimit != nil && *in.MonthlyLimit < 0 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"monthly_limit_minor": "invalid"})
	}
	var userID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE phone_e164 = $1 AND status = 'active'`, in.Phone).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"phone": "not_found"})
	}
	if err != nil {
		return nil, err
	}
	// Rifutja e dikujt që u hoq e kthen atë në punë me rolin dhe kufirin e ri.
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO business_members (business_id, user_id, role, monthly_limit_minor)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, user_id) DO UPDATE
		SET role = EXCLUDED.role, monthly_limit_minor = EXCLUDED.monthly_limit_minor,
		    active = true, updated_at = now()`,
		businessID, userID, in.Role, in.MonthlyLimit); err != nil {
		return nil, err
	}
	members, err := s.Members(ctx, a, businessID)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.UserID == userID {
			return &m, nil
		}
	}
	return nil, httpx.ErrNotFound
}

// RemoveMember — pronari i fundit nuk hiqet: një ndërmarrje pa pronar nuk do të kishte kush ta
// administronte, dhe rreshtat e saj do të mbeteshin të paarritshëm.
func (s *Service) RemoveMember(ctx context.Context, a principal.Actor, businessID, userID uuid.UUID) error {
	if err := s.requireAdmin(ctx, businessID, a.UserID); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var role string
		err := tx.QueryRow(ctx,
			`SELECT role FROM business_members WHERE business_id = $1 AND user_id = $2 AND active FOR UPDATE`,
			businessID, userID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // hequr tashmë; gjendja e kërkuar arrihet
		}
		if err != nil {
			return err
		}
		if role == "owner" {
			var owners int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM business_members WHERE business_id = $1 AND role = 'owner' AND active`,
				businessID).Scan(&owners); err != nil {
				return err
			}
			if owners <= 1 {
				return ErrLastOwner
			}
		}
		_, err = tx.Exec(ctx,
			`UPDATE business_members SET active = false, updated_at = now() WHERE business_id = $1 AND user_id = $2`,
			businessID, userID)
		return err
	})
}

// Authorize — a lejohet ky shpenzim, dhe nga cila llogari.
//
// Nuk shkruan asgjë te libri me qëllim. Një udhëtim i paguar nga ndërmarrja duhet të ndahet
// pikërisht si një i paguar nga kuleta personale — pjesa e shoferit te detyrimi ndaj tij, pjesa
// jonë te komisioni. Nëse kjo funksion do ta postonte vetë shumën, ajo ndarje do të humbte dhe
// libri do të tregonte një komision që nuk e fitoi askush.
//
// Prandaj kthen vetëm kodin e llogarisë që duhet ngarkuar; regjistrimin e bën ai që di si ndahet.
func (s *Service) Authorize(ctx context.Context, businessID, userID uuid.UUID, amount int64) (string, error) {
	if amount <= 0 {
		return "", httpx.ErrValidation.WithFields(map[string]string{"amount_minor": "invalid"})
	}
	var limit *int64
	err := s.pool.QueryRow(ctx,
		`SELECT monthly_limit_minor FROM business_members
		 WHERE business_id = $1 AND user_id = $2 AND active`,
		businessID, userID).Scan(&limit)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotMember
	}
	if err != nil {
		return "", err
	}
	if limit != nil {
		spent, err := s.SpentThisMonth(ctx, businessID, userID)
		if err != nil {
			return "", err
		}
		if spent+amount > *limit {
			return "", ErrLimitReached
		}
	}
	bal, err := s.ledger.Balance(ctx, WalletCode(businessID))
	if err != nil {
		return "", err
	}
	if int64(bal.Minor) < amount {
		return "", ErrInsufficient
	}
	return WalletCode(businessID), nil
}

// SpentThisMonth — nga rreshtat e vërtetë e jo nga një numërator i ruajtur: një numërator mund të
// dalë jashtë sinkronit dhe askush nuk e vëren derisa fatura të dalë e gabuar.
func (s *Service) SpentThisMonth(ctx context.Context, businessID, userID uuid.UUID) (int64, error) {
	var spent int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_minor), 0) FROM business_charges
		 WHERE business_id = $1 AND user_id = $2 AND created_at >= $3`,
		businessID, userID, monthStart(s.now())).Scan(&spent)
	return spent, err
}

// RecordCharge — rreshti i shpenzimit, brenda të njëjtit transaksion me regjistrimin te libri.
// Të ndara, njëri mund të mbetej pa tjetrin dhe fatura mujore nuk do të përputhej me librin.
func RecordCharge(ctx context.Context, tx pgx.Tx, businessID, userID uuid.UUID, kind string, subjectID, ledgerTx uuid.UUID, amount int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO business_charges (business_id, user_id, kind, subject_id, amount_minor, tx_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (kind, subject_id) DO NOTHING`,
		businessID, userID, kind, subjectID, amount, ledgerTx); err != nil {
		return err
	}
	return events.Emit(ctx, tx, "business", businessID.String(), "BusinessCharged", map[string]any{
		"business_id": businessID, "user_id": userID, "kind": kind,
		"subject_id": subjectID, "amount_minor": amount,
	})
}
