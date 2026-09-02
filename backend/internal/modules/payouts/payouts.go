// Package payouts — fitimet dhe payout-et e shoferëve (Faza 1): fitimet nga ledger-i (wallet-i i shoferit:
// çmim − komision për wallet, − komision për cash), llogaria bankare (IBAN i validuar), grupe javore
// payout-esh të krijuara nga Finance: çdo shofer me bilanc ≥ minimumi debitohet (wallet → payout_clearing)
// dhe eksportohet si CSV për bankën (Raiffeisen) derisa të ketë API; pagesa e dështuar kthehet në wallet.
// Bilanci negativ (borxh komisioni nga cash) nuk paguhet — mbetet për t'u kompensuar nga udhëtimet e ardhshme.
package payouts

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/money"
	"krejt.app/backend/internal/modules/ledger"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrIBAN          = &httpx.APIError{Code: "IBAN_INVALID", MessageKey: "errors.payouts.iban_invalid", HTTPStatus: http.StatusUnprocessableEntity}
	ErrBatchState    = &httpx.APIError{Code: "PAYOUT_BATCH_STATE", MessageKey: "errors.payouts.batch_state", HTTPStatus: http.StatusConflict}
	ErrNoBankAccount = &httpx.APIError{Code: "NO_BANK_ACCOUNT", MessageKey: "errors.payouts.no_bank_account", HTTPStatus: http.StatusUnprocessableEntity}
)

// MinPayoutMinor — nën këtë shumë bilanci mbartet javën tjetër.
const MinPayoutMinor = 500

type Service struct {
	pool   *pgxpool.Pool
	ledger *ledger.Service
	now    func() time.Time
}

func New(pool *pgxpool.Pool, led *ledger.Service) *Service {
	return &Service{pool: pool, ledger: led, now: time.Now}
}

func DriverWalletCode(driverID uuid.UUID) string { return "driver:" + driverID.String() + ":wallet" }

// --- IBAN --------------------------------------------------------------------------------

// ValidIBAN — gjatësi sipas shtetit (XK 20, DE 22, AL 28, CH 21…), mod-97 = 1.
func ValidIBAN(s string) (string, bool) {
	s = strings.ToUpper(strings.Join(strings.Fields(s), ""))
	if len(s) < 15 || len(s) > 34 {
		return "", false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
			return "", false
		}
	}
	lengths := map[string]int{"XK": 20, "AL": 28, "DE": 22, "AT": 20, "CH": 21, "MK": 19, "ME": 22, "RS": 22, "IT": 27, "FR": 27, "NL": 18, "BE": 16, "GB": 22, "SI": 19, "HR": 21}
	if n, ok := lengths[s[:2]]; ok && len(s) != n {
		return "", false
	}
	rearr := s[4:] + s[:4]
	var rem int
	for _, c := range rearr {
		var v int
		if c >= 'A' && c <= 'Z' {
			v = int(c-'A') + 10
			rem = (rem*100 + v) % 97
		} else {
			v = int(c - '0')
			rem = (rem*10 + v) % 97
		}
	}
	return s, rem == 1
}

// MaskIBAN — për shfaqje: XK05 **** **** **** 1234.
func MaskIBAN(iban string) string {
	if len(iban) < 8 {
		return "****"
	}
	return iban[:4] + strings.Repeat("*", len(iban)-8) + iban[len(iban)-4:]
}

// --- llogaria bankare e shoferit -------------------------------------------------------------

