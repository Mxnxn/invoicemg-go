package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type clients struct{ pool *pgxpool.Pool }

func (s *Store) Clients() store.Clients { return &clients{pool: s.pool} }

func (c *clients) Visible(ctx context.Context, uid, companyID store.ID) ([]store.Client, error) {
	// visibleScope in SQL: own company, a legacy null company, or shared with this company via
	// the array. Bounded by uid so sharing never crosses admins (#1). id is the tiebreak (#19).
	rows, err := c.pool.Query(ctx, `
		SELECT id, uid, company_id, client_name, client_firm, client_phone, client_gst,
		       client_address, opening_balance, shared_company_ids
		  FROM clients
		 WHERE uid = $1
		   AND (company_id = $2 OR company_id IS NULL OR $2 = ANY (shared_company_ids))
		 ORDER BY id ASC`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()

	out := make([]store.Client, 0)
	for rows.Next() {
		var cl store.Client
		var companyIDCol *string
		var shared []string
		if err := rows.Scan(&cl.ID, &cl.UID, &companyIDCol, &cl.ClientName, &cl.ClientFirm,
			&cl.ClientPhone, &cl.ClientGST, &cl.ClientAddress, &cl.OpeningBalance, &shared); err != nil {
			return nil, fmt.Errorf("reading clients: %w", err)
		}
		if companyIDCol != nil {
			cl.CompanyID = store.ID(*companyIDCol)
		}
		// The column is NOT NULL, so the array is always present here - every Postgres client
		// has a sharing list, unlike a legacy Mongo record. Convert to []store.ID (possibly
		// empty), and set the pointer so the row sends {companies:[...]} like a stamped record.
		ids := make([]store.ID, 0, len(shared))
		for _, id := range shared {
			ids = append(ids, store.ID(id))
		}
		cl.Sharing = &ids
		out = append(out, cl)
	}
	return out, rows.Err()
}
