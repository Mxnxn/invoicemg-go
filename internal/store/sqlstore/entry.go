package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type entries struct{ pool *pgxpool.Pool }

func (s *Store) Entries() store.Entries { return &entries{pool: s.pool} }

// entryCols is the entry projection shared by every read here; the trailing client columns
// are populated only by the join in Add and are left zero elsewhere.
const entryCols = `id, uid, company_id, client_id, invoice_id, material, hsn, description,
	length, width, date, qty, rate, cgst, sgst, igst, discount, charges, amount, advance, total,
	has_issued, created_at, updated_at`

func scanEntry(row pgx.Row) (store.Entry, error) {
	var e store.Entry
	var companyID, clientID, invoiceID *string
	if err := row.Scan(&e.ID, &e.UID, &companyID, &clientID, &invoiceID, &e.Material, &e.Hsn,
		&e.Description, &e.Length, &e.Width, &e.Date, &e.Qty, &e.Rate, &e.Cgst, &e.Sgst, &e.Igst,
		&e.Discount, &e.Charges, &e.Amount, &e.Advance, &e.Total, &e.HasIssued,
		&e.CreatedAt, &e.UpdatedAt); err != nil {
		return store.Entry{}, err
	}
	if clientID != nil {
		e.ClientID = store.ID(*clientID)
	}
	if companyID != nil {
		e.CompanyID = store.ID(*companyID)
	}
	return e, nil
}

func (en *entries) materialHSN(ctx context.Context, companyID store.ID, name string) string {
	var hsn string
	err := en.pool.QueryRow(ctx,
		`SELECT hsn FROM materials WHERE material_name = $1 AND company_id = $2 LIMIT 1`,
		name, string(companyID)).Scan(&hsn)
	if err != nil {
		return ""
	}
	return hsn
}

// ensureSheet makes sure the company's day-sheet for date exists (Node: isDateExists else new Sheet).
func ensureSheet(ctx context.Context, tx pgx.Tx, companyID, uid store.ID, date string) error {
	norm := store.NormalizeDate(date)
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id FROM sheets WHERE company_id = $1 AND date = $2::date`,
		string(companyID), norm).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO sheets (company_id, uid, date) VALUES ($1, $2, $3::date)`,
		string(companyID), string(uid), norm)
	return err
}

