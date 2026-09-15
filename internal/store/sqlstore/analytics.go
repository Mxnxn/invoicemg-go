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

// rank groups invoices by client and joins the client name. valueExpr is the SUM expression;
// having is an optional "HAVING <cond>" (empty for none). Ordered by the value desc, id for
// determinism.
func (a *analytics) rank(ctx context.Context, companyID store.ID, valueExpr, having string) ([]store.ClientRank, error) {
	sql := `
		SELECT c.id, c.client_name, c.client_firm, round(` + valueExpr + `, 2) AS val
		  FROM invoices iv
		  JOIN clients c ON c.id = iv.client_id
		 WHERE iv.company_id = $1
		 GROUP BY c.id, c.client_name, c.client_firm ` + having + `
		 ORDER BY val DESC, c.id ASC`
	rows, err := a.pool.Query(ctx, sql, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("ranking clients: %w", err)
	}
	defer rows.Close()
	out := make([]store.ClientRank, 0)
	for rows.Next() {
		var r store.ClientRank
		if err := rows.Scan(&r.ClientID, &r.ClientName, &r.ClientFirm, &r.Value); err != nil {
			return nil, fmt.Errorf("reading rank: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *analytics) TopSales(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.total_amount)", "")
}
func (a *analytics) TopCredits(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.total_amount - iv.amount)", "HAVING round(SUM(iv.total_amount - iv.amount), 2) > 0")
}
func (a *analytics) TopPaid(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.amount)", "HAVING round(SUM(iv.amount), 2) > 0")
}