type BankAccount struct {
	HolderName string     `json:"holder_name"`
	IBANMasked string     `json:"iban_masked"`
	BankName   *string    `json:"bank_name"`
	VerifiedAt *time.Time `json:"verified_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type BankAccountInput struct {
	HolderName string `json:"holder_name"`
	IBAN       string `json:"iban"`
	BankName   string `json:"bank_name"`
}

func (s *Service) SetBankAccount(ctx context.Context, a principal.Actor, in BankAccountInput) (*BankAccount, error) {
	in.HolderName = strings.Join(strings.Fields(in.HolderName), " ")
	if n := utf8.RuneCountInString(in.HolderName); n < 3 || n > 80 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"holder_name": "invalid"})
	}
	iban, ok := ValidIBAN(in.IBAN)
	if !ok {
		return nil, ErrIBAN
	}
	var bank *string
	if b := strings.TrimSpace(in.BankName); b != "" {
		bank = &b
	}
	var out BankAccount
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM drivers WHERE user_id = $1)`, a.UserID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return httpx.ErrForbidden
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO driver_bank_accounts (driver_id, holder_name, iban, bank_name) VALUES ($1, $2, $3, $4)
			ON CONFLICT (driver_id) DO UPDATE SET holder_name = EXCLUDED.holder_name, iban = EXCLUDED.iban, bank_name = EXCLUDED.bank_name,
			  verified_at = NULL, updated_at = now()
			RETURNING holder_name, iban, bank_name, verified_at, updated_at`, a.UserID, in.HolderName, iban, bank).
			Scan(&out.HolderName, &out.IBANMasked, &out.BankName, &out.VerifiedAt, &out.UpdatedAt); err != nil {
			return err
		}
		out.IBANMasked = MaskIBAN(out.IBANMasked)
		meta, _ := json.Marshal(map[string]any{"iban_masked": out.IBANMasked})
		_, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'driver.bank_account_updated', 'driver', $2, $3)`, a.UserID, a.UserID.String(), meta)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) BankAccount(ctx context.Context, driverID uuid.UUID) (*BankAccount, error) {
	var out BankAccount
	err := s.pool.QueryRow(ctx, `SELECT holder_name, iban, bank_name, verified_at, updated_at FROM driver_bank_accounts WHERE driver_id = $1`, driverID).
		Scan(&out.HolderName, &out.IBANMasked, &out.BankName, &out.VerifiedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.IBANMasked = MaskIBAN(out.IBANMasked)
	return &out, nil
}

// --- fitimet -------------------------------------------------------------------------------

type Earnings struct {
	BalanceMinor  int64  `json:"balance_minor"` // bilanci i wallet-it (mund të jetë negativ = borxh komisioni)
	TodayMinor    int64  `json:"today_minor"`
	WeekMinor     int64  `json:"week_minor"`
	MonthMinor    int64  `json:"month_minor"`
	RidesToday    int    `json:"rides_today"`
	RidesWeek     int    `json:"rides_week"`
	CashCollected int64  `json:"cash_collected_week_minor"` // cash i mbledhur nga klientët (informativ)
	NextPayoutMin int64  `json:"next_payout_min_minor"`
	Currency      string `json:"currency"`
}

func (s *Service) Earnings(ctx context.Context, driverID uuid.UUID) (*Earnings, error) {
	e := &Earnings{Currency: "EUR", NextPayoutMin: MinPayoutMinor}
	uid := driverID
	code := DriverWalletCode(driverID)
	if err := s.ledger.EnsureAccount(ctx, code, "driver", &uid, "liability", "EUR"); err != nil {
		return nil, err
	}
	bal, err := s.ledger.Balance(ctx, code)
	if err != nil {
		return nil, err
	}
	e.BalanceMinor = int64(bal.Minor)
	// fitimet = kredit − debit në wallet-in e shoferit nga udhëtimet (jo payout-et)
	err = s.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN e.created_at >= date_trunc('day', now()) THEN e.credit_minor - e.debit_minor END), 0),
		  COALESCE(SUM(CASE WHEN e.created_at >= date_trunc('week', now()) THEN e.credit_minor - e.debit_minor END), 0),
		  COALESCE(SUM(CASE WHEN e.created_at >= date_trunc('month', now()) THEN e.credit_minor - e.debit_minor END), 0)
		FROM ledger_entries e JOIN ledger_accounts a ON a.id = e.account_id JOIN ledger_transactions t ON t.id = e.tx_id
		WHERE a.code = $1 AND t.kind IN ('ride_fare', 'ride_cash_commission')`, code).Scan(&e.TodayMinor, &e.WeekMinor, &e.MonthMinor)
	if err != nil {
		return nil, err
	}
	err = s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE completed_at >= date_trunc('day', now())), count(*) FILTER (WHERE completed_at >= date_trunc('week', now())),
		       COALESCE(SUM(price_final_minor) FILTER (WHERE payment_method = 'cash' AND completed_at >= date_trunc('week', now())), 0)
		FROM rides WHERE driver_id = $1 AND state = 'completed'`, driverID).Scan(&e.RidesToday, &e.RidesWeek, &e.CashCollected)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// --- payout-et ------------------------------------------------------------------------------

