package company

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Create is POST /company/create (admin): add a company profile. The owner's first company
// becomes the default. Only `name` is required; firm falls back to name.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	name := form.String("name")
	if name == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Company name is required.", Status: httpx.False()})
		return
	}
	created, err := h.companies.Create(ctx, sess.UID, store.CompanyWrite{
		Name: name, Firm: form.String("firm"), Address: form.String("address"), Phone: form.String("phone"),
		Gst: form.String("gst"), AccountNo: form.String("account_no"), Ifsc: form.String("ifsc"), BankName: form.String("bank_name"),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Company created.", Data: toCompanyDTO(created)})
}

// Update is POST /company/update (admin): a partial edit of the owner's company profile.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	companyID := form.String("company_id")
	if companyID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	patch := store.CompanyPatch{}
	for _, f := range []struct {
		key string
		dst **string
	}{
		{"name", &patch.Name}, {"firm", &patch.Firm}, {"address", &patch.Address}, {"phone", &patch.Phone},
		{"gst", &patch.Gst}, {"account_no", &patch.AccountNo}, {"ifsc", &patch.Ifsc}, {"bank_name", &patch.BankName},
	} {
		if form.Present(f.key) {
			v := form.String(f.key)
			*f.dst = &v
		}
	}
	updated, found, err := h.companies.Update(ctx, sess.UID, store.ID(companyID), patch)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Company updated.", Data: toCompanyDTO(updated)})
}

// Switch is POST /company/switch: bind THIS tab (TAB-ID) to one of the owner's active companies.
// Any signed-in user may switch (employees work across profiles); only the ownership+active
// check gates it.
func (h *Handler) Switch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	companyID := form.String("company_id")
	if companyID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	tabID := r.Header.Get("TAB-ID")
	if tabID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "TAB-ID header is required to switch company.", Status: httpx.False()})
		return
	}
	c, found, err := h.companies.FindActive(ctx, sess.UID, store.ID(companyID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	if err := h.sessions.BindCompany(ctx, sess.Token, tabID, sess.UID, c.ID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Company switched.", Data: toCompanyDTO(c)})
}

// Deactivate is POST /company/deactivate (admin): retire a company (never delete - it owns
// invoices and entries). The owner must keep at least one active company and cannot deactivate
// the default without reassigning it first.
func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	companyID := form.String("company_id")
	if companyID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	res, err := h.companies.Deactivate(ctx, sess.UID, store.ID(companyID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch res {
	case store.DeactivateMustKeepOne:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "You must keep at least one active company.", Status: httpx.False()})
	case store.DeactivateNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
	case store.DeactivateIsDefault:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Set another company as default before deactivating this one.", Status: httpx.False()})
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Company deactivated.", Status: httpx.True()})
	}
}
