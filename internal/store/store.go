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
	CreateSession(ctx context.Context, s NewSession) (Session, error)
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
	ClientName     string
	ClientFirm     string
	ClientPhone    string
	ClientGST      string
	ClientAddress  string
	OpeningBalance float64
	Sharing        *[]ID
}

type Clients interface {
	// Visible returns the clients (uid) may READ from company companyID: its own, legacy rows
	// with a null company, and rows shared with it. This is the read side of #1 - reads widen;
	// a write path uses company-only scope and never calls this.
	Visible(ctx context.Context, uid, companyID ID) ([]Client, error)
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
	IsDefault bool
	IsActive  bool
}

type Companies interface {
	// List returns the admin's ACTIVE companies for the switcher, default first.
	List(ctx context.Context, uid ID) ([]Company, error)
	// Active returns one company the admin owns, for the letterhead. ErrNotFound if missing.
	Active(ctx context.Context, companyID, uid ID) (Company, error)
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

type Materials interface {
	// Visible returns the products company companyID may READ: its own plus any shared with it.
	// The read half of #1 (Helpers/SharedRecords.visibleScope) - a write path would not widen.
	Visible(ctx context.Context, companyID ID) ([]Material, error)
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

type People interface {
	// List returns the owner's people (uid), optionally filtered by type ("Employee"/"Supplier").
	// An empty personType means no filter.
	List(ctx context.Context, uid ID, personType string) ([]Person, error)
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

type Quotations interface {
	// List returns a company's quotations (newest first), client_id populated, optionally
	// filtered by clientID (""=no filter).
	List(ctx context.Context, uid, companyID, clientID ID) ([]Quotation, error)
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

type PurchaseInvoices interface {
	// List returns a company's purchase invoices (newest first), supplier_id populated.
	List(ctx context.Context, uid, companyID ID) ([]PurchaseInvoice, error)
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

// Entry is one billed line on an invoice (Model/Entry.js), the pricing fields plus display.
type Entry struct {
	ID            ID
	Description   string
	Material      string
	Hsn           string
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
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int
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

type Invoices interface {
	// List returns a company's invoices with client populated and entries loaded (#6 batched).
	List(ctx context.Context, companyID ID) ([]Invoice, error)
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

type Challans interface {
	List(ctx context.Context, companyID ID) ([]Challan, error)
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

type Expenses interface {
	List(ctx context.Context, companyID ID, f ExpenseFilter) ([]Expense, error)
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

type Wastages interface {
	List(ctx context.Context, companyID ID) ([]Wastage, error)
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

type BatchReceives interface {
	List(ctx context.Context, uid, companyID, clientID ID) ([]BatchReceive, error)
}

// Store is everything together, so main wires one value rather than six.
type Store interface {
	Sessions() Sessions
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
	Invoices() Invoices
	Lookups() Lookups
	Jobs() Jobs
	Challans() Challans
	Expenses() Expenses
	Analytics() Analytics
	Wastages() Wastages
	BatchReceives() BatchReceives

	// Ping is what the health check uses: a service that is up but cannot reach its database
	// is not healthy, and a TCP check would call it healthy.
	Ping(ctx context.Context) error

	Close(ctx context.Context) error
}
