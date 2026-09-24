package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type prodJobDoc struct {
	ID         primitive.ObjectID  `bson:"_id"`
	Queue      string              `bson:"queue"`
	EmployeeID *primitive.ObjectID `bson:"employee_id"`
	CreatedAt  time.Time           `bson:"createdAt"`
	UpdatedAt  time.Time           `bson:"updatedAt"`
	Rows       []struct {
		ID         primitive.ObjectID  `bson:"_id"`
		RowID      string              `bson:"rowId"`
		Queue      string              `bson:"queue"`
		EmployeeID *primitive.ObjectID `bson:"employee_id"`
		CreatedAt  time.Time           `bson:"createdAt"`
	} `bson:"rows"`
}

func (a *analytics) ProductionWip(ctx context.Context, companyID store.ID) (store.ProductionWipData, error) {
	var out store.ProductionWipData
	companyOID, err := objectID(companyID)
	if err != nil {
		return out, err
	}

	cur, err := a.db.Collection(colJobs).Find(ctx, bson.M{"company_id": companyOID},
		options.Find().
			SetProjection(bson.M{"queue": 1, "employee_id": 1, "createdAt": 1, "rows._id": 1, "rows.rowId": 1, "rows.queue": 1, "rows.employee_id": 1, "rows.createdAt": 1}).
			SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return out, fmt.Errorf("wip jobs: %w", err)
	}
	var jobs []prodJobDoc
	if err := cur.All(ctx, &jobs); err != nil {
		return out, err
	}
	empIDs := map[primitive.ObjectID]struct{}{}
	for _, j := range jobs {
		if len(j.Rows) == 0 {
			c := store.ProductionCard{JobID: idOf(j.ID), Key: "", Queue: j.Queue, CreatedAt: j.CreatedAt}
			if j.EmployeeID != nil {
				c.EmployeeID = idOf(*j.EmployeeID)
				empIDs[*j.EmployeeID] = struct{}{}
			}
			out.Cards = append(out.Cards, c)
			continue
		}
		for _, r := range j.Rows {
			key := r.RowID
			if key == "" {
				key = r.ID.Hex()
			}
			c := store.ProductionCard{JobID: idOf(j.ID), Key: key, Queue: r.Queue, CreatedAt: r.CreatedAt}
			if r.EmployeeID != nil {
				c.EmployeeID = idOf(*r.EmployeeID)
				empIDs[*r.EmployeeID] = struct{}{}
			}
			out.Cards = append(out.Cards, c)
		}
	}

	if out.Events, err = a.queueAdvancedEvents(ctx, companyOID, nil); err != nil {
		return out, err
	}
	out.PersonNames, err = a.personNames(ctx, empIDs)
	return out, err
}

func (a *analytics) ProductionThroughput(ctx context.Context, companyID store.ID, windowStart time.Time) (store.ProductionThroughputData, error) {
	var out store.ProductionThroughputData
	companyOID, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	if out.Events, err = a.queueAdvancedEvents(ctx, companyOID, &windowStart); err != nil {
		return out, err
	}

	cur, err := a.db.Collection(colJobs).Find(ctx, bson.M{"company_id": companyOID, "createdAt": bson.M{"$gte": windowStart}},
		options.Find().SetProjection(bson.M{"createdAt": 1, "updatedAt": 1, "queue": 1, "rows.queue": 1}))
	if err != nil {
		return out, fmt.Errorf("throughput jobs: %w", err)
	}
	var jobs []prodJobDoc
	if err := cur.All(ctx, &jobs); err != nil {
		return out, err
	}
	for _, j := range jobs {
		dj := store.ProductionDoneJob{CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, Queue: j.Queue}
		for _, r := range j.Rows {
			dj.RowQueues = append(dj.RowQueues, r.Queue)
		}
		out.DoneJobs = append(out.DoneJobs, dj)
	}
	return out, nil
}

func (a *analytics) queueAdvancedEvents(ctx context.Context, companyOID primitive.ObjectID, since *time.Time) ([]store.ProductionEvent, error) {
	filter := bson.M{"company_id": companyOID, "action": "Queue advanced"}
	if since != nil {
		filter["createdAt"] = bson.M{"$gte": *since}
	}
	cur, err := a.db.Collection(colJobHistories).Find(ctx, filter,
		options.Find().
			SetProjection(bson.M{"job_id": 1, "detail": 1, "fromStage": 1, "toStage": 1, "rowKey": 1, "actorName": 1, "createdAt": 1}).
			SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("queue-advanced events: %w", err)
	}
	var docs []struct {
		JobID     primitive.ObjectID `bson:"job_id"`
		Detail    string             `bson:"detail"`
		FromStage string             `bson:"fromStage"`
		ToStage   string             `bson:"toStage"`
		RowKey    string             `bson:"rowKey"`
		ActorName string             `bson:"actorName"`
		CreatedAt time.Time          `bson:"createdAt"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.ProductionEvent, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.ProductionEvent{
			JobID: idOf(d.JobID), Detail: d.Detail, FromStage: d.FromStage, ToStage: d.ToStage,
			RowKey: d.RowKey, ActorName: d.ActorName, CreatedAt: d.CreatedAt,
		})
	}
	return out, nil
}

func (a *analytics) personNames(ctx context.Context, ids map[primitive.ObjectID]struct{}) (map[store.ID]string, error) {
	out := map[store.ID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	list := make([]primitive.ObjectID, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	cur, err := a.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": list}},
		options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil, fmt.Errorf("person names: %w", err)
	}
	var docs []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[idOf(d.ID)] = d.Name
	}
	return out, nil
}
