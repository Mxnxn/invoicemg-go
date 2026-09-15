package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (s *supplierPayments) OpenInvoices(ctx context.Context, uid, companyID, supplierID store.ID) ([]store.SupplierOpenInvoice, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, invoice_number, date, total, amount FROM purchase_invoices
		 WHERE uid=$1 AND company_id=$2 AND supplier_id=$3 AND COALESCE(amount,0) < total
		 ORDER BY date ASC, created_at ASC, id ASC`, string(uid), string(companyID), string(supplierID))
	if err != nil {
		return nil, fmt.Errorf("open invoices: %w", err)
	}
	defer rows.Close()
	out := make([]store.SupplierOpenInvoice, 0)
	for rows.Next() {
		var v store.SupplierOpenInvoice
		if err := rows.Scan(&v.ID, &v.InvoiceNumber, &v.Date, &v.Total, &v.Amount); err != nil {
			return nil, err
		}
		v.Due = r2(v.Total - v.Amount)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *supplierPayments) Create(ctx context.Context, uid, companyID store.ID, in store.SupplierPaymentWrite) (store.SupplierPayment, store.SupplierPayResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, err
	}
	defer tx.Rollback(ctx)

	var allocs []spAllocJSON

	if in.Mode == "auto" {
		rows, err := tx.Query(ctx, `
			SELECT id, total, COALESCE(amount,0) FROM purchase_invoices
			 WHERE uid=$1 AND company_id=$2 AND supplier_id=$3 AND COALESCE(amount,0) < total
			 ORDER BY date ASC, created_at ASC, id ASC`, string(uid), string(companyID), string(in.SupplierID))
		if err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("auto invoices: %w", err)
		}
		type open struct {
			id            string
			total, amount float64
		}
		var opens []open
		for rows.Next() {
			var o open
			if err := rows.Scan(&o.id, &o.total, &o.amount); err != nil {
				rows.Close()
				return store.SupplierPayment{}, store.SupplierPayResult{}, err
			}
			opens = append(opens, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, err
		}
		remaining := r2(in.Amount)
		for _, o := range opens {
			if remaining <= 0 {
				break
			}
			due := r2(o.total - o.amount)
			if due <= 0 {
				continue
			}
			applied := r2(math.Min(due, remaining))
			allocs = append(allocs, spAllocJSON{PurchaseInvoiceID: o.id, Amount: applied})
			remaining = r2(remaining - applied)
		}
		if remaining > 0 {
			return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayAutoUnallocated, Remaining: remaining}, nil
		}
	} else {
		for _, a := range in.Allocations {
			applied := r2(a.Amount)
			if a.InvoiceID == "" || applied <= 0 {
				continue
			}
			var total, amount float64
			err := tx.QueryRow(ctx, `SELECT total, COALESCE(amount,0) FROM purchase_invoices WHERE id=$1 AND uid=$2 AND company_id=$3 AND supplier_id=$4`,
				string(a.InvoiceID), string(uid), string(companyID), string(in.SupplierID)).Scan(&total, &amount)
			if noRows(err) {
				return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayInvoiceNotFound}, nil
			}
			if err != nil {
				return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("manual invoice lookup: %w", err)
			}
			due := r2(total - amount)
			if applied > due+0.01 {
				return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayOverInvoice, Applied: applied, Due: due}, nil
			}
			allocs = append(allocs, spAllocJSON{PurchaseInvoiceID: string(a.InvoiceID), Amount: applied})
		}
	}

	// Apply only after every allocation validated.
	for _, a := range allocs {
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET amount = COALESCE(amount,0) + $2 WHERE id=$1 AND company_id=$3`,
			a.PurchaseInvoiceID, a.Amount, string(companyID)); err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("apply invoice payment: %w", err)
		}
	}

	allocJSON, _ := json.Marshal(allocsOrEmpty(allocs))
	var bankID any
	if in.BankID != "" {
		bankID = string(in.BankID)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO supplier_payments (uid, company_id, supplier_id, date, amount, note, bank_id, mode, allocations)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		string(uid), string(companyID), string(in.SupplierID), in.Date, in.Amount, in.Note, bankID, in.Mode, allocJSON).Scan(&id); err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("insert supplier payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, err
	}
	out, err := s.getOne(ctx, uid, companyID, store.ID(id))
	return out, store.SupplierPayResult{Status: store.SupplierPayOK}, err
}

