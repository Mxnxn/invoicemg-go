package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type analytics struct{ pool *pgxpool.Pool }

func (s *Store) Analytics() store.Analytics { return &analytics{pool: s.pool} }

// datedSeries reads createdAt/date + one amount column, resolving the effective date
// (createdAt || date) the same way getEffectiveDate does.
func (a *analytics) datedSeries(ctx context.Context, sql string, companyID store.ID) ([]store.DatedAmount, error) {
	rows, err := a.pool.Query(ctx, sql, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("reading revenue series: %w", err)
	}
	defer rows.Close()
	out := make([]store.DatedAmount, 0)
	for rows.Next() {
		var createdAt *time.Time
		var date string
		var amount float64
		if err := rows.Scan(&createdAt, &date, &amount); err != nil {
			return nil, fmt.Errorf("reading revenue row: %w", err)
		}
		if createdAt != nil && !createdAt.IsZero() {
			out = append(out, store.DatedAmount{Date: createdAt.UTC(), Amount: amount})
			continue
		}
		if date != "" {
			if t, err := time.Parse("2006-01-02", date); err == nil {
				out = append(out, store.DatedAmount{Date: t.UTC(), Amount: amount})
			} else if t, err := time.Parse(time.RFC3339, date); err == nil {
				out = append(out, store.DatedAmount{Date: t.UTC(), Amount: amount})
			}
		}
	}
	return out, rows.Err()
}

func (a *analytics) RevenueSeries(ctx context.Context, companyID store.ID, source string) ([]store.DatedAmount, []store.DatedAmount, error) {
	if source == "all" {
		billed, err := a.datedSeries(ctx, `SELECT created_at, date, total  FROM entries WHERE company_id = $1`, companyID)
		if err != nil {
			return nil, nil, err
		}
		collected, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM entries WHERE company_id = $1`, companyID)
		if err != nil {
			return nil, nil, err
		}
		return billed, collected, nil
	}
	// invoices carry no `date`? they do (date text). invoice_received has date too.
	billed, err := a.datedSeries(ctx, `SELECT created_at, date, total_amount FROM invoices WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, nil, err
	}
	collected, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM invoice_received WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, nil, err
	}
	return billed, collected, nil
}

// rank groups invoices by client and joins the client name. valueExpr is the SUM expression;
// having is an optional "HAVING <cond>" (empty for none). Ordered by the value desc, id for
// determinism.
func (a *analytics) rank(ctx context.Context, companyID store.ID, valueExpr, having string) ([]store.ClientRank, error) {
	sql := `
		SELECT c.id, c.client_name, c.client_firm, round(` + valueExpr + `, 2) AS val
		  FROM invoices iv
		  JOIN clients c ON c.id = iv.client_id
		 WHERE iv.company_id = $1
		 GROUP BY c.id, c.client_name, c.client_firm ` + having + `
		 ORDER BY val DESC, c.id ASC`
	rows, err := a.pool.Query(ctx, sql, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("ranking clients: %w", err)
	}
	defer rows.Close()
	out := make([]store.ClientRank, 0)
	for rows.Next() {
		var r store.ClientRank
		if err := rows.Scan(&r.ClientID, &r.ClientName, &r.ClientFirm, &r.Value); err != nil {
			return nil, fmt.Errorf("reading rank: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *analytics) TopSales(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.total_amount)", "")
}
func (a *analytics) TopCredits(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.total_amount - iv.amount)", "HAVING round(SUM(iv.total_amount - iv.amount), 2) > 0")
}
func (a *analytics) TopPaid(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, "SUM(iv.amount)", "HAVING round(SUM(iv.amount), 2) > 0")
}

