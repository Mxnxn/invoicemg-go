package purchaseorder

import (
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/pochanges"
	"github.com/mxnxn/invoicemg-go/internal/posend"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type supplierDTO struct {
	ID      string `json:"_id"`
	Name    string `json:"name"`
	Firm    string `json:"firm"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Gst     string `json:"gst"`
}

type rowDTO struct {
	ID            string  `json:"_id"`
	Description   string  `json:"description"`
	Material      string  `json:"material"`
	Hsn           string  `json:"hsn"`
	Gst           float64 `json:"gst"`
	HasDimensions bool    `json:"hasDimensions"`
	Length        string  `json:"length"`
	Width         string  `json:"width"`
	Rate          float64 `json:"rate"`
	Qty           float64 `json:"qty"`
	Unit          string  `json:"unit"`
	Discount      float64 `json:"discount"`
	Charges       float64 `json:"charges"`
}

type approvalDTO struct {
	State          string      `json:"state"`
	ApprovedBy     *string     `json:"approvedBy"`
	ApprovedByName string      `json:"approvedByName"`
	ApprovedAt     *httpx.Time `json:"approvedAt"`
	Fingerprint    string      `json:"fingerprint"`
}

type poDTO struct {
	ID                string       `json:"_id"`
	UID               string       `json:"uid"`
	CompanyID         string       `json:"company_id"`
	SupplierID        *supplierDTO `json:"supplier_id"`
	PoNumber          string       `json:"poNumber"`
	Date              string       `json:"date"`
	Rows              []rowDTO     `json:"rows"`
	Total             float64      `json:"total"`
	Approval          approvalDTO  `json:"approval"`
	PurchaseInvoiceID *string      `json:"purchaseInvoice_id"`
	ConvertedAt       *httpx.Time  `json:"convertedAt"`
	CreatedAt         httpx.Time   `json:"createdAt"`
	UpdatedAt         httpx.Time   `json:"updatedAt"`
	Send              *posend.State `json:"send,omitempty"`
}

func tptr(t *time.Time) *httpx.Time {
	if t == nil {
		return nil
	}
	v := httpx.NewTime(*t)
	return &v
}

func strPtr(id store.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

func sendStateOf(po store.PurchaseOrder) posend.State {
	current := pochanges.SendFingerprint(toFingerprintPO(po.SupplierID, po.Date, po.Total, po.Rows))
	return posend.Derive(posend.Input{
		Count: po.Send.Count, StoredFP: po.Send.Fingerprint, CurrentFP: current,
		Approved: po.Approval.State == "approved", Converted: po.PurchaseInvoiceID != "",
		SentAt: po.Send.SentAt, ConfirmSentAt: po.Send.ConfirmSentAt,
	})
}

func toPODTO(po store.PurchaseOrder) poDTO {
	dto := poDTO{
		ID: string(po.ID), UID: string(po.UID), CompanyID: string(po.CompanyID),
		PoNumber: po.PoNumber, Date: po.Date, Total: po.Total,
		Approval: approvalDTO{
			State: po.Approval.State, ApprovedBy: strPtr(po.Approval.ApprovedBy),
			ApprovedByName: po.Approval.ApprovedByName, ApprovedAt: tptr(po.Approval.ApprovedAt),
			Fingerprint: po.Approval.Fingerprint,
		},
		PurchaseInvoiceID: strPtr(po.PurchaseInvoiceID), ConvertedAt: tptr(po.ConvertedAt),
		CreatedAt: httpx.NewTime(po.CreatedAt), UpdatedAt: httpx.NewTime(po.UpdatedAt),
		Rows: make([]rowDTO, 0, len(po.Rows)),
	}
	if po.Supplier != nil {
		dto.SupplierID = &supplierDTO{
			ID: string(po.Supplier.ID), Name: po.Supplier.Name, Firm: po.Supplier.Firm,
			Phone: po.Supplier.Phone, Address: po.Supplier.Address, Gst: po.Supplier.Gst,
		}
	}
	for _, r := range po.Rows {
		dto.Rows = append(dto.Rows, rowDTO{
			ID: string(r.ID), Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst,
			HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width, Rate: r.Rate, Qty: r.Qty,
			Unit: r.Unit, Discount: r.Discount, Charges: r.Charges,
		})
	}
	return dto
}

// NextNumber is POST /purchase-order/next-number.
func (h *Handler) NextNumber(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	numbers, err := h.store.Numbers(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	next := docnumber.Next(numbers, "PO-", poClock())
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{"poNumber": next}})
}

// List is POST /purchase-order/list (optional supplier_id filter). Each row carries its send state.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("supplier_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]poDTO, 0, len(list))
	for _, po := range list {
		dto := toPODTO(po)
		s := sendStateOf(po)
		dto.Send = &s
		out = append(out, dto)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: out})
}

type historyDTO struct {
	ID        string         `json:"_id"`
	POID      string         `json:"po_id"`
	ActorType string         `json:"actorType"`
	ActorID   *string        `json:"actorId"`
	ActorName string         `json:"actorName"`
	Action    string         `json:"action"`
	Changes   []store.Change `json:"changes"`
	Detail    string         `json:"detail"`
	CreatedAt httpx.Time     `json:"createdAt"`
}

type noteDTO struct {
	ID         string     `json:"_id"`
	POID       string     `json:"po_id"`
	AuthorType string     `json:"authorType"`
	AuthorID   *string    `json:"authorId"`
	AuthorName string     `json:"authorName"`
	Text       string     `json:"text"`
	CreatedAt  httpx.Time `json:"createdAt"`
	UpdatedAt  httpx.Time `json:"updatedAt"`
}

// Detail is POST /purchase-order/detail: one PO with its history, notes and send state.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	if poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	po, history, notes, found, err := h.store.Detail(r.Context(), sess.UID, sess.CompanyID, store.ID(poID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	s := sendStateOf(po)
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{
		"po": toPODTO(po), "history": toHistoryDTOs(history), "notes": toNoteDTOs(notes), "send": s,
	}})
}

func toHistoryDTOs(rows []store.POHistoryRow) []historyDTO {
	out := make([]historyDTO, 0, len(rows))
	for _, h := range rows {
		ch := h.Changes
		if ch == nil {
			ch = []store.Change{}
		}
		out = append(out, historyDTO{
			ID: string(h.ID), POID: string(h.POID), ActorType: h.ActorType, ActorID: strPtr(h.ActorID),
			ActorName: h.ActorName, Action: h.Action, Changes: ch, Detail: h.Detail, CreatedAt: httpx.NewTime(h.CreatedAt),
		})
	}
	return out
}

func toNoteDTOs(rows []store.PONote) []noteDTO {
	out := make([]noteDTO, 0, len(rows))
	for _, n := range rows {
		out = append(out, noteDTO{
			ID: string(n.ID), POID: string(n.POID), AuthorType: n.AuthorType, AuthorID: strPtr(n.AuthorID),
			AuthorName: n.AuthorName, Text: n.Text, CreatedAt: httpx.NewTime(n.CreatedAt), UpdatedAt: httpx.NewTime(n.UpdatedAt),
		})
	}
	return out
}

// Create is POST /purchase-order/create (requireCreate purchase_orders).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	supplierID := form.String("supplier_id")
	date := form.String("date")
	if supplierID == "" || date == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Supplier and date are required.", Status: httpx.False()})
		return
	}
	numbers, err := h.store.Numbers(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	poNumber := docnumber.Next(numbers, "PO-", poClock())
	rows := parseRows(form.String("rows"))
	po, err := h.store.Create(r.Context(), sess.UID, sess.CompanyID, poNumber, actorOf(r), store.POWrite{
		SupplierID: store.ID(supplierID), Date: date, Rows: rows, Total: computeTotal(rows),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase order created.", Status: httpx.True(), Data: toPODTO(po)})
}

// Update is POST /purchase-order/update (requireCreate purchase_orders).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	if poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	current, _, _, found, err := h.store.Detail(r.Context(), sess.UID, sess.CompanyID, store.ID(poID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	if current.PurchaseInvoiceID != "" {
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "This purchase order has already become a purchase invoice and cannot be edited.", Status: httpx.False()})
		return
	}

	upd := store.POUpdate{}
	after := store.PurchaseOrder{SupplierID: current.SupplierID, Date: current.Date, Rows: current.Rows}
	if form.Present("supplier_id") && form.String("supplier_id") != "" {
		upd.SetSupplier = true
		upd.SupplierID = store.ID(form.String("supplier_id"))
		after.SupplierID = upd.SupplierID
	}
	if form.Present("date") && form.String("date") != "" {
		upd.SetDate = true
		upd.Date = form.String("date")
		after.Date = upd.Date
	}
	if form.Present("rows") {
		upd.SetRows = true
		upd.Rows = parseRows(form.String("rows"))
		after.Rows = upd.Rows
	}
	after.Total = computeTotal(after.Rows)
	upd.Total = after.Total
	upd.NewFingerprint = pochanges.Fingerprint(toFingerprintPO(after.SupplierID, after.Date, after.Total, after.Rows))
	upd.Changes = toStoreChanges(pochanges.Diff(
		toFingerprintPO(current.SupplierID, current.Date, current.Total, current.Rows),
		toFingerprintPO(after.SupplierID, after.Date, after.Total, after.Rows),
	))

	po, res, err := h.store.Update(r.Context(), sess.UID, sess.CompanyID, store.ID(poID), actorOf(r), upd)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !res.Found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	if res.Converted {
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "This purchase order has already become a purchase invoice and cannot be edited.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase order updated.", Status: httpx.True(), Data: toPODTO(po)})
}
