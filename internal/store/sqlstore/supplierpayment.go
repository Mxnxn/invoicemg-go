package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type supplierPayments struct{ pool *pgxpool.Pool }

func (s *Store) SupplierPayments() store.SupplierPayments { return &supplierPayments{pool: s.pool} }

type spAllocJSON struct {
	PurchaseInvoiceID string  `json:"purchase_invoice_id"`
	Amount            float64 `json:"amount"`
}

func (s *supplierPayments) List(ctx context.Context, uid, companyID store.ID) ([]store.SupplierPayment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sp.id, sp.uid, sp.company_id, sp.date, sp.amount, sp.note, sp.mode, sp.allocations,
		       sp.created_at, sp.updated_at,
		       p.id, COALESCE(p.name,''), COALESCE(p.firm,''), COALESCE(p.phone,''),
		       bk.id, COALESCE(bk.name,'')
		  FROM supplier_payments sp
		  LEFT JOIN persons p  ON p.id = sp.supplier_id
		  LEFT JOIN banks   bk ON bk.id = sp.bank_id
		 WHERE sp.uid = $1 AND sp.company_id = $2
		 ORDER BY sp.created_at DESC, sp.id DESC`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing supplier payments: %w", err)
	}
	defer rows.Close()

	type raw struct {
		p      store.SupplierPayment
		allocs []spAllocJSON
	}
	var list []raw
	purIDs := map[string]bool{}
	for rows.Next() {
		var r raw
		var companyIDCol, supIDCol, bankIDCol *string
		var allocB []byte
		if err := rows.Scan(&r.p.ID, &r.p.UID, &companyIDCol, &r.p.Date, &r.p.Amount, &r.p.Note, &r.p.Mode, &allocB,
			&r.p.CreatedAt, &r.p.UpdatedAt,
			&supIDCol, &r.p.SupplierName, &r.p.SupplierFirm, &r.p.SupplierPhone,
			&bankIDCol, &r.p.BankName); err != nil {
			return nil, fmt.Errorf("reading supplier payments: %w", err)
		}
		if companyIDCol != nil {
			r.p.CompanyID = store.ID(*companyIDCol)
		}
		if supIDCol != nil {
			r.p.SupplierID = store.ID(*supIDCol)
		}
		if bankIDCol != nil {
			r.p.BankID = store.ID(*bankIDCol)
		}
		_ = json.Unmarshal(allocB, &r.allocs)
		for _, a := range r.allocs {
			if a.PurchaseInvoiceID != "" {
				purIDs[a.PurchaseInvoiceID] = true
			}
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	purNums, err := labelMap(ctx, s.pool, "SELECT id, invoice_number FROM purchase_invoices WHERE id = ANY($1)", purIDs)
	if err != nil {
		return nil, err
	}

	out := make([]store.SupplierPayment, 0, len(list))
	for _, r := range list {
		var dests []store.ReceiptDestination
		for _, a := range r.allocs {
			if a.PurchaseInvoiceID != "" {
				dests = append(dests, store.ReceiptDestination{Kind: "purchase-invoice", ID: a.PurchaseInvoiceID, Label: purNums[a.PurchaseInvoiceID], Amount: a.Amount})
			}
		}
		r.p.Destinations = dests
		out = append(out, r.p)
	}
	return out, nil
}

// labelMap resolves an id set to one string column, one query. Shared by the receipt lists.
func labelMap(ctx context.Context, pool *pgxpool.Pool, sql string, idset map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	if len(idset) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	rows, err := pool.Query(ctx, sql, ids)
	if err != nil {
		return nil, fmt.Errorf("resolving labels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, label string
		if err := rows.Scan(&id, &label); err != nil {
			return nil, err
		}
		out[id] = label
	}
	return out, rows.Err()
}
