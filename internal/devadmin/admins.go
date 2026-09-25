package devadmin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func tptr(t *time.Time) *httpx.Time {
	if t == nil {
		return nil
	}
	v := httpx.NewTime(*t)
	return &v
}

// parseDate accepts an ISO timestamp or a plain yyyy-mm-dd (what /admin/status may submit).
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// AdminStatus is POST /dev/admin/status (superadmin): activate/deactivate an account or set its
// expiry. Superadmins cannot be deactivated.
func (h *Handler) AdminStatus(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	uid := form.String("uid")
	if uid == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "uid is required.", Status: httpx.False()})
		return
	}
	var isActive *bool
	if form.Present("is_active") {
		v := form.String("is_active") == "true"
		isActive = &v
	}
	setActiveUntil := form.Present("activeUntil")
	var activeUntil *time.Time
	if setActiveUntil {
		if s := form.String("activeUntil"); s != "" {
			if t, err := parseDate(s); err == nil {
				activeUntil = &t
			}
		}
	}
	if isActive == nil && !setActiveUntil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Nothing to update.", Status: httpx.False()})
		return
	}

	target, err := h.users.FindByID(r.Context(), store.ID(uid))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such user.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if target.Role == "superadmin" {
		httpx.Write(w, httpx.Envelope{Code: 403, Message: "Superadmins cannot be deactivated.", Status: httpx.False()})
		return
	}

	updated, found, err := h.users.SetActive(r.Context(), store.ID(uid), isActive, activeUntil, setActiveUntil)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such user.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Updated.", Status: httpx.True(), Data: map[string]any{
		"name": updated.Name, "email": updated.Email, "is_active": updated.IsActive, "activeUntil": tptr(updated.ActiveUntil),
	}})
}

// AdminCompanyLimit is POST /dev/admin/company-limit (superadmin): raise/lower how many companies a
// user may own, never below what they already hold.
func (h *Handler) AdminCompanyLimit(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	uid := form.String("uid")
	if uid == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "uid is required.", Status: httpx.False()})
		return
	}
	limit, err := strconv.Atoi(form.String("companyLimit"))
	if err != nil || limit < 1 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "companyLimit must be 1 or more.", Status: httpx.False()})
		return
	}
	target, err := h.users.FindByID(r.Context(), store.ID(uid))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such user.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	owned, err := h.companies.Count(r.Context(), target.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if limit < owned {
		httpx.Write(w, httpx.Envelope{Code: 422, Status: httpx.False(),
			Message: target.Name + " already owns " + strconv.Itoa(owned) + " companies. The limit cannot be lower."})
		return
	}
	updated, found, err := h.users.SetCompanyLimit(r.Context(), store.ID(uid), limit)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such user.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Updated.", Status: httpx.True(), Data: map[string]any{
		"name": updated.Name, "email": updated.Email, "companyLimit": updated.CompanyLimit,
	}})
}

// Admins is POST /dev/admins (superadmin): every account with its company count, limit, last login
// and status. hasArchive is always false until the wipe/restore ops are ported.
func (h *Handler) Admins(w http.ResponseWriter, r *http.Request) {
	list, err := h.users.AdminList(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	type row struct {
		ID           string      `json:"_id"`
		Name         string      `json:"name"`
		Email        string      `json:"email"`
		Firm         string      `json:"firm"`
		Role         string      `json:"role"`
		IsActive     bool        `json:"is_active"`
		ActiveUntil  *httpx.Time `json:"activeUntil"`
		Companies    int         `json:"companies"`
		CompanyLimit int         `json:"companyLimit"`
		LastLoginAt  *httpx.Time `json:"lastLoginAt"`
		HasArchive   bool        `json:"hasArchive"`
	}
	out := make([]row, 0, len(list))
	for _, u := range list {
		companies, _ := h.companies.Count(r.Context(), u.ID)
		lastLogin, _ := h.sessions.LastLoginAt(r.Context(), u.ID)
		role := u.Role
		if role == "" {
			role = "admin"
		}
		limit := u.CompanyLimit
		if limit < 1 {
			limit = 1
		}
		out = append(out, row{
			ID: string(u.ID), Name: u.Name, Email: u.Email, Firm: u.Firm, Role: role,
			IsActive: u.IsActive, ActiveUntil: tptr(u.ActiveUntil), Companies: companies,
			CompanyLimit: limit, LastLoginAt: tptr(lastLogin), HasArchive: false,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: out})
}
