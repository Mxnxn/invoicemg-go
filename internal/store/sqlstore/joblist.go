package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type jobs struct{ pool *pgxpool.Pool }

func (s *Store) Jobs() store.Jobs { return &jobs{pool: s.pool} }

// alertJSON is the shape of the created_alert / done_alert JSONB columns.
type alertJSON struct {
	Status    string     `json:"status"`
	StatusAt  *time.Time `json:"statusAt"`
	Error     string     `json:"error"`
	SentAt    *time.Time `json:"sentAt"`
	Count     int        `json:"count"`
	RowIDs    []string   `json:"rowIds"`
	Signature string     `json:"signature"`
}

func (a alertJSON) toStore() store.JobAlertChannel {
	return store.JobAlertChannel{
		Status: a.Status, StatusAt: a.StatusAt, Error: a.Error, SentAt: a.SentAt,
		Count: a.Count, RowIDs: a.RowIDs, Signature: a.Signature,
	}
}

func (j *jobs) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.Job, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT jb.id, jb.challan_number, COALESCE(to_char(jb.received_date,'YYYY-MM-DD'),''), jb.total,
		       jb.advance, jb.queue, jb.progress, jb.queue_order, jb.unlocked,
		       jb.created_alert, jb.done_alert, jb.created_at, jb.updated_at,
		       c.id, c.client_name, c.client_firm, c.client_phone, c.client_address,
		       e.id, e.name, v.id, v.name
		  FROM jobs jb
		  LEFT JOIN clients c ON c.id = jb.client_id
		  LEFT JOIN persons e ON e.id = jb.employee_id
		  LEFT JOIN persons v ON v.id = jb.vendor_id
		 WHERE jb.uid = $1 AND jb.company_id = $2 AND ($3 = '' OR jb.client_id = $3)
		 ORDER BY jb.created_at DESC, jb.id DESC`, string(uid), string(companyID), string(clientID))
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	defer rows.Close()

	out := make([]store.Job, 0)
	ids := make([]string, 0)
	byID := map[string]int{}
	for rows.Next() {
		var job store.Job
		var createdAlert, doneAlert []byte
		var cID, cName, cFirm, cPhone, cAddr *string
		var eID, eName, vID, vName *string
		if err := rows.Scan(&job.ID, &job.ChallanNumber, &job.ReceivedDate, &job.Total, &job.Advance,
			&job.Queue, &job.Progress, &job.QueueOrder, &job.Unlocked, &createdAlert, &doneAlert,
			&job.CreatedAt, &job.UpdatedAt,
			&cID, &cName, &cFirm, &cPhone, &cAddr, &eID, &eName, &vID, &vName); err != nil {
			return nil, fmt.Errorf("reading jobs: %w", err)
		}
		job.CreatedAlert = decodeAlert(createdAlert)
		job.DoneAlert = decodeAlert(doneAlert)
		if cID != nil {
			job.Client = &store.JobClient{
				ID: store.ID(*cID), ClientName: deref(cName), ClientFirm: deref(cFirm),
				ClientPhone: deref(cPhone), ClientAddress: deref(cAddr),
			}
		}
		if eID != nil {
			job.Employee = &store.JobPerson{ID: store.ID(*eID), Name: deref(eName)}
		}
		if vID != nil {
			job.Vendor = &store.JobPerson{ID: store.ID(*vID), Name: deref(vName)}
		}
		job.Rows = make([]store.JobRow, 0)
		byID[string(job.ID)] = len(out)
		ids = append(ids, string(job.ID))
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	// One batched query for every job's rows, with the row's populates folded in as joins (#6):
	// quotation number, entry state (has_issued/total/advance), the covering invoice number, and
	// the row employee's name.
	rRows, err := j.pool.Query(ctx, `
		SELECT r.job_id, r.id, r.row_id, r.material, r.description, r.qty, r.has_dimensions,
		       r.length, r.width, r.rate, r.cgst, r.sgst, r.igst, r.discount, r.charges,
		       r.queue, r.progress, r.queue_order, r.created_at, r.updated_at,
		       r.entry_id, en.has_issued, en.total, en.advance, iv.invoice_id,
		       r.quotation_id, qt.quotation_number, r.employee_id, p.name
		  FROM job_rows r
		  LEFT JOIN entries en    ON en.id = r.entry_id
		  LEFT JOIN invoices iv   ON iv.id = en.invoice_id
		  LEFT JOIN quotations qt ON qt.id = r.quotation_id
		  LEFT JOIN persons p     ON p.id = r.employee_id
		 WHERE r.job_id = ANY ($1)
		 ORDER BY r.position ASC, r.id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("listing job rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var jid string
		var r store.JobRow
		var hasDim bool
		var entryID, invNum, quotID, quotNum, empID, empName *string
		var enIssued *bool
		var enTotal, enAdvance *float64
		if err := rRows.Scan(&jid, &r.ID, &r.RowID, &r.Material, &r.Description, &r.Qty, &hasDim,
			&r.Length, &r.Width, &r.Rate, &r.Cgst, &r.Sgst, &r.Igst, &r.Discount, &r.Charges,
			&r.Queue, &r.Progress, &r.QueueOrder, &r.CreatedAt, &r.UpdatedAt,
			&entryID, &enIssued, &enTotal, &enAdvance, &invNum,
			&quotID, &quotNum, &empID, &empName); err != nil {
			return nil, fmt.Errorf("reading job rows: %w", err)
		}
		hd := hasDim
		r.HasDimensions = &hd
		if entryID != nil {
			r.Entry = &store.JobRowEntry{
				ID: store.ID(*entryID), HasIssued: derefBool(enIssued),
				Total: derefFloat(enTotal), Advance: derefFloat(enAdvance),
			}
			r.EntryIssued = derefBool(enIssued)
		}
		if invNum != nil {
			r.InvoiceNumber = *invNum
		}
		if quotID != nil {
			r.Quotation = &store.JobRowQuotation{ID: store.ID(*quotID), QuotationNumber: deref(quotNum)}
		}
		if empID != nil {
			r.Employee = &store.JobPerson{ID: store.ID(*empID), Name: deref(empName)}
		}
		if idx, ok := byID[jid]; ok {
			out[idx].Rows = append(out[idx].Rows, r)
		}
	}
	return out, rRows.Err()
}

func decodeAlert(b []byte) store.JobAlertChannel {
	if len(b) == 0 {
		return store.JobAlertChannel{}
	}
	var a alertJSON
	if err := json.Unmarshal(b, &a); err != nil {
		return store.JobAlertChannel{}
	}
	return a.toStore()
}

func derefBool(b *bool) bool {
	return b != nil && *b
}
func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
