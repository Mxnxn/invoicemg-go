package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type trash struct{ pool *pgxpool.Pool }

func (s *Store) Trash() store.Trash { return &trash{pool: s.pool} }

func (t *trash) PasswordHash(ctx context.Context, uid store.ID) (string, error) {
	var hash string
	err := t.pool.QueryRow(ctx, `SELECT password FROM trash_users WHERE uid = $1`, string(uid)).Scan(&hash)
	if noRows(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("trash password: %w", err)
	}
	return hash, nil
}

func (t *trash) SetPassword(ctx context.Context, uid store.ID, hash string) error {
	_, err := t.pool.Exec(ctx, `
		INSERT INTO trash_users (uid, password) VALUES ($1, $2)
		ON CONFLICT (uid) DO UPDATE SET password = EXCLUDED.password, updated_at = now()`, string(uid), hash)
	if err != nil {
		return fmt.Errorf("set trash password: %w", err)
	}
	return nil
}

func (t *trash) List(ctx context.Context, companyID store.ID) ([]store.TrashRecord, error) {
	rows, err := t.pool.Query(ctx, `
		SELECT tr.id, tr.client_id, COALESCE(c.client_name,''), COALESCE(c.client_firm,''),
		       tr.description, tr.material, tr.rate, tr.qty, tr.has_dimensions, tr.length, tr.width,
		       tr.date, tr.amount, tr.cgst, tr.sgst, tr.igst, tr.created_at, tr.updated_at
		  FROM trash tr LEFT JOIN clients c ON c.id = tr.client_id
		 WHERE tr.company_id = $1 ORDER BY tr.created_at DESC, tr.id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing trash: %w", err)
	}
	defer rows.Close()
	out := make([]store.TrashRecord, 0)
	for rows.Next() {
		var r store.TrashRecord
		var clientID *string
		var hasDim bool
		if err := rows.Scan(&r.ID, &clientID, &r.ClientName, &r.ClientFirm, &r.Description, &r.Material,
			&r.Rate, &r.Qty, &hasDim, &r.Length, &r.Width, &r.Date, &r.Amount, &r.Cgst, &r.Sgst, &r.Igst,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading trash: %w", err)
		}
		hd := hasDim
		r.HasDimensions = &hd
		if clientID != nil {
			r.ClientID = store.ID(*clientID)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
