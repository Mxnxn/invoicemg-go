// Package userinfo serves POST /userinfo/get from routes/UserInfo.js - the admin profile the
// shell loads on mount (buildProfile), a merge of the user account and the acting company.
// Admin-only in Node; the shell only calls it for non-employee sessions.
package userinfo

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	users     store.Users
	companies store.Companies
}

func New(u store.Users, c store.Companies) *Handler { return &Handler{users: u, companies: c} }

// Get is POST /userinfo/get. buildProfile tolerates a missing user or company (Node uses
// ternaries), so a not-found on either just leaves its fields blank rather than erroring.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	user, _ := h.users.FindByID(r.Context(), sess.UID)

	var company store.Company
	if sess.CompanyID != "" {
		company, _ = h.companies.Active(r.Context(), sess.CompanyID, sess.UID)
	}

	role := "admin"
	if user.Role != "" {
		role = user.Role
	}
	var companyID *string
	if company.ID != "" {
		s := string(company.ID)
		companyID = &s
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: profileDTO{
		Name:        user.Name,
		Email:       user.Email,
		Role:        role,
		ActiveUntil: httpx.NewTimePtr(user.ActiveUntil),
		Phone:       company.Phone,
		Firm:        company.Firm,
		Address:     company.Address,
		Gst:         company.Gst,
		URL:         company.URL,
		UpiQr:       company.UpiQr,
		Account:     company.AccountNo,
		Ifsc:        company.Ifsc,
		BankName:    company.BankName,
		CompanyID:   companyID,
		// Not stored/edited here yet, so the Mongoose defaults - which is what an untouched
		// company renders with.
		InvoiceTemplate:   "classic",
		QuotationTemplate: "classic",
		LedgerTemplate:    "classic",
		DocumentFont:      "open-sans",
		DocumentScale:     "normal",
	}})
}

type profileDTO struct {
	Name              string      `json:"name"`
	Email             string      `json:"email"`
	Role              string      `json:"role"`
	ActiveUntil       *httpx.Time `json:"activeUntil"`
	Phone             string      `json:"phone"`
	Firm              string      `json:"firm"`
	Address           string      `json:"address"`
	Gst               string      `json:"gst"`
	URL               string      `json:"url"`
	UpiQr             string      `json:"upiQr"`
	Account           string      `json:"account"`
	Ifsc              string      `json:"ifsc"`
	BankName          string      `json:"bank_name"`
	CompanyID         *string     `json:"company_id"`
	InvoiceTemplate   string      `json:"invoiceTemplate"`
	QuotationTemplate string      `json:"quotationTemplate"`
	LedgerTemplate    string      `json:"ledgerTemplate"`
	DocumentFont      string      `json:"documentFont"`
	DocumentScale     string      `json:"documentScale"`
}