func (en *entries) Add(ctx context.Context, uid, companyID store.ID, in store.EntryWrite) (store.Entry, error) {
	hsn := en.materialHSN(ctx, companyID, in.Material)
	tx, err := en.pool.Begin(ctx)
	if err != nil {
		return store.Entry{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO entries (uid, company_id, client_id, material, hsn, description, length, width,
			date, qty, rate, cgst, sgst, igst, discount, charges, amount, advance, total)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id`,
		string(uid), string(companyID), string(in.ClientID), in.Material, hsn, in.Description,
		in.Length, in.Width, in.Date, in.Qty, in.Rate, in.Cgst, in.Sgst, in.Igst, in.Discount,
		in.Charges, in.Amount, in.Advance, in.Total).Scan(&id)
	if err != nil {
		return store.Entry{}, fmt.Errorf("insert entry: %w", err)
	}
	if err := ensureSheet(ctx, tx, companyID, uid, in.Date); err != nil {
		return store.Entry{}, fmt.Errorf("ensure sheet: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Entry{}, err
	}

	// Reload with the client populated, matching Node's .populate("client_id").
	e, err := scanEntry(en.pool.QueryRow(ctx,
		`SELECT `+entryCols+` FROM entries WHERE id = $1`, id))
	if err != nil {
		return store.Entry{}, err
	}
	if e.ClientID != "" {
		e.Client = en.loadClient(ctx, e.ClientID)
	}
	return e, nil
}

func (en *entries) loadClient(ctx context.Context, id store.ID) *store.EntryClient {
	var c store.EntryClient
	var companyID, uid *string
	var legacy *int64
	err := en.pool.QueryRow(ctx, `
		SELECT id, company_id, uid, client_id, client_name, client_firm, client_phone, client_gst,
			client_address, created_at, updated_at
		  FROM clients WHERE id = $1`, string(id)).Scan(
		&c.ID, &companyID, &uid, &legacy, &c.ClientName, &c.ClientFirm, &c.ClientPhone, &c.ClientGST,
		&c.ClientAddress, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil
	}
	if companyID != nil {
		c.CompanyID = store.ID(*companyID)
	}
	if uid != nil {
		c.UID = store.ID(*uid)
	}
	c.LegacyID = legacy
	return &c
}

func (en *entries) Update(ctx context.Context, uid, companyID store.ID, in store.EntryUpdate) (store.Entry, bool, error) {
	hsn := en.materialHSN(ctx, companyID, in.Material)
	tx, err := en.pool.Begin(ctx)
	if err != nil {
		return store.Entry{}, false, err
	}
	defer tx.Rollback(ctx)

	var oldDate string
	err = tx.QueryRow(ctx,
		`SELECT date FROM entries WHERE id = $1 AND company_id = $2`,
		string(in.EntryID), string(companyID)).Scan(&oldDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}

	// A date change moves the entry to (and if needed creates) the new day's sheet. The old
	// day's sheet row is left in place, exactly as Node leaves the emptied Sheet document.
	if store.NormalizeDate(oldDate) != store.NormalizeDate(in.Date) {
		if err := ensureSheet(ctx, tx, companyID, uid, in.Date); err != nil {
			return store.Entry{}, false, fmt.Errorf("ensure sheet: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE entries SET client_id=$1, material=$2, hsn=$3, description=$4, length=$5, width=$6,
			date=$7, qty=$8, rate=$9, cgst=$10, sgst=$11, igst=$12, discount=$13, charges=$14,
			amount=$15, advance=$16, total=$17, updated_at=now()
		WHERE id=$18 AND company_id=$19`,
		string(in.ClientID), in.Material, hsn, in.Description, in.Length, in.Width, in.Date, in.Qty,
		in.Rate, in.Cgst, in.Sgst, in.Igst, in.Discount, in.Charges, in.Amount, in.Advance, in.Total,
		string(in.EntryID), string(companyID))
	if err != nil {
		return store.Entry{}, false, fmt.Errorf("update entry: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Entry{}, false, err
	}
	e, err := scanEntry(en.pool.QueryRow(ctx,
		`SELECT `+entryCols+` FROM entries WHERE id = $1`, string(in.EntryID)))
	if err != nil {
		return store.Entry{}, false, err
	}
	return e, true, nil
}

func (en *entries) Get(ctx context.Context, uid, companyID, entryID store.ID) (store.Entry, bool, error) {
	e, err := scanEntry(en.pool.QueryRow(ctx,
		`SELECT `+entryCols+` FROM entries WHERE id = $1 AND company_id = $2`,
		string(entryID), string(companyID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	return e, true, nil
}

func (en *entries) List(ctx context.Context, uid, companyID store.ID) ([]store.Entry, error) {
	rows, err := en.pool.Query(ctx,
		`SELECT `+entryCols+` FROM entries WHERE company_id = $1 ORDER BY created_at, id`,
		string(companyID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SinceForClient returns a client's entries created at or after since (zero = no bound), newest
// first, each with its issued-invoice number - the /stats/exports input.
func (en *entries) SinceForClient(ctx context.Context, uid, companyID, clientID store.ID, since time.Time) ([]store.ClientEntryView, error) {
	var sinceArg any
	if !since.IsZero() {
		sinceArg = since
	}
	rows, err := en.pool.Query(ctx, `
		SELECT e.id, e.uid, e.company_id, e.client_id, e.invoice_id, e.material, e.hsn, e.description,
			e.length, e.width, e.date, e.qty, e.rate, e.cgst, e.sgst, e.igst, e.discount, e.charges,
			e.amount, e.advance, e.total, e.has_issued, e.created_at, e.updated_at, i.invoice_id
		  FROM entries e
		  LEFT JOIN invoices i ON i.id = e.invoice_id
		 WHERE e.company_id = $1 AND e.client_id = $2 AND ($3::timestamptz IS NULL OR e.created_at >= $3)
		 ORDER BY e.created_at DESC, e.id DESC`,
		string(companyID), string(clientID), sinceArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.ClientEntryView, 0)
	for rows.Next() {
		var v store.ClientEntryView
		var company, client, entryInvoiceID, invoiceNo *string
		if err := rows.Scan(&v.ID, &v.UID, &company, &client, &entryInvoiceID, &v.Material, &v.Hsn,
			&v.Description, &v.Length, &v.Width, &v.Date, &v.Qty, &v.Rate, &v.Cgst, &v.Sgst, &v.Igst,
			&v.Discount, &v.Charges, &v.Amount, &v.Advance, &v.Total, &v.HasIssued,
			&v.CreatedAt, &v.UpdatedAt, &invoiceNo); err != nil {
			return nil, err
		}
		if company != nil {
			v.CompanyID = store.ID(*company)
		}
		if client != nil {
			v.ClientID = store.ID(*client)
		}
		if invoiceNo != nil {
			v.IssuedInvoiceID = *invoiceNo
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
