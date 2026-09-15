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

func (l *ledger) ClientDues(ctx context.Context, companyID store.ID) (store.ClientDuesData, error) {
	var out store.ClientDuesData
	co := string(companyID)

	invoices, err := clientAmounts(ctx, l.pool, `SELECT client_id, total_amount FROM invoices WHERE company_id = $1`, co)
	if err != nil {
		return out, fmt.Errorf("dues invoices: %w", err)
	}
	received, err := clientAmounts(ctx, l.pool, `SELECT client_id, amount FROM invoice_received WHERE company_id = $1`, co)
	if err != nil {
		return out, fmt.Errorf("dues received: %w", err)
	}
	batch, err := clientAmounts(ctx, l.pool, `SELECT client_id, amount FROM batch_receives WHERE company_id = $1`, co)
	if err != nil {
		return out, fmt.Errorf("dues batch: %w", err)
	}
	out.Dues = store.ComputeClientDues(invoices, received, batch)

	out.Clients = map[string]store.ClientBrief{}
	rows, err := l.pool.Query(ctx, `SELECT id, client_name, client_firm, client_phone FROM clients WHERE company_id = $1`, co)
	if err != nil {
		return out, fmt.Errorf("dues clients: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, firm, phone string
		if err := rows.Scan(&id, &name, &firm, &phone); err != nil {
			return out, err
		}
		out.Clients[id] = store.ClientBrief{Name: name, Firm: firm, Phone: phone}
	}
	return out, rows.Err()
}

// clientAmounts runs a (client_id, amount) projection, dropping rows whose client is NULL
// (a client detached on delete) - a "" key aggregates nothing, matching Node's clientKey.
func clientAmounts(ctx context.Context, pool *pgxpool.Pool, sql, companyID string) ([]store.ClientAmount, error) {
	rows, err := pool.Query(ctx, sql, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.ClientAmount
	for rows.Next() {
		var id *string
		var amount float64
		if err := rows.Scan(&id, &amount); err != nil {
			return nil, err
		}
		key := ""
		if id != nil {
			key = *id
		}
		out = append(out, store.ClientAmount{ClientID: key, Amount: amount})
	}
	return out, rows.Err()
}
