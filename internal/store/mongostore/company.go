package mongostore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type companies struct{ db *mongo.Database }

func (s *Store) Companies() store.Companies { return &companies{db: s.db} }

// companyDoc is the PUBLIC_FIELDS subset of Model/Company.js the shell needs.
type companyDoc struct {
	ID                primitive.ObjectID `bson:"_id"`
	Name              string             `bson:"name"`
	Firm              string             `bson:"firm"`
	Address           string             `bson:"address"`
	Phone             string             `bson:"phone"`
	Gst               string             `bson:"gst"`
	URL               string             `bson:"url"`
	UpiQr             string             `bson:"upiQr"`
	AccountNo         string             `bson:"account_no"`
	Ifsc              string             `bson:"ifsc"`
	BankName          string             `bson:"bank_name"`
	InvoiceTemplate   string             `bson:"invoiceTemplate"`
	QuotationTemplate string             `bson:"quotationTemplate"`
	LedgerTemplate    string             `bson:"ledgerTemplate"`
	DocumentShowUnits bool               `bson:"documentShowUnits"`
	DocumentShowSize  bool               `bson:"documentShowSize"`
	Whatsapp          struct {
		PhoneNumberID     string `bson:"phoneNumberId"`
		BusinessAccountID string `bson:"businessAccountId"`
		ApiToken          string `bson:"apiToken"`
	} `bson:"whatsapp"`
	IsDefault bool `bson:"is_default"`
	IsActive  bool `bson:"is_active"`
}

func (d companyDoc) toStore() store.Company {
	return store.Company{
		ID: idOf(d.ID), Name: d.Name, Firm: d.Firm, Address: d.Address, Phone: d.Phone,
		Gst: d.Gst, URL: d.URL, UpiQr: d.UpiQr, AccountNo: d.AccountNo, Ifsc: d.Ifsc,
		BankName: d.BankName, IsDefault: d.IsDefault, IsActive: d.IsActive,
		InvoiceTemplate: d.InvoiceTemplate, QuotationTemplate: d.QuotationTemplate, LedgerTemplate: d.LedgerTemplate,
		DocumentShowUnits: d.DocumentShowUnits, DocumentShowSize: d.DocumentShowSize,
		WaPhoneNumberID: d.Whatsapp.PhoneNumberID, WaBusinessAccountID: d.Whatsapp.BusinessAccountID, WaAPIToken: d.Whatsapp.ApiToken,
	}
}

func (c *companies) List(ctx context.Context, uid store.ID) ([]store.Company, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	// default first, then oldest, matching routes/Company.js's {is_default:-1, createdAt:1}.
	cur, err := c.db.Collection(colCompanies).Find(ctx,
		bson.M{"uid": uidOID, "is_active": true},
		options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing companies: %w", err)
	}
	defer cur.Close(ctx)

	var docs []companyDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading companies: %w", err)
	}
	out := make([]store.Company, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (c *companies) Active(ctx context.Context, companyID, uid store.ID) (store.Company, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return store.Company{}, store.ErrNotFound
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Company{}, store.ErrNotFound
	}
	var doc companyDoc
	err = c.db.Collection(colCompanies).FindOne(ctx, bson.M{"_id": cOID, "uid": uidOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Company{}, store.ErrNotFound
	}
	if err != nil {
		return store.Company{}, fmt.Errorf("looking up company: %w", err)
	}
	return doc.toStore(), nil
}

func (c *companies) Count(ctx context.Context, uid store.ID) (int, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return 0, err
	}
	n, err := c.db.Collection(colCompanies).CountDocuments(ctx, bson.M{"uid": uidOID})
	if err != nil {
		return 0, fmt.Errorf("counting companies: %w", err)
	}
	return int(n), nil
}

func (c *companies) Create(ctx context.Context, uid store.ID, in store.CompanyWrite) (store.Company, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Company{}, err
	}
	n, err := c.Count(ctx, uid)
	if err != nil {
		return store.Company{}, err
	}
	firm := in.Firm
	if firm == "" {
		firm = in.Name
	}
	isDefault := n == 0
	doc := bson.M{
		"uid": uidOID, "name": in.Name, "firm": firm, "address": in.Address, "phone": in.Phone,
		"gst": in.Gst, "account_no": in.AccountNo, "ifsc": in.Ifsc, "bank_name": in.BankName,
		"is_default": isDefault, "is_active": true,
	}
	res, err := c.db.Collection(colCompanies).InsertOne(ctx, doc)
	if err != nil {
		return store.Company{}, fmt.Errorf("insert company: %w", err)
	}
	out := store.Company{
		Name: in.Name, Firm: firm, Address: in.Address, Phone: in.Phone, Gst: in.Gst,
		AccountNo: in.AccountNo, Ifsc: in.Ifsc, BankName: in.BankName, IsDefault: isDefault, IsActive: true,
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, nil
}

