// Package store is the seam between this service and whatever is holding the data.
//
// This is the single most important file in the project, and it exists for one reason: the
// plan is Postgres, but the sideways phase forbids it. While the Node API is live, both
// services must read and write the SAME documents - a Go service writing Postgres while Node
// writes Mongo means the two disagree about your data the moment anyone creates a job. So the
// Mongo implementation is a temporary tenant behind these interfaces, and moving to SQL later
// is writing a second implementation of this file, not rewriting the handlers.
//
// The rule that keeps that promise: NOTHING outside internal/store/* may import the mongo
// driver. If a bson.M or a primitive.ObjectID appears in a handler, the seam has leaked and
// the Postgres swap stops being a swap.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is what a lookup returns when the thing is not there. Handlers translate it
// into whichever envelope the matching Node route sends - which is not always a 404, so the
// decision belongs to the handler, not here.
var ErrNotFound = errors.New("store: not found")

// ErrBadID is an identifier that is not shaped like one at all - "nonsense" where an ObjectID
// belongs.
//
// It is deliberately NOT ErrNotFound. Mongoose throws a CastError when it cannot cast a string
// to an ObjectID, which lands in each route's catch block, so the Node API answers 500 for a
// malformed id where a reasonable API would answer 404. That is a bug, and reproducing it is
// still right: while both services are live the same client must get the same answer from
// either, and a Go service that "fixed" it would make the pair inconsistent. Fix it in both at
// once, later, or not at all.
var ErrBadID = errors.New("store: malformed id")

// ErrDuplicate is a uniqueness constraint refusing a write - Mongo's E11000, and whatever
// Postgres raises later. Named here rather than leaked as a driver error precisely so the
// handler that turns it into "That unit already exists." survives the swap.
var ErrDuplicate = errors.New("store: already exists")

// ---------------------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------------------

// ID is an opaque identifier. A string, not an ObjectID, precisely so that a Postgres
// implementation returning an integer or a UUID needs no change above this line.
type ID string

func (id ID) String() string { return string(id) }

// Session is what a valid SESSION-TOKEN resolves to - the Go equivalent of the `req.auth`
// object Helpers/TokenHelper.js attaches.
type Session struct {
	UID         ID
	Role        string
	PersonID    ID
	Permissions []string
	SessionID   ID
	Token       string
	CompanyID   ID

	// IsActive and ExpiresAt are carried so the auth layer can apply exactly the rule the
	// Node helper applies, including retiring a session that has aged out.
	IsActive  bool
	ExpiresAt *time.Time
}

// Sessions resolves tokens and the per-tab company binding.
type Sessions interface {
	// FindByToken returns the session for a token, or ErrNotFound.
	FindByToken(ctx context.Context, token string) (Session, error)

	// Deactivate retires a session that has passed its expiry. TokenHelper does this on the
	// way past rather than merely refusing, so an aged-out token cannot be replayed.
	Deactivate(ctx context.Context, sessionID ID) error

	// ResolveCompany returns which company this tab is acting as, creating and persisting the
	// binding to the user's default company when the tab has none yet. Mirrors
	// resolveCompanyId in Helpers/TokenHelper.js exactly, including the persistence - without
	// it a reload moves the tab to a different company.
	ResolveCompany(ctx context.Context, token, tabID string, uid ID) (ID, error)
}

// ---------------------------------------------------------------------------------------
// Days and open work
// ---------------------------------------------------------------------------------------

// DayCount is one date that has work on it, with both counts the Node route sends.
//
// Jobs counts job-id DOCUMENTS; Cards counts the rows inside them. They are very different
// numbers on a day where one job-id carries six cards, and a caller printing one under the
// other's name is simply wrong - which is why both travel.
type DayCount struct {
	Date  string
	Jobs  int
	Cards int
}

// OpenJob is a job-id with at least one card short of Done.
type OpenJob struct {
	ID            ID
	ChallanNumber string
	ReceivedDate  string
	Total         float64
	Cards         int
	OpenCards     int
	ClientName    string
	ClientPhone   string
	HasClient     bool
}

// Days answers the questions the dashboard asks.
type Days interface {
	// CountsByDate groups job-ids by the date an admin put on them (receivedDate).
	CountsByDate(ctx context.Context, companyID ID) ([]DayCount, error)

	// SheetDates is every date a Sheet document exists for, with its id. Sheets no longer
	// define which days exist, but they still contribute dates - a day that only ever held
	// Entries must not disappear - and their ids remain a link target for older URLs.
	SheetDates(ctx context.Context, companyID ID) (map[string]ID, error)

	// OpenJobs lists job-ids carrying a card short of Done, oldest received date first.
	OpenJobs(ctx context.Context, companyID ID, limit int) ([]OpenJob, error)
}

// ---------------------------------------------------------------------------------------
// Units
// ---------------------------------------------------------------------------------------

// Unit is a measurement unit offered on purchase-invoice rows.
type Unit struct {
	ID        ID
	UID       ID
	CompanyID ID
	Name      string
	// Key is Name with case, spacing and punctuation removed, so "SQ. Ft", "sq ft" and
	// "SQ.FT" collapse to one value. It exists as its own field because the uniqueness has
	// to be enforced by an INDEX, and an index cannot normalise on the way in.
	Key       string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Version is Mongoose's __v. It travels because Mongoose includes it in what the client
	// receives today; omitting it would be tidier and would also be a difference.
	Version int
}

type Units interface {
	List(ctx context.Context, companyID ID) ([]Unit, error)
	Create(ctx context.Context, uid, companyID ID, name string) (Unit, error)
	Rename(ctx context.Context, unitID, companyID ID, name string) (Unit, error)
	Delete(ctx context.Context, unitID, companyID ID) error

	// SeedDefaults inserts the default units this company does not already have, matched by
	// NORMALISED name so a company that typed "sq ft" is not given a near-duplicate.
	// Idempotent.
	SeedDefaults(ctx context.Context, uid, companyID ID) error
}

// Store is everything together, so main wires one value rather than six.
type Store interface {
	Sessions() Sessions
	Days() Days
	Units() Units

	// Ping is what the health check uses: a service that is up but cannot reach its database
	// is not healthy, and a TCP check would call it healthy.
	Ping(ctx context.Context) error

	Close(ctx context.Context) error
}
