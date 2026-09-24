// Package sharing serves the per-record sharing writes from routes/Sharing.js: /sharing/preview
// and /sharing/set (Phase 2 of cross-account sharing). Which companies a single product or
// customer is shared with.
//
// Admin only, and scoped through ownedScope (company_id alone, never widened): a company can only
// re-share a record IT owns, so a borrowed product cannot be re-shared onward. Nothing here is
// destructive - un-sharing changes only what the other company may SELECT next and how its
// inventory report matches by name, because every document stores the product/customer as a string
// snapshot (Helpers/SharedRecords.js). That is why the guard is a named confirmation, not a
// migration.
package sharing

import (
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	companies store.Companies
	materials store.Materials
	clients   store.Clients
}

func New(co store.Companies, m store.Materials, cl store.Clients) *Handler {
	return &Handler{companies: co, materials: m, clients: cl}
}

// getter/setter resolve the record kind ("material" | "client") to the owned-scope store calls,
// so the two routes share one dispatch and an unknown kind is a single 422.
func (h *Handler) getter(kind string) (func(*http.Request, store.ID, store.ID) ([]store.ID, bool, error), bool) {
	switch kind {
	case "material":
		return func(r *http.Request, co, id store.ID) ([]store.ID, bool, error) {
			return h.materials.OwnedSharing(r.Context(), co, id)
		}, true
	case "client":
		return func(r *http.Request, co, id store.ID) ([]store.ID, bool, error) {
			return h.clients.OwnedSharing(r.Context(), co, id)
		}, true
	}
	return nil, false
}

func (h *Handler) setter(kind string) (func(*http.Request, store.ID, store.ID, []store.ID) ([]store.ID, bool, error), bool) {
	switch kind {
	case "material":
		return func(r *http.Request, co, id store.ID, comps []store.ID) ([]store.ID, bool, error) {
			return h.materials.SetSharing(r.Context(), co, id, comps)
		}, true
	case "client":
		return func(r *http.Request, co, id store.ID, comps []store.ID) ([]store.ID, bool, error) {
			return h.clients.SetSharing(r.Context(), co, id, comps)
		}, true
	}
	return nil, false
}

// Preview is POST /sharing/preview (admin): what the confirmation dialog needs before anything is
// written - who loses access to a record and their names. Applied only if the person confirms.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	kind := form.String("kind")
	recordID := form.String("record_id")
	get, ok := h.getter(kind)
	if !ok || recordID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	before, found, err := get(r, sess.CompanyID, store.ID(recordID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Not found, or not owned by this company.", Status: httpx.False()})
		return
	}

	after := parseCompanies(form.String("companies"))
	losing := companiesLosingAccess(before, after)

	// Name only the losing companies this admin actually owns, in losing order.
	labels, err := h.ownedLabels(r)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	losingRows := make([]losingCompany, 0, len(losing))
	for _, id := range losing {
		if name, owned := labels[id]; owned {
			losingRows = append(losingRows, losingCompany{ID: string(id), Name: name})
		}
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(),
		Data: map[string]any{
			// requiresConfirmation counts ALL losing ids, not just the owned ones, so a change that
			// removes access is always flagged (Node names it so the dialog can require it typed).
			"losing":               losingRows,
			"requiresConfirmation": len(losing) > 0,
		}})
}

// Set is POST /sharing/set (admin): replace a record's shared-with set. Only companies this admin
// owns survive the filter (a crafted request cannot share into someone else's company), and the
// owning company is always kept - a record un-shared from its own company would vanish from the
// list of the people who created it.
func (h *Handler) Set(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	kind := form.String("kind")
	recordID := form.String("record_id")
	set, ok := h.setter(kind)
	if !ok || recordID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}

	requested := parseCompanies(form.String("companies"))
	labels, err := h.ownedLabels(r)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	companies := make([]store.ID, 0, len(requested)+1)
	added := map[store.ID]struct{}{}
	for _, id := range requested {
		if _, owned := labels[id]; !owned {
			continue
		}
		if _, dup := added[id]; dup {
			continue
		}
		added[id] = struct{}{}
		companies = append(companies, id)
	}
	if _, has := added[sess.CompanyID]; !has {
		companies = append(companies, sess.CompanyID)
	}

	stored, found, err := set(r, sess.CompanyID, store.ID(recordID), companies)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Not found, or not owned by this company.", Status: httpx.False()})
		return
	}
	// Node echoes doc.sharing, which on a product/customer is just { companies }.
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Sharing updated.", Status: httpx.True(),
		Data: map[string]any{"companies": stored}})
}

type losingCompany struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

// ownedLabels maps each company this admin owns to its label (name || firm), for both the losing
// names and the owned-set filter.
func (h *Handler) ownedLabels(r *http.Request) (map[store.ID]string, error) {
	sess := auth.MustFrom(r.Context())
	owned, err := h.companies.List(r.Context(), sess.UID)
	if err != nil {
		return nil, err
	}
	labels := make(map[store.ID]string, len(owned))
	for _, c := range owned {
		name := c.Name
		if name == "" {
			name = c.Firm
		}
		labels[c.ID] = name
	}
	return labels, nil
}

// parseCompanies splits the comma-separated companies field, trimming and dropping blanks - the Go
// twin of Sharing.js parseCompanies.
func parseCompanies(raw string) []store.ID {
	out := make([]store.ID, 0)
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, store.ID(s))
		}
	}
	return out
}

// companiesLosingAccess is the ids in before but not after (Helpers/SharedRecords). Returns []
// when access is only added, so turning sharing ON never warns.
func companiesLosingAccess(before, after []store.ID) []store.ID {
	keeping := make(map[store.ID]struct{}, len(after))
	for _, id := range after {
		keeping[id] = struct{}{}
	}
	out := make([]store.ID, 0)
	for _, id := range before {
		if _, keep := keeping[id]; !keep {
			out = append(out, id)
		}
	}
	return out
}