func (c *companies) Update(ctx context.Context, uid, companyID store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Company{}, false, err
	}
	cOID, err := objectID(companyID)
	if err != nil {
		return store.Company{}, false, nil
	}
	set := bson.M{}
	for key, p := range map[string]*string{
		"name": patch.Name, "firm": patch.Firm, "address": patch.Address, "phone": patch.Phone,
		"gst": patch.Gst, "url": patch.URL, "account_no": patch.AccountNo, "ifsc": patch.Ifsc, "bank_name": patch.BankName,
		"invoiceTemplate": patch.InvoiceTemplate, "quotationTemplate": patch.QuotationTemplate, "ledgerTemplate": patch.LedgerTemplate,
		"whatsapp.phoneNumberId": patch.WaPhoneNumberID, "whatsapp.businessAccountId": patch.WaBusinessAccountID, "whatsapp.apiToken": patch.WaAPIToken,
	} {
		if p != nil {
			set[key] = *p
		}
	}
	for key, p := range map[string]*bool{
		"documentShowUnits": patch.DocumentShowUnits, "documentShowSize": patch.DocumentShowSize,
	} {
		if p != nil {
			set[key] = *p
		}
	}
	update := bson.M{}
	if len(set) > 0 {
		update["$set"] = set
	} else {
		update["$set"] = bson.M{"updatedAt": time.Now().UTC()}
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc companyDoc
	err = c.db.Collection(colCompanies).FindOneAndUpdate(ctx, bson.M{"_id": cOID, "uid": uidOID}, update, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Company{}, false, nil
	}
	if err != nil {
		return store.Company{}, false, fmt.Errorf("update company: %w", err)
	}
	return doc.toStore(), true, nil
}

func (c *companies) FindActive(ctx context.Context, uid, companyID store.ID) (store.Company, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Company{}, false, err
	}
	cOID, err := objectID(companyID)
	if err != nil {
		return store.Company{}, false, nil
	}
	var doc companyDoc
	err = c.db.Collection(colCompanies).FindOne(ctx, bson.M{"_id": cOID, "uid": uidOID, "is_active": true}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Company{}, false, nil
	}
	if err != nil {
		return store.Company{}, false, fmt.Errorf("finding active company: %w", err)
	}
	return doc.toStore(), true, nil
}

func (c *companies) Deactivate(ctx context.Context, uid, companyID store.ID) (store.DeactivateResult, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.DeactivateNotFound, err
	}
	cOID, err := objectID(companyID)
	if err != nil {
		return store.DeactivateNotFound, nil
	}
	active, err := c.db.Collection(colCompanies).CountDocuments(ctx, bson.M{"uid": uidOID, "is_active": true})
	if err != nil {
		return store.DeactivateNotFound, fmt.Errorf("counting active companies: %w", err)
	}
	if active <= 1 {
		return store.DeactivateMustKeepOne, nil
	}
	var doc companyDoc
	err = c.db.Collection(colCompanies).FindOne(ctx, bson.M{"_id": cOID, "uid": uidOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.DeactivateNotFound, nil
	}
	if err != nil {
		return store.DeactivateNotFound, fmt.Errorf("looking up company: %w", err)
	}
	if doc.IsDefault {
		return store.DeactivateIsDefault, nil
	}
	if _, err := c.db.Collection(colCompanies).UpdateOne(ctx, bson.M{"_id": cOID, "uid": uidOID}, bson.M{"$set": bson.M{"is_active": false}}); err != nil {
		return store.DeactivateNotFound, fmt.Errorf("deactivating company: %w", err)
	}
	if _, err := c.db.Collection(colCompanySessions).DeleteMany(ctx, bson.M{"company_id": cOID}); err != nil {
		return store.DeactivateNotFound, fmt.Errorf("clearing tab bindings: %w", err)
	}
	return store.DeactivateOK, nil
}

func (c *companies) SetQueueOrder(ctx context.Context, companyID store.ID, order []string) ([]string, bool, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return nil, false, err
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.M{"queueOrder": 1})
	var doc struct {
		QueueOrder []string `bson:"queueOrder"`
	}
	err = c.db.Collection(colCompanies).
		FindOneAndUpdate(ctx, bson.M{"_id": cOID}, bson.M{"$set": bson.M{"queueOrder": order}}, opts).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("set queue order: %w", err)
	}
	return doc.QueueOrder, true, nil
}

