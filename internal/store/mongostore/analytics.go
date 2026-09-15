package mongostore

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colInvoiceReceived = "invoicereceiveds"

type analytics struct{ db *mongo.Database }

func (s *Store) Analytics() store.Analytics { return &analytics{db: s.db} }

// revRow carries both date sources and every amount field the revenue series reads; the caller
// picks which amount by collection.
type revRow struct {
	CreatedAt   *time.Time `bson:"createdAt"`
	Date        string     `bson:"date"`
	Total       float64    `bson:"total"`
	Amount      float64    `bson:"amount"`
	TotalAmount float64    `bson:"totalAmount"`
}

// effective resolves createdAt || date (getEffectiveDate).
func (r revRow) effective() (time.Time, bool) {
	if r.CreatedAt != nil && !r.CreatedAt.IsZero() {
		return r.CreatedAt.UTC(), true
	}
	if r.Date != "" {
		if t, err := time.Parse("2006-01-02", r.Date); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(time.RFC3339, r.Date); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func (a *analytics) series(ctx context.Context, coll string, filter bson.M, amount func(revRow) float64) ([]store.DatedAmount, error) {
	cur, err := a.db.Collection(coll).Find(ctx, filter,
		options.Find().SetProjection(bson.M{"createdAt": 1, "date": 1, "total": 1, "amount": 1, "totalAmount": 1}))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", coll, err)
	}
	defer cur.Close(ctx)
	var rows []revRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]store.DatedAmount, 0, len(rows))
	for _, r := range rows {
		if t, ok := r.effective(); ok {
			out = append(out, store.DatedAmount{Date: t, Amount: amount(r)})
		}
	}
	return out, nil
}

func (a *analytics) RevenueSeries(ctx context.Context, companyID store.ID, source string) ([]store.DatedAmount, []store.DatedAmount, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, nil, err
	}
	filter := bson.M{"company_id": oid}
	if source == "all" {
		billed, err := a.series(ctx, colEntries, filter, func(r revRow) float64 { return r.Total })
		if err != nil {
			return nil, nil, err
		}
		collected, err := a.series(ctx, colEntries, filter, func(r revRow) float64 { return r.Amount })
		if err != nil {
			return nil, nil, err
		}
		return billed, collected, nil
	}
	billed, err := a.series(ctx, colInvoices, filter, func(r revRow) float64 { return r.TotalAmount })
	if err != nil {
		return nil, nil, err
	}
	collected, err := a.series(ctx, colInvoiceReceived, filter, func(r revRow) float64 { return r.Amount })
	if err != nil {
		return nil, nil, err
	}
	return billed, collected, nil
}

type invRank struct {
	Client      *primitive.ObjectID `bson:"client"`
	TotalAmount float64             `bson:"totalAmount"`
	Amount      float64             `bson:"amount"`
}

func (a *analytics) rank(ctx context.Context, companyID store.ID, value func(invRank) float64, dropNonPositive bool) ([]store.ClientRank, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := a.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"client": 1, "totalAmount": 1, "amount": 1}))
	if err != nil {
		return nil, fmt.Errorf("ranking invoices: %w", err)
	}
	defer cur.Close(ctx)
	var invs []invRank
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	sums := map[primitive.ObjectID]float64{}
	order := []primitive.ObjectID{}
	for _, iv := range invs {
		if iv.Client == nil {
			continue
		}
		if _, ok := sums[*iv.Client]; !ok {
			order = append(order, *iv.Client)
		}
		sums[*iv.Client] += value(iv)
	}
	names, err := a.clientNames(ctx, order)
	if err != nil {
		return nil, err
	}
	out := []store.ClientRank{}
	for _, cid := range order {
		c, ok := names[cid]
		if !ok {
			continue
		}
		v := round2mongo(sums[cid])
		if dropNonPositive && v <= 0 {
			continue
		}
		out = append(out, store.ClientRank{ClientID: idOf(cid), ClientName: c.name, ClientFirm: c.firm, Value: v})
	}
	sortRanksDesc(out)
	return out, nil
}

