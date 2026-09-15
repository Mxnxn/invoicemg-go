package entry

import (
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// entryDTO renders one entry in the shape routes/Entry.js returns: the raw Mongo document,
// with client_id either a raw id string or - when the store populated it (entry/add) - the
// nested client object.
func entryDTO(e store.Entry) map[string]any {
	m := map[string]any{
		"_id":         string(e.ID),
		"uid":         idOrNil(e.UID),
		"company_id":  idOrNil(e.CompanyID),
		"description": e.Description,
		"material":    e.Material,
		"hsn":         e.Hsn,
		"rate":        e.Rate,
		"qty":         e.Qty,
		"length":      e.Length,
		"width":       e.Width,
		"date":        e.Date,
		"amount":      e.Amount,
		"cgst":        e.Cgst,
		"sgst":        e.Sgst,
		"igst":        e.Igst,
		"total":       e.Total,
		"advance":     e.Advance,
		"discount":    e.Discount,
		"charges":     e.Charges,
		"has_issued":  e.HasIssued,
		"createdAt":   httpx.NewTime(e.CreatedAt),
		"updatedAt":   httpx.NewTime(e.UpdatedAt),
		"__v":         e.Version,
	}
	if e.QuotationID != "" {
		m["quotation_id"] = string(e.QuotationID)
	} else {
		m["quotation_id"] = nil
	}
	if e.Client != nil {
		m["client_id"] = clientDTO(e.Client)
	} else {
		m["client_id"] = idOrNil(e.ClientID)
	}
	return m
}

// clientDTO is the populated client_id: the client's scalar fields under their Mongo names.
// Node's .populate returns the whole client document including its entries array; that array
// is intentionally omitted here (it does not exist relationally and no consumer reads it).
func clientDTO(c *store.EntryClient) map[string]any {
	return map[string]any{
		"_id":           string(c.ID),
		"company_id":    idOrNil(c.CompanyID),
		"uid":           idOrNil(c.UID),
		"client_id":     c.LegacyID,
		"clientName":    c.ClientName,
		"clientFirm":    c.ClientFirm,
		"clientPhone":   c.ClientPhone,
		"clientGST":     c.ClientGST,
		"clientAddress": c.ClientAddress,
		"createdAt":     httpx.NewTime(c.CreatedAt),
		"updatedAt":     httpx.NewTime(c.UpdatedAt),
	}
}

func idOrNil(id store.ID) any {
	if id == "" {
		return nil
	}
	return string(id)
}
