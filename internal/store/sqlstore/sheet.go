package sqlstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type sheets struct{ pool *pgxpool.Pool }

func (s *Store) Sheets() store.Sheets { return &sheets{pool: s.pool} }

// Get returns the day-sheet's date and its entries with each client populated. Membership is
// by date - an entry belongs to the sheet whose company and calendar day it shares - since the
// relational schema has no sheet_id on entries (Mongo's Sheet.entries array).
func (sh *sheets) Get(ctx context.Context, companyID, sheetID store.ID) (string, []store.Entry, bool, error) {
	var date string
	err := sh.pool.QueryRow(ctx,
		`SELECT to_char(date, 'YYYY-MM-DD') FROM sheets WHERE id = $1 AND company_id = $2`,
		string(sheetID), string(companyID)).Scan(&date)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, err
	}

	rows, err := sh.pool.Query(ctx, `
		SELECT e.id, e.uid, e.company_id, e.client_id, e.material, e.hsn, e.description,
			e.length, e.width, e.date, e.qty, e.rate, e.cgst, e.sgst, e.igst, e.discount,
			e.charges, e.amount, e.advance, e.total, e.has_issued, e.created_at, e.updated_at,
			c.id, c.company_id, c.uid, c.client_id, c.client_name, c.client_firm, c.client_phone,
			c.client_gst, c.client_address, c.created_at, c.updated_at
		  FROM entries e
		  JOIN clients c ON c.id = e.client_id
		 WHERE e.company_id = $1 AND left(e.date, 10) = $2
		 ORDER BY e.created_at, e.id`,
		string(companyID), date)
	if err != nil {
		return "", nil, false, err
	}
	defer rows.Close()

	out := make([]store.Entry, 0)
	for rows.Next() {
		var e store.Entry
		var eCompany, eClient *string
		var c store.EntryClient
		var cCompany, cUID *string
		var cLegacy *int64
		if err := rows.Scan(&e.ID, &e.UID, &eCompany, &eClient, &e.Material, &e.Hsn, &e.Description,
			&e.Length, &e.Width, &e.Date, &e.Qty, &e.Rate, &e.Cgst, &e.Sgst, &e.Igst, &e.Discount,
			&e.Charges, &e.Amount, &e.Advance, &e.Total, &e.HasIssued, &e.CreatedAt, &e.UpdatedAt,
			&c.ID, &cCompany, &cUID, &cLegacy, &c.ClientName, &c.ClientFirm, &c.ClientPhone,
			&c.ClientGST, &c.ClientAddress, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return "", nil, false, err
		}
		if eCompany != nil {
			e.CompanyID = store.ID(*eCompany)
		}
		if eClient != nil {
			e.ClientID = store.ID(*eClient)
		}
		if cCompany != nil {
			c.CompanyID = store.ID(*cCompany)
		}
		if cUID != nil {
			c.UID = store.ID(*cUID)
		}
		c.LegacyID = cLegacy
		e.Client = &c
		out = append(out, e)
	}
	return date, out, true, rows.Err()
}
