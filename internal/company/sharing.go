package company

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
)

// Sharing is POST /company/sharing (admin): flip the acting company's cross-account reporting
// toggle (routes/Company.js). requireAdmin, not merely because it is a setting - turning it on
// widens what the /shared/* reports expose across the owner's companies, which is the owner's
// call, not an employee's.
//
// Only an explicit "true" turns it on. Absent, "false" or a typo all resolve to off, matching the
// fail-closed rule Companies.Scope follows. Echoes the company's sharing config, as Node returns
// company.sharing.
func (h *Handler) Sharing(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	if sess.CompanyID == "" {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No company found for this account.", Status: httpx.False()})
		return
	}
	form, _ := httpx.ReadForm(r)
	on := form.String("reportsAcrossCompanies") == "true"
	found, err := h.companies.SetReportsAcrossCompanies(r.Context(), sess.CompanyID, sess.UID, on)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{
		Code: 200, Message: "Operation successful.", Status: httpx.True(),
		Data: sharingCfgDTO{ReportsAcrossCompanies: on},
	})
}
