package client

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// writeJSON encodes an arbitrary body over HTTP 200, for the write responses whose shape
// (a `status:true` on a 422, an `error` object) the {code,message,status} Envelope can't carry.
func writeJSON(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(body)
}

// dupBody is Node's duplicate-field 422: status:true and an error object naming the field.
func dupBody(dup store.Dup) map[string]any {
	if dup == store.DupPhone {
		return map[string]any{"code": 422, "message": "A client with this phone number already exists.", "status": true,
			"error": map[string]any{"client_phone": "This phone number is already registered to another client."}}
	}
	return map[string]any{"code": 422, "message": "A client with this GST number already exists.", "status": true,
		"error": map[string]any{"client_gst": "This GST number is already registered to another client."}}
}

func clientData(c store.Client) map[string]any {
	return map[string]any{
		"_id": string(c.ID), "client_id": c.LegacyID, "clientName": c.ClientName,
		"clientFirm": c.ClientFirm, "clientPhone": c.ClientPhone, "clientGST": c.ClientGST,
		"clientAddress": c.ClientAddress,
	}
}

// Add is POST /client/add: create a customer (company-scoped), with the same field validation,
// per-company GST/phone uniqueness, and optional "also a supplier" convenience as routes/Client.js.
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	uid := form.String("uid")
	in := readWrite(form)
	if uid == "" || in.ClientName == "" || in.ClientFirm == "" || in.ClientPhone == "" || in.ClientGST == "" || in.ClientAddress == "" {
		writeJSON(w, map[string]any{"code": 422, "message": "Invalid request", "status": true})
		return
	}
	if len(in.ClientPhone) != 10 {
		writeJSON(w, map[string]any{"code": 422, "message": "Missing parameter.", "status": true,
			"error": map[string]any{"client_phone": "client_phone is invalid."}})
		return
	}
	if len(in.ClientGST) != 15 {
		writeJSON(w, map[string]any{"code": 422, "message": "Missing parameter.", "status": true,
			"error": map[string]any{"client_gst": "client_gst is invalid."}})
		return
	}

	created, dup, err := h.store.Create(ctx, sess.CompanyID, store.ID(uid), time.Now().UnixMilli(), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup != store.DupNone {
		writeJSON(w, dupBody(dup))
		return
	}

	// "Also a supplier": best-effort - the client is already saved, so a failure here must not
	// fail the request. Only attempted when the flag is the string "true", as Node checks.
	supplierCreated := false
	if form.String("also_supplier") == "true" {
		if ok, serr := h.store.EnsureSupplier(ctx, store.ID(uid), in); serr == nil {
			supplierCreated = ok
		}
	}

	message := "Client has added."
	if supplierCreated {
		message = "Client added, and added as a supplier."
	}
	data := clientData(created)
	data["supplierCreated"] = supplierCreated
	writeJSON(w, map[string]any{"code": 200, "message": message, "data": data})
}

// Update is POST /client/update: edit a customer's fields (company-scoped), same uniqueness rules.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	clientID := form.String("client_id")
	in := readWrite(form)
	if clientID == "" || in.ClientName == "" || in.ClientFirm == "" || in.ClientPhone == "" || in.ClientGST == "" || in.ClientAddress == "" {
		writeJSON(w, map[string]any{"code": 422, "message": "Invalid request", "status": true})
		return
	}

	updated, dup, found, err := h.store.Update(ctx, sess.CompanyID, store.ID(clientID), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup != store.DupNone {
		writeJSON(w, dupBody(dup))
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}
	writeJSON(w, map[string]any{"code": 200, "message": "Operation successful.", "data": clientData(updated)})
}

// Remove is POST /client/remove: hard-delete a customer (company-scoped), matching Node's
// findOneAndDelete - this domain does NOT route deletes through Trash.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	clientID := form.String("client_id")
	if clientID == "" {
		writeJSON(w, map[string]any{"code": 422, "message": "Invalid request", "status": true})
		return
	}
	found, err := h.store.Delete(ctx, sess.CompanyID, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Client has removed.", Status: httpx.True()})
}

func readWrite(form *httpx.Form) store.ClientWrite {
	return store.ClientWrite{
		ClientName:    form.String("client_name"),
		ClientFirm:    form.String("client_firm"),
		ClientPhone:   form.String("client_phone"),
		ClientGST:     form.String("client_gst"),
		ClientAddress: form.String("client_address"),
	}
}
