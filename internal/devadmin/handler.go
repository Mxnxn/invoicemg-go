// Package devadmin serves the superadmin operator console from routes/DevAdmin.js. Only the
// landing-enquiry views are ported so far (list + mark handled); everything under /dev is
// superadmin-gated.
package devadmin

import (
	"net/http"
	"sync"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	enquiries      store.Enquiries
	regTokens      store.RegistrationTokens
	passwordResets store.PasswordResetRequests
	users          store.Users
	sessions       store.Sessions
	devTotpSecret  string
	now            func() time.Time

	mu        sync.Mutex
	usedSteps map[int64]time.Time
	attempts  []time.Time
}

func New(e store.Enquiries, rt store.RegistrationTokens, pr store.PasswordResetRequests, u store.Users, s store.Sessions, devTotpSecret string) *Handler {
	return &Handler{
		enquiries: e, regTokens: rt, passwordResets: pr, users: u, sessions: s,
		devTotpSecret: devTotpSecret, now: time.Now, usedSteps: map[int64]time.Time{},
	}
}

type enquiryDTO struct {
	ID          string     `json:"_id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	CompanyName string     `json:"companyName"`
	Note        string     `json:"note"`
	Source      string     `json:"source"`
	UserAgent   string     `json:"userAgent"`
	Handled     bool       `json:"handled"`
	CreatedAt   httpx.Time `json:"createdAt"`
	UpdatedAt   httpx.Time `json:"updatedAt"`
	Version     int        `json:"__v"`
}

// Enquiries is POST /dev/enquiries: the newest landing enquiries (superadmin).
func (h *Handler) Enquiries(w http.ResponseWriter, r *http.Request) {
	list, err := h.enquiries.List(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]enquiryDTO, 0, len(list))
	for _, q := range list {
		out = append(out, enquiryDTO{
			ID: string(q.ID), Name: q.Name, Email: q.Email, Phone: q.Phone, CompanyName: q.CompanyName,
			Note: q.Note, Source: q.Source, UserAgent: q.UserAgent, Handled: q.Handled,
			CreatedAt: httpx.NewTime(q.CreatedAt), UpdatedAt: httpx.NewTime(q.UpdatedAt), Version: q.Version,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: out})
}

// EnquiriesHandled is POST /dev/enquiries/handled: flip an enquiry's handled flag (superadmin).
func (h *Handler) EnquiriesHandled(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	id := form.String("enquiry_id")
	if id == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	if err := h.enquiries.SetHandled(r.Context(), store.ID(id), form.String("handled") == "true"); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Updated."})
}
