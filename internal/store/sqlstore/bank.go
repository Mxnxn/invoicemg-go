package sqlstore

import (
	"context"
	"fmt"

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
