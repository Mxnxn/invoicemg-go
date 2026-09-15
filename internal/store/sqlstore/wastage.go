package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type wastages struct{ pool *pgxpool.Pool }

func (s *Store) Wastages() store.Wastages { return &wastages{pool: s.pool} }

func (w *wastages) List(ctx context.Context, companyID store.ID) ([]store.Wastage, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT id, uid, company_id, material_name, rate, purchase_rate, cost_total, length, height, total, date, created_at, updated_at
		  FROM wastages WHERE company_id = $1 ORDER BY created_at DESC, id DESC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing wastages: %w", err)
	}
	defer rows.Close()
	out := make([]store.Wastage, 0)
	for rows.Next() {
		var wa store.Wastage
		var companyIDCol *string
		if err := rows.Scan(&wa.ID, &wa.UID, &companyIDCol, &wa.MaterialName, &wa.Rate, &wa.PurchaseRate,
			&wa.CostTotal, &wa.Length, &wa.Height, &wa.Total, &wa.Date, &wa.CreatedAt, &wa.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading wastages: %w", err)
		}
		if companyIDCol != nil {
			wa.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, wa)
	}
	return out, rows.Err()
}
