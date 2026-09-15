package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// getDoc loads one owner+company scoped purchase invoice and populates its supplier.
func (p *purchaseInvoices) getDoc(ctx context.Context, uid, companyID, invoiceID store.ID) (store.PurchaseInvoice, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseInvoice{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseInvoice{}, false, err
	}
	iOID, err := objectID(invoiceID)
	if err != nil {
		return store.PurchaseInvoice{}, false, nil
	}
	var doc purchaseInvoiceDoc
	err = p.db.Collection(colPurchaseInvoices).FindOne(ctx, bson.M{"_id": iOID, "uid": uidOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PurchaseInvoice{}, false, nil
	}
	if err != nil {
		return store.PurchaseInvoice{}, false, fmt.Errorf("reading purchase invoice: %w", err)
	}
	suppliers, err := p.suppliersByID(ctx, []purchaseInvoiceDoc{doc})
	if err != nil {
		return store.PurchaseInvoice{}, false, err
	}
	return doc.toStore(suppliers), true, nil
}

// purchaseRowDocs turns submitted rows into BSON subdocs. Purchase rows are by-quantity
// (hasDimensions false) with a fresh id each.
func purchaseRowDocs(rows []store.PurchaseRowInput) []bson.M {
	out := make([]bson.M, 0, len(rows))
	for _, r := range rows {
		out = append(out, bson.M{
			"_id": primitive.NewObjectID(), "description": r.Description, "material": r.Material,
			"hsn": r.Hsn, "gst": r.Gst, "hasDimensions": false, "rate": r.Rate, "qty": r.Qty,
			"unit": r.Unit, "discount": r.Discount, "charges": r.Charges,
		})
	}
	return out
}

func (p *purchaseInvoices) Create(ctx context.Context, uid, companyID store.ID, in store.PurchaseInvoiceWrite) (store.PurchaseInvoice, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseInvoice{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseInvoice{}, err
	}
	supplierOID, err := objectID(in.SupplierID)
	if err != nil {
		return store.PurchaseInvoice{}, store.ErrBadID
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "supplier_id": supplierOID, "date": in.Date,
		"invoiceNumber": in.InvoiceNumber, "rows": purchaseRowDocs(in.Rows), "total": in.Total, "amount": 0,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := p.db.Collection(colPurchaseInvoices).InsertOne(ctx, doc)
	if err != nil {
		return store.PurchaseInvoice{}, fmt.Errorf("insert purchase invoice: %w", err)
	}
	oid, _ := res.InsertedID.(primitive.ObjectID)
	out, _, err := p.getDoc(ctx, uid, companyID, idOf(oid))
	return out, err
}

func (p *purchaseInvoices) Update(ctx context.Context, uid, companyID, invoiceID store.ID, in store.PurchaseInvoiceUpdate) (store.PurchaseInvoice, store.PurchaseUpdateResult, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, err
	}
	iOID, err := objectID(invoiceID)
	if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{Status: store.PurchaseUpdateNotFound}, nil
	}
	scope := bson.M{"_id": iOID, "uid": uidOID, "company_id": companyOID}
	var existing purchaseInvoiceDoc
	err = p.db.Collection(colPurchaseInvoices).FindOne(ctx, scope).Decode(&existing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{Status: store.PurchaseUpdateNotFound}, nil
	}
	if err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, fmt.Errorf("looking up purchase invoice: %w", err)
	}

	set := bson.M{"updatedAt": time.Now().UTC()}
	if in.SupplierID != nil {
		supplierOID, err := objectID(*in.SupplierID)
		if err != nil {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, store.ErrBadID
		}
		set["supplier_id"] = supplierOID
	}
	if in.Date != nil {
		set["date"] = *in.Date
	}
	if in.InvoiceNumber != nil {
		set["invoiceNumber"] = *in.InvoiceNumber
	}
	if in.Rows != nil {
		if in.NewTotal < existing.Amount-0.01 {
			return store.PurchaseInvoice{}, store.PurchaseUpdateResult{Status: store.PurchaseUpdatePaidExceeds, AmountPaid: existing.Amount, NewTotal: in.NewTotal}, nil
		}
		set["rows"] = purchaseRowDocs(*in.Rows)
		set["total"] = in.NewTotal
	}
	if _, err := p.db.Collection(colPurchaseInvoices).UpdateOne(ctx, scope, bson.M{"$set": set}); err != nil {
		return store.PurchaseInvoice{}, store.PurchaseUpdateResult{}, fmt.Errorf("update purchase invoice: %w", err)
	}
	out, _, err := p.getDoc(ctx, uid, companyID, invoiceID)
	return out, store.PurchaseUpdateResult{Status: store.PurchaseUpdateOK}, err
}

func (p *purchaseInvoices) Delete(ctx context.Context, uid, companyID, invoiceID store.ID) (store.PurchaseDeleteStatus, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseDeleteNotFound, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseDeleteNotFound, err
	}
	iOID, err := objectID(invoiceID)
	if err != nil {
		return store.PurchaseDeleteNotFound, nil
	}
	// Block if a supplier payment allocates to this invoice.
	paid, err := p.db.Collection(colSupplierPayments).CountDocuments(ctx,
		bson.M{"company_id": companyOID, "allocations.purchase_invoice_id": iOID})
	if err != nil {
		return store.PurchaseDeleteNotFound, fmt.Errorf("checking supplier payments: %w", err)
	}
	if paid > 0 {
		return store.PurchaseDeleteHasPayment, nil
	}
	res, err := p.db.Collection(colPurchaseInvoices).DeleteOne(ctx, bson.M{"_id": iOID, "uid": uidOID, "company_id": companyOID})
	if err != nil {
		return store.PurchaseDeleteNotFound, fmt.Errorf("delete purchase invoice: %w", err)
	}
	if res.DeletedCount == 0 {
		return store.PurchaseDeleteNotFound, nil
	}
	return store.PurchaseDeleteOK, nil
}
