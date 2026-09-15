package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (q *quotations) Numbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := q.db.Collection(colQuotations).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetProjection(bson.M{"quotationNumber": 1}))
	if err != nil {
		return nil, fmt.Errorf("quotation numbers: %w", err)
	}
	var docs []struct {
		Number string `bson:"quotationNumber"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.Number)
	}
	return out, nil
}

// getDoc loads one owner+company scoped quotation and populates its client.
func (q *quotations) getDoc(ctx context.Context, uid, companyID, quotationID store.ID) (store.Quotation, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Quotation{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Quotation{}, false, err
	}
	qOID, err := objectID(quotationID)
	if err != nil {
		return store.Quotation{}, false, nil
	}
	var doc quotationDoc
	err = q.db.Collection(colQuotations).FindOne(ctx, bson.M{"_id": qOID, "uid": uidOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Quotation{}, false, nil
	}
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("reading quotation: %w", err)
	}
	clients, err := q.clientsByID(ctx, []quotationDoc{doc})
	if err != nil {
		return store.Quotation{}, false, err
	}
	return doc.toStore(clients), true, nil
}

func (q *quotations) Get(ctx context.Context, uid, companyID, quotationID store.ID) (store.Quotation, bool, error) {
	return q.getDoc(ctx, uid, companyID, quotationID)
}

// rowDocs turns the submitted rows into BSON subdocuments. keepJob supplies the job_id to carry
// for a row whose input ID matches an existing row (update); a valid input ID is reused so the
// subdoc _id stays stable, otherwise a fresh ObjectID is minted.
func rowDocs(rows []store.QuotationRowInput, keepJob map[primitive.ObjectID]*primitive.ObjectID) []bson.M {
	out := make([]bson.M, 0, len(rows))
	for _, r := range rows {
		id := primitive.NewObjectID()
		var jobID *primitive.ObjectID
		if r.ID != "" {
			if oid, err := primitive.ObjectIDFromHex(string(r.ID)); err == nil {
				id = oid
				if j, ok := keepJob[oid]; ok {
					jobID = j
				}
			}
		}
		out = append(out, bson.M{
			"_id": id, "material": r.Material, "description": r.Description, "hasDimensions": true,
			"length": r.Length, "width": r.Width, "qty": r.Qty, "rate": r.Rate,
			"cgst": r.Cgst, "sgst": r.Sgst, "discount": r.Discount, "charges": r.Charges, "job_id": jobID,
		})
	}
	return out
}

func (q *quotations) Create(ctx context.Context, uid, companyID store.ID, in store.QuotationWrite) (store.Quotation, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Quotation{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Quotation{}, false, err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return store.Quotation{}, false, store.ErrBadID
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "client_id": clientOID,
		"quotationNumber": in.QuotationNumber, "date": in.Date, "rows": rowDocs(in.Rows, nil),
		"createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := q.db.Collection(colQuotations).InsertOne(ctx, doc)
	if isDuplicate(err) {
		return store.Quotation{}, true, nil
	}
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("insert quotation: %w", err)
	}
	oid, _ := res.InsertedID.(primitive.ObjectID)
	out, _, err := q.getDoc(ctx, uid, companyID, idOf(oid))
	return out, false, err
}

func (q *quotations) Update(ctx context.Context, uid, companyID, quotationID store.ID, in store.QuotationUpdate) (store.Quotation, bool, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Quotation{}, false, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Quotation{}, false, false, err
	}
	qOID, err := objectID(quotationID)
	if err != nil {
		return store.Quotation{}, false, false, nil
	}
	scope := bson.M{"_id": qOID, "uid": uidOID, "company_id": companyOID}
	var existing quotationDoc
	err = q.db.Collection(colQuotations).FindOne(ctx, scope).Decode(&existing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Quotation{}, false, false, nil
	}
	if err != nil {
		return store.Quotation{}, false, false, fmt.Errorf("looking up quotation: %w", err)
	}

	set := bson.M{"updatedAt": time.Now().UTC()}
	if in.ClientID != nil {
		clientOID, err := objectID(*in.ClientID)
		if err != nil {
			return store.Quotation{}, false, false, store.ErrBadID
		}
		set["client_id"] = clientOID
	}
	if in.Date != nil {
		set["date"] = *in.Date
	}
	if in.QuotationNumber != nil {
		set["quotationNumber"] = *in.QuotationNumber
	}
	if in.Rows != nil {
		keep := map[primitive.ObjectID]*primitive.ObjectID{}
		for _, r := range existing.Rows {
			keep[r.ID] = r.JobID
		}
		set["rows"] = rowDocs(*in.Rows, keep)
	}
	_, err = q.db.Collection(colQuotations).UpdateOne(ctx, scope, bson.M{"$set": set})
	if isDuplicate(err) {
		return store.Quotation{}, true, false, nil
	}
	if err != nil {
		return store.Quotation{}, false, false, fmt.Errorf("update quotation: %w", err)
	}
	out, _, err := q.getDoc(ctx, uid, companyID, quotationID)
	return out, false, true, err
}

func (q *quotations) Delete(ctx context.Context, uid, companyID, quotationID store.ID) (bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	qOID, err := objectID(quotationID)
	if err != nil {
		return false, nil
	}
	res, err := q.db.Collection(colQuotations).DeleteOne(ctx, bson.M{"_id": qOID, "uid": uidOID, "company_id": companyOID})
	if err != nil {
		return false, fmt.Errorf("delete quotation: %w", err)
	}
	return res.DeletedCount > 0, nil
}

func (q *quotations) RowDelete(ctx context.Context, uid, companyID, quotationID, rowID store.ID) (store.Quotation, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Quotation{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Quotation{}, false, err
	}
	qOID, err := objectID(quotationID)
	if err != nil {
		return store.Quotation{}, false, nil
	}
	scope := bson.M{"_id": qOID, "uid": uidOID, "company_id": companyOID}
	// $pull is a no-op when the row id does not match, mirroring Node's optional-chaining delete.
	var pull any = rowID
	if oid, err := primitive.ObjectIDFromHex(string(rowID)); err == nil {
		pull = oid
	}
	res, err := q.db.Collection(colQuotations).UpdateOne(ctx, scope,
		bson.M{"$pull": bson.M{"rows": bson.M{"_id": pull}}, "$set": bson.M{"updatedAt": time.Now().UTC()}})
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("delete quotation row: %w", err)
	}
	if res.MatchedCount == 0 {
		return store.Quotation{}, false, nil
	}
	out, _, err := q.getDoc(ctx, uid, companyID, quotationID)
	return out, true, err
}
