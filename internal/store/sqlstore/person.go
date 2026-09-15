package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type people struct{ pool *pgxpool.Pool }

func (s *Store) People() store.People { return &people{pool: s.pool} }

func (p *people) List(ctx context.Context, uid store.ID, personType string) ([]store.Person, error) {
	// Optional type filter: $2 = '' means no filter (the People vs Suppliers tabs pass a type).
	// id is the tiebreak (#19).
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, type, email, phone, firm, address, gst, opening_balance, is_active,
		       permissions, notify_po_created, notify_po_updated, notify_po_confirmed, created_at
		  FROM persons
		 WHERE uid = $1 AND ($2 = '' OR type = $2)
		 ORDER BY id ASC`, string(uid), personType)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	defer rows.Close()

	out := make([]store.Person, 0)
	for rows.Next() {
		var pr store.Person
		var email *string
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Type, &email, &pr.Phone, &pr.Firm, &pr.Address,
			&pr.Gst, &pr.OpeningBalance, &pr.IsActive, &pr.Permissions,
			&pr.NotifyPoCreated, &pr.NotifyPoUpdated, &pr.NotifyPoConfirmed, &pr.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading people: %w", err)
		}
		if email != nil {
			pr.Email = *email
		}
		if pr.Permissions == nil {
			pr.Permissions = []string{}
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}
