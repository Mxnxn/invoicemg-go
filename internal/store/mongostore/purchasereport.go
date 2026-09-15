package mongostore

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type purchaseReport struct{ db *mongo.Database }

func (s *Store) PurchaseReport() store.PurchaseReport { return &purchaseReport{db: s.db} }

func (p *purchaseReport) SupplierDues(ctx context.Context, uid, companyID store.ID) (store.SupplierDuesData, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.SupplierDuesData{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.SupplierDuesData{}, err
	}
	out := store.SupplierDuesData{Suppliers: map[string]store.SupplierInfo{}}

	cur, err := p.db.Collection(colPurchaseInvoices).Find(ctx, bson.M{"company_id": companyOID})
	if err != nil {
		return store.SupplierDuesData{}, fmt.Errorf("purchase invoices: %w", err)
	}
	var invs []struct {
		SupplierID *primitive.ObjectID `bson:"supplier_id"`
		Total      float64             `bson:"total"`
		Amount     float64             `bson:"amount"`
	}
	if err := cur.All(ctx, &invs); err != nil {
		return store.SupplierDuesData{}, err
	}
	for _, iv := range invs {
		if iv.SupplierID == nil {
			continue
		}
		out.Invoices = append(out.Invoices, store.SupplierDueInvoice{SupplierID: idOf(*iv.SupplierID), Total: iv.Total, Amount: iv.Amount})
	}

	sCur, err := p.db.Collection(colPersons).Find(ctx, bson.M{"uid": uidOID, "type": "Supplier"})
	if err != nil {
		return store.SupplierDuesData{}, fmt.Errorf("suppliers: %w", err)
	}
	var sups []struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Firm    string             `bson:"firm"`
		Phone   string             `bson:"phone"`
		Gst     string             `bson:"gst"`
		Address string             `bson:"address"`
	}
	if err := sCur.All(ctx, &sups); err != nil {
		return store.SupplierDuesData{}, err
	}
	for _, sp := range sups {
		out.Suppliers[sp.ID.Hex()] = store.SupplierInfo{ID: idOf(sp.ID), Name: sp.Name, Firm: sp.Firm, Phone: sp.Phone, GST: sp.Gst, Address: sp.Address}
	}
	return out, nil
}

func (p *purchaseReport) SupplierStatement(ctx context.Context, uid, companyID, supplierID store.ID) (store.SupplierStatementData, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.SupplierStatementData{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.SupplierStatementData{}, err
	}
	supplierOID, err := objectID(supplierID)
	if err != nil {
		return store.SupplierStatementData{}, nil
	}

	var out store.SupplierStatementData
	var sp struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Firm    string             `bson:"firm"`
		Phone   string             `bson:"phone"`
		Gst     string             `bson:"gst"`
		Address string             `bson:"address"`
	}
	err = p.db.Collection(colPersons).FindOne(ctx, bson.M{"_id": supplierOID, "uid": uidOID, "type": "Supplier"}).Decode(&sp)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.SupplierStatementData{}, nil
	}
	if err != nil {
		return store.SupplierStatementData{}, err
	}
	out.Found = true
	out.Supplier = store.SupplierInfo{ID: idOf(sp.ID), Name: sp.Name, Firm: sp.Firm, Phone: sp.Phone, GST: sp.Gst, Address: sp.Address}

	iCur, err := p.db.Collection(colPurchaseInvoices).Find(ctx, bson.M{"supplier_id": supplierOID, "company_id": companyOID})
	if err != nil {
		return store.SupplierStatementData{}, fmt.Errorf("supplier invoices: %w", err)
	}
	var invs []struct {
		Date          string  `bson:"date"`
		InvoiceNumber string  `bson:"invoiceNumber"`
		Total         float64 `bson:"total"`
	}
	if err := iCur.All(ctx, &invs); err != nil {
		return store.SupplierStatementData{}, err
	}
	for _, iv := range invs {
		out.Invoices = append(out.Invoices, store.SupplierLedgerInvoice{Date: iv.Date, InvoiceNumber: iv.InvoiceNumber, Total: iv.Total})
	}

	pCur, err := p.db.Collection(colSupplierPayments).Find(ctx, bson.M{"supplier_id": supplierOID, "company_id": companyOID})
	if err != nil {
		return store.SupplierStatementData{}, fmt.Errorf("supplier payments: %w", err)
	}
	var pays []struct {
		Date   string  `bson:"date"`
		Amount float64 `bson:"amount"`
		Note   string  `bson:"note"`
	}
	if err := pCur.All(ctx, &pays); err != nil {
		return store.SupplierStatementData{}, err
	}
	for _, pay := range pays {
		out.Payments = append(out.Payments, store.SupplierLedgerPayment{Date: pay.Date, Amount: pay.Amount, Note: pay.Note})
	}
	return out, nil
}
