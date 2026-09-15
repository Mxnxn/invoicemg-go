package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type batchReceives struct{ pool *pgxpool.Pool }

func (s *Store) BatchReceives() store.BatchReceives { return &batchReceives{pool: s.pool} }

type brAllocJSON struct {
	JobID  string  `json:"job_id"`
	Amount float64 `json:"amount"`
}
type brEntryAllocJSON struct {
	InvoiceID string  `json:"invoice_id"`
	Amount    float64 `json:"amount"`
}

func (b *batchReceives) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.BatchReceive, error) {
	rows, err := b.pool.Query(ctx, `
		SELECT br.id, br.uid, br.company_id, br.invoice_id, br.date, br.amount, br.note, br.mode,
		       br.allocations, br.entry_allocations, br.created_at, br.updated_at,
		       c.id, COALESCE(c.client_name,''), COALESCE(c.client_firm,''), COALESCE(c.client_phone,''),
		       bk.id, COALESCE(bk.name,'')
		  FROM batch_receives br
		  LEFT JOIN clients c  ON c.id = br.client_id
		  LEFT JOIN banks   bk ON bk.id = br.bank_id
		 WHERE br.uid = $1 AND br.company_id = $2 AND ($3 = '' OR br.client_id = $3)
		 ORDER BY br.created_at DESC, br.id DESC`, string(uid), string(companyID), string(clientID))
	if err != nil {
		return nil, fmt.Errorf("listing batch receives: %w", err)
	}
	defer rows.Close()

	type raw struct {
		br      store.BatchReceive
		invoice *string
		allocs  []brAllocJSON
		eallocs []brEntryAllocJSON
	}
	var list []raw
	jobIDs, invIDs := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var r raw
		var companyIDCol, invoiceID, clientIDCol, bankIDCol *string
		var allocB, eallocB []byte
		if err := rows.Scan(&r.br.ID, &r.br.UID, &companyIDCol, &invoiceID, &r.br.Date, &r.br.Amount, &r.br.Note, &r.br.Mode,
			&allocB, &eallocB, &r.br.CreatedAt, &r.br.UpdatedAt,
			&clientIDCol, &r.br.ClientName, &r.br.ClientFirm, &r.br.ClientPhone,
			&bankIDCol, &r.br.BankName); err != nil {
			return nil, fmt.Errorf("reading batch receives: %w", err)
		}
		if companyIDCol != nil {
			r.br.CompanyID = store.ID(*companyIDCol)
		}
		if clientIDCol != nil {
			r.br.ClientID = store.ID(*clientIDCol)
		}
		if bankIDCol != nil {
			r.br.BankID = store.ID(*bankIDCol)
		}
		r.invoice = invoiceID
		_ = json.Unmarshal(allocB, &r.allocs)
		_ = json.Unmarshal(eallocB, &r.eallocs)
		if invoiceID != nil && *invoiceID != "" {
			invIDs[*invoiceID] = true
		}
		for _, a := range r.allocs {
			if a.JobID != "" {
				jobIDs[a.JobID] = true
			}
		}
		for _, a := range r.eallocs {
			if a.InvoiceID != "" {
				invIDs[a.InvoiceID] = true
			}
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	jobNums, err := b.labels(ctx, "SELECT id, challan_number FROM jobs WHERE id = ANY($1)", jobIDs)
	if err != nil {
		return nil, err
	}
	invNums, err := b.labels(ctx, "SELECT id, invoice_id FROM invoices WHERE id = ANY($1)", invIDs)
	if err != nil {
		return nil, err
	}

	out := make([]store.BatchReceive, 0, len(list))
	for _, r := range list {
		var dests []store.ReceiptDestination
		if r.invoice != nil && *r.invoice != "" {
			dests = append(dests, store.ReceiptDestination{Kind: "invoice", ID: *r.invoice, Label: invNums[*r.invoice], Amount: r.br.Amount})
		}
		for _, a := range r.allocs {
			if a.JobID != "" {
				dests = append(dests, store.ReceiptDestination{Kind: "job", ID: a.JobID, Label: jobNums[a.JobID], Amount: a.Amount})
			}
		}
		for _, a := range r.eallocs {
			if a.InvoiceID == "" {
				continue
			}
			merged := false
			for i := range dests {
				if dests[i].Kind == "invoice" && dests[i].ID == a.InvoiceID {
					dests[i].Amount = float64(int64((dests[i].Amount+a.Amount)*100+0.5)) / 100
					merged = true
					break
				}
			}
			if !merged {
				dests = append(dests, store.ReceiptDestination{Kind: "invoice", ID: a.InvoiceID, Label: invNums[a.InvoiceID], Amount: a.Amount})
			}
		}
		r.br.Destinations = dests
		out = append(out, r.br)
	}
	return out, nil
}

func (b *batchReceives) labels(ctx context.Context, sql string, idset map[string]bool) (map[string]string, error) {
	out := map[string]string{}
	if len(idset) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	rows, err := b.pool.Query(ctx, sql, ids)
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