func (c *companies) Numbering(ctx context.Context, companyID, uid store.ID) (map[string]json.RawMessage, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Numbering bson.M `bson:"numbering"`
	}
	err = c.db.Collection(colCompanies).
		FindOne(ctx, bson.M{"_id": cOID, "uid": uidOID}, options.FindOne().SetProjection(bson.M{"numbering": 1})).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return map[string]json.RawMessage{}, nil // read never 404s; defaults fill the gaps
	}
	if err != nil {
		return nil, fmt.Errorf("read numbering: %w", err)
	}
	out := make(map[string]json.RawMessage, len(doc.Numbering))
	for k, v := range doc.Numbering {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encode numbering %q: %w", k, err)
		}
		out[k] = b
	}
	return out, nil
}

func (c *companies) SetNumbering(ctx context.Context, companyID, uid store.ID, kind string, format json.RawMessage) (bool, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	var v any
	if err := json.Unmarshal(format, &v); err != nil {
		return false, fmt.Errorf("decode numbering format: %w", err)
	}
	// kind is whitelisted by the handler (DEFAULT_FORMATS), so the dotted key is safe.
	res, err := c.db.Collection(colCompanies).UpdateOne(ctx,
		bson.M{"_id": cOID, "uid": uidOID}, bson.M{"$set": bson.M{"numbering." + kind: v}})
	if err != nil {
		return false, fmt.Errorf("set numbering: %w", err)
	}
	return res.MatchedCount > 0, nil
}

func (c *companies) SetReportsAcrossCompanies(ctx context.Context, companyID, uid store.ID, on bool) (bool, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	res, err := c.db.Collection(colCompanies).UpdateOne(ctx,
		bson.M{"_id": cOID, "uid": uidOID}, bson.M{"$set": bson.M{"sharing.reportsAcrossCompanies": on}})
	if err != nil {
		return false, fmt.Errorf("set sharing toggle: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// Scope is Helpers/CompanyScope.scopeFor. Fail-closed: any lookup trouble narrows to the acting
// company alone.
func (c *companies) Scope(ctx context.Context, companyID, uid store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return []store.ID{companyID}, false, map[store.ID]string{}, nil
	}
	var acting struct {
		Name    string             `bson:"name"`
		Firm    string             `bson:"firm"`
		UID     primitive.ObjectID `bson:"uid"`
		Sharing struct {
			ReportsAcrossCompanies bool `bson:"reportsAcrossCompanies"`
		} `bson:"sharing"`
	}
	err = c.db.Collection(colCompanies).
		FindOne(ctx, bson.M{"_id": cOID}, options.FindOne().SetProjection(bson.M{"name": 1, "firm": 1, "uid": 1, "sharing": 1})).
		Decode(&acting)
	if err != nil {
		// No company (or a read error): the acting company alone, unlabelled - fail closed.
		return []store.ID{companyID}, false, map[store.ID]string{}, nil
	}
	label := acting.Name
	if label == "" {
		label = acting.Firm
	}
	if !acting.Sharing.ReportsAcrossCompanies || acting.UID.IsZero() {
		return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
	}
	owned, err := c.ownedLabels(ctx, acting.UID)
	if err != nil || len(owned) == 0 {
		return []store.ID{companyID}, false, map[store.ID]string{companyID: label}, nil
	}
	ids := make([]store.ID, 0, len(owned))
	labels := make(map[store.ID]string, len(owned))
	has := false
	for id, l := range owned {
		ids = append(ids, id)
		labels[id] = l
		if id == companyID {
			has = true
		}
	}
	if !has {
		ids = append(ids, companyID)
		labels[companyID] = label
	}
	return ids, len(ids) > 1, labels, nil
}

// ownedLabels maps every company id the owner has to its display label.
func (c *companies) ownedLabels(ctx context.Context, uidOID primitive.ObjectID) (map[store.ID]string, error) {
	cur, err := c.db.Collection(colCompanies).
		Find(ctx, bson.M{"uid": uidOID}, options.Find().SetProjection(bson.M{"name": 1, "firm": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
		Firm string             `bson:"firm"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make(map[store.ID]string, len(docs))
	for _, d := range docs {
		l := d.Name
		if l == "" {
			l = d.Firm
		}
		out[idOf(d.ID)] = l
	}
	return out, nil
}
