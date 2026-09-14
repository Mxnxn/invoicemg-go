package mongostore

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type clients struct{ db *mongo.Database }

func (s *Store) Clients() store.Clients { return &clients{db: s.db} }

// clientDoc is the subset of Model/Client.js the customer list reads. Sharing is a pointer so an
// absent field decodes to nil (Node omits the key) rather than to an empty struct.
type clientDoc struct {
	ID             primitive.ObjectID  `bson:"_id"`
	UID            primitive.ObjectID  `bson:"uid"`
	CompanyID      *primitive.ObjectID `bson:"company_id"`
	ClientName     string              `bson:"clientName"`
	ClientFirm     string              `bson:"clientFirm"`
	ClientPhone    string              `bson:"clientPhone"`
	ClientGST      string              `bson:"clientGST"`
	ClientAddress  string              `bson:"clientAddress"`
	OpeningBalance float64             `bson:"openingBalance"`
	Sharing        *struct {
		Companies []primitive.ObjectID `bson:"companies"`
	} `bson:"sharing"`
}

func (d clientDoc) toStore() store.Client {
	c := store.Client{
		ID:             idOf(d.ID),
		UID:            idOf(d.UID),
		ClientName:     d.ClientName,
		ClientFirm:     d.ClientFirm,
		ClientPhone:    d.ClientPhone,
		ClientGST:      d.ClientGST,
		ClientAddress:  d.ClientAddress,
		OpeningBalance: d.OpeningBalance,
	}
	if d.CompanyID != nil {
		c.CompanyID = idOf(*d.CompanyID)
	}
	if d.Sharing != nil {
		// present, even if empty - make(0) so a stamped-but-unshared record sends {companies:[]}.
		ids := make([]store.ID, 0, len(d.Sharing.Companies))
		for _, id := range d.Sharing.Companies {
			ids = append(ids, idOf(id))
		}
		c.Sharing = &ids
	}
	return c
}

func (c *clients) Visible(ctx context.Context, uid, companyID store.ID) ([]store.Client, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// visibleScope, bounded by uid: own company, legacy null-company rows, or shared with this
	// company (Helpers/SharedRecords.js). uid keeps it inside one admin, so sharing can never
	// cross accounts. No sort, matching routes/Client.js (natural order); the sqlstore adds the
	// id tiebreak (#19).
	filter := bson.M{
		"uid": uidOID,
		"$or": bson.A{
			bson.M{"company_id": companyOID},
			bson.M{"company_id": nil},
			bson.M{"sharing.companies": companyOID},
		},
	}
	cur, err := c.db.Collection(colClients).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer cur.Close(ctx)

	var docs []clientDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading clients: %w", err)
	}
	out := make([]store.Client, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}