func (a *analytics) PaymentGaps(ctx context.Context, companyID store.ID) ([]float64, error) {
	// days from the invoice's billed date to its last receipt, for fully-paid invoices.
	rows, err := a.pool.Query(ctx, `
		SELECT EXTRACT(EPOCH FROM (
		         GREATEST(MAX(COALESCE(rc.created_at, to_timestamp(NULLIF(rc.date,''),'YYYY-MM-DD'))),
		                  MIN(COALESCE(iv.created_at, to_timestamp(NULLIF(iv.date,''),'YYYY-MM-DD'))))
		       - MIN(COALESCE(iv.created_at, to_timestamp(NULLIF(iv.date,''),'YYYY-MM-DD')))
		     )) / 86400.0 AS gap_days
		  FROM invoices iv
		  JOIN invoice_received rc ON rc.invoice_id = iv.id
		 WHERE iv.company_id = $1 AND iv.total_amount > 0 AND iv.amount >= iv.total_amount
		 GROUP BY iv.id`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("payment gaps: %w", err)
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var g float64
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		if g >= 0 {
			out = append(out, g)
		}
	}
	return out, rows.Err()
}

func (a *analytics) PendingSince(ctx context.Context, companyID store.ID) ([]time.Time, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT COALESCE(created_at, to_timestamp(NULLIF(date,''),'YYYY-MM-DD'))
		  FROM invoices
		 WHERE company_id = $1 AND total_amount > 0 AND amount < total_amount`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("pending since: %w", err)
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var t *time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, t.UTC())
		}
	}
	return out, rows.Err()
}

func (a *analytics) Payables(ctx context.Context, companyID store.ID) (float64, []store.SupplierDue, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT pi.total, pi.amount, COALESCE(s.name, '')
		  FROM purchase_invoices pi
		  LEFT JOIN persons s ON s.id = pi.supplier_id
		 WHERE pi.company_id = $1`, string(companyID))
	if err != nil {
		return 0, nil, fmt.Errorf("payables: %w", err)
	}
	defer rows.Close()
	var prs []store.PayableRow
	for rows.Next() {
		var r store.PayableRow
		if err := rows.Scan(&r.Total, &r.Amount, &r.Supplier); err != nil {
			return 0, nil, err
		}
		prs = append(prs, r)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	total, bySup := store.SumPayables(prs)
	return total, bySup, nil
}

