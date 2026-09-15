package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type lookups struct{ pool *pgxpool.Pool }

func (s *Store) Lookups() store.Lookups { return &lookups{pool: s.pool} }

func (l *lookups) Clients(ctx context.Context, uid, companyID store.ID) ([]store.LookupClient, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT id, client_name, client_firm, client_phone
		  FROM clients WHERE uid = $1 AND company_id = $2 ORDER BY id ASC`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("lookup clients: %w", err)
	}
	defer rows.Close()
	out := make([]store.LookupClient, 0)
	for rows.Next() {
		var c store.LookupClient
		if err := rows.Scan(&c.ID, &c.ClientName, &c.ClientFirm, &c.ClientPhone); err != nil {
			return nil, fmt.Errorf("reading client lookups: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (l *lookups) Materials(ctx context.Context, companyID store.ID) ([]store.LookupMaterial, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT id, material_name, material_rate, hsn, tax
		  FROM materials WHERE company_id = $1 ORDER BY id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("lookup materials: %w", err)
	}
	defer rows.Close()
	out := make([]store.LookupMaterial, 0)
	for rows.Next() {
		var m store.LookupMaterial
		if err := rows.Scan(&m.ID, &m.MaterialName, &m.MaterialRate, &m.Hsn, &m.Tax); err != nil {
			return nil, fmt.Errorf("reading material lookups: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (l *lookups) People(ctx context.Context, uid store.ID) ([]store.LookupPerson, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT id, name, type FROM persons WHERE uid = $1 AND is_active ORDER BY id ASC`, string(uid))
	if err != nil {
		return nil, fmt.Errorf("lookup people: %w", err)
	}
	defer rows.Close()
	out := make([]store.LookupPerson, 0)
	for rows.Next() {
		var p store.LookupPerson
		if err := rows.Scan(&p.ID, &p.Name, &p.Type); err != nil {
			return nil, fmt.Errorf("reading people lookups: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