type Batch struct {
	ID          uuid.UUID  `json:"id"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	Status      string     `json:"status"`
	TotalMinor  int64      `json:"total_minor"`
	ItemCount   int        `json:"item_count"`
	CreatedAt   time.Time  `json:"created_at"`
	ExportedAt  *time.Time `json:"exported_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type Item struct {
	ID            uuid.UUID `json:"id"`
	BatchID       uuid.UUID `json:"batch_id"`
	DriverID      uuid.UUID `json:"driver_id"`
	AmountMinor   int64     `json:"amount_minor"`
	Currency      string    `json:"currency"`
	IBANMasked    string    `json:"iban_masked"`
	HolderName    string    `json:"holder_name"`
	Status        string    `json:"status"`
	FailureReason *string   `json:"failure_reason"`
	CreatedAt     time.Time `json:"created_at"`
}

const batchCols = `id, period_start, period_end, status, total_minor, item_count, created_at, exported_at, completed_at`

func scanBatch(row pgx.Row) (*Batch, error) {
	var b Batch
	if err := row.Scan(&b.ID, &b.PeriodStart, &b.PeriodEnd, &b.Status, &b.TotalMinor, &b.ItemCount, &b.CreatedAt, &b.ExportedAt, &b.CompletedAt); err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBatch — Finance krijon grupin javor: çdo shofer i miratuar me llogari bankare dhe bilanc ≥ minimum
// debitohet nga wallet-i (ledger, idempotent për shofer+grup) dhe hyn si zë 'pending'.
func (s *Service) CreateBatch(ctx context.Context, finance principal.Actor, periodStart, periodEnd time.Time) (*Batch, error) {
	if !periodEnd.After(periodStart) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"period": "invalid"})
	}
	var batch *Batch
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		b, err := scanBatch(tx.QueryRow(ctx, `INSERT INTO payout_batches (period_start, period_end, created_by) VALUES ($1, $2, $3) RETURNING `+batchCols,
			periodStart, periodEnd, finance.UserID))
		if err != nil {
			return err
		}
		batch = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	// kandidatët: shoferë të miratuar me IBAN
	rows, err := s.pool.Query(ctx, `SELECT d.user_id, b.iban, b.holder_name FROM drivers d JOIN driver_bank_accounts b ON b.driver_id = d.user_id WHERE d.status = 'approved'`)
	if err != nil {
		return nil, err
	}
	type cand struct {
		id     uuid.UUID
		iban   string
		holder string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.iban, &c.holder); err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	var total int64
	count := 0
	for _, c := range cands {
		bal, err := s.ledger.Balance(ctx, DriverWalletCode(c.id))
		if err != nil && !errors.Is(err, ledger.ErrAccountMissing) {
			return nil, err
		}
		amount := int64(bal.Minor)
		if amount < MinPayoutMinor {
			continue
		}
		itemID := uuid.New()
		txID, err := s.ledger.Post(ctx, ledger.Transaction{Kind: "driver_payout", Reference: "payout_item:" + itemID.String(),
			IdempotencyKey: "payout:" + batch.ID.String() + ":" + c.id.String(), Currency: "EUR",
			Postings: []ledger.Posting{{AccountCode: DriverWalletCode(c.id), Debit: money.Minor(amount)}, {AccountCode: "krejt:payout_clearing", Credit: money.Minor(amount)}}})
		if err != nil {
			return nil, err
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO payout_items (id, batch_id, driver_id, amount_minor, iban, holder_name, ledger_tx_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			itemID, batch.ID, c.id, amount, c.iban, c.holder, txID); err != nil {
			return nil, err
		}
		total += amount
		count++
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		b, err := scanBatch(tx.QueryRow(ctx, `UPDATE payout_batches SET total_minor = $2, item_count = $3 WHERE id = $1 RETURNING `+batchCols, batch.ID, total, count))
		if err != nil {
			return err
		}
		batch = b
		meta, _ := json.Marshal(map[string]any{"total_minor": total, "items": count})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'payouts.batch_created', 'payout_batch', $2, $3)`, finance.UserID, batch.ID.String(), meta); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "payout_batch", batch.ID.String(), "PayoutBatchCreated", map[string]any{"batch_id": batch.ID, "total_minor": total, "items": count})
	})
	if err != nil {
		return nil, err
	}
	return batch, nil
}

// ExportCSV — skedari për bankën (kolona standarde; IBAN i plotë vetëm këtu, i audituar). Grupi → 'exported'.
func (s *Service) ExportCSV(ctx context.Context, finance principal.Actor, batchID uuid.UUID) ([]byte, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, driver_id, amount_minor, currency, iban, holder_name FROM payout_items WHERE batch_id = $1 AND status = 'pending' ORDER BY created_at`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"item_id", "driver_id", "amount", "currency", "iban", "holder_name", "reference"})
	n := 0
	for rows.Next() {
		var id, did uuid.UUID
		var amount int64
		var cur, iban, holder string
		if err := rows.Scan(&id, &did, &amount, &cur, &iban, &holder); err != nil {
			return nil, err
		}
		_ = w.Write([]string{id.String(), did.String(), fmt.Sprintf("%d.%02d", amount/100, amount%100), strings.TrimSpace(cur), iban, holder, "KREJT payout " + id.String()[:8]})
		n++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	w.Flush()
	if n == 0 {
		return nil, httpx.ErrNotFound
	}
	if _, err := s.pool.Exec(ctx, `UPDATE payout_batches SET status = 'exported', exported_at = COALESCE(exported_at, now()) WHERE id = $1 AND status IN ('pending','exported')`, batchID); err != nil {
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'payouts.batch_exported', 'payout_batch', $2, jsonb_build_object('items', $3::int))`, finance.UserID, batchID.String(), n)
	return buf.Bytes(), nil
}

// SettleItem — Finance shënon zërin paid/failed pas përgjigjes së bankës; dështimi i kthen paratë në wallet.
func (s *Service) SettleItem(ctx context.Context, finance principal.Actor, itemID uuid.UUID, status, reason string) (*Item, error) {
	if status != "paid" && status != "failed" {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"status": "invalid"})
	}
	var it Item
	var batchID uuid.UUID
	err := s.pool.QueryRow(ctx, `UPDATE payout_items SET status = $2, failure_reason = NULLIF($3, ''), updated_at = now()
		WHERE id = $1 AND status = 'pending'
		RETURNING id, batch_id, driver_id, amount_minor, currency, iban, holder_name, status, failure_reason, created_at`, itemID, status, reason).
		Scan(&it.ID, &batchID, &it.DriverID, &it.AmountMinor, &it.Currency, &it.IBANMasked, &it.HolderName, &it.Status, &it.FailureReason, &it.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	it.BatchID = batchID
	it.IBANMasked = MaskIBAN(it.IBANMasked)
	it.Currency = strings.TrimSpace(it.Currency)
	if status == "failed" {
		if _, err := s.ledger.Post(ctx, ledger.Transaction{Kind: "driver_payout_reversal", Reference: "payout_item:" + itemID.String(),
			IdempotencyKey: "payout_reversal:" + itemID.String(), Currency: it.Currency,
			Postings: []ledger.Posting{{AccountCode: "krejt:payout_clearing", Debit: money.Minor(it.AmountMinor)}, {AccountCode: DriverWalletCode(it.DriverID), Credit: money.Minor(it.AmountMinor)}}}); err != nil {
			return nil, err
		}
	}
	meta, _ := json.Marshal(map[string]any{"status": status, "reason": reason, "driver_id": it.DriverID, "amount_minor": it.AmountMinor})
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'payouts.item_settled', 'payout_item', $2, $3)`, finance.UserID, itemID.String(), meta)
	// grupi mbyllet kur asnjë zë s'është pending
	_, _ = s.pool.Exec(ctx, `UPDATE payout_batches b SET status = 'completed', completed_at = now()
		WHERE b.id = $1 AND NOT EXISTS (SELECT 1 FROM payout_items i WHERE i.batch_id = b.id AND i.status = 'pending')`, batchID)
	_ = events.Emit(ctx, s.pool, "payout_item", itemID.String(), "DriverPayoutSettled", map[string]any{"item_id": itemID, "driver_id": it.DriverID, "status": status, "amount_minor": it.AmountMinor})
	return &it, nil
}

func (s *Service) Batches(ctx context.Context, limit int) ([]Batch, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT `+batchCols+` FROM payout_batches ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Batch{}
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *Service) Items(ctx context.Context, batchID *uuid.UUID, driverID *uuid.UUID, limit int) ([]Item, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT id, batch_id, driver_id, amount_minor, currency, iban, holder_name, status, failure_reason, created_at FROM payout_items
		WHERE ($1::uuid IS NULL OR batch_id = $1) AND ($2::uuid IS NULL OR driver_id = $2) ORDER BY created_at DESC LIMIT $3`, batchID, driverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.BatchID, &it.DriverID, &it.AmountMinor, &it.Currency, &it.IBANMasked, &it.HolderName, &it.Status, &it.FailureReason, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.IBANMasked = MaskIBAN(it.IBANMasked)
		it.Currency = strings.TrimSpace(it.Currency)
		out = append(out, it)
	}
	return out, rows.Err()
}
