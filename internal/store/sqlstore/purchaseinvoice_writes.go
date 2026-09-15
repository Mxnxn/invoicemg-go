package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// get loads one populated purchase invoice (supplier + rows), owner+company scoped.
func (p *purchaseInvoices) get(ctx context.Context, uid, companyID, invoiceID store.ID) (store.PurchaseInvoice, bool, error) {
	var inv store.PurchaseInvoice
	var companyCol *string
	var sID, sName, sFirm, sPhone *string
	err := p.pool.QueryRow(ctx, `
		SELECT pi.id, pi.uid, pi.company_id, pi.date, pi.invoice_number, pi.total, pi.amount, pi.created_at, pi.updated_at,
		       s.id, s.name, s.firm, s.phone
		  FROM purchase_invoices pi LEFT JOIN persons s ON s.id = pi.supplier_id
		 WHERE pi.id=$1 AND pi.uid=$2 AND pi.company_id=$3`, string(invoiceID), string(uid), string(companyID)).
		Scan(&inv.ID, &inv.UID, &companyCol, &inv.Date, &inv.InvoiceNumber, &inv.Total, &inv.Amount,
			&inv.CreatedAt, &inv.UpdatedAt, &sID, &sName, &sFirm, &sPhone)
	if noRows(err) {
		return store.PurchaseInvoice{}, false, nil
	}
	if err != nil {
		return store.PurchaseInvoice{}, false, fmt.Errorf("reading purchase invoice: %w", err)
	}
	if companyCol != nil {
		inv.CompanyID = store.ID(*companyCol)
	}
	if sID != nil {
		inv.Supplier = &store.PurchaseSupplier{ID: store.ID(*sID), Name: deref(sName), Firm: deref(sFirm), Phone: deref(sPhone)}
	}
	inv.Rows = make([]store.PurchaseInvoiceRow, 0)
	rRows, err := p.pool.Query(ctx, `
		SELECT id, description, material, hsn, gst, has_dimensions, length, width, rate, qty, unit, discount, charges, created_at, updated_at
		  FROM purchase_invoice_rows WHERE invoice_id=$1 ORDER BY position ASC, id ASC`, string(invoiceID))
	if err != nil {
		return store.PurchaseInvoice{}, false, fmt.Errorf("reading purchase invoice rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var r store.PurchaseInvoiceRow
		var hasDim bool
		if err := rRows.Scan(&r.ID, &r.Description, &r.Material, &r.Hsn, &r.Gst, &hasDim, &r.Length, &r.Width,
			&r.Rate, &r.Qty, &r.Unit, &r.Discount, &r.Charges, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return store.PurchaseInvoice{}, false, err
		}
		hd := hasDim
		r.HasDimensions = &hd
		inv.Rows = append(inv.Rows, r)
	}
	return inv, true, rRows.Err()
}

// insertPurchaseRows writes the row set in order. Purchase rows are by-quantity (has_dimensions
// false), and default length/width to "1".
func insertPurchaseRows(ctx context.Context, tx pgx.Tx, invoiceID store.ID, rows []store.PurchaseRowInput) error {
	for i, r := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO purchase_invoice_rows (invoice_id, position, description, material, hsn, gst, has_dimensions, rate, qty, unit, discount, charges)
			VALUES ($1,$2,$3,$4,$5,$6,false,$7,$8,$9,$10,$11)`,
			string(invoiceID), i, r.Description, r.Material, r.Hsn, r.Gst, r.Rate, r.Qty, r.Unit, r.Discount, r.Charges); err != nil {
			return err
		}
	}
	return nil
}

func (p *purchaseInvoices) Create(ctx context.Context, uid, companyID store.ID, in store.PurchaseInvoiceWrite) (store.PurchaseInvoice, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseInvoice{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO purchase_invoices (uid, company_id, supplier_id, date, invoice_number, total)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		string(uid), string(companyID), string(in.SupplierID), in.Date, in.InvoiceNumber, in.Total).Scan(&id); err != nil {
		return store.PurchaseInvoice{}, fmt.Errorf("insert purchase invoice: %w", err)
	}
	if err := insertPurchaseRows(ctx, tx, store.ID(id), in.Rows); err != nil {
		return store.PurchaseInvoice{}, fmt.Errorf("insert purchase invoice rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PurchaseInvoice{}, err
	}
	out, _, err := p.get(ctx, uid, companyID, store.ID(id))
	return out, err
}

func (p *purchaseInvoices) Update(ctx context.Context, uid, companyID, invoiceID store.ID, in store.PurchaseInvoiceUpdate) (store.PurchaseInvoice, store.PurchaseUpdateResult, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
	}
	defer tx.Rollback(ctx)

	var amount float64
	if err := tx.QueryRow(ctx, `SELECT amount FROM purchase_invoices WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(invoiceID), string(uid), string(companyID)).Scan(&amount); noRows(err) {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{Status: store.PurchaseUpdateNotFound}, nil
	} else if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, fmt.Errorf("looking up purchase invoice: %w", err)
	}

	if in.SupplierID != nil {
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET supplier_id=$2 WHERE id=$1`, string(invoiceID), string(*in.SupplierID)); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
	}
	if in.Date != nil {
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET date=$2 WHERE id=$1`, string(invoiceID), *in.Date); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
	}
	if in.InvoiceNumber != nil {
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET invoice_number=$2 WHERE id=$1`, string(invoiceID), *in.InvoiceNumber); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
	}
	if in.Rows != nil {
		// Editing rows recalculates the total; refuse if that drops below what is already paid.
		if in.NewTotal < amount-0.01 {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{Status: store.PurchaseUpdatePaidExceeds, AmountPaid: amount, NewTotal: in.NewTotal}, nil
		}
		if _, err := tx.Exec(ctx, `DELETE FROM purchase_invoice_rows WHERE invoice_id=$1`, string(invoiceID)); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
		if err := insertPurchaseRows(ctx, tx, invoiceID, *in.Rows); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET total=$2 WHERE id=$1`, string(invoiceID), in.NewTotal); err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET updated_at=now() WHERE id=$1`, string(invoiceID)); err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
	}
	out, _, err := p.get(ctx, uid, companyID, invoiceID)
	return out, store.PurchaseUpdateResult{Status: store.PurchaseUpdateOK}, err
}

func (p *purchaseInvoices) Delete(ctx context.Context, uid, companyID, invoiceID store.ID) (store.PurchaseDeleteStatus, error) {
	// A supplier payment allocating to this invoice blocks the delete (allocations is a JSONB
	// array of {purchase_invoice_id, amount}); the @> containment test matches any element.
	var paid int
	needle := fmt.Sprintf(`[{"purchase_invoice_id":%q}]`, string(invoiceID))
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM supplier_payments WHERE company_id=$1 AND allocations @> $2::jsonb`,
		string(companyID), needle).Scan(&paid); err != nil {
		return store.PurchaseDeleteNotFound, fmt.Errorf("checking supplier payments: %w", err)
	}
	if paid > 0 {
		return store.PurchaseDeleteHasPayment, nil
	}
	tag, err := p.pool.Exec(ctx, `DELETE FROM purchase_invoices WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(invoiceID), string(uid), string(companyID))
	if err != nil {
		return store.PurchaseDeleteNotFound, fmt.Errorf("delete purchase invoice: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.PurchaseDeleteNotFound, nil
	}
	return store.PurchaseDeleteOK, nil
}
