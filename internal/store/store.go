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
	"encoding/json"
	"errors"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/pochanges"
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

	// BindCompany points this (token, tabID) tab at companyID (upsert), for /company/switch.
	BindCompany(ctx context.Context, token, tabID string, uid, companyID ID) error

	// DeactivateOthers retires every ACTIVE session of uid except the one holding keepToken -
	// what /user/password/change does so a password change signs out the other devices while
	// leaving the caller signed in.
	DeactivateOthers(ctx context.Context, uid ID, keepToken string) error
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

// Ordering convention (#19): every method that returns a list gives a TOTAL order, so the
// same query cannot return the same rows in two arrangements - parity.js compares array
// order, and a LIMIT makes the boundary set itself turn on ties. The mongostore keeps Node's
// exact sort and relies on Mongo's natural order for ties, which is what makes it match the
// live Node service; the sqlstore makes that tiebreak explicit with a trailing id, because
// Postgres leaves ties arbitrary. ids sort ascending by creation time in both stores
// (ObjectID hex, and ULIDs for new rows), so the two agree. Handlers never re-sort a list the
// store already ordered; where one must sort in memory it uses a stable sort.
//
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

// ---------------------------------------------------------------------------------------
// Users and signing in
// ---------------------------------------------------------------------------------------

// User is an account, as far as signing in is concerned.
type User struct {
	ID           ID
	Email        string
	PasswordHash string
	Name         string
	Firm         string
	Role         string
	// CompanyLimit is how many company profiles this admin may create (the switcher's cap).
	CompanyLimit int
	// ActiveUntil is what actually locks a login out. nil means no expiry.
	ActiveUntil *time.Time
	TotpEnabled bool
	TotpSecret  string
}

// NewSession is a session about to be written. Separate from Session because the caller
// supplies some fields and the store supplies the rest.
type NewSession struct {
	Token       string
	UID         ID
	Role        string
	PersonID    ID
	Permissions []string
	Remembered  bool
	ExpiresAt   *time.Time
}

type Users interface {
	// FindByEmail returns the account for a NORMALISED email, or ErrNotFound. Normalisation
	// belongs to the caller so both backends cannot disagree about what "the same address"
	// means.
	FindByEmail(ctx context.Context, email string) (User, error)
	// FindByID returns the account by id, for the profile the shell loads. ErrNotFound if gone.
	FindByID(ctx context.Context, uid ID) (User, error)
	// UpdateProfile sets a user's editable account fields (email, name) for /userinfo/update;
	// found is false when no such user.
	UpdateProfile(ctx context.Context, uid ID, email, name string) (User, bool, error)
	CreateSession(ctx context.Context, s NewSession) (Session, error)
	// UpdatePassword replaces a user's bcrypt hash (for /user/password/change). found is false
	// when no such user. The caller has already verified the current password.
	UpdatePassword(ctx context.Context, uid ID, passwordHash string) (found bool, err error)
	// Register creates a user (self-registration, /user/register). dup is true when the email is
	// already taken; nothing is written then.
	Register(ctx context.Context, email, passwordHash, name string, activeUntil time.Time) (uid ID, dup bool, err error)
	// SetTotpSecret stores a (not-yet-enabled) TOTP secret during enrolment (/user/totp/setup).
	SetTotpSecret(ctx context.Context, uid ID, secret string) error
	// SetTotpEnabled flips the enabled flag (/user/totp/enable).
	SetTotpEnabled(ctx context.Context, uid ID, enabled bool) error
	// ClearTotp turns 2FA off and forgets the secret (/user/totp/disable).
	ClearTotp(ctx context.Context, uid ID) error
}

// ---------------------------------------------------------------------------------------
// Alert - the public customer link (routes/Alert.js)
// ---------------------------------------------------------------------------------------

// AlertJob is one job and its rows, for the page a customer opens from a WhatsApp link. It
// carries only this job: never another job, never an account balance - see routes/Alert.js.
type AlertJob struct {
	ID            ID
	ChallanNumber string
	ReceivedDate  string
	Queue         string
	ClientID      ID
	CompanyID     ID
	UID           ID
	// CreatedAt is the fallback receivedDate for a job raised before that field existed, the
	// same substitution Model/Job.js's toJSON makes. nil when unknown.
	CreatedAt *time.Time
	Rows      []AlertRow
}

// AlertRow is one line item. The pricing fields feed jobmath; the rest are shown as-is.
// HasDimensions is a pointer because absent means by-dimension (a pre-existing row), not false.
type AlertRow struct {
	ID            ID
	RowID         string
	Description   string
	Material      string
	Qty           float64
	HasDimensions *bool
	Length        string
	Width         string
	Rate          float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Charges       float64
	Queue         string
	Progress      string
}

// AlertClient is the customer's display names, the only client fields the page shows.
type AlertClient struct {
	Name string
	Firm string
}

// AlertCompany is the letterhead - the lines every document in this app prints.
type AlertCompany struct {
	Name    string
	Firm    string
	Phone   string
	URL     string
	Address string
	Gst     string
}

// ReviewScores is the five 1-5 ratings a customer leaves, from Model/JobReview.js. overall is
// the customer's own summary, deliberately not the mean of the other four.
type ReviewScores struct {
	Quality       int
	Speed         int
	Communication int
	Satisfaction  int
	Overall       int
}

// AlertReview is a job's review as the page reads it on load - the projected fields
// (_id/scores/comment/createdAt), no __v, matching Mongoose's inclusion projection.
type AlertReview struct {
	ID        ID
	Scores    ReviewScores
	Comment   string
	CreatedAt time.Time
}

// NewReview is a customer review about to be written. CompanyID may be empty, in which case the
// store falls back to the owner's default company - the same resolution routes/Alert.js does for
// a job raised before company stamping.
type NewReview struct {
	JobID         ID
	UID           ID
	CompanyID     ID
	ClientID      ID
	JobcardID     string
	ChallanNumber string
	ClientName    string
	Scores        ReviewScores
	Comment       string
}

// Alerts serves the public customer link. Job returns ErrBadID for a non-id and ErrNotFound
// for an unknown one; Client and Company return ErrNotFound (which the handler renders as
// blank, matching Node's optional chaining) for an empty or unknown id, because a job's
// company_id is nullable and a missing party is normal here, not an error.
type Alerts interface {
	Job(ctx context.Context, id ID) (AlertJob, error)
	Client(ctx context.Context, id ID) (AlertClient, error)
	Company(ctx context.Context, id ID) (AlertCompany, error)
	// Review returns the job's review, or ErrNotFound when it has none (the page's reviewed:false).
	Review(ctx context.Context, jobID ID) (AlertReview, error)
	// CreateReview writes one review. ErrDuplicate if the job already has one (the unique
	// index is the real guarantee, since the page is unauthenticated and a refresh is one
	// tap away); ErrNotFound if no company can be resolved for the owner.
	CreateReview(ctx context.Context, r NewReview) error
}

// ---------------------------------------------------------------------------------------
// Banks
// ---------------------------------------------------------------------------------------

// Bank is a bank account, returned whole - the Node /bank/list sends the entire record, __v
// and timestamps included, so all of it travels.
type Bank struct {
	ID             ID
	UID            ID
	CompanyID      ID
	Name           string
	OpeningBalance float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int
}

type Banks interface {
	// List returns a company's bank accounts in name order.
	List(ctx context.Context, companyID ID) ([]Bank, error)
	// Create inserts a company-scoped bank account (name only; openingBalance defaults to 0),
	// owned by uid, and returns the stored record.
	Create(ctx context.Context, companyID, uid ID, name string) (Bank, error)
	// Update patches a company-owned bank's name and, ONLY when openingBalance is non-nil, its
	// opening balance - Node writes the balance only when the field was submitted, so a
	// name-only edit cannot wipe a balance. found is false when no such bank exists for the
	// company. A malformed id is ErrBadID on the document store (Node's CastError -> 500); the
	// relational store has text ids and simply finds nothing (found=false -> 404).
	Update(ctx context.Context, companyID, bankID ID, name string, openingBalance *float64) (bank Bank, found bool, err error)
	// Remove deletes a company-owned bank only when nothing references it. inUse is the count of
	// receipts + supplier payments + expenses pointing at the bank; when it is > 0 the bank is
	// kept (found is not meaningful) so the handler can refuse with that exact count. When inUse
	// is 0, found reports whether a bank was actually deleted (false -> 404).
	Remove(ctx context.Context, companyID, bankID ID) (inUse int, found bool, err error)
	// ReportData gathers the company's cash-moving rows (batch receipts, supplier payments,
	// expenses) plus its banks' opening balances, for the Bank Report (/bank/report). The pure
	// bankledger.Compute turns these into per-bank ledgers. InvoiceReceived is deliberately not
	// included - the report tracks transfers and expenses, not invoice settlement.
	ReportData(ctx context.Context, companyID ID) (BankReportData, error)
}

// BankTxn is one cash-moving row for the Bank Report (its sign is applied by bankledger).
type BankTxn struct {
	BankID ID
	Date   string
	Amount float64
	Note   string
}

// BankOpening is a bank's carried-in balance for the Bank Report.
type BankOpening struct {
	ID             ID
	Name           string
	OpeningBalance float64
}

// BankReportData is the raw input to bankledger.Compute for one company.
type BankReportData struct {
	BatchReceives    []BankTxn
	SupplierPayments []BankTxn
	Expenses         []BankTxn
	Banks            []BankOpening
}

// ---------------------------------------------------------------------------------------
// Clients
// ---------------------------------------------------------------------------------------

// Client is a customer as the list reads it. Sharing is a POINTER because nil and empty differ
// on the wire: Node omits the `sharing` key for a legacy record that never had the field, and
// sends {companies:[]} for a stamped-but-unshared one. CompanyID is "" for a legacy row whose
// company_id is null.
type Client struct {
	ID             ID
	UID            ID
	CompanyID      ID
	LegacyID       any // Client.client_id: Node stores it as a String, Postgres as bigint; echoed as stored
	ClientName     string
	ClientFirm     string
	ClientPhone    string
	ClientGST      string
	ClientAddress  string
	OpeningBalance float64
	Sharing        *[]ID
}

// ClientEntryView is one of a client's entries as /client/get populates it: the entry, plus
// its issued-invoice link (invoiceId is the human number) and its quotation link. On the
// relational store there is no entries.quotation_id column yet, so Quotation* stay empty there.
type ClientEntryView struct {
	Entry
	IssuedID        ID
	IssuedInvoiceID string
	QuotationID     ID
	QuotationNumber string
}

// ClientDetail is the /client/get payload: a client's raw fields with its entries populated
// (newest first). batchUpdates are attached by the handler from BatchReceives.
type ClientDetail struct {
	ID            ID
	UID           ID
	CompanyID     ID
	LegacyID      any
	ClientName    string
	ClientFirm    string
	ClientPhone   string
	ClientGST     string
	ClientAddress string
	Entries       []ClientEntryView
}

// ClientWrite is the writable field set of /client/add and /client/update.
type ClientWrite struct {
	ClientName    string
	ClientFirm    string
	ClientPhone   string
	ClientGST     string
	ClientAddress string
}

// Dup names the field a client uniqueness check tripped on: "" (none), "client_gst", or
// "client_phone". GST is checked before phone, matching routes/Client.js.
type Dup string

const (
	DupNone  Dup = ""
	DupGST   Dup = "client_gst"
	DupPhone Dup = "client_phone"
)

