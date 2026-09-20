package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type companies struct{ pool *pgxpool.Pool }

func (s *Store) Companies() store.Companies { return &companies{pool: s.pool} }

const companyColumns = `id, name, firm, address, phone, gst, url, upi_qr, account_no, ifsc, bank_name, invoice_template, quotation_template, ledger_template, document_show_units, document_show_size, wa_phone_number_id, wa_business_account_id, wa_api_token, is_default, is_active`

func scanCompany(row interface {
	Scan(...any) error
}) (store.Company, error) {
	var c store.Company
	err := row.Scan(&c.ID, &c.Name, &c.Firm, &c.Address, &c.Phone, &c.Gst, &c.URL,
		&c.UpiQr, &c.AccountNo, &c.Ifsc, &c.BankName, &c.InvoiceTemplate, &c.QuotationTemplate,
		&c.LedgerTemplate, &c.DocumentShowUnits, &c.DocumentShowSize,
		&c.WaPhoneNumberID, &c.WaBusinessAccountID, &c.WaAPIToken, &c.IsDefault, &c.IsActive)
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
	url := orElse(patch.URL, cur.URL)
	accountNo := orElse(patch.AccountNo, cur.AccountNo)
	ifsc := orElse(patch.Ifsc, cur.Ifsc)
	bankName := orElse(patch.BankName, cur.BankName)
	invoiceTpl := orElse(patch.InvoiceTemplate, cur.InvoiceTemplate)
	quotationTpl := orElse(patch.QuotationTemplate, cur.QuotationTemplate)
	ledgerTpl := orElse(patch.LedgerTemplate, cur.LedgerTemplate)
	waPhone := orElse(patch.WaPhoneNumberID, cur.WaPhoneNumberID)
	waBiz := orElse(patch.WaBusinessAccountID, cur.WaBusinessAccountID)
	waToken := orElse(patch.WaAPIToken, cur.WaAPIToken)
	orElseBool := func(p *bool, def bool) bool {
		if p != nil {
			return *p
		}
		return def
	}
	showUnits := orElseBool(patch.DocumentShowUnits, cur.DocumentShowUnits)
	showSize := orElseBool(patch.DocumentShowSize, cur.DocumentShowSize)

	company, err := scanCompany(c.pool.QueryRow(ctx, `
		UPDATE companies SET name=$3, firm=$4, address=$5, phone=$6, gst=$7, url=$17, account_no=$8, ifsc=$9, bank_name=$10,
			invoice_template=$11, quotation_template=$12, ledger_template=$13,
			wa_phone_number_id=$14, wa_business_account_id=$15, wa_api_token=$16,
			document_show_units=$18, document_show_size=$19, updated_at=now()
		 WHERE id=$1 AND uid=$2 RETURNING `+companyColumns,
		string(companyID), string(uid), name, firm, address, phone, gst, accountNo, ifsc, bankName,
		invoiceTpl, quotationTpl, ledgerTpl, waPhone, waBiz, waToken, url, showUnits, showSize))
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

func (c *companies) SetQueueOrder(ctx context.Context, companyID store.ID, order []string) ([]string, bool, error) {
	var stored []string
	err := c.pool.QueryRow(ctx,
		`UPDATE companies SET queue_order=$2, updated_at=now() WHERE id=$1 RETURNING queue_order`,
		string(companyID), order).Scan(&stored)
	if noRows(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("set queue order: %w", err)
	}
	return stored, true, nil
}

func (c *companies) Numbering(ctx context.Context, companyID, uid store.ID) (map[string]json.RawMessage, error) {
	var raw []byte
	err := c.pool.QueryRow(ctx, `SELECT numbering FROM companies WHERE id=$1 AND uid=$2`,
		string(companyID), string(uid)).Scan(&raw)
	if noRows(err) {
		return map[string]json.RawMessage{}, nil // read never 404s
	}
	if err != nil {
		return nil, fmt.Errorf("read numbering: %w", err)
	}
	out := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode numbering: %w", err)
		}
	}
	return out, nil
}

func (c *companies) SetNumbering(ctx context.Context, companyID, uid store.ID, kind string, format json.RawMessage) (bool, error) {
	// jsonb_set with a parameterised path key is injection-safe; kind is also handler-whitelisted.
	tag, err := c.pool.Exec(ctx,
		`UPDATE companies SET numbering = jsonb_set(numbering, ARRAY[$3], $4::jsonb, true), updated_at=now()
		  WHERE id=$1 AND uid=$2`,
		string(companyID), string(uid), kind, string(format))
	if err != nil {
		return false, fmt.Errorf("set numbering: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (c *companies) SetReportsAcrossCompanies(ctx context.Context, companyID, uid store.ID, on bool) (bool, error) {
	tag, err := c.pool.Exec(ctx,
		`UPDATE companies SET reports_across_companies=$3, updated_at=now() WHERE id=$1 AND uid=$2`,
		string(companyID), string(uid), on)
	if err != nil {
		return false, fmt.Errorf("set sharing toggle: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Scope is Helpers/CompanyScope.scopeFor. Fail-closed to the acting company on any trouble.
func (c *companies) Scope(ctx context.Context, companyID, uid store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	var name, firm, ownerUID string
	var on bool
	err := c.pool.QueryRow(ctx,
		`SELECT name, firm, uid, reports_across_companies FROM companies WHERE id=$1`, string(companyID)).
		Scan(&name, &firm, &ownerUID, &on)
	if err != nil {
		return []store.ID{companyID}, false, map[store.ID]string{}, nil
	}
	label := name
	if label == "" {
		label = firm
	}
	if !on || ownerUID == "" {
		return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
	}
	rows, err := c.pool.Query(ctx, `SELECT id, name, firm FROM companies WHERE uid=$1`, ownerUID)
	if err != nil {
		return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
	}
	defer rows.Close()
	ids := make([]store.ID, 0)
	labels := map[store.ID]string{}
	has := false
	for rows.Next() {
		var id, n, f string
		if err := rows.Scan(&id, &n, &f); err != nil {
			return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
		}
		l := n
		if l == "" {
			l = f
		}
		ids = append(ids, store.ID(id))
		labels[store.ID(id)] = l
		if store.ID(id) == companyID {
			has = true
		}
	}
	if len(ids) == 0 {
		return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
	}
	if !has {
		ids = append(ids, companyID)
		labels[companyID] = label
	}
	return ids, len(ids) > 1, labels, nil
}
