// Package trash serves the delete-vault routes from routes/Trash.js - a per-user password gate
// (setPassword) and the gated list (get). Behind the trash feature. /get carries a top-level
// `state` field (set-password / password-required / unlocked), so its responses are encoded
// directly rather than through the standard envelope.
package trash

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Trash }

func New(s store.Trash) *Handler { return &Handler{store: s} }

func writeJSON(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(body)
}

// SetPassword is POST /trash/setPassword - upserts the vault password (bcrypt).
func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	pw := form.String("password")
	if pw == "" {
		httpx.Invalid(w, "")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if err := h.store.SetPassword(r.Context(), sess.UID, string(hash)); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Password set successful.", Status: httpx.True()})
}

// Get is POST /trash/get - the state machine matching routes/Trash.js exactly.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	hash, err := h.store.PasswordHash(r.Context(), sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if hash == "" {
		writeJSON(w, map[string]any{"code": 200, "state": "set-password", "message": "Set a password first.", "status": true})
		return
	}
	if pw := form.String("password"); pw == "" {
		writeJSON(w, map[string]any{"code": 200, "state": "password-required", "message": "Password Require.", "status": true})
		return
	} else if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) != nil {
		writeJSON(w, map[string]any{"code": 422, "message": "Invalid password.", "status": false})
		return
	}

	list, err := h.store.List(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		hasDim := true
		if t.HasDimensions != nil {
			hasDim = *t.HasDimensions
		}
		row := map[string]any{
			"_id": string(t.ID), "description": t.Description, "material": t.Material, "rate": t.Rate,
			"qty": t.Qty, "hasDimensions": hasDim, "length": t.Length, "width": t.Width, "date": t.Date,
			"amount": t.Amount, "cgst": t.Cgst, "sgst": t.Sgst, "igst": t.Igst,
			"createdAt": httpx.NewTime(t.CreatedAt), "updatedAt": httpx.NewTime(t.UpdatedAt), "__v": t.Version,
		}
		if t.ClientID != "" {
			row["client_id"] = map[string]any{"_id": string(t.ClientID), "clientName": t.ClientName, "clientFirm": t.ClientFirm}
		} else {
			row["client_id"] = nil
		}
		out = append(out, row)
	}
	writeJSON(w, map[string]any{"code": 200, "state": "unlocked", "data": out, "message": "Successful."})
}
