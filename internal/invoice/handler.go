// Package invoice serves POST /invoice/getAll from routes/Invoice.js - the invoices list, each
// row carrying the issuer's letterhead, the client's details, the populated entries, and the
// computed totals. Behind the invoices feature. The other invoice routes (get,
// getClientInvoices, save, paid, remove, next-number, entries-jobs, getReceived, and the XLSX
// export/download) are ported across the sibling files in this package.
package invoice

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	invoices   store.Invoices
	companies  store.Companies
	users      store.Users
	clients    store.Clients
	exportsDir string
}

func New(i store.Invoices, c store.Companies, u store.Users, cl store.Clients, exportsDir string) *Handler {
	return &Handler{invoices: i, companies: c, users: u, clients: cl, exportsDir: exportsDir}
}

// List is POST /invoice/getAll. The message is "Successful!" (with the bang), exactly as Node
// sends it, and there is no `status` field.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	list, err := h.invoices.List(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// issuerFor: user + company letterhead, resolved once (both may be missing -> blank fields).
	user, _ := h.users.FindByID(ctx, sess.UID)
	var company store.Company
	if sess.CompanyID != "" {
		company, _ = h.companies.Active(ctx, sess.CompanyID, sess.UID)
	}

	out := make([]invoiceDTO, 0, len(list))
	for _, inv := range list {
		out = append(out, build(inv, user, company))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Successful!", Data: out})
}

func build(inv store.Invoice, user store.User, company store.Company) invoiceDTO {
	entries := make([]entryDTO, 0, len(inv.Entries))
	var notTaxAmount, computed float64
	for _, e := range inv.Entries {
		notTaxAmount += e.Amount
		// total falls back to the summed, rupee-rounded entry totals when the invoice carries
		// no stored totalAmount - Helpers/EntryTotals.entryTotal + RoundOffWithAmount.
		computed += entrymath.RoundOffWithAmount(entrymath.Total(entrymath.Entry{
			Amount: e.Amount, Discount: e.Discount, Charges: e.Charges,
			Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst,
		}))
		entries = append(entries, toEntryDTO(e))
	}
	total := computed
	if inv.TotalAmount != 0 {
		total = round2(inv.TotalAmount)
	}

	dto := invoiceDTO{
		ID: string(inv.ID), Date: inv.Date, InvoiceNumber: inv.InvoiceID,
		Account: company.AccountNo, BankName: company.BankName, Email: user.Email,
		Firm: company.Firm, Gst: company.Gst, IFSC: company.Ifsc, URL: company.URL,
		UpiQr: company.UpiQr, Name: user.Name, Phone: company.Phone, Address: company.Address,
		Entries:        entries,
		ReceivedAmount: inv.Amount,
		CreatedAt:      httpx.NewTime(inv.CreatedAt),
		NotTaxAmount:   notTaxAmount,
		Total:          total,
		Ready:          false,
		View:           false,
	}
	if inv.Client != nil {
		cid := string(inv.Client.ID)
		uid := string(inv.Client.UID)
		dto.ClientID = &cid
		dto.UID = &uid
		dto.ClientAddress = inv.Client.ClientAddress
		dto.ClientFirm = inv.Client.ClientFirm
		dto.ClientGST = inv.Client.ClientGST
		dto.ClientName = inv.Client.ClientName
		dto.ClientPhone = inv.Client.ClientPhone
	}
	return dto
}

// round2 is Math.round(n*100)/100, matching invoice.totalAmount.toFixed(2) read as a number.
func round2(n float64) float64 { return float64(int64(n*100+0.5)) / 100 }

func toEntryDTO(e store.Entry) entryDTO {
	hasDim := true
	if e.HasDimensions != nil {
		hasDim = *e.HasDimensions
	}
	return entryDTO{
		ID: string(e.ID), Description: e.Description, Material: e.Material, Hsn: e.Hsn,
		Rate: e.Rate, Qty: e.Qty, HasDimensions: hasDim, Length: e.Length, Width: e.Width,
		Date: e.Date, Amount: e.Amount, Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst,
		Discount: e.Discount, Charges: e.Charges, Advance: e.Advance, Total: e.Total,
		CreatedAt: httpx.NewTime(e.CreatedAt), UpdatedAt: httpx.NewTime(e.UpdatedAt), Version: e.Version,
	}
}

type invoiceDTO struct {
	ID             string     `json:"_id"`
	Date           string     `json:"date"`
	InvoiceNumber  string     `json:"invoiceNumber"`
	Account        string     `json:"account"`
	BankName       string     `json:"bank_name"`
	Email          string     `json:"email"`
	Firm           string     `json:"firm"`
	Gst            string     `json:"gst"`
	IFSC           string     `json:"ifsc"`
	URL            string     `json:"url"`
	UpiQr          string     `json:"upiQr"`
	Name           string     `json:"name"`
	Phone          string     `json:"phone"`
	Address        string     `json:"address"`
	Entries        []entryDTO `json:"entries"`
	ClientID       *string    `json:"client_id"`
	UID            *string    `json:"uid"`
	ClientAddress  string     `json:"clientAddress"`
	ClientFirm     string     `json:"clientFirm"`
	ClientGST      string     `json:"clientGST"`
	ClientName     string     `json:"clientName"`
	ClientPhone    string     `json:"clientPhone"`
	ReceivedAmount float64    `json:"receivedAmount"`
	CreatedAt      httpx.Time `json:"createdAt"`
	NotTaxAmount   float64    `json:"notTaxAmount"`
	Total          float64    `json:"total"`
	Ready          bool       `json:"ready"`
	View           bool       `json:"view"`
}

type entryDTO struct {
	ID            string     `json:"_id"`
	Description   string     `json:"description"`
	Material      string     `json:"material"`
	Hsn           string     `json:"hsn"`
	Rate          float64    `json:"rate"`
	Qty           float64    `json:"qty"`
	HasDimensions bool       `json:"hasDimensions"`
	Length        string     `json:"length"`
	Width         string     `json:"width"`
	Date          string     `json:"date"`
	Amount        float64    `json:"amount"`
	Cgst          float64    `json:"cgst"`
	Sgst          float64    `json:"sgst"`
	Igst          float64    `json:"igst"`
	Discount      float64    `json:"discount"`
	Charges       float64    `json:"charges"`
	Advance       float64    `json:"advance"`
	Total         float64    `json:"total"`
	CreatedAt     httpx.Time `json:"createdAt"`
	UpdatedAt     httpx.Time `json:"updatedAt"`
	Version       int        `json:"__v"`
}
