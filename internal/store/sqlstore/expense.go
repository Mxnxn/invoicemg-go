package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type expenses struct{ pool *pgxpool.Pool }

func (s *Store) Expenses() store.Expenses { return &expenses{pool: s.pool} }

func (e *expenses) List(ctx context.Context, companyID store.ID, f store.ExpenseFilter) ([]store.Expense, error) {
	// bank_id populated via LEFT JOIN (#6). Empty filter fields drop out via the $N='' guards,
	// the same shape as the other optional-filter lists.
	rows, err := e.pool.Query(ctx, `
		SELECT x.id, x.uid, x.company_id, x.date, x.amount, x.notes, x.created_at, x.updated_at,
		       b.id, b.name
		  FROM expenses x
		  LEFT JOIN banks b ON b.id = x.bank_id
		 WHERE x.company_id = $1
		   AND ($2 = '' OR x.bank_id = $2)
		   AND ($3 = '' OR x.date >= $3)
		   AND ($4 = '' OR x.date <= $4)
		 ORDER BY x.date DESC, x.created_at DESC, x.id DESC`,
		string(companyID), string(f.BankID), f.From, f.To)
	if err != nil {
		return nil, fmt.Errorf("listing expenses: %w", err)
	}
	defer rows.Close()
	out := make([]store.Expense, 0)
	for rows.Next() {
		var ex store.Expense
		var companyIDCol, bankID, bankName *string
		if err := rows.Scan(&ex.ID, &ex.UID, &companyIDCol, &ex.Date, &ex.Amount, &ex.Notes,
			&ex.CreatedAt, &ex.UpdatedAt, &bankID, &bankName); err != nil {
			return nil, fmt.Errorf("reading expenses: %w", err)
		}
		if companyIDCol != nil {
			ex.CompanyID = store.ID(*companyIDCol)
		}
		if bankID != nil {
			ex.Bank = &store.ExpenseBank{ID: store.ID(*bankID), Name: deref(bankName)}
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}
