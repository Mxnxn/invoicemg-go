package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type challans struct{ pool *pgxpool.Pool }

func (s *Store) Challans() store.Challans { return &challans{pool: s.pool} }

func (c *challans) List(ctx context.Context, companyID store.ID) ([]store.Challan, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id, uid, company_id, company_name, description, date, type, quantity, amount
		  FROM challans WHERE company_id = $1 ORDER BY id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing challans: %w", err)
	}
	defer rows.Close()
	out := make([]store.Challan, 0)
	for rows.Next() {
		var ch store.Challan
		var companyIDCol *string
		if err := rows.Scan(&ch.ID, &ch.UID, &companyIDCol, &ch.CompanyName, &ch.Description,
			&ch.Date, &ch.Type, &ch.Quantity, &ch.Amount); err != nil {
			return nil, fmt.Errorf("reading challans: %w", err)
		}
		if companyIDCol != nil {
			ch.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}
