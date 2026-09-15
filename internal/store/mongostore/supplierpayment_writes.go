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

func (s *supplierPayments) OpenInvoices(ctx context.Context, uid, companyID, supplierID store.ID) ([]store.SupplierOpenInvoice, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	supOID, err := objectID(supplierID)
	if err != nil {
		return nil, store.ErrBadID
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID, "supplier_id": supOID,
		"$expr": bson.M{"$lt": bson.A{bson.M{"$ifNull": bson.A{"$amount", 0}}, "$total"}}}
	cur, err := s.db.Collection(colPurchaseInvoices).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "createdAt", Value: 1}}).
			SetProjection(bson.M{"invoiceNumber": 1, "date": 1, "total": 1, "amount": 1}))
	if err != nil {
		return nil, fmt.Errorf("open invoices: %w", err)
	}
	var docs []struct {
		ID            primitive.ObjectID `bson:"_id"`
		InvoiceNumber string             `bson:"invoiceNumber"`
		Date          string             `bson:"date"`
		Total         float64            `bson:"total"`
		Amount        float64            `bson:"amount"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.SupplierOpenInvoice, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.SupplierOpenInvoice{
			ID: idOf(d.ID), InvoiceNumber: d.InvoiceNumber, Date: d.Date, Total: d.Total, Amount: d.Amount,
			Due: brRound2(d.Total - d.Amount),
		})
	}
	return out, nil
}

func (s *supplierPayments) Create(ctx context.Context, uid, companyID store.ID, in store.SupplierPaymentWrite) (store.SupplierPayment, store.SupplierPayResult, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, err
	}
	supOID, err := objectID(in.SupplierID)
	if err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, store.ErrBadID
	}

	type planned struct {
		invoiceOID primitive.ObjectID
		amount     float64
	}
	var plan []planned

	if in.Mode == "auto" {
		cur, err := s.db.Collection(colPurchaseInvoices).Find(ctx,
			bson.M{"uid": uidOID, "company_id": companyOID, "supplier_id": supOID,
				"$expr": bson.M{"$lt": bson.A{bson.M{"$ifNull": bson.A{"$amount", 0}}, "$total"}}},
			options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "createdAt", Value: 1}}).
				SetProjection(bson.M{"total": 1, "amount": 1}))
		if err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("auto invoices: %w", err)
		}
		var opens []struct {
			ID     primitive.ObjectID `bson:"_id"`
			Total  float64            `bson:"total"`
			Amount float64            `bson:"amount"`
		}
		if err := cur.All(ctx, &opens); err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, err
		}
		remaining := brRound2(in.Amount)
		for _, o := range opens {
			if remaining <= 0 {
				break
			}
			due := brRound2(o.Total - o.Amount)
			if due <= 0 {
				continue
			}
			applied := due
			if remaining < due {
				applied = remaining
			}
			applied = brRound2(applied)
			plan = append(plan, planned{invoiceOID: o.ID, amount: applied})
			remaining = brRound2(remaining - applied)
		}
		if remaining > 0 {
			return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayAutoUnallocated, Remaining: remaining}, nil
		}
	} else {
		for _, a := range in.Allocations {
			applied := brRound2(a.Amount)
			if a.InvoiceID == "" || applied <= 0 {
				continue
			}
			invOID, err := objectID(a.InvoiceID)
			if err != nil {
				return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayInvoiceNotFound}, nil
			}
			var inv struct {
				Total  float64 `bson:"total"`
				Amount float64 `bson:"amount"`
			}
			err = s.db.Collection(colPurchaseInvoices).FindOne(ctx,
				bson.M{"_id": invOID, "uid": uidOID, "company_id": companyOID, "supplier_id": supOID}).Decode(&inv)
			if errors.Is(err, mongo.ErrNoDocuments) {
				return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayInvoiceNotFound}, nil
			}
			if err != nil {
				return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("manual invoice lookup: %w", err)
			}
			due := brRound2(inv.Total - inv.Amount)
			if applied > due+0.01 {
				return store.SupplierPayment{}, store.SupplierPayResult{Status: store.SupplierPayOverInvoice, Applied: applied, Due: due}, nil
			}
			plan = append(plan, planned{invoiceOID: invOID, amount: applied})
		}
	}

	allocDocs := bson.A{}
	for _, p := range plan {
		if _, err := s.db.Collection(colPurchaseInvoices).UpdateByID(ctx, p.invoiceOID, bson.M{"$inc": bson.M{"amount": p.amount}}); err != nil {
			return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("apply invoice payment: %w", err)
		}
		allocDocs = append(allocDocs, bson.M{"purchase_invoice_id": p.invoiceOID, "amount": p.amount})
	}

	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "supplier_id": supOID, "date": in.Date, "amount": in.Amount,
		"note": in.Note, "mode": in.Mode, "allocations": allocDocs, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	if in.BankID != "" {
		if bankOID, err := objectID(in.BankID); err == nil {
			doc["bank_id"] = bankOID
		}
	}
	res, err := s.db.Collection(colSupplierPayments).InsertOne(ctx, doc)
	if err != nil {
		return store.SupplierPayment{}, store.SupplierPayResult{}, fmt.Errorf("insert supplier payment: %w", err)
	}
	oid, _ := res.InsertedID.(primitive.ObjectID)
	out, err := s.getOne(ctx, uid, companyID, idOf(oid))
	return out, store.SupplierPayResult{Status: store.SupplierPayOK}, err
}

func (s *supplierPayments) Delete(ctx context.Context, uid, companyID, paymentID store.ID) (bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	payOID, err := objectID(paymentID)
	if err != nil {
		return false, nil
	}
	var doc struct {
		Allocations []spAlloc `bson:"allocations"`
	}
	err = s.db.Collection(colSupplierPayments).FindOne(ctx, bson.M{"_id": payOID, "uid": uidOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up supplier payment: %w", err)
	}
	for _, a := range doc.Allocations {
		if a.PurchaseInvoiceID == nil {
			continue
		}
		var inv struct {
			Amount float64 `bson:"amount"`
		}
		if err := s.db.Collection(colPurchaseInvoices).FindOne(ctx, bson.M{"_id": *a.PurchaseInvoiceID, "company_id": companyOID}).Decode(&inv); err != nil {
			continue
		}
		if _, err := s.db.Collection(colPurchaseInvoices).UpdateByID(ctx, *a.PurchaseInvoiceID,
			bson.M{"$set": bson.M{"amount": brRound2(maxF(0, inv.Amount-a.Amount))}}); err != nil {
			return false, fmt.Errorf("reverse invoice payment: %w", err)
		}
	}
	if _, err := s.db.Collection(colSupplierPayments).DeleteOne(ctx, bson.M{"_id": payOID}); err != nil {
		return false, fmt.Errorf("delete supplier payment: %w", err)
	}
	return true, nil
}

func (s *supplierPayments) getOne(ctx context.Context, uid, companyID, paymentID store.ID) (store.SupplierPayment, error) {
	list, err := s.List(ctx, uid, companyID)
	if err != nil {
		return store.SupplierPayment{}, err
	}
	for _, p := range list {
		if p.ID == paymentID {
			return p, nil
		}
	}
	return store.SupplierPayment{}, fmt.Errorf("supplier payment %s not found after create", paymentID)
}
