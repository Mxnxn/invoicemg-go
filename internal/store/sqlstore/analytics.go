package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type analytics struct{ pool *pgxpool.Pool }

func (s *Store) Analytics() store.Analytics { return &analytics{pool: s.pool} }

// datedSeries reads createdAt/date + one amount column, resolving the effective date
// (createdAt || date) the same way getEffectiveDate does.
func (a *analytics) datedSeries(ctx context.Context, sql string, companyID store.ID) ([]store.DatedAmount, error) {
	rows, err := a.pool.Query(ctx, sql, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("reading revenue series: %w", err)
	}
	defer rows.Close()
	out := make([]store.DatedAmount, 0)
	for rows.Next() {
		var createdAt *time.Time
		var date string
		var amount float64
		if err := rows.Scan(&createdAt, &date, &amount); err != nil {
			return nil, fmt.Errorf("reading revenue row: %w", err)
		}
		if createdAt != nil && !createdAt.IsZero() {
			out = append(out, store.DatedAmount{Date: createdAt.UTC(), Amount: amount})
			continue
		}
		if date != "" {
			if t, err := time.Parse("2006-01-02", date); err == nil {
				out = append(out, store.DatedAmount{Date: t.UTC(), Amount: amount})
			} else if t, err := time.Parse(time.RFC3339, date); err == nil {
				out = append(out, store.DatedAmount{Date: t.UTC(), Amount: amount})
			}
		}
	}
	return out, rows.Err()
}

func (a *analytics) RevenueSeries(ctx context.Context, companyID store.ID, source string) ([]store.DatedAmount, []store.DatedAmount, error) {
	if source == "all" {
		billed, err := a.datedSeries(ctx, `SELECT created_at, date, total  FROM entries WHERE company_id = $1`, companyID)
		if err != nil {
			return nil, nil, err
		}
		collected, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM entries WHERE company_id = $1`, companyID)
		if err != nil {
			return nil, nil, err
		}
		return billed, collected, nil
	}
	// invoices carry no `date`? they do (date text). invoice_received has date too.
	billed, err := a.datedSeries(ctx, `SELECT created_at, date, total_amount FROM invoices WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, nil, err
	}
	collected, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM invoice_received WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, nil, err
	}
	return billed, collected, nil
}
