package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/platform/httpx"
)

// Me — profili i përdoruesit (§16 Profile): identiteti, kapacitetet, bilanci i wallet-it të mbyllur
// (llogaritet nga ledger-i, kurrë nga një kolonë të ndryshueshme).
type Me struct {
	ID           uuid.UUID  `json:"id"`
	Phone        string     `json:"phone"`
	Email        *string    `json:"email"`
	FullName     *string    `json:"full_name"`
	Locale       string     `json:"locale"`
	Capabilities []string   `json:"capabilities"`
	Wallet       WalletView `json:"wallet"`
	CreatedAt    time.Time  `json:"created_at"`
}

type WalletView struct {
	BalanceMinor int64  `json:"balance_minor"`
	Currency     string `json:"currency"`
	Closed       bool   `json:"closed_loop"` // gjithmonë true në V1 (§5): pa P2P, pa tërheqje
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*Me, error) {
	var m Me
	err := s.pool.QueryRow(ctx, `
		SELECT id, phone_e164, email, full_name, locale, created_at FROM users WHERE id = $1 AND status = 'active'`, userID).
		Scan(&m.ID, &m.Phone, &m.Email, &m.FullName, &m.Locale, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrUnauthorized
	}
	if err != nil {
		return nil, httpx.ErrInternal.With(err)
	}
	caps, err := loadCapabilities(ctx, s.pool, userID)
	if err != nil {
		return nil, httpx.ErrInternal.With(err)
	}
	m.Capabilities = caps
	bal, err := s.ledger.Balance(ctx, "user:"+userID.String()+":wallet")
	if err != nil {
		bal = money.EUR(0)
	}
	m.Wallet = WalletView{BalanceMinor: int64(bal.Minor), Currency: bal.Currency, Closed: true}
	return &m, nil
}
