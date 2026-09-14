package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type companies struct{ pool *pgxpool.Pool }

func (s *Store) Companies() store.Companies { return &companies{pool: s.pool} }

const companyColumns = `id, name, firm, address, phone, gst, url, upi_qr, account_no, ifsc, bank_name, is_default, is_active`

func scanCompany(row interface {
	Scan(...any) error
}) (store.Company, error) {
	var c store.Company
	err := row.Scan(&c.ID, &c.Name, &c.Firm, &c.Address, &c.Phone, &c.Gst, &c.URL,
		&c.UpiQr, &c.AccountNo, &c.Ifsc, &c.BankName, &c.IsDefault, &c.IsActive)
	return c, err
}

func (c *companies) List(ctx context.Context, uid store.ID) ([]store.Company, error) {
	// default first, then oldest, then id (#19 tiebreak) - matching Node's {is_default:-1,createdAt:1}.
	rows, err := c.pool.Query(ctx,
		`SELECT `+companyColumns+` FROM companies WHERE uid = $1 AND is_active
		 ORDER BY is_default DESC, created_at ASC, id ASC`, string(uid))
	if err != nil {
		return nil, fmt.Errorf("listing companies: %w", err)
	}
	defer rows.Close()

	out := make([]store.Company, 0)
	for rows.Next() {
		company, err := scanCompany(rows)
		if err != nil {
			return nil, fmt.Errorf("reading companies: %w", err)
		}
		out = append(out, company)
	}
	return out, rows.Err()
}

func (c *companies) Active(ctx context.Context, companyID, uid store.ID) (store.Company, error) {
	company, err := scanCompany(c.pool.QueryRow(ctx,
		`SELECT `+companyColumns+` FROM companies WHERE id = $1 AND uid = $2`, string(companyID), string(uid)))
	if noRows(err) {
		return store.Company{}, store.ErrNotFound
	}
	if err != nil {
		return store.Company{}, fmt.Errorf("looking up company: %w", err)
	}
	return company, nil
}
