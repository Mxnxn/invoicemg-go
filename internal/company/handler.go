// Package company serves the company-profile reads from routes/Company.js - /company/list (the
// switcher) and /company/active (the letterhead). Writes (create/update/switch/numbering) are
// not ported. The nested config (exportTemplate, sharing) is returned at its defaults, which is
// correct for a profile that has not customised it and is what the shell needs to render.
package company

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	companies store.Companies
	users     store.Users
	sessions  store.Sessions
}

func New(c store.Companies, u store.Users, s store.Sessions) *Handler {
	return &Handler{companies: c, users: u, sessions: s}
}

// List is POST /company/list: the switcher's companies plus the add-control's limit flags.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.companies.List(r.Context(), sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// companyLimit rides along so the Add control can disable itself without a second request;
	// the create route still enforces it. Math.max(1, limit) like the Node route.
	limit := 1
	if user, err := h.users.FindByID(r.Context(), sess.UID); err == nil && user.CompanyLimit > limit {
		limit = user.CompanyLimit
	}

	companies := make([]companyDTO, 0, len(list))
	for _, c := range list {
		companies = append(companies, toCompanyDTO(c))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: listDTO{
		Companies:       companies,
		ActiveCompanyID: string(sess.CompanyID),
		CompanyLimit:    limit,
		CanAddCompany:   sess.Role == "superadmin" || len(list) < limit,
	}})
}

// Active is POST /company/active: the acting company, for the letterhead.
func (h *Handler) Active(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	if sess.CompanyID == "" {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No company found for this account.", Status: httpx.False()})
		return
	}
	c, err := h.companies.Active(r.Context(), sess.CompanyID, sess.UID)
	if err == store.ErrNotFound {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: toCompanyDTO(c)})
}

func toCompanyDTO(c store.Company) companyDTO {
	return companyDTO{
		ID: string(c.ID), Name: c.Name, Firm: c.Firm, Address: c.Address, Phone: c.Phone,
		Gst: c.Gst, URL: c.URL, UpiQr: c.UpiQr, AccountNo: c.AccountNo, Ifsc: c.Ifsc,
		BankName: c.BankName, IsDefault: c.IsDefault, IsActive: c.IsActive,
		// Defaults, since this config is not stored/edited here yet - the Mongoose schema
		// defaults for a company that never customised them.
		ExportTemplate: exportTemplateDTO{Logo: true, Firm: true, Address: true, Phone: true, Gst: true, ShowPeriod: true},
		Sharing:        sharingCfgDTO{ReportsAcrossCompanies: false},
	}
}

type listDTO struct {
	Companies       []companyDTO `json:"companies"`
	ActiveCompanyID string       `json:"active_company_id"`
	CompanyLimit    int          `json:"company_limit"`
	CanAddCompany   bool         `json:"can_add_company"`
}

type companyDTO struct {
	ID             string            `json:"_id"`
	Name           string            `json:"name"`
	Firm           string            `json:"firm"`
	Address        string            `json:"address"`
	Phone          string            `json:"phone"`
	Gst            string            `json:"gst"`
	URL            string            `json:"url"`
	UpiQr          string            `json:"upiQr"`
	AccountNo      string            `json:"account_no"`
	Ifsc           string            `json:"ifsc"`
	BankName       string            `json:"bank_name"`
	IsDefault      bool              `json:"is_default"`
	IsActive       bool              `json:"is_active"`
	ExportTemplate exportTemplateDTO `json:"exportTemplate"`
	Sharing        sharingCfgDTO     `json:"sharing"`
}

type exportTemplateDTO struct {
	Logo       bool `json:"logo"`
	Firm       bool `json:"firm"`
	Address    bool `json:"address"`
	Phone      bool `json:"phone"`
	Gst        bool `json:"gst"`
	ShowPeriod bool `json:"showPeriod"`
}

type sharingCfgDTO struct {
	ReportsAcrossCompanies bool `json:"reportsAcrossCompanies"`
}
