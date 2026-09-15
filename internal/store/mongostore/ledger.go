package mongostore

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type ledger struct{ db *mongo.Database }

func (s *Store) Ledger() store.Ledger { return &ledger{db: s.db} }

func (l *ledger) ClientStatement(ctx context.Context, companyID, clientID store.ID) (store.LedgerClientData, error) {
	var out store.LedgerClientData
	coOID, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	cOID, err := objectID(clientID)
	if err != nil {
		return out, nil // a malformed id is just "not found"
	}
	var client struct {
		ClientName    string `bson:"clientName"`
		ClientFirm    string `bson:"clientFirm"`
		ClientGST     string `bson:"clientGST"`
		ClientAddress string `bson:"clientAddress"`
	}
	err = l.db.Collection(colClients).FindOne(ctx, bson.M{"_id": cOID, "company_id": coOID}).Decode(&client)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return out, nil // Found stays false
	}
	if err != nil {
		return out, fmt.Errorf("ledger client: %w", err)
	}
	out.Found = true
	out.ClientName, out.ClientFirm, out.ClientGST, out.ClientAddress = client.ClientName, client.ClientFirm, client.ClientGST, client.ClientAddress

	// invoices (bills)
	invCur, err := l.db.Collection(colInvoices).Find(ctx, bson.M{"client": cOID, "company_id": coOID},
		options.Find().SetProjection(bson.M{"date": 1, "invoiceId": 1, "totalAmount": 1}))
	if err != nil {
		return out, fmt.Errorf("ledger invoices: %w", err)
	}
	var invs []struct {
		Date        string  `bson:"date"`
		InvoiceID   string  `bson:"invoiceId"`
		TotalAmount float64 `bson:"totalAmount"`
	}
	if err := invCur.All(ctx, &invs); err != nil {
		return out, err
	}
	for _, iv := range invs {
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(iv.Date), Type: "Sales Invoice", InvoiceNo: iv.InvoiceID, Bill: iv.TotalAmount, Seq: 0})
	}

	// receipts (invoice_received, invoice_id -> invoiceId)
	recCur, err := l.db.Collection(colInvoiceReceived).Find(ctx, bson.M{"client": cOID, "company_id": coOID},
		options.Find().SetProjection(bson.M{"date": 1, "amount": 1, "invoice_id": 1}))
	if err != nil {
		return out, fmt.Errorf("ledger receipts: %w", err)
	}
	var recs []struct {
		Date      string              `bson:"date"`
		Amount    float64             `bson:"amount"`
		InvoiceID *primitive.ObjectID `bson:"invoice_id"`
	}
	if err := recCur.All(ctx, &recs); err != nil {
		return out, err
	}
	invNos := map[primitive.ObjectID]string{}
	{
		ids := map[primitive.ObjectID]struct{}{}
		for _, r := range recs {
			if r.InvoiceID != nil {
				ids[*r.InvoiceID] = struct{}{}
			}
		}
		if len(ids) > 0 {
			list := make([]primitive.ObjectID, 0, len(ids))
			for id := range ids {
				list = append(list, id)
			}
			c, err := l.db.Collection(colInvoices).Find(ctx, bson.M{"_id": bson.M{"$in": list}}, options.Find().SetProjection(bson.M{"invoiceId": 1}))
			if err != nil {
				return out, err
			}
			var rows []struct {
				ID  primitive.ObjectID `bson:"_id"`
				Num string             `bson:"invoiceId"`
			}
			if err := c.All(ctx, &rows); err != nil {
				return out, err
			}
			for _, r := range rows {
				invNos[r.ID] = r.Num
			}
		}
	}
	for _, r := range recs {
		no := ""
		if r.InvoiceID != nil {
			no = invNos[*r.InvoiceID]
		}
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(r.Date), Type: "Receipts", InvoiceNo: no, Receipt: r.Amount, Seq: 1})
	}

	// batch receives
	brCur, err := l.db.Collection(colBatchReceives).Find(ctx, bson.M{"client": cOID, "company_id": coOID},
		options.Find().SetProjection(bson.M{"date": 1, "amount": 1}))
	if err != nil {
		return out, fmt.Errorf("ledger batch: %w", err)
	}
	var brs []struct {
		Date   string  `bson:"date"`
		Amount float64 `bson:"amount"`
	}
	if err := brCur.All(ctx, &brs); err != nil {
		return out, err
	}
	for _, br := range brs {
		out.Txns = append(out.Txns, store.LedgerTxn{Date: store.NormalizeDate(br.Date), Type: "Receipts", Receipt: br.Amount, Seq: 1})
	}
	return out, nil
}

func (l *ledger) ClientDues(ctx context.Context, companyID store.ID) (store.ClientDuesData, error) {
	var out store.ClientDuesData
	oid, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	scope := bson.M{"company_id": oid}

	invoices, err := l.clientAmounts(ctx, colInvoices, scope, "totalAmount")
	if err != nil {
		return out, fmt.Errorf("dues invoices: %w", err)
	}
	received, err := l.clientAmounts(ctx, colInvoiceReceived, scope, "amount")
	if err != nil {
		return out, fmt.Errorf("dues received: %w", err)
	}
	batch, err := l.clientAmounts(ctx, colBatchReceives, scope, "amount")
	if err != nil {
		return out, fmt.Errorf("dues batch: %w", err)
	}
	out.Dues = store.ComputeClientDues(invoices, received, batch)

	out.Clients = map[string]store.ClientBrief{}
	cur, err := l.db.Collection(colClients).Find(ctx, scope,
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientPhone": 1}))
	if err != nil {
		return out, fmt.Errorf("dues clients: %w", err)
	}
	var clients []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Name  string             `bson:"clientName"`
		Firm  string             `bson:"clientFirm"`
		Phone string             `bson:"clientPhone"`
	}
	if err := cur.All(ctx, &clients); err != nil {
		return out, err
	}
	for _, c := range clients {
		out.Clients[c.ID.Hex()] = store.ClientBrief{Name: c.Name, Firm: c.Firm, Phone: c.Phone}
	}
	return out, nil
}

// clientAmounts projects the `client` ref and one amount field, keying by hex id ("" when the
// ref is absent, which aggregates nothing - Node's clientKey of a missing client).
func (l *ledger) clientAmounts(ctx context.Context, coll string, scope bson.M, amountField string) ([]store.ClientAmount, error) {
	cur, err := l.db.Collection(coll).Find(ctx, scope, options.Find().SetProjection(bson.M{"client": 1, amountField: 1}))
	if err != nil {
		return nil, err
	}
	var raw []bson.M
	if err := cur.All(ctx, &raw); err != nil {
		return nil, err
	}
	out := make([]store.ClientAmount, 0, len(raw))
	for _, r := range raw {
		key := ""
		if cid, ok := r["client"].(primitive.ObjectID); ok {
			key = cid.Hex()
		}
		amount := 0.0
		switch v := r[amountField].(type) {
		case float64:
			amount = v
		case int32:
			amount = float64(v)
		case int64:
			amount = float64(v)
		}
		out = append(out, store.ClientAmount{ClientID: key, Amount: amount})
	}
	return out, nil
}
