// Package ledger — regjistri financiar me dy hyrje (§23). Çdo lëvizje parash kalon këtu.
// Rregullat: numra të plotë në cent, debi = kredi për transaksion, hyrje të pandryshueshme,
// idempotency_key unik (riprovimi kthen të njëjtin transaksion), PostgreSQL autoritativ.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/money"
)

var (
	ErrUnbalanced     = errors.New("ledger: postings are not balanced")
	ErrEmpty          = errors.New("ledger: no postings")
	ErrInvalidPosting = errors.New("ledger: posting must have exactly one of debit or credit > 0")
	ErrAccountMissing = errors.New("ledger: account not found")
	ErrCurrency       = errors.New("ledger: mixed currencies in one transaction")
)

// Posting — një rresht i transaksionit. Saktësisht një nga Debit/Credit > 0.
type Posting struct {
	AccountCode string
	Debit       money.Minor
	Credit      money.Minor
}

type Transaction struct {
	ID             uuid.UUID
	Kind           string
	Reference      string
	IdempotencyKey string
	Currency       string
	Postings       []Posting
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Validate kontrollon rregullat pa prekur bazën — përdoret edhe në teste njësie.
func Validate(postings []Posting) error {
	if len(postings) == 0 {
		return ErrEmpty
	}
	var d, c money.Minor
	for _, p := range postings {
		switch {
		case p.Debit > 0 && p.Credit == 0:
			d += p.Debit
		case p.Credit > 0 && p.Debit == 0:
			c += p.Credit
		default:
			return ErrInvalidPosting
		}
		if p.AccountCode == "" {
			return ErrAccountMissing
		}
	}
	if d != c {
		return fmt.Errorf("%w: debit=%d credit=%d", ErrUnbalanced, d, c)
	}
	return nil
}

// Post regjistron transaksionin atomikisht. Nëse idempotency_key ekziston, kthen ID-në ekzistuese
// pa krijuar asgjë (§24 idempotency). Baza e riverifikon balancën me trigger në COMMIT.
func (s *Service) Post(ctx context.Context, tx Transaction) (uuid.UUID, error) {
	if err := Validate(tx.Postings); err != nil {
		return uuid.Nil, err
	}
	if tx.Currency == "" {
		tx.Currency = "EUR"
	}
	if tx.IdempotencyKey == "" || tx.Kind == "" || tx.Reference == "" {
		return uuid.Nil, errors.New("ledger: kind, reference and idempotency key are required")
	}

	var id uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(dbtx pgx.Tx) error {
		// idempotencë: riprovimi i të njëjtës kërkesë nuk dyfishon asgjë
		err := dbtx.QueryRow(ctx, `SELECT id FROM ledger_transactions WHERE idempotency_key = $1`, tx.IdempotencyKey).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := dbtx.QueryRow(ctx,
			`INSERT INTO ledger_transactions (kind, reference, idempotency_key, currency) VALUES ($1,$2,$3,$4) RETURNING id`,
			tx.Kind, tx.Reference, tx.IdempotencyKey, tx.Currency).Scan(&id); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" { // garë: dikush e futi para nesh
				return dbtx.QueryRow(ctx, `SELECT id FROM ledger_transactions WHERE idempotency_key = $1`, tx.IdempotencyKey).Scan(&id)
			}
			return err
		}
		for _, p := range tx.Postings {
			ct, err := dbtx.Exec(ctx, `
				INSERT INTO ledger_entries (tx_id, account_id, debit_minor, credit_minor, currency)
				SELECT $1, a.id, $3, $4, $5 FROM ledger_accounts a WHERE a.code = $2 AND a.currency = $5`,
				id, p.AccountCode, int64(p.Debit), int64(p.Credit), tx.Currency)
			if err != nil {
				return err
			}
			if ct.RowsAffected() == 0 {
				return fmt.Errorf("%w: %s (%s)", ErrAccountMissing, p.AccountCode, tx.Currency)
			}
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// EnsureAccount krijon llogarinë nëse mungon (p.sh. user:{id}:wallet në regjistrim).
func (s *Service) EnsureAccount(ctx context.Context, code, ownerType string, ownerID *uuid.UUID, kind, currency string) error {
	if currency == "" {
		currency = "EUR"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ledger_accounts (code, owner_type, owner_id, kind, currency)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT (code) DO NOTHING`, code, ownerType, ownerID, kind, currency)
	return err
}

// Balance kthen bilancin e llogarisë: credit − debit (pozitiv për detyrime ndaj përdoruesit, p.sh. wallet).
// Llogaritet gjithmonë nga hyrjet — asnjë kolonë "balance" që mund të ndryshohet.
func (s *Service) Balance(ctx context.Context, accountCode string) (money.Amount, error) {
	var minor int64
	var currency string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(e.credit_minor) - SUM(e.debit_minor), 0), a.currency
		FROM ledger_accounts a LEFT JOIN ledger_entries e ON e.account_id = a.id
		WHERE a.code = $1 GROUP BY a.currency`, accountCode).Scan(&minor, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return money.Amount{}, ErrAccountMissing
	}
	if err != nil {
		return money.Amount{}, err
	}
	return money.Amount{Minor: money.Minor(minor), Currency: currency}, nil
}

// UserWalletCode — kodi i llogarisë së wallet-it të mbyllur të përdoruesit (detyrim i platformës ndaj tij).
func UserWalletCode(userID uuid.UUID) string { return "user:" + userID.String() + ":wallet" }
