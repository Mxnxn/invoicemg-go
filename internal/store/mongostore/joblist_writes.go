package mongostore

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (j *jobs) ChallanNumbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := j.db.Collection(colJobs).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetProjection(bson.M{"challanNumber": 1}))
	if err != nil {
		return nil, fmt.Errorf("challan numbers: %w", err)
	}
	var docs []struct {
		N string `bson:"challanNumber"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.N)
	}
	return out, nil
}

func (j *jobs) ByEntry(ctx context.Context, uid, companyID, entryID store.ID) (store.Job, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Job{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Job{}, false, err
	}
	entryOID, err := objectID(entryID)
	if err != nil {
		return store.Job{}, false, nil
	}
	var doc struct {
		ClientID interface{} `bson:"client_id"`
	}
	err = j.db.Collection(colJobs).FindOne(ctx, bson.M{"rows.entry_id": entryOID, "uid": uidOID, "company_id": companyOID},
		options.FindOne().SetProjection(bson.M{"client_id": 1})).Decode(&doc)
	if err != nil {
		return store.Job{}, false, nil
	}
	list, err := j.List(ctx, uid, companyID, "")
	if err != nil {
		return store.Job{}, false, err
	}
	for _, job := range list {
		for _, r := range job.Rows {
			if r.Entry != nil && r.Entry.ID == entryID {
				return job, true, nil
			}
		}
	}
	return store.Job{}, false, nil
}
