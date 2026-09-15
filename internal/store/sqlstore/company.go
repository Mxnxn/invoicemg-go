package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type companies struct{ pool *pgxpool.Pool }

func (s *Store) Companies() store.Companies { return &companies{pool: s.pool} }

const companyColumns = `id, name, firm, address, phone, gst, url, upi_qr, account_no, ifsc, bank_name, invoice_template, quotation_template, ledger_template, wa_phone_number_id, wa_business_account_id, wa_api_token, is_default, is_active`

func scanCompany(row interface {
	Scan(...any) error
}) (store.Company, error) {
	var c store.Company
	err := row.Scan(&c.ID, &c.Name, &c.Firm, &c.Address, &c.Phone, &c.Gst, &c.URL,
		&c.UpiQr, &c.AccountNo, &c.Ifsc, &c.BankName, &c.InvoiceTemplate, &c.QuotationTemplate,
		&c.LedgerTemplate, &c.WaPhoneNumberID, &c.WaBusinessAccountID, &c.WaAPIToken, &c.IsDefault, &c.IsActive)
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

func (c *companies) Count(ctx context.Context, uid store.ID) (int, error) {
	var n int
	if err := c.pool.QueryRow(ctx, `SELECT count(*) FROM companies WHERE uid = $1`, string(uid)).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting companies: %w", err)
	}
	return n, nil
}

func (c *companies) Create(ctx context.Context, uid store.ID, in store.CompanyWrite) (store.Company, error) {
	n, err := c.Count(ctx, uid)
	if err != nil {
		return store.Company{}, err
	}
	firm := in.Firm
	if firm == "" {
		firm = in.Name // Node's `firm || name`
	}
	company, err := scanCompany(c.pool.QueryRow(ctx, `
		INSERT INTO companies (uid, name, firm, address, phone, gst, account_no, ifsc, bank_name, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+companyColumns,
		string(uid), in.Name, firm, in.Address, in.Phone, in.Gst, in.AccountNo, in.Ifsc, in.BankName, n == 0))
	if err != nil {
		return store.Company{}, fmt.Errorf("insert company: %w", err)
	}
	return company, nil
}

func (c *companies) Update(ctx context.Context, uid, companyID store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	// Load, apply the submitted fields, write back the editable set - the same read-modify-write
	// shape the person update uses, so an omitted field is genuinely left alone.
	cur, err := c.Active(ctx, companyID, uid)
	if err == store.ErrNotFound {
		return store.Company{}, false, nil
	}
	if err != nil {
		return store.Company{}, false, err
	}
	orElse := func(p *string, def string) string {
		if p != nil {
			return *p
		}
		return def
	}
	name := orElse(patch.Name, cur.Name)
	firm := orElse(patch.Firm, cur.Firm)
	address := orElse(patch.Address, cur.Address)
	phone := orElse(patch.Phone, cur.Phone)
	gst := orElse(patch.Gst, cur.Gst)
	accountNo := orElse(patch.AccountNo, cur.AccountNo)
	ifsc := orElse(patch.Ifsc, cur.Ifsc)
	bankName := orElse(patch.BankName, cur.BankName)
	invoiceTpl := orElse(patch.InvoiceTemplate, cur.InvoiceTemplate)
	quotationTpl := orElse(patch.QuotationTemplate, cur.QuotationTemplate)
	ledgerTpl := orElse(patch.LedgerTemplate, cur.LedgerTemplate)
	waPhone := orElse(patch.WaPhoneNumberID, cur.WaPhoneNumberID)
	waBiz := orElse(patch.WaBusinessAccountID, cur.WaBusinessAccountID)
	waToken := orElse(patch.WaAPIToken, cur.WaAPIToken)

	company, err := scanCompany(c.pool.QueryRow(ctx, `
		UPDATE companies SET name=$3, firm=$4, address=$5, phone=$6, gst=$7, account_no=$8, ifsc=$9, bank_name=$10,
			invoice_template=$11, quotation_template=$12, ledger_template=$13,
			wa_phone_number_id=$14, wa_business_account_id=$15, wa_api_token=$16, updated_at=now()
		 WHERE id=$1 AND uid=$2 RETURNING `+companyColumns,
		string(companyID), string(uid), name, firm, address, phone, gst, accountNo, ifsc, bankName,
		invoiceTpl, quotationTpl, ledgerTpl, waPhone, waBiz, waToken))
	if noRows(err) {
		return store.Company{}, false, nil
	}
	if err != nil {
		return store.Company{}, false, fmt.Errorf("update company: %w", err)
	}
	return company, true, nil
}

func (c *companies) FindActive(ctx context.Context, uid, companyID store.ID) (store.Company, bool, error) {
	company, err := scanCompany(c.pool.QueryRow(ctx,
		`SELECT `+companyColumns+` FROM companies WHERE id=$1 AND uid=$2 AND is_active`, string(companyID), string(uid)))
	if noRows(err) {
		return store.Company{}, false, nil
	}
	if err != nil {
		return store.Company{}, false, fmt.Errorf("finding active company: %w", err)
	}
	return company, true, nil
}

func (c *companies) Deactivate(ctx context.Context, uid, companyID store.ID) (store.DeactivateResult, error) {
	var active int
	if err := c.pool.QueryRow(ctx, `SELECT count(*) FROM companies WHERE uid=$1 AND is_active`, string(uid)).Scan(&active); err != nil {
		return store.DeactivateNotFound, fmt.Errorf("counting active companies: %w", err)
	}
	if active <= 1 {
		return store.DeactivateMustKeepOne, nil
	}
	var isDefault bool
	err := c.pool.QueryRow(ctx, `SELECT is_default FROM companies WHERE id=$1 AND uid=$2`, string(companyID), string(uid)).Scan(&isDefault)
	if noRows(err) {
		return store.DeactivateNotFound, nil
	}
	if err != nil {
		return store.DeactivateNotFound, fmt.Errorf("looking up company: %w", err)
	}
	if isDefault {
		return store.DeactivateIsDefault, nil
	}
	if _, err := c.pool.Exec(ctx, `UPDATE companies SET is_active=false, updated_at=now() WHERE id=$1 AND uid=$2`, string(companyID), string(uid)); err != nil {
		return store.DeactivateNotFound, fmt.Errorf("deactivating company: %w", err)
	}
	// Strand no tab on a dead company; the next request falls back to the default.
	if _, err := c.pool.Exec(ctx, `DELETE FROM company_sessions WHERE company_id=$1`, string(companyID)); err != nil {
		return store.DeactivateNotFound, fmt.Errorf("clearing tab bindings: %w", err)
	}
	return store.DeactivateOK, nil
}
