package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type purchaseReport struct{ pool *pgxpool.Pool }

func (s *Store) PurchaseReport() store.PurchaseReport { return &purchaseReport{pool: s.pool} }

func (p *purchaseReport) SupplierDues(ctx context.Context, uid, companyID store.ID) (store.SupplierDuesData, error) {
	out := store.SupplierDuesData{Suppliers: map[string]store.SupplierInfo{}}

	rows, err := p.pool.Query(ctx,
		`SELECT supplier_id, total, amount FROM purchase_invoices WHERE company_id = $1`, string(companyID))
	if err != nil {
		return store.SupplierDuesData{}, fmt.Errorf("purchase invoices: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sid *string
		var total, amount float64
		if err := rows.Scan(&sid, &total, &amount); err != nil {
			return store.SupplierDuesData{}, err
		}
		if sid == nil {
			continue
		}
		out.Invoices = append(out.Invoices, store.SupplierDueInvoice{SupplierID: store.ID(*sid), Total: total, Amount: amount})
	}
	if err := rows.Err(); err != nil {
		return store.SupplierDuesData{}, err
	}

	// Suppliers are scoped by uid, not company - the roster is shared across company profiles.
	sRows, err := p.pool.Query(ctx,
		`SELECT id, name, firm, phone, gst, address FROM persons WHERE uid = $1 AND type = 'Supplier'`, string(uid))
	if err != nil {
		return store.SupplierDuesData{}, fmt.Errorf("suppliers: %w", err)
	}
	defer sRows.Close()
	for sRows.Next() {
		var si store.SupplierInfo
		if err := sRows.Scan(&si.ID, &si.Name, &si.Firm, &si.Phone, &si.GST, &si.Address); err != nil {
			return store.SupplierDuesData{}, err
		}
		out.Suppliers[string(si.ID)] = si
	}
	return out, sRows.Err()
}

func (p *purchaseReport) SupplierStatement(ctx context.Context, uid, companyID, supplierID store.ID) (store.SupplierStatementData, error) {
	var out store.SupplierStatementData
	err := p.pool.QueryRow(ctx,
		`SELECT id, name, firm, phone, gst, address FROM persons WHERE id = $1 AND uid = $2 AND type = 'Supplier'`,
		string(supplierID), string(uid)).
		Scan(&out.Supplier.ID, &out.Supplier.Name, &out.Supplier.Firm, &out.Supplier.Phone, &out.Supplier.GST, &out.Supplier.Address)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SupplierStatementData{}, nil
	}
	if err != nil {
		return store.SupplierStatementData{}, err
	}
	out.Found = true

	iRows, err := p.pool.Query(ctx,
		`SELECT date, invoice_number, total FROM purchase_invoices WHERE supplier_id = $1 AND company_id = $2`,
		string(supplierID), string(companyID))
	if err != nil {
		return store.SupplierStatementData{}, fmt.Errorf("supplier invoices: %w", err)
	}
	defer iRows.Close()
	for iRows.Next() {
		var inv store.SupplierLedgerInvoice
		if err := iRows.Scan(&inv.Date, &inv.InvoiceNumber, &inv.Total); err != nil {
			return store.SupplierStatementData{}, err
		}
		out.Invoices = append(out.Invoices, inv)
	}
	if err := iRows.Err(); err != nil {
		return store.SupplierStatementData{}, err
	}

	pRows, err := p.pool.Query(ctx,
		`SELECT date, amount, note FROM supplier_payments WHERE supplier_id = $1 AND company_id = $2`,
		string(supplierID), string(companyID))
	if err != nil {
		return store.SupplierStatementData{}, fmt.Errorf("supplier payments: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var pay store.SupplierLedgerPayment
		if err := pRows.Scan(&pay.Date, &pay.Amount, &pay.Note); err != nil {
			return store.SupplierStatementData{}, err
		}
		out.Payments = append(out.Payments, pay)
	}
	return out, pRows.Err()
}