func allocsOrEmpty(a []spAllocJSON) []spAllocJSON {
	if a == nil {
		return []spAllocJSON{}
	}
	return a
}

func (s *supplierPayments) Delete(ctx context.Context, uid, companyID, paymentID store.ID) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var allocB []byte
	err = tx.QueryRow(ctx, `SELECT allocations FROM supplier_payments WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(paymentID), string(uid), string(companyID)).Scan(&allocB)
	if noRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up supplier payment: %w", err)
	}
	var allocs []spAllocJSON
	_ = json.Unmarshal(allocB, &allocs)
	for _, a := range allocs {
		if a.PurchaseInvoiceID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE purchase_invoices SET amount = GREATEST(0, COALESCE(amount,0) - $2) WHERE id=$1 AND company_id=$3`,
			a.PurchaseInvoiceID, a.Amount, string(companyID)); err != nil {
			return false, fmt.Errorf("reverse invoice payment: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM supplier_payments WHERE id=$1`, string(paymentID)); err != nil {
		return false, fmt.Errorf("delete supplier payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// getOne resolves a single supplier payment the way List does (supplier/bank populated,
// purchase-invoice destinations).
func (s *supplierPayments) getOne(ctx context.Context, uid, companyID, paymentID store.ID) (store.SupplierPayment, error) {
	var p store.SupplierPayment
	var companyCol, supCol, bankCol *string
	var allocB []byte
	err := s.pool.QueryRow(ctx, `
		SELECT sp.id, sp.uid, sp.company_id, sp.date, sp.amount, sp.note, sp.mode, sp.allocations, sp.created_at, sp.updated_at,
		       p.id, COALESCE(p.name,''), COALESCE(p.firm,''), COALESCE(p.phone,''),
		       bk.id, COALESCE(bk.name,'')
		  FROM supplier_payments sp
		  LEFT JOIN persons p ON p.id = sp.supplier_id
		  LEFT JOIN banks bk ON bk.id = sp.bank_id
		 WHERE sp.id=$1 AND sp.uid=$2 AND sp.company_id=$3`, string(paymentID), string(uid), string(companyID)).
		Scan(&p.ID, &p.UID, &companyCol, &p.Date, &p.Amount, &p.Note, &p.Mode, &allocB, &p.CreatedAt, &p.UpdatedAt,
			&supCol, &p.SupplierName, &p.SupplierFirm, &p.SupplierPhone, &bankCol, &p.BankName)
	if err != nil {
		return store.SupplierPayment{}, fmt.Errorf("reading supplier payment: %w", err)
	}
	if companyCol != nil {
		p.CompanyID = store.ID(*companyCol)
	}
	if supCol != nil {
		p.SupplierID = store.ID(*supCol)
	}
	if bankCol != nil {
		p.BankID = store.ID(*bankCol)
	}
	var allocs []spAllocJSON
	_ = json.Unmarshal(allocB, &allocs)
	purIDs := map[string]bool{}
	for _, a := range allocs {
		if a.PurchaseInvoiceID != "" {
			purIDs[a.PurchaseInvoiceID] = true
		}
	}
	purNums, err := labelMap(ctx, s.pool, "SELECT id, invoice_number FROM purchase_invoices WHERE id = ANY($1)", purIDs)
	if err != nil {
		return store.SupplierPayment{}, err
	}
	var dests []store.ReceiptDestination
	for _, a := range allocs {
		if a.PurchaseInvoiceID != "" {
			dests = append(dests, store.ReceiptDestination{Kind: "purchase-invoice", ID: a.PurchaseInvoiceID, Label: purNums[a.PurchaseInvoiceID], Amount: a.Amount})
		}
	}
	p.Destinations = dests
	return p, nil
}
