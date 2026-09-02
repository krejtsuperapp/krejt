// Package wallet — KREJT Wallet i mbyllur (§5, §23): bilanci nga ledger-i, limitet, historiku i
// transaksioneve (hyrjet e ledger-it të përdoruesit). Asnjë transfer mes përdoruesve, asnjë tërheqje.
package wallet

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
	limits Limits
}

type Limits struct {
	MinTopUpMinor   int64 `json:"min_topup_minor"`
	MaxTopUpMinor   int64 `json:"max_topup_minor"`
	DailyTopUpMinor int64 `json:"daily_topup_minor"`
}

func New(pool *pgxpool.Pool, led *ledger.Service, limits Limits) *Service {
	return &Service{pool: pool, ledger: led, limits: limits}
}

type Overview struct {
	BalanceMinor int64  `json:"balance_minor"`
	Currency     string `json:"currency"`
	ClosedLoop   bool   `json:"closed_loop"` // gjithmonë true (§5)
	Limits       Limits `json:"limits"`
}

type Transaction struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`         // wallet_topup_card, ride_fare, ride_cancellation_fee, wallet_topup_refund …
	Reference   string    `json:"reference"`    // ride:{id}, payment_intent:{id}
	AmountMinor int64     `json:"amount_minor"` // + hyrje, − dalje nga wallet-i
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) Overview(ctx context.Context, a principal.Actor) (*Overview, error) {
	code := ledger.UserWalletCode(a.UserID)
	uid := a.UserID
	if err := s.ledger.EnsureAccount(ctx, code, "user", &uid, "liability", "EUR"); err != nil {
		return nil, err
	}
	bal, err := s.ledger.Balance(ctx, code)
	if err != nil && !errors.Is(err, ledger.ErrAccountMissing) {
		return nil, err
	}
	cur := bal.Currency
	if cur == "" {
		cur = "EUR"
	}
	return &Overview{BalanceMinor: int64(bal.Minor), Currency: cur, ClosedLoop: true, Limits: s.limits}, nil
}

// Transactions — historiku (nga më i riu), cursor = before (created_at).
func (s *Service) Transactions(ctx context.Context, a principal.Actor, before *time.Time, limit int) ([]Transaction, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if before == nil {
		t := time.Now().Add(time.Hour)
		before = &t
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.kind, t.reference, e.credit_minor - e.debit_minor, e.currency, e.created_at
		FROM ledger_entries e
		JOIN ledger_accounts a ON a.id = e.account_id
		JOIN ledger_transactions t ON t.id = e.tx_id
		WHERE a.code = $1 AND e.created_at < $2
		ORDER BY e.created_at DESC LIMIT $3`, ledger.UserWalletCode(a.UserID), *before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		var x Transaction
		if err := rows.Scan(&x.ID, &x.Kind, &x.Reference, &x.AmountMinor, &x.Currency, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("GET /api/v1/wallet", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		ov, err := s.Overview(r.Context(), a)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, ov)
	})))
	mux.Handle("GET /api/v1/wallet/transactions", requireAuth(principal.Handler(func(w http.ResponseWriter, r *http.Request, a principal.Actor) {
		var before *time.Time
		if v := r.URL.Query().Get("before"); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				httpx.WriteError(w, r, httpx.ErrValidation.WithFields(map[string]string{"before": "invalid"}))
				return
			}
			before = &t
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		items, err := s.Transactions(r.Context(), a, before, limit)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	})))
}
