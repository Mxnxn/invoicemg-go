package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colBatchReceives = "batchreceives"

type batchReceives struct{ db *mongo.Database }

func (s *Store) BatchReceives() store.BatchReceives { return &batchReceives{db: s.db} }

type brAlloc struct {
	JobID  *primitive.ObjectID `bson:"job_id"`
	Amount float64             `bson:"amount"`
}
type brEntryAlloc struct {
	InvoiceID *primitive.ObjectID `bson:"invoice_id"`
	Amount    float64             `bson:"amount"`
}
type brDoc struct {
	ID          primitive.ObjectID  `bson:"_id"`
	UID         primitive.ObjectID  `bson:"uid"`
	CompanyID   *primitive.ObjectID `bson:"company_id"`
	Client      *primitive.ObjectID `bson:"client"`
	InvoiceID   *primitive.ObjectID `bson:"invoice_id"`
	BankID      *primitive.ObjectID `bson:"bank_id"`
	Date        string              `bson:"date"`
	Amount      float64             `bson:"amount"`
	Note        string              `bson:"note"`
	Mode        string              `bson:"mode"`
	Allocations []brAlloc           `bson:"allocations"`
	EntryAllocs []brEntryAlloc      `bson:"entryAllocations"`
	CreatedAt   time.Time           `bson:"createdAt"`
	UpdatedAt   time.Time           `bson:"updatedAt"`
	Version     int                 `bson:"__v"`
}

func (b *batchReceives) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.BatchReceive, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID}
	if clientID != "" {
		cOID, err := objectID(clientID)
		if err != nil {
			return nil, store.ErrBadID
		}
		filter["client"] = cOID
	}
	cur, err := b.db.Collection(colBatchReceives).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing batch receives: %w", err)
	}
	defer cur.Close(ctx)
	var docs []brDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}

	clientIDs, bankIDs := map[primitive.ObjectID]struct{}{}, map[primitive.ObjectID]struct{}{}
	jobIDs, invIDs := map[primitive.ObjectID]struct{}{}, map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		addID(clientIDs, d.Client)
		addID(bankIDs, d.BankID)
		addID(invIDs, d.InvoiceID)
		for _, a := range d.Allocations {
			addID(jobIDs, a.JobID)
		}
		for _, a := range d.EntryAllocs {
			addID(invIDs, a.InvoiceID)
		}
	}
	clients, err := b.clientTriples(ctx, keys(clientIDs))
	if err != nil {
		return nil, err
	}
	banks, err := b.nameMap(ctx, colBanks, keys(bankIDs), "name")
	if err != nil {
		return nil, err
	}
	jobNums, err := b.nameMap(ctx, colJobs, keys(jobIDs), "challanNumber")
	if err != nil {
		return nil, err
	}
	invNums, err := b.nameMap(ctx, colInvoices, keys(invIDs), "invoiceId")
	if err != nil {
		return nil, err
	}

	out := make([]store.BatchReceive, 0, len(docs))
	for _, d := range docs {
		br := store.BatchReceive{
			ID: idOf(d.ID), UID: idOf(d.UID), Date: d.Date, Amount: d.Amount, Note: d.Note, Mode: d.Mode,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		}
		if d.CompanyID != nil {
			br.CompanyID = idOf(*d.CompanyID)
		}
		if d.BankID != nil {
			br.BankID, br.BankName = idOf(*d.BankID), banks[*d.BankID]
		}
		if d.Client != nil {
			c := clients[*d.Client]
			br.ClientID, br.ClientName, br.ClientFirm, br.ClientPhone = idOf(*d.Client), c.name, c.firm, c.phone
		}
		var dests []store.ReceiptDestination
		if d.InvoiceID != nil {
			dests = append(dests, store.ReceiptDestination{Kind: "invoice", ID: idOf(*d.InvoiceID).String(), Label: invNums[*d.InvoiceID], Amount: d.Amount})
		}
		for _, a := range d.Allocations {
			if a.JobID != nil {
				dests = append(dests, store.ReceiptDestination{Kind: "job", ID: idOf(*a.JobID).String(), Label: jobNums[*a.JobID], Amount: a.Amount})
			}
		}
		for _, a := range d.EntryAllocs {
			if a.InvoiceID == nil {
				continue
			}
			id := idOf(*a.InvoiceID).String()
			merged := false
			for i := range dests {
				if dests[i].Kind == "invoice" && dests[i].ID == id {
					dests[i].Amount = brRound2(dests[i].Amount + a.Amount)
					merged = true
					break
				}
			}
			if !merged {
				dests = append(dests, store.ReceiptDestination{Kind: "invoice", ID: id, Label: invNums[*a.InvoiceID], Amount: a.Amount})
			}
		}
		br.Destinations = dests
		out = append(out, br)
	}
	return out, nil
}

func brRound2(n float64) float64 { return float64(int64(n*100+0.5)) / 100 }

type clientTriple struct{ name, firm, phone string }

func (b *batchReceives) clientTriples(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]clientTriple, error) {
	out := map[primitive.ObjectID]clientTriple{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := b.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientPhone": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Name  string             `bson:"clientName"`
		Firm  string             `bson:"clientFirm"`
		Phone string             `bson:"clientPhone"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = clientTriple{r.Name, r.Firm, r.Phone}
	}
	return out, nil
}

// nameMap resolves ids to one string field, in one query - the batched label lookup.
func (b *batchReceives) nameMap(ctx context.Context, coll string, ids []primitive.ObjectID, field string) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := b.db.Collection(coll).Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{field: 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []bson.M
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		id, _ := r["_id"].(primitive.ObjectID)
		s, _ := r[field].(string)
		out[id] = s
	}
	return out, nil
}
