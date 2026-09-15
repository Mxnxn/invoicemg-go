package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type statistics struct{ pool *pgxpool.Pool }

func (s *Store) Statistics() store.Statistics { return &statistics{pool: s.pool} }

func (s *statistics) Clients(ctx context.Context, companyID store.ID) ([]store.LookupClient, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, client_name, client_firm FROM clients WHERE company_id = $1 ORDER BY id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("stats clients: %w", err)
	}
	defer rows.Close()
	out := make([]store.LookupClient, 0)
	for rows.Next() {
		var c store.LookupClient
		if err := rows.Scan(&c.ID, &c.ClientName, &c.ClientFirm); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *statistics) Client(ctx context.Context, companyID, clientID store.ID) (store.LookupClient, error) {
	var c store.LookupClient
	err := s.pool.QueryRow(ctx, `SELECT id, client_name, client_firm FROM clients WHERE id = $1 AND company_id = $2`, string(clientID), string(companyID)).
		Scan(&c.ID, &c.ClientName, &c.ClientFirm)
	if noRows(err) {
		return store.LookupClient{}, store.ErrNotFound
	}
	if err != nil {
		return store.LookupClient{}, err
	}
	return c, nil
}

func (s *statistics) StatEntries(ctx context.Context, companyID store.ID, monthName string, paid bool, clientID store.ID) ([]store.StatEntry, error) {
	// date ILIKE '%Mon%' reproduces the Node month-regex on the string date (#31); the total
	// flag is total=0 (paid) or total>0.
	flag := "e.total > 0"
	if paid {
		flag = "e.total = 0"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT material, length, width, qty, date, has_dimensions
		  FROM entries e
		 WHERE e.company_id = $1 AND e.date ILIKE '%'||$2||'%' AND `+flag+`
		   AND ($3 = '' OR e.client_id = $3)`, string(companyID), monthName, string(clientID))
	if err != nil {
		return nil, fmt.Errorf("stat entries: %w", err)
	}
	defer rows.Close()
	out := make([]store.StatEntry, 0)
	for rows.Next() {
		var e store.StatEntry
		var hasDim bool
		if err := rows.Scan(&e.Material, &e.Length, &e.Width, &e.Qty, &e.Date, &hasDim); err != nil {
			return nil, err
		}
		hd := hasDim
		e.HasDimensions = &hd
		out = append(out, e)
	}
	return out, rows.Err()
}