type Clients interface {
	// Visible returns the clients (uid) may READ from company companyID: its own, legacy rows
	// with a null company, and rows shared with it. This is the read side of #1 - reads widen;
	// a write path uses company-only scope and never calls this.
	Visible(ctx context.Context, uid, companyID ID) ([]Client, error)
	// Create inserts a company-scoped client, stamping LegacyID with the given millis. A
	// per-company GST or phone collision inserts nothing and returns the tripped Dup.
	Create(ctx context.Context, companyID, uid ID, legacyID int64, in ClientWrite) (Client, Dup, error)
	// Update mutates a company-scoped client's writable fields. found is false when no such row;
	// a collision with a DIFFERENT row returns the tripped Dup and writes nothing.
	Update(ctx context.Context, companyID, clientID ID, in ClientWrite) (c Client, dup Dup, found bool, err error)
	// Delete hard-deletes a company-scoped client (Node's findOneAndDelete). found is false on a miss.
	Delete(ctx context.Context, companyID, clientID ID) (found bool, err error)
	// EnsureSupplier creates a Supplier person for uid from these client details, unless one
	// already matches on GST (or on phone when the GST is empty). Reports whether it created one.
	EnsureSupplier(ctx context.Context, uid ID, in ClientWrite) (created bool, err error)
	// Get is /client/get: one company-scoped client with its entries populated (issued invoice
	// number and quotation number); found is false on a miss or another company's row.
	Get(ctx context.Context, companyID, clientID ID) (ClientDetail, bool, error)
	// SharedList returns the projected customers across a set of companies (firm-sorted), for the
	// /shared/customers report. Read-only and caller-scoped to the owner's companies.
	SharedList(ctx context.Context, companyIDs []ID) ([]SharedClient, error)
	// OwnedSharing returns a company-OWNED customer's sharing.companies list (ownedScope, never
	// widened - Helpers/SharedRecords.ownedScope). found is false on a miss or another company's
	// row. For /sharing/preview.
	OwnedSharing(ctx context.Context, companyID, clientID ID) (companies []ID, found bool, err error)
	// SetSharing replaces a company-OWNED customer's sharing.companies (ownedScope) and returns the
	// stored list. found is false on a miss. For /sharing/set.
	SetSharing(ctx context.Context, companyID, clientID ID, companies []ID) (stored []ID, found bool, err error)
	// SetNotifyPreference sets ONE tri-state notify flag (field is "notifyOnCreate" or
	// "notifyOnUpdate"; value nil clears to null "ask each time") on a company-scoped client and
	// returns BOTH flags as stored. found is false on a miss. field is whitelisted by the store.
	SetNotifyPreference(ctx context.Context, companyID, clientID ID, field string, value *bool) (onCreate, onUpdate *bool, found bool, err error)
	// NotifyPreferences lists a company's clients with their notify flags, firm-sorted. When
	// answeredOnly is true (the route's default) only clients who have answered EITHER flag are
	// returned; false returns every client (the "choose an answer" tab).
	NotifyPreferences(ctx context.Context, companyID ID, answeredOnly bool) ([]ClientNotify, error)
}

// SharedClient is the /shared/customers projection: a customer plus its owning company id (the
// handler adds the company's label).
type SharedClient struct {
	ID          ID
	ClientName  string
	ClientFirm  string
	ClientPhone string
	ClientGST   string
	CompanyID   ID
}

// SharedMaterial is the /shared/materials projection: a product plus its owning company id.
type SharedMaterial struct {
	ID           ID
	MaterialName string
	MaterialRate float64
	PurchaseRate float64
	Hsn          string
	Unit         string
	CompanyID    ID
}

// MaterialDuplicate is the /shared/duplicate-materials source row: a product plus how many
// companies it is shared with (SharedWith = len(sharing.companies)). The handler groups these by
// InventoryMath.normaliseKey to surface the same product entered separately in two companies.
type MaterialDuplicate struct {
	ID           ID
	MaterialName string
	MaterialRate float64
	PurchaseRate float64
	Hsn          string
	Unit         string
	CompanyID    ID
	SharedWith   int
}

// ClientNotify is the /client/notify-preferences projection: a customer plus their two tri-state
// notify flags. The flags are pointers so nil (unanswered) is distinct from &false on the wire.
type ClientNotify struct {
	ID             ID
	ClientName     string
	ClientFirm     string
	ClientPhone    string
	NotifyOnCreate *bool
	NotifyOnUpdate *bool
}

// ---------------------------------------------------------------------------------------
// Companies
// ---------------------------------------------------------------------------------------

// Company is one business profile - the PUBLIC_FIELDS routes/Company.js exposes to the shell.
// The nested config (exportTemplate, sharing, numbering, whatsapp) is not stored here yet; the
// handler fills those with defaults, which is correct for a company that has not customised
// them and is enough for the shell to render.
type Company struct {
	ID        ID
	Name      string
	Firm      string
	Address   string
	Phone     string
	Gst       string
	URL       string
	UpiQr     string
	AccountNo string
	Ifsc      string
	BankName  string
	// Per-company PDF template choices (default "classic") and WhatsApp Cloud API credentials.
	InvoiceTemplate     string
	QuotationTemplate   string
	LedgerTemplate      string
	WaPhoneNumberID     string
	WaBusinessAccountID string
	WaAPIToken          string
	// Whether the invoice prints each line's unit of measure and/or breaks the size out into its
	// own column. Off by default so existing paperwork is unchanged.
	DocumentShowUnits bool
	DocumentShowSize  bool
	IsDefault           bool
	IsActive            bool
}

// CompanyWrite is the field set of /company/create.
type CompanyWrite struct {
	Name      string
	Firm      string
	Address   string
	Phone     string
	Gst       string
	AccountNo string
	Ifsc      string
	BankName  string
}

// CompanyPatch is /company/update's partial edit: a nil field is not submitted (left as-is).
type CompanyPatch struct {
	Name                *string
	Firm                *string
	Address             *string
	Phone               *string
	Gst                 *string
	URL                 *string
	AccountNo           *string
	Ifsc                *string
	BankName            *string
	InvoiceTemplate     *string
	QuotationTemplate   *string
	LedgerTemplate      *string
	WaPhoneNumberID     *string
	WaBusinessAccountID *string
	WaAPIToken          *string
	// Invoice display toggles - a nil means the field was not submitted. Booleans can be turned
	// OFF, so the handler reads presence (not truthiness) before pointing these at a value.
	DocumentShowUnits *bool
	DocumentShowSize  *bool
}

// DeactivateResult is the outcome of /company/deactivate's guard rules.
type DeactivateResult int

const (
	DeactivateOK          DeactivateResult = iota // deactivated
	DeactivateNotFound                            // no such company for this owner
	DeactivateMustKeepOne                         // this is the owner's last active company
	DeactivateIsDefault                           // the default must be reassigned first
)

type Companies interface {
	// List returns the admin's ACTIVE companies for the switcher, default first.
	List(ctx context.Context, uid ID) ([]Company, error)
	// Active returns one company the admin owns, for the letterhead. ErrNotFound if missing.
	Active(ctx context.Context, companyID, uid ID) (Company, error)
	// Count returns how many companies the owner has (any state), for the first-company default.
	Count(ctx context.Context, uid ID) (int, error)
	// Create inserts a company owned by uid; the owner's first company becomes the default.
	Create(ctx context.Context, uid ID, in CompanyWrite) (Company, error)
	// Update applies a partial patch to an owner-scoped company; found is false on a miss.
	Update(ctx context.Context, uid, companyID ID, patch CompanyPatch) (c Company, found bool, err error)
	// FindActive returns an owner-scoped ACTIVE company (for /company/switch). found is false on
	// a miss or an inactive company.
	FindActive(ctx context.Context, uid, companyID ID) (c Company, found bool, err error)
	// Deactivate flips is_active off on an owner-scoped company, enforcing the keep-one and
	// not-the-default guards, and clears any tab bindings that pointed at it.
	Deactivate(ctx context.Context, uid, companyID ID) (DeactivateResult, error)
	// SetQueueOrder saves the company's default job queue pipeline (/jobs/queue/default). Scoped
	// by company id alone (Node does not add uid here); found is false on a miss. Returns the
	// stored order.
	SetQueueOrder(ctx context.Context, companyID ID, order []string) (stored []string, found bool, err error)
	// Numbering returns the owner-scoped company's stored per-kind numbering formats as raw JSON
	// (empty map when none set). /company/numbering fills the gaps from DEFAULT_FORMATS itself, so
	// this never 404s - a missing company just yields an empty map.
	Numbering(ctx context.Context, companyID, uid ID) (map[string]json.RawMessage, error)
	// SetNumbering upserts ONE kind's format on the owner-scoped company (/numbering/update).
	// found is false on a miss (-> 404).
	SetNumbering(ctx context.Context, companyID, uid ID, kind string, format json.RawMessage) (found bool, err error)
	// SetReportsAcrossCompanies flips the cross-account reporting toggle on the owner-scoped
	// company (/company/sharing). found is false on a miss.
	SetReportsAcrossCompanies(ctx context.Context, companyID, uid ID, on bool) (found bool, err error)
	// Scope resolves which companies a READ may span (Helpers/CompanyScope.scopeFor): the acting
	// company alone, unless its reportsAcrossCompanies toggle is on, in which case every company
	// the same owner has (uid-bounded, fail-closed). Returns the ids, whether it widened, and a
	// companyID->label map for rendering each row's origin.
	Scope(ctx context.Context, companyID, uid ID) (companyIDs []ID, shared bool, labels map[ID]string, err error)
}

// ProductionCard is one work card for the WIP panel: a job's row, or the whole job when it
// predates per-row tracking (Key ""). Key is the per-job grouping key (rowId, or the row's id when
// rowId is blank).
type ProductionCard struct {
	JobID      ID
	Key        string
	Queue      string
	EmployeeID ID
	CreatedAt  time.Time
}

// ProductionEvent is a "Queue advanced" job-history row (structured fields where present, else the
// display detail string).
type ProductionEvent struct {
	JobID     ID
	Detail    string
	FromStage string
	ToStage   string
	RowKey    string
	ActorName string
	CreatedAt time.Time
}

// ProductionWipData is the raw input to /analytics/production/wip.
type ProductionWipData struct {
	Cards       []ProductionCard
	Events      []ProductionEvent
	PersonNames map[ID]string
}

// ProductionDoneJob is a job's create/finish span plus its rows' queues, for cycle time.
type ProductionDoneJob struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Queue     string
	RowQueues []string
}

// ProductionThroughputData is the raw input to /analytics/production/throughput.
type ProductionThroughputData struct {
	Events   []ProductionEvent
	DoneJobs []ProductionDoneJob
}

// ---------------------------------------------------------------------------------------
// Materials (products)
// ---------------------------------------------------------------------------------------

// PriceHistoryEntry is one rate change on a product.
type PriceHistoryEntry struct {
	MaterialRate float64
	PurchaseRate float64
	ChangedAt    time.Time
}

