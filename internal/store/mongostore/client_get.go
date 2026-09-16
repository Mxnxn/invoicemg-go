package mongostore

import (
	"context"
	"errors"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Get is /client/get: the client with its entries populated newest-first, each carrying its
// issued-invoice number and quotation number (routes/Client.js populate).
func (c *clients) Get(ctx context.Context, companyID, clientID store.ID) (store.ClientDetail, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.ClientDetail{}, false, err
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return store.ClientDetail{}, false, err
	}
	var doc struct {
		ID            primitive.ObjectID   `bson:"_id"`
		UID           *primitive.ObjectID  `bson:"uid"`
		CompanyID     *primitive.ObjectID  `bson:"company_id"`
		LegacyID      any                  `bson:"client_id"`
		ClientName    string               `bson:"clientName"`
		ClientFirm    string               `bson:"clientFirm"`
		ClientPhone   string               `bson:"clientPhone"`
		ClientGST     string               `bson:"clientGST"`
		ClientAddress string               `bson:"clientAddress"`
		Entries       []primitive.ObjectID `bson:"entries"`
	}
	err = c.db.Collection(colClients).FindOne(ctx, bson.M{"_id": clientOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.ClientDetail{}, false, nil
	}
	if err != nil {
		return store.ClientDetail{}, false, err
	}

	d := store.ClientDetail{
		ID: idOf(doc.ID), LegacyID: doc.LegacyID, ClientName: doc.ClientName, ClientFirm: doc.ClientFirm,
		ClientPhone: doc.ClientPhone, ClientGST: doc.ClientGST, ClientAddress: doc.ClientAddress,
	}
	if doc.UID != nil {
		d.UID = idOf(*doc.UID)
	}
	if doc.CompanyID != nil {
		d.CompanyID = idOf(*doc.CompanyID)
	}

	d.Entries = make([]store.ClientEntryView, 0, len(doc.Entries))
	for _, id := range doc.Entries {
		var ed struct {
			fullEntryDoc `bson:",inline"`
			Issued       *primitive.ObjectID `bson:"issued"`
			QuotationID  *primitive.ObjectID `bson:"quotation_id"`
		}
		if err := c.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": id}).Decode(&ed); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			return store.ClientDetail{}, false, err
		}
		v := store.ClientEntryView{Entry: ed.fullEntryDoc.toStore()}
		if ed.Issued != nil {
			v.IssuedID = idOf(*ed.Issued)
			v.IssuedInvoiceID = c.invoiceNumber(ctx, *ed.Issued)
		}
		if ed.QuotationID != nil {
			v.QuotationID = idOf(*ed.QuotationID)
			v.QuotationNumber = c.quotationNumber(ctx, *ed.QuotationID)
		}
		d.Entries = append(d.Entries, v)
	}
	// Newest first, matching the populate's { sort: { createdAt: -1 } }.
	sort.SliceStable(d.Entries, func(i, j int) bool {
		return d.Entries[i].CreatedAt.After(d.Entries[j].CreatedAt)
	})
	return d, true, nil
}

func (c *clients) invoiceNumber(ctx context.Context, id primitive.ObjectID) string {
	var doc struct {
		InvoiceID string `bson:"invoiceId"`
	}
	_ = c.db.Collection(colInvoices).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	return doc.InvoiceID
}

func (c *clients) quotationNumber(ctx context.Context, id primitive.ObjectID) string {
	var doc struct {
		QuotationNumber string `bson:"quotationNumber"`
	}
	_ = c.db.Collection(colQuotations).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	return doc.QuotationNumber
}
