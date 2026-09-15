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