// Material is a product as the list reads it - the whole record, like Node's lean() getall.
// Sharing is a *[]ID for the same nil-vs-empty reason as Client.
type Material struct {
	ID           ID
	UID          ID
	CompanyID    ID
	MaterialName string
	MaterialRate float64
	PurchaseRate float64
	Unit         string
	Hsn          string
	Tax          float64
	Sharing      *[]ID
	PriceHistory []PriceHistoryEntry
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

// MaterialWrite is the writable field set of /material/add and /material/update.
type MaterialWrite struct {
	MaterialName string
	MaterialRate float64
	PurchaseRate float64
	Hsn          string
	Tax          float64
}

type Materials interface {
	// Visible returns the products company companyID may READ: its own plus any shared with it.
	// The read half of #1 (Helpers/SharedRecords.visibleScope) - a write path would not widen.
	Visible(ctx context.Context, companyID ID) ([]Material, error)
	// Create inserts a company-scoped product owned by uid, returning the stored record.
	Create(ctx context.Context, companyID, uid ID, in MaterialWrite) (Material, error)
	// Update mutates a company-scoped product; found is false on a miss. When either rate
	// changes it appends a PriceHistory entry holding the OLD rates (routes/Material.js).
	Update(ctx context.Context, companyID, materialID ID, in MaterialWrite) (m Material, found bool, err error)
	// Delete hard-deletes a company-scoped product (Node's findOneAndDelete; a miss is not an
	// error - /material/remove answers 200 regardless).
	Delete(ctx context.Context, companyID, materialID ID) error
	// Get reads one company-scoped product by id (/material/get); found is false on a miss.
	Get(ctx context.Context, companyID, materialID ID) (Material, bool, error)
	// SharedList returns the projected products across a set of companies (name-sorted), for the
	// /shared/materials report. Read-only and caller-scoped to the owner's companies.
	SharedList(ctx context.Context, companyIDs []ID) ([]SharedMaterial, error)
	// OwnedSharing returns a company-OWNED product's sharing.companies list (ownedScope, never
	// widened). found is false on a miss or another company's product. For /sharing/preview.
	OwnedSharing(ctx context.Context, companyID, materialID ID) (companies []ID, found bool, err error)
	// SetSharing replaces a company-OWNED product's sharing.companies (ownedScope) and returns the
	// stored list. found is false on a miss. For /sharing/set.
	SetSharing(ctx context.Context, companyID, materialID ID, companies []ID) (stored []ID, found bool, err error)
	// DuplicatesSource returns every product across the owner's full company set with its
	// shared-with count, for /shared/duplicate-materials (grouped by normalised name in the
	// handler). Always the owner's whole set, never the report toggle's scope.
	DuplicatesSource(ctx context.Context, companyIDs []ID) ([]MaterialDuplicate, error)
	// SetUnit sets a company-OWNED product's unit (ownedScope, not sharing-widened) and returns
	// its name+unit. found is false when no owned product matches; borrowed is true when the id
	// exists but belongs to another company (shared in), so the handler can name that case.
	SetUnit(ctx context.Context, companyID, materialID ID, unit string) (name, savedUnit string, found, borrowed bool, err error)
}

// ---------------------------------------------------------------------------------------
// People (employees and suppliers)
// ---------------------------------------------------------------------------------------

// Person is an employee or a supplier, as /person/list projects it. The NotifyPo* flags are
// pointers because Model/Person.js gives them no default: unset means ABSENT on the wire, not
// false, so the handler omits a nil rather than sending it.
type Person struct {
	ID                ID
	Name              string
	Type              string
	Email             string
	Phone             string
	Firm              string
	Address           string
	Gst               string
	OpeningBalance    float64
	IsActive          bool
	Permissions       []string
	NotifyPoCreated   *bool
	NotifyPoUpdated   *bool
	NotifyPoConfirmed *bool
	CreatedAt         time.Time
}

// PersonWrite is the field set of /person/create. Email is a pointer so an empty submission
// stores NULL (Node's `email || null`); PasswordHash is set only for an Employee being granted a
// login, and Permissions is already normalised by the handler.
type PersonWrite struct {
	Name         string
	Type         string
	Email        *string
	Phone        string
	Firm         string
	Address      string
	Gst          string
	PasswordHash *string
	Permissions  []string
}

// PersonPatch is /person/update's partial edit: a nil field is "not submitted" and left as-is.
// EmailSet distinguishes "email omitted" from "email set to empty" (which stores NULL).
type PersonPatch struct {
	Name         *string
	Type         *string
	EmailSet     bool
	Email        *string
	Phone        *string
	Firm         *string
	Address      *string
	Gst          *string
	IsActive     *bool
	Permissions  *[]string
	PasswordHash *string
}

// EmployeeAuth is the credential-bearing view of an Employee person, for /person/login.
type EmployeeAuth struct {
	ID           ID
	UID          ID
	Name         string
	Email        string
	PasswordHash string
	IsActive     bool
	Permissions  []string
}

type People interface {
	// List returns the owner's people (uid), optionally filtered by type ("Employee"/"Supplier").
	// An empty personType means no filter.
	List(ctx context.Context, uid ID, personType string) ([]Person, error)
	// Create inserts an owner-scoped person. dupEmail is true (and nothing is inserted) when the
	// email is already registered to another of this owner's people.
	Create(ctx context.Context, uid ID, in PersonWrite) (p Person, dupEmail bool, err error)
	// FindEmployeeByEmail returns the Employee with this exact email and its credentials, for
	// the employee portal login. found is false when there is no such employee.
	FindEmployeeByEmail(ctx context.Context, email string) (EmployeeAuth, bool, error)
	// Update applies a partial patch to an owner-scoped person. found is false on a miss;
	// dupEmail is true when the new email collides with another of the owner's people.
	Update(ctx context.Context, uid, personID ID, patch PersonPatch) (p Person, dupEmail bool, found bool, err error)
	// Delete hard-deletes an owner-scoped person. found is false on a miss.
	Delete(ctx context.Context, uid, personID ID) (found bool, err error)
	// SetNotifyField sets ONE tri-state WhatsApp-notify flag on an owner-scoped person: value
	// nil clears it to null ("follow the default"), &true/&false set it. field is a NotifyPo*
	// wire name, whitelisted by both the handler and the store (an unknown field is an error,
	// never a dynamic column). found is false on a miss.
	SetNotifyField(ctx context.Context, uid, personID ID, field string, value *bool) (p Person, found bool, err error)
}

// NotifyFields is the whitelist of tri-state supplier-notify flags a person carries
// (Helpers/NotifyPreference.NOTIFY_FIELDS). The map value is unused; membership is the point.
// Handlers validate a requested field against this before it ever reaches a store.
var NotifyFields = map[string]struct{}{
	"notifyPoCreated":   {},
	"notifyPoUpdated":   {},
	"notifyPoConfirmed": {},
}

// ---------------------------------------------------------------------------------------
// Quotations
// ---------------------------------------------------------------------------------------

// QuotationClient is the populated client object the list nests in place of client_id
// (Mongoose .populate). nil when the client was deleted (a dangling ref populates to null).
type QuotationClient struct {
	ID            ID
	ClientName    string
	ClientFirm    string
	ClientPhone   string
	ClientAddress string
	ClientGST     string
}

// QuotationRow is one line of a quotation. HasDimensions is a *bool (absent = by-dimension).
// JobID is nil until the row has started a job.
type QuotationRow struct {
	ID            ID
	Material      string
	Description   string
	HasDimensions *bool
	Length        string
	Width         string
	Qty           float64
	Rate          float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Charges       float64
	JobID         ID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Quotation is the whole record the list returns, client_id populated into Client.
type Quotation struct {
	ID              ID
	UID             ID
	CompanyID       ID
	QuotationNumber string
	Date            string
	Client          *QuotationClient
	Rows            []QuotationRow
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int
}

// QuotationRowInput is one submitted row (parseRows). ID is set only on update, to match an
// existing row so its JobID linkage survives the rewrite; a blank ID is a brand-new row.
type QuotationRowInput struct {
	ID          ID
	Material    string
	Description string
	Length      string
	Width       string
	Qty         float64
	Rate        float64
	Cgst        float64
	Sgst        float64
	Discount    float64
	Charges     float64
}

// QuotationWrite is the field set of /quotation/create.
type QuotationWrite struct {
	ClientID        ID
	QuotationNumber string
	Date            string
	Rows            []QuotationRowInput
}

// QuotationUpdate is /quotation/update's partial edit; a nil field was not submitted. Rows, when
// present, replace the row set (preserving JobID for rows whose ID matches an existing one).
type QuotationUpdate struct {
	ClientID        *ID
	QuotationNumber *string
	Date            *string
	Rows            *[]QuotationRowInput
}

type Quotations interface {
	// List returns a company's quotations (newest first), client_id populated, optionally
	// filtered by clientID (""=no filter).
	List(ctx context.Context, uid, companyID, clientID ID) ([]Quotation, error)
	// Numbers returns every quotation number for the owner+company, for the next-number helper.
	Numbers(ctx context.Context, uid, companyID ID) ([]string, error)
	// Get returns one populated quotation (owner+company scoped); found is false on a miss.
	Get(ctx context.Context, uid, companyID, quotationID ID) (q Quotation, found bool, err error)
	// Create inserts a quotation with its rows; dupNumber is true when the number already exists.
	Create(ctx context.Context, uid, companyID ID, in QuotationWrite) (q Quotation, dupNumber bool, err error)
	// Update applies a partial patch; replacing rows keeps each matched row's JobID. found is
	// false on a miss, dupNumber true on a number collision.
	Update(ctx context.Context, uid, companyID, quotationID ID, in QuotationUpdate) (q Quotation, dupNumber bool, found bool, err error)
	// Delete removes a quotation (owner+company scoped). found is false on a miss.
	Delete(ctx context.Context, uid, companyID, quotationID ID) (found bool, err error)
	// RowDelete removes one row from a quotation and returns the re-populated quotation. found is
	// false when the quotation is missing (a missing row is a no-op, matching Node's `?.deleteOne`).
	RowDelete(ctx context.Context, uid, companyID, quotationID, rowID ID) (q Quotation, found bool, err error)
	// AddRowToJob converts one quotation row into a job row: it appends to the job already built
	// from this quotation, or creates a new job when there is none, then stamps the row's job_id
	// and logs a System note + history entry. See QuotationRowToJobResult for the outcomes.
	AddRowToJob(ctx context.Context, uid, companyID, quotationID, rowID ID) (QuotationRowToJobResult, error)
}

// QuotationRowToJobStatus is the outcome of AddRowToJob.
type QuotationRowToJobStatus int

const (
	QRJOk                QuotationRowToJobStatus = iota // converted
	QRJQuotationNotFound                                // no such quotation for this owner/company
	QRJRowNotFound                                      // no such row in the quotation
	QRJAlreadyAdded                                     // the row is already on a job
	QRJDupChallan                                       // a challan-number collision (retryable)
)

// QuotationRowToJobResult carries the converted quotation and job (both populated) plus whether
// a new job was created (which decides the message and the "Created" vs "Updated" history).
type QuotationRowToJobResult struct {
	Status    QuotationRowToJobStatus
	IsNew     bool
	Quotation Quotation
	Job       Job
}

// ---------------------------------------------------------------------------------------
// Purchase invoices
// ---------------------------------------------------------------------------------------

// PurchaseSupplier is the populated supplier object (name/firm/phone) the list nests in place
// of supplier_id. nil when the supplier was deleted.
type PurchaseSupplier struct {
	ID    ID
	Name  string
	Firm  string
	Phone string
}

// PurchaseInvoiceRow is one line of a supplier bill. HasDimensions is a *bool; purchase rows
// default to false (by quantity), unlike job/quotation rows.
type PurchaseInvoiceRow struct {
	ID            ID
	Description   string
	Material      string
	Hsn           string
	Gst           float64
	HasDimensions *bool
	Length        string
	Width         string
	Rate          float64
	Qty           float64
	Unit          string
	Discount      float64
	Charges       float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PurchaseInvoice is the whole record the list returns, supplier_id populated.
type PurchaseInvoice struct {
	ID            ID
	UID           ID
	CompanyID     ID
	Supplier      *PurchaseSupplier
	Date          string
	InvoiceNumber string
	Rows          []PurchaseInvoiceRow
	Total         float64
	Amount        float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int
}

// PurchaseRowInput is one submitted purchase-invoice row (normalizeRow), already coerced.
type PurchaseRowInput struct {
	Description string
	Material    string
	Hsn         string
	Gst         float64
	Rate        float64
	Qty         float64
	Unit        string
	Discount    float64
	Charges     float64
}

// PurchaseInvoiceWrite is the field set of /purchase-invoice/create; Total is precomputed by the
// handler via the shared row-total formula.
type PurchaseInvoiceWrite struct {
	SupplierID    ID
	Date          string
	InvoiceNumber string
	Rows          []PurchaseRowInput
	Total         float64
}

// PurchaseInvoiceUpdate is /purchase-invoice/update's partial edit; a nil field was not
// submitted. When Rows is set, NewTotal is the recomputed total the store guards against.
type PurchaseInvoiceUpdate struct {
	SupplierID    *ID
	Date          *string
	InvoiceNumber *string
	Rows          *[]PurchaseRowInput
	NewTotal      float64
}

// PurchaseUpdateStatus is the outcome of a purchase-invoice update.
type PurchaseUpdateStatus int

const (
	PurchaseUpdateOK          PurchaseUpdateStatus = iota
	PurchaseUpdateNotFound                         // no such invoice for this owner+company
	PurchaseUpdatePaidExceeds                      // the new total would fall below what is paid
)

// PurchaseUpdateResult carries the status plus the amounts the refusal message needs.
type PurchaseUpdateResult struct {
	Status     PurchaseUpdateStatus
	AmountPaid float64
	NewTotal   float64
}

// PurchaseDeleteStatus is the outcome of a purchase-invoice delete.
type PurchaseDeleteStatus int

const (
	PurchaseDeleteOK         PurchaseDeleteStatus = iota
	PurchaseDeleteNotFound                        // no such invoice
	PurchaseDeleteHasPayment                      // a supplier payment allocates to it
)

type PurchaseInvoices interface {
	// List returns a company's purchase invoices (newest first), supplier_id populated.
	List(ctx context.Context, uid, companyID ID) ([]PurchaseInvoice, error)
	// Create inserts a purchase invoice with its rows and the precomputed total.
	Create(ctx context.Context, uid, companyID ID, in PurchaseInvoiceWrite) (PurchaseInvoice, error)
	// Update applies a partial patch. When rows change, it stores in.NewTotal but refuses
	// (PurchaseUpdatePaidExceeds) if that would drop below what is already paid.
	Update(ctx context.Context, uid, companyID, invoiceID ID, in PurchaseInvoiceUpdate) (PurchaseInvoice, PurchaseUpdateResult, error)
	// Delete removes an invoice unless a supplier payment allocates to it (PurchaseDeleteHasPayment).
	Delete(ctx context.Context, uid, companyID, invoiceID ID) (PurchaseDeleteStatus, error)
	// Numbers returns a company's existing purchase invoice numbers, for the /next-number suggestion.
	Numbers(ctx context.Context, companyID ID) ([]string, error)
}

// ---------------------------------------------------------------------------------------
// Purchase orders (routes/PurchaseOrder.js)
// ---------------------------------------------------------------------------------------

// POSupplier is the populated supplier on a purchase order.
type POSupplier struct {
	ID      ID
	Name    string
	Firm    string
	Phone   string
	Address string
	Gst     string
}

// PORow is one purchase-order line (a field-for-field twin of a purchase-invoice row).
type PORow struct {
	ID            ID
	Description   string
	Material      string
	Hsn           string
	Gst           float64
	HasDimensions bool
	Length        string
	Width         string
	Rate          float64
	Qty           float64
	Unit          string
	Discount      float64
	Charges       float64
}

// POApproval is the sign-off state; Fingerprint is the approved price-bearing content.
type POApproval struct {
	State          string
	ApprovedBy     ID
	ApprovedByName string
	ApprovedAt     *time.Time
	Fingerprint    string
}

// POSend is the minimal stored send state PoSend.sendState derives from.
type POSend struct {
	Count         int
	Fingerprint   string
	SentAt        *time.Time
	ConfirmSentAt *time.Time
}

// PurchaseOrder is one order with its rows, approval, convert link and send state.
type PurchaseOrder struct {
	ID                ID
	UID               ID
	CompanyID         ID
	SupplierID        ID
	Supplier          *POSupplier
	PoNumber          string
	Date              string
	Total             float64
	Rows              []PORow
	Approval          POApproval
	PurchaseInvoiceID ID
	ConvertedAt       *time.Time
	Send              POSend
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Change is one field-level audit difference (Helpers/PoChanges.diff), stored on a PO history row.
type Change struct {
	Field string `json:"field" bson:"field"`
	From  string `json:"from" bson:"from"`
	To    string `json:"to" bson:"to"`
}

// POHistoryRow is one audit-trail entry for a purchase order.
type POHistoryRow struct {
	ID        ID
	POID      ID
	ActorType string
	ActorID   ID
	ActorName string
	Action    string
	Changes   []Change
	Detail    string
	CreatedAt time.Time
}

// PONote is one note on a purchase order.
type PONote struct {
	ID         ID
	POID       ID
	AuthorType string
	AuthorID   ID
	AuthorName string
	Text       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// POWrite is the writable content of a purchase order (rows + total precomputed by the handler).
type POWrite struct {
	SupplierID ID
	Date       string
	Rows       []PORow
	Total      float64
}

// POUpdate is a partial edit of a purchase order. Present* flags distinguish "not submitted" from
// "cleared" for the optional fields (Node's `if (req.body.x)` / `!== undefined`).
type POUpdate struct {
	SupplierID    ID
	SetSupplier   bool
	Date          string
	SetDate       bool
	Rows          []PORow
	SetRows       bool
	Total         float64
	NewFingerprint string // fingerprint of the edited document, to compare against the approved one
	Changes       []Change
}

// POUpdateResult reports what an update did, for the handler's response + logging.
type POUpdateResult struct {
	Found            bool
	Converted        bool // 409: already a purchase invoice
	RevokedApproval  bool // approval dropped because price-bearing content changed
}

// EnquiryWrite is a landing-page enquiry before the store applies casing (name/company titleCase,
// note sentenceCase, email lowercase).
type EnquiryWrite struct {
	Name        string
	Email       string
	Phone       string
	CompanyName string
	Note        string
	Source      string
	UserAgent   string
}

// RegistrationToken is a one-time invite for /user/register (minted by the dev panel).
type RegistrationToken struct {
	ID        ID
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	UsedBy    ID
	CreatedAt time.Time
}

// RegistrationTokens mints and redeems registration invites.
type RegistrationTokens interface {
	// FindByToken looks up a token by its value; found is false on a miss.
	FindByToken(ctx context.Context, token string) (RegistrationToken, bool, error)
	// MarkUsed stamps a token as redeemed by uid.
	MarkUsed(ctx context.Context, id, uid ID) error
	// Create issues a new token valid until expiresAt.
	Create(ctx context.Context, token string, expiresAt time.Time) (RegistrationToken, error)
	// List returns the newest tokens (capped 25) for the dev panel.
	List(ctx context.Context) ([]RegistrationToken, error)
}

// PasswordResetRequest is a self-service ask that a superadmin resolves.
type PasswordResetRequest struct {
	ID         ID
	Email      string
	UID        ID
	Status     string
	Note       string
	ResolvedAt *time.Time
	CreatedAt  time.Time
}

// PasswordResetRequests queues password-reset asks.
type PasswordResetRequests interface {
	// Create records a pending request (email + optional uid).
	Create(ctx context.Context, email string, uid ID) error
	// HasPending reports whether an unresolved request already exists for the email.
	HasPending(ctx context.Context, email string) (bool, error)
	// ListPending returns the pending requests (capped 50) for the dev panel.
	ListPending(ctx context.Context) ([]PasswordResetRequest, error)
	// Get fetches one request; found is false on a miss.
	Get(ctx context.Context, id ID) (PasswordResetRequest, bool, error)
	// Resolve sets a request's status (done|dismissed) and stamps resolvedAt.
	Resolve(ctx context.Context, id ID, status string) error
}

// Enquiry is a stored landing-page enquiry (the dev panel's list row).
type Enquiry struct {
	ID          ID
	Name        string
	Email       string
	Phone       string
	CompanyName string
	Note        string
	Source      string
	UserAgent   string
	Handled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int
}

// Enquiries records landing-page demo/pricing enquiries (routes/Enquiry.js).
type Enquiries interface {
	Create(ctx context.Context, in EnquiryWrite) (ID, error)
	// List returns the newest enquiries (capped 200) for the dev panel.
	List(ctx context.Context) ([]Enquiry, error)
	// SetHandled flips an enquiry's handled flag (dev panel). A miss is not an error (Node's updateOne).
	SetHandled(ctx context.Context, id ID, handled bool) error
}

// POActionStatus is the outcome of a purchase-order state transition (approve/revoke).
type POActionStatus int

const (
	POActionOK POActionStatus = iota
	POActionNotFound
	POActionConverted      // already a purchase invoice
	POActionNoRows         // approve refused: no rows
	POActionAlreadyApproved
	POActionNotApproved    // revoke refused: not approved
)

// POFingerprint is Helpers/PoChanges.fingerprint over a stored purchase order, for approve.
func POFingerprint(po PurchaseOrder) string {
	p := pochanges.PO{SupplierID: string(po.SupplierID), Date: po.Date, Total: po.Total}
	for _, r := range po.Rows {
		p.Rows = append(p.Rows, pochanges.Row{
			Material: r.Material, Description: r.Description, Unit: r.Unit, Hsn: r.Hsn,
			Qty: r.Qty, Rate: r.Rate, Discount: r.Discount, Charges: r.Charges, Gst: r.Gst,
		})
	}
	return pochanges.Fingerprint(p)
}

type PurchaseOrders interface {
	// Numbers returns a company's existing PO numbers (owner+company scoped), for /next-number.
	Numbers(ctx context.Context, uid, companyID ID) ([]string, error)
	// Create inserts a PO with its rows and logs "Created". poNumber is generated by the handler.
	Create(ctx context.Context, uid, companyID ID, poNumber string, actor NoteActor, in POWrite) (PurchaseOrder, error)
	// List returns a company's POs (newest first), supplier populated; supplierID "" = all.
	List(ctx context.Context, uid, companyID, supplierID ID) ([]PurchaseOrder, error)
	// Detail returns one PO (supplier populated) with its history and notes (newest first).
	Detail(ctx context.Context, uid, companyID, poID ID) (po PurchaseOrder, history []POHistoryRow, notes []PONote, found bool, err error)
	// Update applies a partial edit; refuses a converted PO (409), replaces rows when submitted,
	// recomputes the total, and drops approval to draft when the price-bearing content changed.
	// Logs "Updated" (with changes) and, on a revoke, "Approval revoked".
	Update(ctx context.Context, uid, companyID, poID ID, actor NoteActor, in POUpdate) (PurchaseOrder, POUpdateResult, error)
	// Approve sets sign-off with a fresh fingerprint, refusing a converted PO, an empty PO, or one
	// already approved. Logs "Approved".
	Approve(ctx context.Context, uid, companyID, poID ID, actor NoteActor) (PurchaseOrder, POActionStatus, error)
	// Revoke withdraws approval (only from an approved PO). Logs "Approval revoked".
	Revoke(ctx context.Context, uid, companyID, poID ID, actor NoteActor) (PurchaseOrder, POActionStatus, error)
	// AddNote appends a note (author from the actor) and logs "Note added". found is false on a miss.
	AddNote(ctx context.Context, uid, companyID, poID ID, actor NoteActor, text string) (PONote, bool, error)
	// EditNote edits a note within the author's 24h window; logs "Note edited". found false on a
	// miss, forbidden true when the actor is not the author or the window has passed.
	EditNote(ctx context.Context, companyID, noteID ID, actor NoteActor, text string) (n PONote, found, forbidden bool, err error)
	// PendingApprovals derives the approval queue: `pending` is every draft, un-converted PO with
	// rows (Helpers/PoChanges.isAwaitingApproval); `visible` drops the ones this actor has dismissed
	// at their current version (a PO edited since a dismissal reappears).
	PendingApprovals(ctx context.Context, uid, companyID, actorID ID) (pending, visible []PurchaseOrder, err error)
	// DismissApprovals records "seen" for this actor: prunes dismissals for orders no longer
	// pending, then upserts one per target (all currently-visible, or the single poID). Returns the
	// number of targets dismissed and the fresh visible/total counts. targets 0 means nothing to
	// dismiss (-> 404).
	DismissApprovals(ctx context.Context, uid, companyID, actorID ID, actorType string, all bool, poID ID) (targets, freshVisible, freshTotal int, err error)
	// Convert mints a purchase invoice from an approved PO (same rows + total), links it back and
	// logs "Converted to Purchase Invoice". status is NotFound, Converted (already), or NotApproved.
	// On success it returns the reloaded PO (now converted) and the new invoice id.
	Convert(ctx context.Context, uid, companyID, poID ID, actor NoteActor, invoiceNumber, date string) (po PurchaseOrder, invoiceID ID, status POActionStatus, err error)
	// PublicView is the unauthenticated supplier link (GET /po-public/:supplier_id/:po_id): both ids
	// must match the same order. found is false on a mismatch. Closed is true once the order became a
	// purchase invoice (the link is spent) - then no order/company detail is returned.
	PublicView(ctx context.Context, supplierID, poID ID) (PublicPO, bool, error)
}

// PublicPOCompany is the letterhead a supplier link carries (resolved from the ORDER's company).
type PublicPOCompany struct {
	Firm         string
	Address      string
	Phone        string
	Gst          string
	URL          string
	DocumentFont string
}

// PublicPO is the payload of the public purchase-order link (Helpers/PoPublic.publicPurchaseOrder).
type PublicPO struct {
	Closed       bool
	PoNumber     string
	Date         string
	Total        float64
	SupplierName string
	SupplierFirm string
	Rows         []PORow
	Company      PublicPOCompany
}

// ---------------------------------------------------------------------------------------
// Invoices
// ---------------------------------------------------------------------------------------

// InvoiceClient is the populated client on an invoice.
type InvoiceClient struct {
	ID            ID
	UID           ID
	ClientName    string
	ClientFirm    string
	ClientPhone   string
	ClientAddress string
	ClientGST     string
}

// InvoiceHistoryEntry is one of the invoice's entries for /invoice/history (the pricing fields are
// carried so the handler can recompute the display total via entrymath).
type InvoiceHistoryEntry struct {
	EntryID     ID
	Date        string
	Material    string
	Description string
	Qty         float64
	Rate        float64
	Amount      float64
	Advance     float64
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InvoiceHistoryJob is one job an invoice's entries came from.
type InvoiceHistoryJob struct {
	JobID         ID
	ChallanNumber string
}

// InvoiceHistoryTrail is one audit-trail row for the invoice's jobs (the handler adds the job's
// challan number from the jobs list).
type InvoiceHistoryTrail struct {
	At        time.Time
	Action    string
	Detail    string
	ActorName string
	JobID     ID
}

// InvoiceHistory is the assembled /invoice/history payload (challan numbers on the trail are joined
// by the handler).
type InvoiceHistory struct {
	InvoiceNumber string
	Entries       []InvoiceHistoryEntry
	Jobs          []InvoiceHistoryJob
	Trail         []InvoiceHistoryTrail
}

// Entry is one billed line on an invoice (Model/Entry.js), the pricing fields plus display.
type Entry struct {
	ID            ID
	Description   string
	Material      string
	Hsn           string
	// Unit is the product's unit of measure, snapshotted from the Material at conversion like Hsn.
	Unit          string
	Rate          float64
	Qty           float64
	HasDimensions *bool
	Length        string
	Width         string
	Date          string
	Amount        float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Charges       float64
	Advance       float64
	Total         float64
	HasIssued     bool
	ClientID      ID
	CompanyID     ID
	UID           ID
	QuotationID   ID
	// Client is the populated client_id, set only where a route populates it (entry/add).
	// Elsewhere it is nil and ClientID carries the raw id.
	Client    *EntryClient
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int
}

// EntryClient is the subset of a client populated onto an entry's client_id (entry/add).
type EntryClient struct {
	ID            ID
	CompanyID     ID
	UID           ID
	LegacyID      any
	ClientName    string
	ClientFirm    string
	ClientPhone   string
	ClientGST     string
	ClientAddress string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EntryWrite is /entry/add: a fully specified line item. HSN is resolved from the material
// name at write time (a snapshot), so it is not part of the input.
type EntryWrite struct {
	ClientID    ID
	Date        string
	Material    string
	Description string
	Length      string
	Width       string
	Qty         float64
	Rate        float64
	Amount      float64
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	Total       float64
	Advance     float64
}

// EntryUpdate is /entry/update: like EntryWrite plus the target id. A date change moves the
// entry between the day-sheets (creating the new day's sheet if it does not exist).
type EntryUpdate struct {
	EntryID ID
	EntryWrite
}

// Entries is the /entry domain: line items grouped into day-sheets. add/update keep the
// sheet-of-day membership in step; remove (soft-delete to Trash) is deferred.
type Entries interface {
	Add(ctx context.Context, uid, companyID ID, in EntryWrite) (Entry, error)
	Update(ctx context.Context, uid, companyID ID, in EntryUpdate) (Entry, bool, error)
	Get(ctx context.Context, uid, companyID, entryID ID) (Entry, bool, error)
	List(ctx context.Context, uid, companyID ID) ([]Entry, error)
	// SinceForClient returns a client's entries created at or after `since`, newest first, each
	// carrying its issued-invoice number - the input to the /stats/exports spreadsheet. A zero
	// `since` means no lower bound (the "ft" full-time mode).
	SinceForClient(ctx context.Context, uid, companyID, clientID ID, since time.Time) ([]ClientEntryView, error)
}

// Sheets is the /sheet domain. Only /sheet/get is real (update/remove/getall are empty Node
// stubs); it returns a day-sheet's entries, each with its client populated, for the handler to
// group by client. Membership is by date: an entry belongs to the sheet with the same
// company and (normalized) date.
type Sheets interface {
	Get(ctx context.Context, companyID, sheetID ID) (date string, entries []Entry, found bool, err error)
}

// Invoice is one invoice with its client populated and its entries loaded.
type Invoice struct {
	ID          ID
	InvoiceID   string
	Date        string
	Amount      float64
	TotalAmount float64
	Client      *InvoiceClient
	Entries     []Entry
	CreatedAt   time.Time
}

// InvoiceReceivedRow is one payment recorded against an invoice (getReceived), bank populated.
type InvoiceReceivedRow struct {
	ID        ID
	Date      string
	Amount    float64
	Note      string
	InvoiceID ID
	BankID    ID
	BankName  string
	CreatedAt time.Time
	Version   int
}

// InvoiceSaveInput is /invoice/save (issuance): bundle these entries onto an invoice numbered
// InvNo for a client. If an invoice with that number exists it is rewritten in place.
type InvoiceSaveInput struct {
	Date     string
	ClientID ID
	InvNo    string
	EntryIDs []ID
}

// InvoicePaidAlloc is one manual entry allocation on /invoice/paid.
type InvoicePaidAlloc struct {
	EntryID ID
	Amount  float64
}

// InvoicePaidInput is /invoice/paid: record a payment against an invoice, spread over its entries.
type InvoicePaidInput struct {
	InvoiceID      ID
	ReceivedAmount float64
	Mode           string // "manual" | "auto"
	Date           string
	BankID         ID
	Note           string
	Allocations    []InvoicePaidAlloc
}

type Invoices interface {
	// List returns a company's invoices with client populated and entries loaded (#6 batched).
	List(ctx context.Context, companyID ID) ([]Invoice, error)
	// Numbers returns every invoice number for the company, for the next-number helper.
	Numbers(ctx context.Context, companyID ID) ([]string, error)
	// EntryJobLabels maps each of these entry ids to the challan number of the job it came from
	// (Job.rows[].entry_id is the only link), for the close-invoice modal.
	EntryJobLabels(ctx context.Context, companyID ID, entryIDs []ID) (map[string]string, error)
	// Received returns the payments recorded against an invoice, bank populated.
	Received(ctx context.Context, companyID, invoiceID ID) ([]InvoiceReceivedRow, error)
	// ReceivedByClient returns all payments recorded against a client's invoices (the
	// receivedHistory of /invoice/getClientInvoices), newest queries aside, in insertion order.
	ReceivedByClient(ctx context.Context, companyID, clientID ID) ([]InvoiceReceivedRow, error)
	// Save issues (or re-issues) an invoice from the given entries and returns its id; found is
	// false only for the not-applicable cases (always true here). It marks entries issued and
	// computes amount (Σ advance) and totalAmount (Σ RoundOffWithAmount(amount·1.18)).
	Save(ctx context.Context, uid, companyID ID, in InvoiceSaveInput) (invoiceID ID, err error)
	// Paid records a payment against an invoice, spreading it over the invoice's entries (and the
	// jobs they came from) and logging an InvoiceReceived. found is false when the invoice is
	// missing; manualErr names a manual-allocation problem (empty JSON handled by the handler).
	Paid(ctx context.Context, companyID ID, in InvoicePaidInput) (found bool, err error)
	// Remove deletes an invoice and un-issues its entries. found is false on a miss.
	Remove(ctx context.Context, companyID, invoiceID ID) (found bool, err error)
	// History assembles /invoice/history: the invoice's number and entries, the jobs its entries
	// came from (resolved via the row->entry chain, since Go invoices don't store job_ids), and
	// those jobs' audit trail (newest first, capped 200). found is false on a miss/other company.
	History(ctx context.Context, companyID, invoiceID ID) (InvoiceHistory, bool, error)
}

// ---------------------------------------------------------------------------------------
// Lifecycle lookups (name/rate pickers for the Job and Quotation forms)
// ---------------------------------------------------------------------------------------

// LookupClient/Material/Person are the projected rows the form dropdowns need - narrow,
// uid+company scoped, and (unlike the sharing-widened list reads) never widened.
type LookupClient struct {
	ID          ID
	ClientName  string
	ClientFirm  string
	ClientPhone string
}

type LookupMaterial struct {
	ID           ID
	MaterialName string
	MaterialRate float64
	Hsn          string
	Tax          float64
}

type LookupPerson struct {
	ID   ID
	Name string
	Type string
}

type Lookups interface {
	Clients(ctx context.Context, uid, companyID ID) ([]LookupClient, error)
	Materials(ctx context.Context, companyID ID) ([]LookupMaterial, error)
	// People returns the owner's ACTIVE people (name+type). The owner-as-Admin row is prepended
	// by the handler, not here.
	People(ctx context.Context, uid ID) ([]LookupPerson, error)
}

// ---------------------------------------------------------------------------------------
// Jobs (the Lifecycle board)
// ---------------------------------------------------------------------------------------

// JobPerson is a populated employee or vendor (name only).
type JobPerson struct {
	ID   ID
	Name string
}

// JobClient is the populated client on a job.
type JobClient struct {
	ID             ID
	ClientName     string
	ClientFirm     string
	ClientPhone    string
	ClientAddress  string
	NotifyOnCreate *bool
	NotifyOnUpdate *bool
}

// InvoiceableRow is one job row as the invoice picker reads it: its queue and IGST, plus the base
// amount / issued state of its entry (HasEntry false when the row is not yet converted).
type InvoiceableRow struct {
	EntryID   ID
	Queue     string
	Igst      float64
	Amount    float64
	HasIssued bool
	HasEntry  bool
}

// InvoiceableJob is one of a client's jobs for /invoice/invoiceable-jobs.
type InvoiceableJob struct {
	ID            ID
	ChallanNumber string
	ReceivedDate  string
	Queue         string
	Total         float64
	Rows          []InvoiceableRow
}

// JobRowEntry is the populated rows[].entry_id (has_issued/total/advance).
type JobRowEntry struct {
	ID        ID
	HasIssued bool
	Total     float64
	Advance   float64
}

// JobRowQuotation is the populated rows[].quotation_id (quotationNumber).
type JobRowQuotation struct {
	ID              ID
	QuotationNumber string
}

// JobRow is one card. It carries the pricing/queue fields joblifecycle needs, the populated
// employee/quotation/entry, and EntryIssued (whether its entry is currently on an invoice).
type JobRow struct {
	ID            ID
	RowID         string
	Material      string
	Description   string
	Qty           float64
	HasDimensions *bool
	Length        string
	Width         string
	Rate          float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Charges       float64
	Queue         string
	Progress      string
	QueueOrder    []string
	Employee      *JobPerson
	Quotation     *JobRowQuotation
	Entry         *JobRowEntry
	EntryIssued   bool
	// InvoiceNumber is the number of the invoice this row's entry landed on, "" if none.
	InvoiceNumber string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// JobAlertChannel is a stored alert channel (created or done).
type JobAlertChannel struct {
	Status    string
	StatusAt  *time.Time
	Error     string
	SentAt    *time.Time
	Count     int
	RowIDs    []string
	Signature string
}

// Job is one job with everything the board needs: fields, the populated parties, its rows, and
// the two alert channels. The handler derives invoiceState/lock/alerts from it (joblifecycle).
type Job struct {
	ID            ID
	ChallanNumber string
	ReceivedDate  string
	Total         float64
	Advance       float64
	Queue         string
	Progress      string
	QueueOrder    []string
	Unlocked      bool
	Client        *JobClient
	Employee      *JobPerson
	Vendor        *JobPerson
	Rows          []JobRow
	CreatedAlert  JobAlertChannel
	DoneAlert     JobAlertChannel
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int
}

type Jobs interface {
	// List returns a company's jobs (newest first), all six populations resolved and each row's
	// entry invoice-state filled, optionally filtered by clientID (""=no filter).
	List(ctx context.Context, uid, companyID, clientID ID) ([]Job, error)
	// ChallanNumbers returns every job's challan number for the owner+company (next-challan helper).
	ChallanNumbers(ctx context.Context, uid, companyID ID) ([]string, error)
	// ByEntry returns the populated job whose rows carry entryID; found is false when none does.
	ByEntry(ctx context.Context, uid, companyID, entryID ID) (Job, bool, error)
	// Create inserts a job and its rows, logs a "Created" history entry, and returns the populated
	// job. dupChallan is true (nothing inserted) when the challan number already exists.
	Create(ctx context.Context, in JobCreateInput) (j Job, dupChallan bool, err error)
	// Update applies a partial job edit and (when rows are submitted) the row diff, logs an
	// "Updated" history entry, and returns the populated job. found is false on a miss; dupChallan
	// on a number collision; emptyRows when the submitted row set would leave the job with none.
	Update(ctx context.Context, in JobUpdateInput) (j Job, found, dupChallan, emptyRows bool, err error)

	// The lifecycle-board transitions. Each mutates the job (or one row), logs its history
	// entries, and returns the populated job with a JobTxStatus (OK / not-found / needs-assignee).
	// Assign sets an employee OR vendor (clearing the other) and recomputes progress.
	Assign(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, kind string, personID ID) (Job, JobTxStatus, error)
	// Progress moves the job's progress; "Unassigned" clears the assignee, "Complete" advances the
	// queue stage (clearing the assignee for the next stage); other values just set progress.
	Progress(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, progress string) (Job, JobTxStatus, error)
	// SetQueue jumps the job to a given stage (clearing the assignee); a no-op when already there.
	SetQueue(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, queue string) (Job, JobTxStatus, error)
	// QueueOrder persists the job's drag-reordered stage list.
	QueueOrder(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, order []string) (Job, JobTxStatus, error)
	// RowAssign sets one row's employee and recomputes its progress.
	RowAssign(ctx context.Context, uid, companyID, jobID, rowID ID, actor NoteActor, employeeID ID) (Job, JobTxStatus, error)
	// RowSetQueue jumps one row to a stage (clearing its employee, progress→Assign).
	RowSetQueue(ctx context.Context, uid, companyID, jobID, rowID ID, actor NoteActor, queue string) (Job, JobTxStatus, error)
	// RowQueueOrder persists one row's drag-reordered stage list.
	RowQueueOrder(ctx context.Context, uid, companyID, jobID, rowID ID, actor NoteActor, order []string) (Job, JobTxStatus, error)
	// RowProgress moves one row's progress; "Complete" advances its stage (clearing its employee).
	RowProgress(ctx context.Context, uid, companyID, jobID, rowID ID, actor NoteActor, progress string) (Job, JobTxStatus, error)
	// Unlock sets the job's `unlocked` flag to wanted, logging an "Updated" history entry ("Unlocked
	// for editing" / "Re-locked") only when it actually changes. changed reports whether it did, so
	// the handler can answer "No change." vs "Job unlocked."/"Job locked.". The populated job comes
	// back in every OK case, including no-change.
	Unlock(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, wanted bool) (j Job, changed bool, status JobTxStatus, err error)
	// RowsCompleteAll marks every not-yet-Done row of the job as Done (clearing the assignee,
	// progress→Assign), logging one STRUCTURED "Queue advanced" history entry per moved row. It
	// refuses (JobTxLocked) when the invoice lock forbids queue changes (invoiced and not
	// unlocked), reproducing refusedByLock("queue"). moved is how many rows changed - 0 means
	// "every row was already done" (still a 200 with the populated job).
	RowsCompleteAll(ctx context.Context, uid, companyID, jobID ID, actor NoteActor) (j Job, moved int, status JobTxStatus, err error)
	// Delete moves a job to Trash: it snapshots the whole job (source "Job", its rows and
	// job-level fields, for a future restore), logs a "Trashed" history entry, then removes the
	// job. It refuses (JobTxLocked) when the invoice lock forbids deletion (invoiced and not
	// unlocked), reproducing refusedByLock("delete"). JobTxJobNotFound when no such job.
	Delete(ctx context.Context, uid, companyID, jobID ID, actor NoteActor) (JobTxStatus, error)
	// ConvertToEntries turns each Done, not-yet-converted row of the named jobs into a billable
	// Entry (stamping the row with its entry_id), logs a "Converted to Entry" history entry per
	// job, and returns the created entries and the re-populated jobs. found is false when none of
	// the jobs has a convertible row.
	ConvertToEntries(ctx context.Context, uid, companyID ID, jobIDs []ID, actor NoteActor) (entries []Entry, jobs []Job, found bool, err error)
	// InvoiceableJobs returns a client's jobs (newest first, capped at 200) with the row + entry
	// state the invoice picker needs to decide what can be billed (/invoice/invoiceable-jobs). The
	// per-row billable math and the "everything done" filter live in the handler.
	InvoiceableJobs(ctx context.Context, companyID, clientID ID) ([]InvoiceableJob, error)
}

// JobRowPatch is one row submitted to /lifecycle/jobs/update. ID (the existing row's id) matches
// it to a current row; a blank ID (or an unmatched one) is a brand-new row.
type JobRowPatch struct {
	ID          ID
	Material    string
	Description string
	Length      string
	Width       string
	Qty         float64
	Rate        float64
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	QuotationID ID
}

// JobUpdateInput is /lifecycle/jobs/update. Each *field is nil when not submitted. RowsSet marks
// whether a rows array was sent (its diff runs only then).
type JobUpdateInput struct {
	UID           ID
	CompanyID     ID
	JobID         ID
	ClientID      *ID
	ChallanNumber *string
	ReceivedDate  *string
	Advance       *float64
	RowsSet       bool
	Rows          []JobRowPatch
	Actor         NoteActor
}

// JobRowWrite is one submitted job row (normalizeRow), already coerced; QuotationID may be blank.
type JobRowWrite struct {
	Material    string
	Description string
	Length      string
	Width       string
	Qty         float64
	Rate        float64
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	QuotationID ID
}

// JobCreateInput is /lifecycle/jobs/create. Progress is "In Progress" when an assignee is set,
// else "Unassigned"; Total is the handler-computed job total; Actor logs the history entry.
type JobCreateInput struct {
	UID           ID
	CompanyID     ID
	ClientID      ID
	EmployeeID    ID
	VendorID      ID
	ChallanNumber string
	ReceivedDate  string
	Advance       float64
	Total         float64
	Progress      string
	Rows          []JobRowWrite
	Actor         NoteActor
}

// NoteActor identifies who is acting on a note/history entry: an admin acts as their user, an
// employee as their person (Helpers/Lifecycle actorFromAuth).
type NoteActor struct {
	Role     string // "admin" | "employee"
	UID      ID
	PersonID ID
}

// ActorID is the id this actor is recorded under (uid for admin, personId otherwise).
func (a NoteActor) ActorID() ID {
	if a.Role == "admin" {
		return a.UID
	}
	return a.PersonID
}

// JobNote is one note on a job (Model/JobNote.js).
type JobNote struct {
	ID         ID
	JobID      ID
	AuthorType string
	AuthorID   ID
	AuthorName string
	Text       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Version    int
}

// JobHistoryRow is one audit-trail entry (Model/JobHistory.js).
type JobHistoryRow struct {
	ID        ID
	JobID     ID
	ActorType string
	ActorID   ID
	ActorName string
	Action    string
	Detail    string
	FromStage string
	ToStage   string
	RowKey    string
	CreatedAt time.Time
	Version   int
}

type JobNotes interface {
	// NotesList returns a job's notes, newest first.
	NotesList(ctx context.Context, uid, companyID, jobID ID) ([]JobNote, error)
	// NoteCreate adds a note (resolving the actor's name) and logs a "Note added" history entry.
	NoteCreate(ctx context.Context, uid, companyID, jobID ID, actor NoteActor, text string) (JobNote, error)
	// NoteUpdate edits a note within the author's 24h window. found is false on a miss; forbidden
	// is true when the actor is not the author or the window has closed.
	NoteUpdate(ctx context.Context, uid, companyID, noteID ID, actor NoteActor, text string) (n JobNote, found, forbidden bool, err error)
	// HistoryList returns a job's audit trail, newest first.
	HistoryList(ctx context.Context, uid, companyID, jobID ID) ([]JobHistoryRow, error)
}

// ---------------------------------------------------------------------------------------
// Challans
// ---------------------------------------------------------------------------------------

// Challan is one delivery-challan log row (Model/Challan.js has no timestamps).
type Challan struct {
	ID          ID
	UID         ID
	CompanyID   ID
	CompanyName string
	Description string
	Date        string
	Type        string
	Quantity    float64
	Amount      float64
	Version     int
}

// ChallanWrite is /challan/new: a cash/billing delivery challan. Node's type-present branch is
// a dead TDZ crash, so only the "In CASH" (no type) path ever creates one; the handler forces
// the type accordingly.
type ChallanWrite struct {
	CompanyName string
	Description string
	Date        string
	Type        string
	Quantity    float64
	Amount      float64
}

type Challans interface {
	List(ctx context.Context, companyID ID) ([]Challan, error)
	// Create inserts a company-scoped challan owned by uid, returning the stored record.
	Create(ctx context.Context, companyID, uid ID, in ChallanWrite) (Challan, error)
}

// ---------------------------------------------------------------------------------------
// Expenses
// ---------------------------------------------------------------------------------------

// ExpenseBank is the populated bank_id (name only).
type ExpenseBank struct {
	ID   ID
	Name string
}

// Expense is money out, tied to a bank. bank_id is populated into Bank.
type Expense struct {
	ID        ID
	CompanyID ID
	UID       ID
	Bank      *ExpenseBank
	Date      string
	Amount    float64
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int
}

// ExpenseFilter narrows a list: an empty field means no filter on it.
type ExpenseFilter struct {
	BankID ID
	From   string
	To     string
}

// ExpenseWrite is the writable field set of /expense/create.
type ExpenseWrite struct {
	BankID ID
	Date   string
	Amount float64
	Notes  string
}

type Expenses interface {
	List(ctx context.Context, companyID ID, f ExpenseFilter) ([]Expense, error)
	// Create records a company-scoped expense owned by uid and returns it with the bank populated
	// (Node's save + findById().populate). For /expense/create.
	Create(ctx context.Context, companyID, uid ID, in ExpenseWrite) (Expense, error)
	// Delete hard-deletes a company-scoped expense (Node's findOneAndDelete). found is false on a
	// miss or another company's row. For /expense/remove.
	Delete(ctx context.Context, companyID, expenseID ID) (found bool, err error)
}

// ---------------------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------------------

// DatedAmount is one amount with its effective date (createdAt||date already resolved), the
// input SumIntoBuckets needs.
type DatedAmount struct {
	Date   time.Time
	Amount float64
}

// ReviewRow is one job review, for the reviews aggregation.
type ReviewRow struct {
	ID            ID
	ChallanNumber string
	JobID         ID
	ClientID      ID
	ClientName    string
	Scores        ReviewScores
	Comment       string
	CreatedAt     time.Time
}

// UnbilledEntry is one not-yet-invoiced entry, for the unbilled report. HasDate is false when
// neither createdAt nor date resolved.
type UnbilledEntry struct {
	Value      float64
	Date       time.Time
	HasDate    bool
	ClientID   ID
	ClientName string
	ClientFirm string
}

// GstLine is one taxable line (amount + the three GST percentages).
type GstLine struct {
	Amount float64
	Cgst   float64
	Sgst   float64
	Igst   float64
}

// GstDoc is one invoice or purchase invoice for the GST report.
type GstDoc struct {
	InvoiceNo string
	Date      string // YYYY-MM-DD (already normalised)
	PartyName string
	GstNo     string
	Lines     []GstLine
}

// SupplierDue is one supplier's outstanding payable.
type SupplierDue struct {
	Name string
	Due  float64
}

// ClientRank is a client with a single ranked money value (sales, dues, or paid).
type ClientRank struct {
	ClientID   ID
	ClientName string
	ClientFirm string
	Value      float64
}

// Analytics serves the reporting reads.
type Analytics interface {
	// ClientRank is one client's ranked total (sales / dues / paid).
	// TopSales/TopCredits/TopPaid return them sorted desc; TopCredits/TopPaid drop non-positive.
	TopSales(ctx context.Context, companyID ID) ([]ClientRank, error)
	TopCredits(ctx context.Context, companyID ID) ([]ClientRank, error)
	TopPaid(ctx context.Context, companyID ID) ([]ClientRank, error)
	// Reviews returns a company's job reviews (newest first), optionally within [from,to] dates.
	Reviews(ctx context.Context, companyID ID, from, to string) ([]ReviewRow, error)
	// GstSales/GstPurchases return the GST report's documents (invoices / purchase invoices)
	// with their taxable lines and party details.
	GstSales(ctx context.Context, companyID ID) ([]GstDoc, error)
	GstPurchases(ctx context.Context, companyID ID) ([]GstDoc, error)
	// Receipts returns every money-in event (invoice_received + batch_receives) with its
	// effective date and amount, for the payout-by-weekday chart.
	Receipts(ctx context.Context, companyID ID) ([]DatedAmount, error)
	// OutstandingInvoices returns each unpaid invoice's due amount and billed date (due>0).
	OutstandingInvoices(ctx context.Context, companyID ID) ([]DatedAmount, error)
	// UnbilledEntries returns entries not yet invoiced (has_issued false), for the billing-lag report.
	UnbilledEntries(ctx context.Context, companyID ID) ([]UnbilledEntry, error)
	// PaymentGaps returns, per fully-paid invoice, days from invoice date to its last receipt.
	PaymentGaps(ctx context.Context, companyID ID) ([]float64, error)
	// PendingSince returns the effective date of each partially-paid invoice.
	PendingSince(ctx context.Context, companyID ID) ([]time.Time, error)
	// Payables totals what is owed to suppliers, and the per-supplier breakdown (desc).
	Payables(ctx context.Context, companyID ID) (total float64, bySupplier []SupplierDue, err error)
	// RevenueSeries returns the billed and collected dated amounts for the revenue chart.
	// source "all" reads entries (total, amount); "invoiced" reads invoices (totalAmount) and
	// invoice_received (amount). Each amount carries createdAt when set, else the typed date.
	RevenueSeries(ctx context.Context, companyID ID, source string) (billed, collected []DatedAmount, err error)
	// Cashflow returns the cashflow dashboard's raw inputs (all company-scoped, dates
	// normalized). The handler does the date filtering, month bucketing, and cost matching.
	Cashflow(ctx context.Context, companyID ID) (CashflowData, error)
	// ProductionWip gathers the open-work cards, the "Queue advanced" history (ascending) and the
	// holder names for /analytics/production/wip. A job with no rows is itself one card.
	ProductionWip(ctx context.Context, companyID ID) (ProductionWipData, error)
	// ProductionThroughput gathers the "Queue advanced" events since windowStart and the jobs
	// created since then (with their row queues) for /analytics/production/throughput.
	ProductionThroughput(ctx context.Context, companyID ID, windowStart time.Time) (ProductionThroughputData, error)
}

// CashflowRow is a dated money row (receipts, transfers, supplier payments, purchase invoices).
type CashflowRow struct {
	Date   string // normalized YYYY-MM-DD
	Amount float64
}

// CashflowInvoice carries both the billed total and the collected amount of one invoice.
type CashflowInvoice struct {
	Date        string
	TotalAmount float64
	Amount      float64
}

// CashflowSoldEntry is an invoiced entry's sold volume, keyed by material name so the handler
// can look up the cost basis (entries carry only the sell rate).
type CashflowSoldEntry struct {
	Date     string
	Material string
	Qty      float64
}

// CashflowWastage carries the selling total and the cost basis.
type CashflowWastage struct {
	Date      string
	Total     float64
	CostTotal float64
}

// CashflowMaterial is a product's name and buy/sell rates, for cost matching and the margin table.
type CashflowMaterial struct {
	Name         string
	MaterialRate float64
	PurchaseRate float64
}

// CashflowData is the cashflow report's raw inputs.
type CashflowData struct {
	Invoices         []CashflowInvoice
	Received         []CashflowRow
	BatchReceives    []CashflowRow
	SupplierPayments []CashflowRow
	PurchaseInvoices []CashflowRow
	SoldEntries      []CashflowSoldEntry
	Wastages         []CashflowWastage
	Materials        []CashflowMaterial
}

// ---------------------------------------------------------------------------------------
// Wastage
// ---------------------------------------------------------------------------------------

type Wastage struct {
	ID           ID
	UID          ID
	CompanyID    ID
	MaterialName string
	Rate         float64
	PurchaseRate float64
	CostTotal    float64
	Length       float64
	Height       float64
	Total        float64
	Date         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

// WastageWrite is the field set of /wastage/add. Rate/Length/Height/Total are numbers Mongoose
// casts from the submitted strings; PurchaseRate/CostTotal default to 0 when omitted.
type WastageWrite struct {
	MaterialName string
	Rate         float64
	PurchaseRate float64
	CostTotal    float64
	Length       float64
	Height       float64
	Total        float64
	Date         string
}

// MaterialAvg is one distinct product name for the wastage picker (/wastage/materials): names
// are collapsed case-insensitively and the rates averaged across the duplicates.
type MaterialAvg struct {
	MaterialName string
	MaterialRate float64
	PurchaseRate float64
}

type Wastages interface {
	List(ctx context.Context, companyID ID) ([]Wastage, error)
	// Create inserts a company-scoped wastage record owned by uid, returning the stored row.
	Create(ctx context.Context, companyID, uid ID, in WastageWrite) (Wastage, error)
	// MaterialsSummary is /wastage/materials: distinct products by lowercased name with averaged
	// rates, name-sorted - the wastage form's material picker.
	MaterialsSummary(ctx context.Context, companyID ID) ([]MaterialAvg, error)
	// Delete hard-deletes a company-scoped wastage record (Node's findOneAndDelete). found is
	// false on a miss (-> 404). A malformed id is ErrBadID on the document store (500).
	Delete(ctx context.Context, companyID, wastageID ID) (found bool, err error)
}

// ---------------------------------------------------------------------------------------
// Batch receives (client lump payments)
// ---------------------------------------------------------------------------------------

// ReceiptDestination is one place a receipt's money went, with the resolved label.
type ReceiptDestination struct {
	Kind   string // "job" | "invoice" | "purchase-invoice"
	ID     string
	Label  string
	Amount float64
}

// BatchReceive is one client lump payment with its client/bank populated and destinations
// resolved (which jobs/invoices it settled).
type BatchReceive struct {
	ID           ID
	UID          ID
	CompanyID    ID
	BankName     string
	BankID       ID
	Date         string
	Amount       float64
	Note         string
	Mode         string
	ClientName   string
	ClientFirm   string
	ClientPhone  string
	ClientID     ID
	Destinations []ReceiptDestination
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

// BatchOpenJob is one not-fully-paid job for the manual picker / auto preview.
type BatchOpenJob struct {
	ID            ID
	ChallanNumber string
	ReceivedDate  string
	Total         float64
	Advance       float64
	Remaining     float64
	EntryCount    int
}

// BatchAllocInput is one manual job allocation submitted with a batch receive.
type BatchAllocInput struct {
	JobID  ID
	Amount float64
}

// BatchReceiveWrite is the field set of /batch-receive/create. In auto mode Allocations is
// ignored and the store spreads Amount over the client's unpaid jobs oldest-first; in manual
// mode the store applies exactly these allocations.
type BatchReceiveWrite struct {
	ClientID    ID
	Amount      float64
	Mode        string // "auto" | "manual"
	Note        string
	BankID      ID
	Date        string
	Allocations []BatchAllocInput
}

type BatchReceives interface {
	List(ctx context.Context, uid, companyID, clientID ID) ([]BatchReceive, error)
	// OpenJobs returns a client's not-fully-paid jobs (advance < total), oldest first.
	OpenJobs(ctx context.Context, uid, companyID, clientID ID) ([]BatchOpenJob, error)
	// Create records a client lump payment, applying it to jobs (and downstream to their
	// invoiced entries/invoices) per the mode, and returns the resolved record.
	Create(ctx context.Context, uid, companyID ID, in BatchReceiveWrite) (BatchReceive, error)
	// Delete reverses everything the record applied (job advances, entry/invoice propagation)
	// and removes it. found is false on a miss.
	Delete(ctx context.Context, uid, companyID, batchID ID) (found bool, err error)
	// CreateSimple is /client/batchUpdate: a plain logged receipt with no allocation and no
	// entry side effects (Node's auto-apply is disabled). found is false when the client is not
	// the caller's. date defaults to today when blank.
	CreateSimple(ctx context.Context, uid, companyID, clientID ID, amount float64, date, note string) (b BatchReceive, found bool, err error)
	// UpdateSimple is /client/batchReceiveUpdate: patch amount/note/date on a receipt (nil = leave
	// as is), company-scoped (Node was unscoped). found is false on a miss.
	UpdateSimple(ctx context.Context, companyID, batchID ID, amount *float64, note, date *string) (b BatchReceive, found bool, err error)
	// DeleteSimple is /client/batchReceiveDelete: a plain delete with no allocation reversal
	// (Node's fill logic is disabled), company-scoped. found is false on a miss.
	DeleteSimple(ctx context.Context, companyID, batchID ID) (found bool, err error)
}

// SupplierPayment is one payment to a supplier, mirror of BatchReceive (supplier/bank
// populated, purchase-invoice destinations resolved).
type SupplierPayment struct {
	ID            ID
	UID           ID
	CompanyID     ID
	SupplierID    ID
	SupplierName  string
	SupplierFirm  string
	SupplierPhone string
	BankID        ID
	BankName      string
	Date          string
	Amount        float64
	Note          string
	Mode          string
	Destinations  []ReceiptDestination
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int
}

// SupplierOpenInvoice is one not-fully-paid purchase invoice for the manual picker / auto preview.
type SupplierOpenInvoice struct {
	ID            ID
	InvoiceNumber string
	Date          string
	Total         float64
	Amount        float64
	Due           float64
}

// SupplierAllocInput is one manual purchase-invoice allocation submitted with a payment.
type SupplierAllocInput struct {
	InvoiceID ID
	Amount    float64
}

// SupplierPaymentWrite is the field set of /supplier-payment/create.
type SupplierPaymentWrite struct {
	SupplierID  ID
	Amount      float64
	Mode        string // "auto" | "manual"
	Note        string
	BankID      ID
	Date        string
	Allocations []SupplierAllocInput
}

// SupplierPayStatus is the outcome of a supplier-payment create.
type SupplierPayStatus int

const (
	SupplierPayOK              SupplierPayStatus = iota
	SupplierPayAutoUnallocated                   // auto: payment exceeds total outstanding
	SupplierPayInvoiceNotFound                   // manual: a named invoice is gone
	SupplierPayOverInvoice                       // manual: an allocation exceeds that invoice's due
)

// SupplierPayResult carries the status plus the amounts a refusal message needs.
type SupplierPayResult struct {
	Status    SupplierPayStatus
	Remaining float64 // AutoUnallocated
	Applied   float64 // OverInvoice
	Due       float64 // OverInvoice
}

type SupplierPayments interface {
	List(ctx context.Context, uid, companyID ID) ([]SupplierPayment, error)
	// OpenInvoices returns a supplier's not-fully-paid purchase invoices (amount < total), oldest first.
	OpenInvoices(ctx context.Context, uid, companyID, supplierID ID) ([]SupplierOpenInvoice, error)
	// Create records a payment to a supplier, applying it to their purchase invoices per the mode
	// (auto = oldest-first, refusing an unallocatable excess; manual = the submitted allocations,
	// each capped at that invoice's due), and returns the resolved record.
	Create(ctx context.Context, uid, companyID ID, in SupplierPaymentWrite) (SupplierPayment, SupplierPayResult, error)
	// Delete reverses each allocation on the paid invoices (never below zero) and removes the
	// record. found is false on a miss.
	Delete(ctx context.Context, uid, companyID, paymentID ID) (found bool, err error)
}

// ---------------------------------------------------------------------------------------
// Trash (the delete vault)
// ---------------------------------------------------------------------------------------

// TrashRecord is one deleted entry snapshot, client populated.
type TrashRecord struct {
	ID            ID
	ClientID      ID
	ClientName    string
	ClientFirm    string
	Description   string
	Material      string
	Rate          float64
	Qty           float64
	HasDimensions *bool
	Length        string
	Width         string
	Date          string
	Amount        float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int
}

type Trash interface {
	// PasswordHash returns the vault password hash for a user, or "" if the vault is unset.
	PasswordHash(ctx context.Context, uid ID) (string, error)
	// SetPassword upserts the vault password hash.
	SetPassword(ctx context.Context, uid ID, hash string) error
	// List returns a company's trashed records, client populated.
	List(ctx context.Context, companyID ID) ([]TrashRecord, error)
}

// ---------------------------------------------------------------------------------------
// Inventory
// ---------------------------------------------------------------------------------------

// InvMaterial/InvPurchaseRow/InvJobRow/InvWastageRow are the raw rows the stock report rolls up
// (kept store-side so internal/inventorymath stays pure).
type InvMaterial struct {
	ID           string
	MaterialName string
	Unit         string
	PurchaseRate float64
}
type InvPurchaseRow struct {
	ID        string
	Material  string
	Qty       float64
	Rate      float64
	CompanyID string
}
type InvJobRow struct {
	ID            string
	Material      string
	Length        string
	Width         string
	Qty           float64
	HasDimensions *bool
	CompanyID     string
}
type InvWastageRow struct {
	ID           string
	MaterialName string
	Length       float64
	Height       float64
	CompanyID    string
}

// InventoryData is everything the stock report reads, for one company (report sharing across an
// owner's companies is a Postgres follow-up; this fails closed to the acting company).
type InventoryData struct {
	Materials    []InvMaterial
	PurchaseRows []InvPurchaseRow
	JobRows      []InvJobRow
	WastageRows  []InvWastageRow
}

type Inventory interface {
	Data(ctx context.Context, companyID ID, from, to string) (InventoryData, error)
}

// ---------------------------------------------------------------------------------------
// Statistics (dashboard)
// ---------------------------------------------------------------------------------------

// StatEntry is the square-footage-relevant subset of an entry.
type StatEntry struct {
	Material      string
	Length        string
	Width         string
	Qty           float64
	Date          string
	HasDimensions *bool
}

type Statistics interface {
	// Clients returns the company's clients for the dashboard dropdown.
	Clients(ctx context.Context, companyID ID) ([]LookupClient, error)
	// StatEntries returns entries matching the total flag (paid: total==0, else total>0) and
	// whose date string contains monthName (the Node month-regex, #31), optionally one client.
	StatEntries(ctx context.Context, companyID ID, monthName string, paid bool, clientID ID) ([]StatEntry, error)
	// Client returns one client's names, or ErrNotFound.
	Client(ctx context.Context, companyID, clientID ID) (LookupClient, error)
}

// ---------------------------------------------------------------------------------------
// Ledger
// ---------------------------------------------------------------------------------------

// LedgerTxn is one ledger line before balancing (a bill or a receipt).
type LedgerTxn struct {
	Date      string
	Type      string // "Sales Invoice" | "Receipts"
	InvoiceNo string
	Bill      float64
	Receipt   float64
	Seq       int // 0 bills, 1 receipts, so a same-day receipt sorts after the bill
}

// LedgerClientData is a client's statement inputs. Node's /ledger/client computes the opening
// balance from the transactions that fall before `from` (starting at zero) and never consults
// the client's stored openingBalance, so that field is deliberately absent here.
type LedgerClientData struct {
	Found         bool
	ClientName    string
	ClientFirm    string
	ClientGST     string
	ClientAddress string
	Txns          []LedgerTxn
}

type Ledger interface {
	ClientStatement(ctx context.Context, companyID, clientID ID) (LedgerClientData, error)
	// ClientDues is the all-client receivables report's inputs: per-client aggregates plus the
	// client directory to label and filter them by (Helpers/ClientDues.js, /ledger/dues).
	ClientDues(ctx context.Context, companyID ID) (ClientDuesData, error)
}

// SupplierDueInvoice is one purchase invoice reduced to what the payables dues report needs.
type SupplierDueInvoice struct {
	SupplierID ID
	Total      float64
	Amount     float64
}

// SupplierInfo labels a supplier in the payables reports (Person, type Supplier, scoped by uid).
type SupplierInfo struct {
	ID      ID
	Name    string
	Firm    string
	Phone   string
	GST     string
	Address string
}

// SupplierDuesData feeds /purchase-report/dues: the company's purchase invoices plus the uid's
// supplier directory to label and filter them.
type SupplierDuesData struct {
	Invoices  []SupplierDueInvoice
	Suppliers map[string]SupplierInfo
}

// SupplierLedgerInvoice / SupplierLedgerPayment are the chronological inputs of one supplier's
// ledger (/purchase-report/supplier).
type SupplierLedgerInvoice struct {
	Date          string
	InvoiceNumber string
	Total         float64
}
type SupplierLedgerPayment struct {
	Date   string
	Amount float64
	Note   string
}

// SupplierStatementData is one supplier's ledger inputs; Found is false for an unknown supplier.
type SupplierStatementData struct {
	Found    bool
	Supplier SupplierInfo
	Invoices []SupplierLedgerInvoice
	Payments []SupplierLedgerPayment
}

// PurchaseReport is the payables side of the reporting routes (routes/PurchaseReport.js), the
// mirror of Ledger: it returns raw aggregates and the handler formats them.
type PurchaseReport interface {
	// SupplierDues returns the company's purchase invoices and the uid's supplier directory.
	SupplierDues(ctx context.Context, uid, companyID ID) (SupplierDuesData, error)
	// SupplierStatement returns one supplier's invoices and payments for its running ledger.
	SupplierStatement(ctx context.Context, uid, companyID, supplierID ID) (SupplierStatementData, error)
}

// Store is everything together, so main wires one value rather than six.
// ---------------------------------------------------------------------------------------
// Settings (per-owner UI preferences, from routes/Settings.js + Model/UserSetting.js)
// ---------------------------------------------------------------------------------------

// SettingsSection names one of the two independent preference blobs a UserSetting holds.
// They live side by side so dragging a column cannot race a font change and overwrite it,
// exactly as Model/UserSetting.js keeps `appearance` and `tables` as separate Mixed fields.
type SettingsSection string

const (
	SettingsAppearance SettingsSection = "appearance"
	SettingsTables     SettingsSection = "tables"
)

// Settings is the per-person UI-preference store. The owner is the Person for an employee
// session and the User for an admin one (personId || uid), never the company - a preference
// follows whoever is logged in, as Model/UserSetting.js explains.
type Settings interface {
	// Get returns the owner's stored blob for one section as raw JSON. It returns (nil, nil)
	// when the owner has no row at all - the first-login case, where Node's `row ? row.section
	// : null` sends null. Once any update has created the row, the OTHER section reads back as
	// `{}` (the schema default that setDefaultsOnInsert writes), never null.
	Get(ctx context.Context, ownerID ID, section SettingsSection) (json.RawMessage, error)

	// Set upserts one section for the owner and returns the stored value. On the insert that
	// creates the row, the other section is initialised to `{}`, reproducing Mongoose's
	// setDefaultsOnInsert so a later read of it is `{}` and not null. value is a validated JSON
	// object (the handler has already rejected non-objects with the Node 422).
	Set(ctx context.Context, ownerID ID, section SettingsSection, value json.RawMessage) (json.RawMessage, error)
}

type Store interface {
	Sessions() Sessions
	Settings() Settings
	Days() Days
	Units() Units
	Users() Users
	Alerts() Alerts
	Banks() Banks
	Clients() Clients
	Companies() Companies
	Materials() Materials
	People() People
	Quotations() Quotations
	PurchaseInvoices() PurchaseInvoices
	PurchaseOrders() PurchaseOrders
	Enquiries() Enquiries
	RegistrationTokens() RegistrationTokens
	PasswordResetRequests() PasswordResetRequests
	Invoices() Invoices
	Entries() Entries
	Sheets() Sheets
	Lookups() Lookups
	Jobs() Jobs
	JobNotes() JobNotes
	Challans() Challans
	Expenses() Expenses
	Analytics() Analytics
	Wastages() Wastages
	BatchReceives() BatchReceives
	SupplierPayments() SupplierPayments
	Trash() Trash
	Inventory() Inventory
	Statistics() Statistics
	Ledger() Ledger
	PurchaseReport() PurchaseReport

	// Ping is what the health check uses: a service that is up but cannot reach its database
	// is not healthy, and a TCP check would call it healthy.
	Ping(ctx context.Context) error

	Close(ctx context.Context) error
}