func (a *analytics) OutstandingInvoices(ctx context.Context, companyID store.ID) ([]store.DatedAmount, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT COALESCE(created_at, to_timestamp(NULLIF(date,''),'YYYY-MM-DD')), (total_amount - amount)
		  FROM invoices WHERE company_id = $1 AND (total_amount - amount) > 0`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("outstanding invoices: %w", err)
	}
	defer rows.Close()
	out := []store.DatedAmount{}
	for rows.Next() {
		var t *time.Time
		var due float64
		if err := rows.Scan(&t, &due); err != nil {
			return nil, err
		}
		if t != nil {
			out = append(out, store.DatedAmount{Date: t.UTC(), Amount: due})
		}
	}
	return out, rows.Err()
}

func (a *analytics) UnbilledEntries(ctx context.Context, companyID store.ID) ([]store.UnbilledEntry, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT e.total, COALESCE(e.created_at, to_timestamp(NULLIF(e.date,''),'YYYY-MM-DD')),
		       c.id, COALESCE(c.client_name,''), COALESCE(c.client_firm,'')
		  FROM entries e LEFT JOIN clients c ON c.id = e.client_id
		 WHERE e.company_id = $1 AND e.has_issued = false`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("unbilled entries: %w", err)
	}
	defer rows.Close()
	out := make([]store.UnbilledEntry, 0)
	for rows.Next() {
		var e store.UnbilledEntry
		var t *time.Time
		var cid *string
		if err := rows.Scan(&e.Value, &t, &cid, &e.ClientName, &e.ClientFirm); err != nil {
			return nil, err
		}
		if t != nil {
			e.Date, e.HasDate = t.UTC(), true
		}
		if cid != nil {
			e.ClientID = store.ID(*cid)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (a *analytics) Reviews(ctx context.Context, companyID store.ID, from, to string) ([]store.ReviewRow, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT id, challan_number, job_id, client_id, client_name, comment, created_at,
		       quality, speed, communication, satisfaction, overall
		  FROM job_reviews
		 WHERE company_id = $1
		   AND ($2 = '' OR created_at >= to_timestamp($2,'YYYY-MM-DD'))
		   AND ($3 = '' OR created_at < to_timestamp($3,'YYYY-MM-DD') + interval '1 day')
		 ORDER BY created_at DESC, id DESC`, string(companyID), from, to)
	if err != nil {
		return nil, fmt.Errorf("reviews: %w", err)
	}
	defer rows.Close()
	out := make([]store.ReviewRow, 0)
	for rows.Next() {
		var r store.ReviewRow
		var jobID, clientID *string
		var s store.ReviewScores
		if err := rows.Scan(&r.ID, &r.ChallanNumber, &jobID, &clientID, &r.ClientName, &r.Comment, &r.CreatedAt,
			&s.Quality, &s.Speed, &s.Communication, &s.Satisfaction, &s.Overall); err != nil {
			return nil, err
		}
		if jobID != nil {
			r.JobID = store.ID(*jobID)
		}
		if clientID != nil {
			r.ClientID = store.ID(*clientID)
		}
		r.Scores = s
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *analytics) Receipts(ctx context.Context, companyID store.ID) ([]store.DatedAmount, error) {
	rec, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM invoice_received WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, err
	}
	batch, err := a.datedSeries(ctx, `SELECT created_at, date, amount FROM batch_receives WHERE company_id = $1`, companyID)
	if err != nil {
		return nil, err
	}
	return append(rec, batch...), nil
}

func (a *analytics) GstSales(ctx context.Context, companyID store.ID) ([]store.GstDoc, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT iv.id, iv.invoice_id, iv.date, COALESCE(NULLIF(c.client_firm,''), c.client_name, ''), COALESCE(c.client_gst,'')
		  FROM invoices iv LEFT JOIN clients c ON c.id = iv.client_id
		 WHERE iv.company_id = $1 ORDER BY iv.created_at ASC, iv.id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("gst sales: %w", err)
	}
	defer rows.Close()
	docs := []store.GstDoc{}
	ids := []string{}
	byID := map[string]int{}
	for rows.Next() {
		var id string
		var doc store.GstDoc
		if err := rows.Scan(&id, &doc.InvoiceNo, &doc.Date, &doc.PartyName, &doc.GstNo); err != nil {
			return nil, err
		}
		doc.Date = store.NormalizeDate(doc.Date)
		byID[id] = len(docs)
		ids = append(ids, id)
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return docs, nil
	}
	er, err := a.pool.Query(ctx, `SELECT invoice_id, amount, cgst, sgst, igst FROM entries WHERE invoice_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer er.Close()
	for er.Next() {
		var iid string
		var l store.GstLine
		if err := er.Scan(&iid, &l.Amount, &l.Cgst, &l.Sgst, &l.Igst); err != nil {
			return nil, err
		}
		if i, ok := byID[iid]; ok {
			docs[i].Lines = append(docs[i].Lines, l)
		}
	}
	return docs, er.Err()
}

func (a *analytics) GstPurchases(ctx context.Context, companyID store.ID) ([]store.GstDoc, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT pi.id, pi.invoice_number, pi.date, COALESCE(s.name,''), COALESCE(s.gst,'')
		  FROM purchase_invoices pi LEFT JOIN persons s ON s.id = pi.supplier_id
		 WHERE pi.company_id = $1 ORDER BY pi.created_at ASC, pi.id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("gst purchases: %w", err)
	}
	defer rows.Close()
	docs := []store.GstDoc{}
	ids := []string{}
	byID := map[string]int{}
	for rows.Next() {
		var id string
		var doc store.GstDoc
		if err := rows.Scan(&id, &doc.InvoiceNo, &doc.Date, &doc.PartyName, &doc.GstNo); err != nil {
			return nil, err
		}
		doc.Date = store.NormalizeDate(doc.Date)
		byID[id] = len(docs)
		ids = append(ids, id)
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return docs, nil
	}
	pr, err := a.pool.Query(ctx, `SELECT invoice_id, rate, qty, discount, charges, gst FROM purchase_invoice_rows WHERE invoice_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer pr.Close()
	for pr.Next() {
		var iid string
		var rate, qty, disc, charges, gst float64
		if err := pr.Scan(&iid, &rate, &qty, &disc, &charges, &gst); err != nil {
			return nil, err
		}
		if i, ok := byID[iid]; ok {
			docs[i].Lines = append(docs[i].Lines, store.GstLine{Amount: rate*qty - disc + charges, Cgst: gst / 2, Sgst: gst / 2})
		}
	}
	return docs, pr.Err()
}

func (a *analytics) Cashflow(ctx context.Context, companyID store.ID) (store.CashflowData, error) {
	var out store.CashflowData
	co := string(companyID)

	// invoices: billed + collected
	if rows, err := a.pool.Query(ctx, `SELECT date, total_amount, amount FROM invoices WHERE company_id = $1`, co); err != nil {
		return out, fmt.Errorf("cashflow invoices: %w", err)
	} else {
		for rows.Next() {
			var date string
			var total, amount float64
			if err := rows.Scan(&date, &total, &amount); err != nil {
				rows.Close()
				return out, err
			}
			out.Invoices = append(out.Invoices, store.CashflowInvoice{Date: store.NormalizeDate(date), TotalAmount: total, Amount: amount})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}

	// (date, amount) collections
	for _, q := range []struct {
		sql  string
		dest *[]store.CashflowRow
	}{
		{`SELECT date, amount FROM invoice_received WHERE company_id = $1`, &out.Received},
		{`SELECT date, amount FROM batch_receives WHERE company_id = $1`, &out.BatchReceives},
		{`SELECT date, amount FROM supplier_payments WHERE company_id = $1`, &out.SupplierPayments},
		{`SELECT date, total FROM purchase_invoices WHERE company_id = $1`, &out.PurchaseInvoices},
	} {
		rows, err := a.pool.Query(ctx, q.sql, co)
		if err != nil {
			return out, fmt.Errorf("cashflow rows: %w", err)
		}
		for rows.Next() {
			var date string
			var amount float64
			if err := rows.Scan(&date, &amount); err != nil {
				rows.Close()
				return out, err
			}
			*q.dest = append(*q.dest, store.CashflowRow{Date: store.NormalizeDate(date), Amount: amount})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}

	// sold entries (invoiced only)
	if rows, err := a.pool.Query(ctx, `SELECT date, material, qty FROM entries WHERE company_id = $1 AND has_issued = true`, co); err != nil {
		return out, fmt.Errorf("cashflow entries: %w", err)
	} else {
		for rows.Next() {
			var date, material string
			var qty float64
			if err := rows.Scan(&date, &material, &qty); err != nil {
				rows.Close()
				return out, err
			}
			out.SoldEntries = append(out.SoldEntries, store.CashflowSoldEntry{Date: store.NormalizeDate(date), Material: material, Qty: qty})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}

	// wastages
	if rows, err := a.pool.Query(ctx, `SELECT date, total, cost_total FROM wastages WHERE company_id = $1`, co); err != nil {
		return out, fmt.Errorf("cashflow wastages: %w", err)
	} else {
		for rows.Next() {
			var date string
			var total, cost float64
			if err := rows.Scan(&date, &total, &cost); err != nil {
				rows.Close()
				return out, err
			}
			out.Wastages = append(out.Wastages, store.CashflowWastage{Date: store.NormalizeDate(date), Total: total, CostTotal: cost})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}

	// materials (cost basis + margins)
	if rows, err := a.pool.Query(ctx, `SELECT material_name, material_rate, purchase_rate FROM materials WHERE company_id = $1`, co); err != nil {
		return out, fmt.Errorf("cashflow materials: %w", err)
	} else {
		for rows.Next() {
			var name string
			var sell, buy float64
			if err := rows.Scan(&name, &sell, &buy); err != nil {
				rows.Close()
				return out, err
			}
			out.Materials = append(out.Materials, store.CashflowMaterial{Name: name, MaterialRate: sell, PurchaseRate: buy})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return out, err
		}
	}
	return out, nil
}
