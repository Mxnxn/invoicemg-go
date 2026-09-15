// Package entry serves the /entry domain from routes/Entry.js: line items grouped into
// per-day sheets. add/update keep the sheet-of-day membership in step and snapshot the
// material's HSN at write time. /entry/remove is a soft-delete into Trash and is deferred.
package entry

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Entries }

func New(s store.Entries) *Handler { return &Handler{store: s} }

// addFields are the request keys /entry/add requires to be present and non-empty, exactly as
// the Node handler's truthy guard checks them (item_length/item_width map to length/width).
var addFields = []string{"client_id", "date", "material", "description", "item_length",
	"item_width", "qty", "rate", "amount", "cgst", "sgst", "total", "advance"}

func readWrite(f *httpx.Form) store.EntryWrite {
	return store.EntryWrite{
		ClientID:    store.ID(f.String("client_id")),
		Date:        f.String("date"),
		Material:    f.String("material"),
		Description: f.String("description"),
		Length:      f.String("item_length"),
		Width:       f.String("item_width"),
		Qty:         f.Float("qty", 0),
		Rate:        f.Float("rate", 0),
		Amount:      f.Float("amount", 0),
		Cgst:        f.Float("cgst", 0),
		Sgst:        f.Float("sgst", 0),
		// Node's /entry/add and /update never read igst; entries from this route carry 0.
		Igst:     0,
		Discount: f.Float("discount", 0),
		Charges:  f.Float("charges", 0),
		Total:    f.Float("total", 0),
		Advance:  f.Float("advance", 0),
	}
}

// Add is POST /entry/add (admin): create a line item, ensure its day-sheet exists, and return
// the entry with its client_id populated. Message "Entry saved successfully."
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if len(form.Missing(addFields...)) > 0 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	e, err := h.store.Add(r.Context(), sess.UID, sess.CompanyID, readWrite(form))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Entry saved successfully.", Data: EntryJSON(e)})
}

// Update is POST /entry/update (admin): rewrite a line item, moving it between day-sheets when
// its date changes. Node echoes the updated doc with status:false, which is preserved here.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("entry_id") || len(form.Missing(addFields...)) > 0 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	in := store.EntryUpdate{EntryID: store.ID(form.String("entry_id")), EntryWrite: readWrite(form)}
	e, found, err := h.store.Update(r.Context(), sess.UID, sess.CompanyID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		// Node dereferences a null updatedData only in the 500 catch; a missing id there yields
		// data:null with status:false, which this mirrors.
		httpx.Write(w, httpx.Envelope{Code: 200, Data: httpx.Null, Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Data: EntryJSON(e), Status: httpx.False()})
}

// Get is POST /entry/get (admin): one entry by id, scoped to the company.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("entry_id") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	e, found, err := h.store.Get(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("entry_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		// Node returns nothing (falls through) when the entry is absent; the client sees an
		// empty body. Match with data:null and no message.
		httpx.Write(w, httpx.Envelope{Code: 200, Data: httpx.Null})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: EntryJSON(e)})
}

// GetAll is POST /entry/getall (admin): every entry in the company.
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, EntryJSON(e))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}