type clientNameFirm struct{ name, firm string }

func (a *analytics) clientNames(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]clientNameFirm, error) {
	out := map[primitive.ObjectID]clientNameFirm{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := a.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"clientName"`
		Firm string             `bson:"clientFirm"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = clientNameFirm{name: r.Name, firm: r.Firm}
	}
	return out, nil
}

func (a *analytics) TopSales(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, func(i invRank) float64 { return i.TotalAmount }, false)
}
func (a *analytics) TopCredits(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, func(i invRank) float64 { return i.TotalAmount - i.Amount }, true)
}
func (a *analytics) TopPaid(ctx context.Context, c store.ID) ([]store.ClientRank, error) {
	return a.rank(ctx, c, func(i invRank) float64 { return i.Amount }, true)
}

func round2mongo(n float64) float64 { return math.Floor(n*100+0.5) / 100 }

func sortRanksDesc(rs []store.ClientRank) {
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Value > rs[j].Value })
}

func (a *analytics) PaymentGaps(ctx context.Context, companyID store.ID) ([]float64, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// fully-paid invoices, keyed by id, with their effective (billed) date.
	cur, err := a.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"createdAt": 1, "date": 1, "amount": 1, "totalAmount": 1}))
	if err != nil {
		return nil, fmt.Errorf("payment gaps invoices: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		ID          primitive.ObjectID `bson:"_id"`
		CreatedAt   *time.Time         `bson:"createdAt"`
		Date        string             `bson:"date"`
		Amount      float64            `bson:"amount"`
		TotalAmount float64            `bson:"totalAmount"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	billed := map[primitive.ObjectID]time.Time{}
	var paidIDs []primitive.ObjectID
	for _, iv := range invs {
		if iv.TotalAmount > 0 && iv.Amount >= iv.TotalAmount {
			if t, ok := (revRow{CreatedAt: iv.CreatedAt, Date: iv.Date}).effective(); ok {
				billed[iv.ID] = t
				paidIDs = append(paidIDs, iv.ID)
			}
		}
	}
	if len(paidIDs) == 0 {
		return nil, nil
	}
	rc, err := a.db.Collection(colInvoiceReceived).Find(ctx, bson.M{"company_id": oid, "invoice_id": bson.M{"$in": paidIDs}},
		options.Find().SetProjection(bson.M{"invoice_id": 1, "createdAt": 1, "date": 1}))
	if err != nil {
		return nil, fmt.Errorf("payment gaps receipts: %w", err)
	}
	defer rc.Close(ctx)
	var recs []struct {
		InvoiceID primitive.ObjectID `bson:"invoice_id"`
		CreatedAt *time.Time         `bson:"createdAt"`
		Date      string             `bson:"date"`
	}
	if err := rc.All(ctx, &recs); err != nil {
		return nil, err
	}
	last := map[primitive.ObjectID]time.Time{}
	for _, r := range recs {
		if t, ok := (revRow{CreatedAt: r.CreatedAt, Date: r.Date}).effective(); ok {
			if cur, ok := last[r.InvoiceID]; !ok || t.After(cur) {
				last[r.InvoiceID] = t
			}
		}
	}
	var gaps []float64
	for id, paid := range last {
		if b, ok := billed[id]; ok {
			if d := paid.Sub(b).Hours() / 24; d >= 0 {
				gaps = append(gaps, d)
			}
		}
	}
	return gaps, nil
}

func (a *analytics) PendingSince(ctx context.Context, companyID store.ID) ([]time.Time, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := a.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"createdAt": 1, "date": 1, "amount": 1, "totalAmount": 1}))
	if err != nil {
		return nil, fmt.Errorf("pending invoices: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		CreatedAt   *time.Time `bson:"createdAt"`
		Date        string     `bson:"date"`
		Amount      float64    `bson:"amount"`
		TotalAmount float64    `bson:"totalAmount"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	var out []time.Time
	for _, iv := range invs {
		if iv.TotalAmount > 0 && iv.Amount < iv.TotalAmount {
			if t, ok := (revRow{CreatedAt: iv.CreatedAt, Date: iv.Date}).effective(); ok {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func (a *analytics) Payables(ctx context.Context, companyID store.ID) (float64, []store.SupplierDue, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return 0, nil, err
	}
	cur, err := a.db.Collection(colPurchaseInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"total": 1, "amount": 1, "supplier_id": 1}))
	if err != nil {
		return 0, nil, fmt.Errorf("payables invoices: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		Total      float64             `bson:"total"`
		Amount     float64             `bson:"amount"`
		SupplierID *primitive.ObjectID `bson:"supplier_id"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return 0, nil, err
	}
	supIDs := map[primitive.ObjectID]struct{}{}
	for _, iv := range invs {
		addID(supIDs, iv.SupplierID)
	}
	names, err := a.personNamesForPayables(ctx, keys(supIDs))
	if err != nil {
		return 0, nil, err
	}
	rows := make([]store.PayableRow, 0, len(invs))
	for _, iv := range invs {
		name := ""
		if iv.SupplierID != nil {
			name = names[*iv.SupplierID]
		}
		rows = append(rows, store.PayableRow{Total: iv.Total, Amount: iv.Amount, Supplier: name})
	}
	total, bySup := store.SumPayables(rows)
	return total, bySup, nil
}

func (a *analytics) personNamesForPayables(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := a.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

func (a *analytics) OutstandingInvoices(ctx context.Context, companyID store.ID) ([]store.DatedAmount, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := a.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"createdAt": 1, "date": 1, "amount": 1, "totalAmount": 1}))
	if err != nil {
		return nil, fmt.Errorf("outstanding invoices: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		CreatedAt   *time.Time `bson:"createdAt"`
		Date        string     `bson:"date"`
		Amount      float64    `bson:"amount"`
		TotalAmount float64    `bson:"totalAmount"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	out := []store.DatedAmount{}
	for _, iv := range invs {
		due := iv.TotalAmount - iv.Amount
		if due <= 0 {
			continue
		}
		if t, ok := (revRow{CreatedAt: iv.CreatedAt, Date: iv.Date}).effective(); ok {
			out = append(out, store.DatedAmount{Date: t, Amount: due})
		}
	}
	return out, nil
}

func (a *analytics) UnbilledEntries(ctx context.Context, companyID store.ID) ([]store.UnbilledEntry, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := a.db.Collection(colEntries).Find(ctx, bson.M{"company_id": oid, "has_issued": false},
		options.Find().SetProjection(bson.M{"client_id": 1, "createdAt": 1, "date": 1, "total": 1}))
	if err != nil {
		return nil, fmt.Errorf("unbilled entries: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ClientID  *primitive.ObjectID `bson:"client_id"`
		CreatedAt *time.Time          `bson:"createdAt"`
		Date      string              `bson:"date"`
		Total     float64             `bson:"total"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	cids := map[primitive.ObjectID]struct{}{}
	for _, r := range rows {
		addID(cids, r.ClientID)
	}
	names, err := a.clientNames(ctx, keys(cids))
	if err != nil {
		return nil, err
	}
	out := make([]store.UnbilledEntry, 0, len(rows))
	for _, r := range rows {
		e := store.UnbilledEntry{Value: r.Total}
		if t, ok := (revRow{CreatedAt: r.CreatedAt, Date: r.Date}).effective(); ok {
			e.Date, e.HasDate = t, true
		}
		if r.ClientID != nil {
			if c, ok := names[*r.ClientID]; ok {
				e.ClientID, e.ClientName, e.ClientFirm = idOf(*r.ClientID), c.name, c.firm
			}
		}
		out = append(out, e)
	}
	return out, nil
}

func (a *analytics) Reviews(ctx context.Context, companyID store.ID, from, to string) ([]store.ReviewRow, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"company_id": oid}
	if from != "" || to != "" {
		dr := bson.M{}
		if from != "" {
			if t, err := time.Parse("2006-01-02", from); err == nil {
				dr["$gte"] = t.UTC()
			}
		}
		if to != "" {
			if t, err := time.Parse("2006-01-02", to); err == nil {
				dr["$lte"] = t.UTC().Add(24*time.Hour - time.Nanosecond)
			}
		}
		if len(dr) > 0 {
			filter["createdAt"] = dr
		}
	}
	cur, err := a.db.Collection(colJobReviews).Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("reviews: %w", err)
	}
	defer cur.Close(ctx)
	var docs []struct {
		ID            primitive.ObjectID  `bson:"_id"`
		ChallanNumber string              `bson:"challanNumber"`
		JobID         primitive.ObjectID  `bson:"job_id"`
		ClientID      *primitive.ObjectID `bson:"client_id"`
		ClientName    string              `bson:"clientName"`
		Comment       string              `bson:"comment"`
		CreatedAt     time.Time           `bson:"createdAt"`
		Scores        struct {
			Quality       int `bson:"quality"`
			Speed         int `bson:"speed"`
			Communication int `bson:"communication"`
			Satisfaction  int `bson:"satisfaction"`
			Overall       int `bson:"overall"`
		} `bson:"scores"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.ReviewRow, 0, len(docs))
	for _, d := range docs {
		r := store.ReviewRow{
			ID: idOf(d.ID), ChallanNumber: d.ChallanNumber, JobID: idOf(d.JobID), ClientName: d.ClientName,
			Comment: d.Comment, CreatedAt: d.CreatedAt,
			Scores: store.ReviewScores{Quality: d.Scores.Quality, Speed: d.Scores.Speed, Communication: d.Scores.Communication, Satisfaction: d.Scores.Satisfaction, Overall: d.Scores.Overall},
		}
		if d.ClientID != nil {
			r.ClientID = idOf(*d.ClientID)
		}
		out = append(out, r)
	}
	return out, nil
}

func (a *analytics) Receipts(ctx context.Context, companyID store.ID) ([]store.DatedAmount, error) {
	rec, err := a.series(ctx, colInvoiceReceived, bson.M{"company_id": mustOID(companyID)}, func(r revRow) float64 { return r.Amount })
	if err != nil {
		return nil, err
	}
	batch, err := a.series(ctx, "batchreceives", bson.M{"company_id": mustOID(companyID)}, func(r revRow) float64 { return r.Amount })
	if err != nil {
		return nil, err
	}
	return append(rec, batch...), nil
}

// mustOID converts for the internal Receipts filter; companyID is server-derived so it is valid.
func mustOID(id store.ID) primitive.ObjectID {
	oid, _ := objectID(id)
	return oid
}

func (a *analytics) GstSales(ctx context.Context, companyID store.ID) ([]store.GstDoc, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// invoices with entries populated (amount + gst%) and client (firm/name/gst).
	cur, err := a.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"invoiceId": 1, "date": 1, "client": 1, "entries": 1}))
	if err != nil {
		return nil, fmt.Errorf("gst sales: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		InvoiceID string               `bson:"invoiceId"`
		Date      string               `bson:"date"`
		Client    *primitive.ObjectID  `bson:"client"`
		Entries   []primitive.ObjectID `bson:"entries"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	// batch client + entry lookups
	cids := map[primitive.ObjectID]struct{}{}
	eids := map[primitive.ObjectID]struct{}{}
	for _, iv := range invs {
		addID(cids, iv.Client)
		for _, e := range iv.Entries {
			eids[e] = struct{}{}
		}
	}
	clients, err := a.gstClients(ctx, keys(cids))
	if err != nil {
		return nil, err
	}
	entries, err := a.gstEntryLines(ctx, keys(eids))
	if err != nil {
		return nil, err
	}
	out := make([]store.GstDoc, 0, len(invs))
	for _, iv := range invs {
		doc := store.GstDoc{InvoiceNo: iv.InvoiceID, Date: normalizeGstDate(iv.Date)}
		if iv.Client != nil {
			c := clients[*iv.Client]
			doc.PartyName = c.firm
			if doc.PartyName == "" {
				doc.PartyName = c.name
			}
			doc.GstNo = c.gst
		}
		for _, eid := range iv.Entries {
			if l, ok := entries[eid]; ok {
				doc.Lines = append(doc.Lines, l)
			}
		}
		out = append(out, doc)
	}
	return out, nil
}

func (a *analytics) GstPurchases(ctx context.Context, companyID store.ID) ([]store.GstDoc, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := a.db.Collection(colPurchaseInvoices).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"invoiceNumber": 1, "date": 1, "supplier_id": 1, "rows": 1}))
	if err != nil {
		return nil, fmt.Errorf("gst purchases: %w", err)
	}
	defer cur.Close(ctx)
	var invs []struct {
		InvoiceNumber string              `bson:"invoiceNumber"`
		Date          string              `bson:"date"`
		SupplierID    *primitive.ObjectID `bson:"supplier_id"`
		Rows          []struct {
			Rate     float64 `bson:"rate"`
			Qty      float64 `bson:"qty"`
			Discount float64 `bson:"discount"`
			Charges  float64 `bson:"charges"`
			Gst      float64 `bson:"gst"`
		} `bson:"rows"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return nil, err
	}
	sids := map[primitive.ObjectID]struct{}{}
	for _, iv := range invs {
		addID(sids, iv.SupplierID)
	}
	sups, err := a.gstSuppliers(ctx, keys(sids))
	if err != nil {
		return nil, err
	}
	out := make([]store.GstDoc, 0, len(invs))
	for _, iv := range invs {
		doc := store.GstDoc{InvoiceNo: iv.InvoiceNumber, Date: normalizeGstDate(iv.Date)}
		if iv.SupplierID != nil {
			s := sups[*iv.SupplierID]
			doc.PartyName = s.name
			doc.GstNo = s.gst
		}
		for _, r := range iv.Rows {
			gross := r.Rate*r.Qty - r.Discount + r.Charges
			doc.Lines = append(doc.Lines, store.GstLine{Amount: gross, Cgst: r.Gst / 2, Sgst: r.Gst / 2})
		}
		out = append(out, doc)
	}
	return out, nil
}

type gstClient struct{ name, firm, gst string }
type gstSupplier struct{ name, gst string }

func (a *analytics) gstClients(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]gstClient, error) {
	out := map[primitive.ObjectID]gstClient{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := a.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientGST": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"clientName"`
		Firm string             `bson:"clientFirm"`
		Gst  string             `bson:"clientGST"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = gstClient{r.Name, r.Firm, r.Gst}
	}
	return out, nil
}

func (a *analytics) gstSuppliers(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]gstSupplier, error) {
	out := map[primitive.ObjectID]gstSupplier{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := a.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{"name": 1, "gst": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
		Gst  string             `bson:"gst"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = gstSupplier{r.Name, r.Gst}
	}
	return out, nil
}

func (a *analytics) gstEntryLines(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]store.GstLine, error) {
	out := map[primitive.ObjectID]store.GstLine{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := a.db.Collection(colEntries).Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{"amount": 1, "cgst": 1, "sgst": 1, "igst": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID     primitive.ObjectID `bson:"_id"`
		Amount float64            `bson:"amount"`
		Cgst   float64            `bson:"cgst"`
		Sgst   float64            `bson:"sgst"`
		Igst   float64            `bson:"igst"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = store.GstLine{Amount: r.Amount, Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst}
	}
	return out, nil
}

// normalizeGstDate is Helpers/NormalizeDate.normalizeDate: passes YYYY-MM-DD through, converts
// a "Wkd Mon DD YYYY" toString form, else returns as-is.
func normalizeGstDate(raw string) string { return store.NormalizeDate(raw) }

func (a *analytics) Cashflow(ctx context.Context, companyID store.ID) (store.CashflowData, error) {
	var out store.CashflowData
	oid, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	scope := bson.M{"company_id": oid}

	// invoices: billed + collected
	{
		cur, err := a.db.Collection(colInvoices).Find(ctx, scope, options.Find().SetProjection(bson.M{"date": 1, "totalAmount": 1, "amount": 1}))
		if err != nil {
			return out, fmt.Errorf("cashflow invoices: %w", err)
		}
		var rows []struct {
			Date        string  `bson:"date"`
			TotalAmount float64 `bson:"totalAmount"`
			Amount      float64 `bson:"amount"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return out, err
		}
		for _, r := range rows {
			out.Invoices = append(out.Invoices, store.CashflowInvoice{Date: store.NormalizeDate(r.Date), TotalAmount: r.TotalAmount, Amount: r.Amount})
		}
	}

	// (date, amount) collections; purchase invoices read `total` into the amount slot.
	for _, q := range []struct {
		coll   string
		amount string
		dest   *[]store.CashflowRow
	}{
		{colInvoiceReceived, "amount", &out.Received},
		{colBatchReceives, "amount", &out.BatchReceives},
		{colSupplierPayments, "amount", &out.SupplierPayments},
		{colPurchaseInvoices, "total", &out.PurchaseInvoices},
	} {
		cur, err := a.db.Collection(q.coll).Find(ctx, scope, options.Find().SetProjection(bson.M{"date": 1, q.amount: 1}))
		if err != nil {
			return out, fmt.Errorf("cashflow rows: %w", err)
		}
		var raw []bson.M
		if err := cur.All(ctx, &raw); err != nil {
			return out, err
		}
		for _, r := range raw {
			*q.dest = append(*q.dest, store.CashflowRow{Date: store.NormalizeDate(asString(r["date"])), Amount: asFloat(r[q.amount])})
		}
	}

	// sold entries (invoiced only)
	{
		cur, err := a.db.Collection(colEntries).Find(ctx, bson.M{"company_id": oid, "has_issued": true}, options.Find().SetProjection(bson.M{"date": 1, "material": 1, "qty": 1}))
		if err != nil {
			return out, fmt.Errorf("cashflow entries: %w", err)
		}
		var rows []struct {
			Date     string  `bson:"date"`
			Material string  `bson:"material"`
			Qty      float64 `bson:"qty"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return out, err
		}
		for _, r := range rows {
			out.SoldEntries = append(out.SoldEntries, store.CashflowSoldEntry{Date: store.NormalizeDate(r.Date), Material: r.Material, Qty: r.Qty})
		}
	}

	// wastages
	{
		cur, err := a.db.Collection(colWastages).Find(ctx, scope, options.Find().SetProjection(bson.M{"date": 1, "total": 1, "cost_total": 1}))
		if err != nil {
			return out, fmt.Errorf("cashflow wastages: %w", err)
		}
		var rows []struct {
			Date      string  `bson:"date"`
			Total     float64 `bson:"total"`
			CostTotal float64 `bson:"cost_total"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return out, err
		}
		for _, r := range rows {
			out.Wastages = append(out.Wastages, store.CashflowWastage{Date: store.NormalizeDate(r.Date), Total: r.Total, CostTotal: r.CostTotal})
		}
	}

	// materials
	{
		cur, err := a.db.Collection(colMaterials).Find(ctx, scope, options.Find().SetProjection(bson.M{"material_name": 1, "material_rate": 1, "purchase_rate": 1}))
		if err != nil {
			return out, fmt.Errorf("cashflow materials: %w", err)
		}
		var rows []struct {
			Name string  `bson:"material_name"`
			Sell float64 `bson:"material_rate"`
			Buy  float64 `bson:"purchase_rate"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return out, err
		}
		for _, r := range rows {
			out.Materials = append(out.Materials, store.CashflowMaterial{Name: r.Name, MaterialRate: r.Sell, PurchaseRate: r.Buy})
		}
	}
	return out, nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
