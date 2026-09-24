package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type banks struct{ pool *pgxpool.Pool }

func (s *Store) Banks() store.Banks { return &banks{pool: s.pool} }

func (b *banks) List(ctx context.Context, companyID store.ID) ([]store.Bank, error) {
	// id is the tiebreak (#19) so equal names cannot arrange two ways; Postgres has no natural
	// order to fall back on the way Mongo does.
	rows, err := b.pool.Query(ctx, `
		SELECT id, uid, company_id, name, opening_balance, created_at, updated_at
		  FROM banks
		 WHERE company_id = $1
		 ORDER BY name ASC, id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing banks: %w", err)
	}
	defer rows.Close()

	out := make([]store.Bank, 0)
	for rows.Next() {
		var bank store.Bank
		var companyIDCol *string
		if err := rows.Scan(&bank.ID, &bank.UID, &companyIDCol, &bank.Name,
			&bank.OpeningBalance, &bank.CreatedAt, &bank.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading banks: %w", err)
		}
		if companyIDCol != nil {
			bank.CompanyID = store.ID(*companyIDCol)
		}
		// Version stays 0: Postgres has no __v, and Node never versions a bank, so its response
		// always carries __v:0 - which the zero value reproduces.
		out = append(out, bank)
	}
	return out, rows.Err()
}

func (b *banks) ReportData(ctx context.Context, companyID store.ID) (store.BankReportData, error) {
	var out store.BankReportData
	var err error
	// note column differs: batch receipts and supplier payments use "note", expenses use "notes".
	if out.BatchReceives, err = b.reportTxns(ctx, "batch_receives", "note", companyID); err != nil {
		return out, err
	}
	if out.SupplierPayments, err = b.reportTxns(ctx, "supplier_payments", "note", companyID); err != nil {
		return out, err
	}
	if out.Expenses, err = b.reportTxns(ctx, "expenses", "notes", companyID); err != nil {
		return out, err
	}
	rows, err := b.pool.Query(ctx, `SELECT id, name, opening_balance FROM banks WHERE company_id = $1`, string(companyID))
	if err != nil {
		return out, fmt.Errorf("bank report banks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bo store.BankOpening
		if err := rows.Scan(&bo.ID, &bo.Name, &bo.OpeningBalance); err != nil {
			return out, fmt.Errorf("reading bank report banks: %w", err)
		}
		out.Banks = append(out.Banks, bo)
	}
	return out, rows.Err()
}

// reportTxns reads one cash-moving table's rows (table/noteCol are fixed literals, never user
// input). bank_id is nullable - a null becomes an empty id, which bankledger buckets as Unassigned.
func (b *banks) reportTxns(ctx context.Context, table, noteCol string, companyID store.ID) ([]store.BankTxn, error) {
	q := fmt.Sprintf(`SELECT bank_id, date, amount, %s FROM %s WHERE company_id = $1`, noteCol, table)
	rows, err := b.pool.Query(ctx, q, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("bank report %s: %w", table, err)
	}
	defer rows.Close()
	var out []store.BankTxn
	for rows.Next() {
		var t store.BankTxn
		var bankID *string
		if err := rows.Scan(&bankID, &t.Date, &t.Amount, &t.Note); err != nil {
			return nil, fmt.Errorf("reading bank report %s: %w", table, err)
		}
		if bankID != nil {
			t.BankID = store.ID(*bankID)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (b *banks) Create(ctx context.Context, companyID, uid store.ID, name string) (store.Bank, error) {
	var out store.Bank
	var companyCol *string
	err := b.pool.QueryRow(ctx, `
		INSERT INTO banks (uid, company_id, name)
		VALUES ($1, $2, $3)
		RETURNING id, uid, company_id, name, opening_balance, created_at, updated_at`,
		string(uid), string(companyID), name).
		Scan(&out.ID, &out.UID, &companyCol, &out.Name, &out.OpeningBalance, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return store.Bank{}, fmt.Errorf("insert bank: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	return out, nil
}

func (b *banks) Update(ctx context.Context, companyID, bankID store.ID, name string, openingBalance *float64) (store.Bank, bool, error) {
	// openingBalance is written only when it was submitted (non-nil), so a name-only edit keeps
	// the stored balance. Two statements rather than a COALESCE trick: the column is NOT NULL, so
	// there is no null sentinel to lean on, and "leave it alone" must mean not touching it.
	q := `UPDATE banks SET name = $3, updated_at = now()
	       WHERE id = $1 AND company_id = $2
	   RETURNING id, uid, company_id, name, opening_balance, created_at, updated_at`
	args := []any{string(bankID), string(companyID), name}
	if openingBalance != nil {
		q = `UPDATE banks SET name = $3, opening_balance = $4, updated_at = now()
		      WHERE id = $1 AND company_id = $2
		  RETURNING id, uid, company_id, name, opening_balance, created_at, updated_at`
		args = append(args, *openingBalance)
	}
	var out store.Bank
	var companyCol *string
	err := b.pool.QueryRow(ctx, q, args...).
		Scan(&out.ID, &out.UID, &companyCol, &out.Name, &out.OpeningBalance, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Bank{}, false, nil
	}
	if err != nil {
		return store.Bank{}, false, fmt.Errorf("updating bank: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	return out, true, nil
}

func (b *banks) Remove(ctx context.Context, companyID, bankID store.ID) (int, bool, error) {
	// One round trip for the three reference counts; a bank money has moved through cannot be
	// removed without orphaning those rows. Company-scoped, so another company's bank_id counts
	// zero and falls through to the delete, which then affects no rows (404) - matching Node.
	var inUse int
	err := b.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM batch_receives   WHERE bank_id = $1 AND company_id = $2)
		     + (SELECT count(*) FROM supplier_payments WHERE bank_id = $1 AND company_id = $2)
		     + (SELECT count(*) FROM expenses          WHERE bank_id = $1 AND company_id = $2)`,
		string(bankID), string(companyID)).Scan(&inUse)
	if err != nil {
		return 0, false, fmt.Errorf("counting bank references: %w", err)
	}
	if inUse > 0 {
		return inUse, false, nil
	}
	tag, err := b.pool.Exec(ctx, `DELETE FROM banks WHERE id = $1 AND company_id = $2`,
		string(bankID), string(companyID))
	if err != nil {
		return 0, false, fmt.Errorf("deleting bank: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, false, nil
	}
	return 0, true, nil
}
