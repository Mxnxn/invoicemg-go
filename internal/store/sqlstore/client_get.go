package sqlstore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Get is /client/get: the client with its entries populated newest-first, each carrying its
// issued-invoice number. company_id is server-derived, so this can never be widened by the
// caller. The relational schema has no entries.quotation_id, so the quotation link stays empty
// (Node populates it from Entry.quotation_id); that gap is tracked with the schema.
func (c *clients) Get(ctx context.Context, companyID, clientID store.ID) (store.ClientDetail, bool, error) {
	var d store.ClientDetail
	var companyCol, uidCol *string
	var legacy *int64
	err := c.pool.QueryRow(ctx, `
		SELECT id, uid, company_id, client_id, client_name, client_firm, client_phone,
			client_gst, client_address
		  FROM clients WHERE id = $1 AND company_id = $2`,
		string(clientID), string(companyID)).Scan(
		&d.ID, &uidCol, &companyCol, &legacy, &d.ClientName, &d.ClientFirm, &d.ClientPhone,
		&d.ClientGST, &d.ClientAddress)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ClientDetail{}, false, nil
	}
	if err != nil {
		return store.ClientDetail{}, false, err
	}
	if companyCol != nil {
		d.CompanyID = store.ID(*companyCol)
	}
	if uidCol != nil {
		d.UID = store.ID(*uidCol)
	}
	d.LegacyID = legacy

	rows, err := c.pool.Query(ctx, `
		SELECT e.id, e.uid, e.company_id, e.client_id, e.material, e.hsn, e.description,
			e.length, e.width, e.date, e.qty, e.rate, e.cgst, e.sgst, e.igst, e.discount,
			e.charges, e.amount, e.advance, e.total, e.has_issued, e.created_at, e.updated_at,
			e.invoice_id, i.invoice_id
		  FROM entries e
		  LEFT JOIN invoices i ON i.id = e.invoice_id
		 WHERE e.client_id = $1 AND e.company_id = $2
		 ORDER BY e.created_at DESC, e.id DESC`,
		string(clientID), string(companyID))
	if err != nil {
		return store.ClientDetail{}, false, err
	}
	defer rows.Close()
	d.Entries = make([]store.ClientEntryView, 0)
	for rows.Next() {
		var v store.ClientEntryView
		var eCompany, eClient, issuedFK, invoiceNo *string
		if err := rows.Scan(&v.ID, &v.UID, &eCompany, &eClient, &v.Material, &v.Hsn, &v.Description,
			&v.Length, &v.Width, &v.Date, &v.Qty, &v.Rate, &v.Cgst, &v.Sgst, &v.Igst, &v.Discount,
			&v.Charges, &v.Amount, &v.Advance, &v.Total, &v.HasIssued, &v.CreatedAt, &v.UpdatedAt,
			&issuedFK, &invoiceNo); err != nil {
			return store.ClientDetail{}, false, err
		}
		if eCompany != nil {
			v.CompanyID = store.ID(*eCompany)
		}
		if eClient != nil {
			v.ClientID = store.ID(*eClient)
		}
		if issuedFK != nil {
			v.IssuedID = store.ID(*issuedFK)
		}
		if invoiceNo != nil {
			v.IssuedInvoiceID = *invoiceNo
		}
		d.Entries = append(d.Entries, v)
	}
	return d, true, rows.Err()
}
