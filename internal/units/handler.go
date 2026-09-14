// Package units is Configure > Products' unit list - the first domain with WRITES.
//
// Reads were the safe place to start. This is where the sideways phase gets its teeth: both
// services write into the same collection, so a Go route that normalises a name differently,
// or skips the pre-validate hook that maintains `key`, creates exactly the duplicate the
// unique index exists to prevent. Every rule below is copied from the Node model, not
// reinvented.
package units

import (
	"errors"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ Units store.Units }

func New(units store.Units) *Handler { return &Handler{Units: units} }

// unitResponse is the shape Mongoose serialises a Unit document into. __v travels because
// Mongoose includes it and the client receives it today; dropping it would be tidier and
// would also be a difference.
type unitResponse struct {
	ID        string     `json:"_id"`
	UID       string     `json:"uid"`
	CompanyID *string    `json:"company_id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"`
	CreatedAt httpx.Time `json:"createdAt"`
	UpdatedAt httpx.Time `json:"updatedAt"`
	V         int        `json:"__v"`
}

func toResponse(u store.Unit) unitResponse {
	out := unitResponse{
		ID:        u.ID.String(),
		UID:       u.UID.String(),
		Name:      u.Name,
		Key:       u.Key,
		CreatedAt: httpx.NewTime(u.CreatedAt),
		UpdatedAt: httpx.NewTime(u.UpdatedAt),
		V:         u.Version,
	}
	if u.CompanyID != "" {
		s := u.CompanyID.String()
		out.CompanyID = &s
	}
	return out
}

// List is POST /unit/list.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	// Seeded here as well as on the inventory report, because Configure > Products is where a
	// unit actually gets attached to a product - a company that opens the product form before
	// ever opening the report would otherwise be offered an empty select. Idempotent.
	//
	// A failed seed must NOT fail the list: the company keeps whatever units it has. Matching
	// the Node route, which swallows this error on purpose.
	if err := h.Units.SeedDefaults(ctx, sess.UID, sess.CompanyID); err != nil {
		httpx.LogSwallowed("seeding default units", err)
	}

	list, err := h.Units.List(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	out := make([]unitResponse, 0, len(list))
	for _, u := range list {
		out = append(out, toResponse(u))
	}
	// No `status` on this route - the Node handler sends {code, message, data}.
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// Create is POST /unit/create.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	form, err := httpx.ReadForm(r)
	if err != nil {
		httpx.Invalid(w, "")
		return
	}
	name := form.String("name")
	if name == "" {
		httpx.Invalid(w, "")
		return
	}

	created, err := h.Units.Create(ctx, sess.UID, sess.CompanyID, name)
	// The unique index on (company_id, key) fired: this company already has a unit whose name
	// normalises to the same thing - "SQ. Ft" against an existing "sq ft". A duplicate is a
	// thing to tell the user about, not a 500.
	if errors.Is(err, store.ErrDuplicate) {
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "That unit already exists.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Unit created.", Data: toResponse(created)})
}

// Update is POST /unit/update.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	form, err := httpx.ReadForm(r)
	if err != nil {
		httpx.Invalid(w, "")
		return
	}
	unitID, name := form.String("unit_id"), form.String("name")
	if unitID == "" || name == "" {
		httpx.Invalid(w, "")
		return
	}

	updated, err := h.Units.Rename(ctx, store.ID(unitID), sess.CompanyID, name)
	// A malformed id answers 500 here, not 404, because that is what the Node route answers:
	// Mongoose's CastError reaches its catch block. Verified against the running service, not
	// assumed - the first draft of this handler guessed 404 and was wrong.
	if errors.Is(err, store.ErrBadID) {
		httpx.Internal(w, err)
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Unit not found.", Status: httpx.False()})
		return
	}
	if errors.Is(err, store.ErrDuplicate) {
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "That unit already exists.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Unit updated.", Data: toResponse(updated)})
}

// Delete is POST /unit/delete.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	form, err := httpx.ReadForm(r)
	if err != nil {
		httpx.Invalid(w, "")
		return
	}
	unitID := form.String("unit_id")
	if unitID == "" {
		httpx.Invalid(w, "")
		return
	}

	// Deleting something that is not there answers 200, because findOneAndDelete on a missing
	// id is not an error in the Node route - a Go handler that 404'd would refuse a
	// double-click that Node accepts. A MALFORMED id is different: that is a CastError and a
	// 500 on both sides.
	if err := h.Units.Delete(ctx, store.ID(unitID), sess.CompanyID); err != nil && !errors.Is(err, store.ErrNotFound) {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Unit deleted.", Status: httpx.True()})
}
