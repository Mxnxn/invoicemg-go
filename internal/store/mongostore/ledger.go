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
