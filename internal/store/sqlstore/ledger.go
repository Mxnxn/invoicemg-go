package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type ledger struct{ pool *pgxpool.Pool }

func (s *Store) Ledger() store.Ledger { return &ledger{pool: s.pool} }

func (l *ledger) ClientStatement(ctx context.Context, companyID, clientID store.ID) (store.LedgerClientData, error) {
	var out store.LedgerClientData
	co, c := string(companyID), string(clientID)

	err := l.pool.QueryRow(ctx, `
		SELECT client_name, client_firm, client_gst, client_address
		  FROM clients WHERE id = $1 AND company_id = $2`, c, co).
		Scan(&out.ClientName, &out.ClientFirm, &out.ClientGST, &out.ClientAddress)
	if noRows(err) {
		return out, nil // Found stays false -> handler answers 404
	}
	if err != nil {
		return out, fmt.Errorf("ledger client: %w", err)
	}
	out.Found = true

	// bills
	invRows, err := l.pool.Query(ctx, `SELECT date, invoice_id, total_amount FROM invoices WHERE client_id = $1 AND company_id = $2`, c, co)
	if err != nil {
		return out, fmt.Errorf("ledger invoices: %w", err)
	}
	for invRows.Next() {
		var date, no string
		var total float64
		if err := invRows.Scan(&date, &no, &total); err != nil {
			invRows.Close()
			return out, err
		}
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(date), Type: "Sales Invoice", InvoiceNo: no, Bill: total, Seq: 0})
	}
	invRows.Close()
	if err := invRows.Err(); err != nil {
		return out, err
	}

	// receipts (join the invoice's printed number, as Node's .populate("invoice_id","invoiceId"))
	recRows, err := l.pool.Query(ctx, `
		SELECT rc.date, rc.amount, COALESCE(iv.invoice_id,'')
		  FROM invoice_received rc LEFT JOIN invoices iv ON iv.id = rc.invoice_id
		 WHERE rc.client_id = $1 AND rc.company_id = $2`, c, co)
	if err != nil {
		return out, fmt.Errorf("ledger receipts: %w", err)
	}
	for recRows.Next() {
		var date, no string
		var amount float64
		if err := recRows.Scan(&date, &amount, &no); err != nil {
			recRows.Close()
			return out, err
		}
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(date), Type: "Receipts", InvoiceNo: no, Receipt: amount, Seq: 1})
	}
	recRows.Close()
	if err := recRows.Err(); err != nil {
		return out, err
	}

	// batch receives (no invoice number)
	brRows, err := l.pool.Query(ctx, `SELECT date, amount FROM batch_receives WHERE client_id = $1 AND company_id = $2`, c, co)
	if err != nil {
		return out, fmt.Errorf("ledger batch: %w", err)
	}
	for brRows.Next() {
		var date string
		var amount float64
		if err := brRows.Scan(&date, &amount); err != nil {
			brRows.Close()
			return out, err
		}
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(date), Type: "Receipts", Receipt: amount, Seq: 1})
	}
	brRows.Close()
	return out, brRows.Err()
}
