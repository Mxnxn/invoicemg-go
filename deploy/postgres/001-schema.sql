-- The relational shape of InvoiceMG.
--
-- Applied automatically: the postgres image runs everything in /docker-entrypoint-initdb.d
-- once, on an empty data directory. To re-apply, drop the volume:
--
--   docker compose -f docker-compose.local.yml down -v
--
-- Two decisions worth knowing before reading further.
--
-- IDs ARE TEXT, NOT UUID. Every id in the live system is a 24-character Mongo ObjectID hex
-- string. Keeping them as text means a future import copies ids across unchanged, so anything
-- holding one - a bookmarked /sheet/<id>, a WhatsApp link, a row in another system - still
-- resolves. A uuid column would force every id to be rewritten and every external reference to
-- break. New rows created here use gen_ulid() below, which is sortable by creation time the
-- way an ObjectID is.
--
-- JOB ROWS ARE THEIR OWN TABLE. In Mongo they are a subdocument array, which is why counting
-- "cards short of Done" needs $elemMatch and an aggregate. Here it is a join and a WHERE, and
-- the queue stage can finally be indexed. This is the single biggest reason to want SQL for
-- this application - the reports all count rows, and rows were never addressable.

-- gen_ulid() below needs gen_random_bytes(), which lives in pgcrypto - not in core, and not
-- enabled by default on the postgres image. Without this the whole schema aborts on the
-- function definition and NOTHING is created.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- A sortable, ObjectID-shaped identifier: 4-byte seconds + 8 random bytes, hex. Same 24
-- characters and the same "sorts by creation time" property, so a mixed table of imported and
-- new ids still orders correctly.
CREATE OR REPLACE FUNCTION gen_ulid() RETURNS text AS $$
    SELECT lpad(to_hex(floor(extract(epoch FROM now()))::bigint), 8, '0')
        || encode(gen_random_bytes(8), 'hex');
$$ LANGUAGE sql VOLATILE;

-- ---------------------------------------------------------------------------------------
-- Tenancy
-- ---------------------------------------------------------------------------------------

CREATE TABLE users (
    id            text PRIMARY KEY DEFAULT gen_ulid(),
    email         text NOT NULL UNIQUE,
    -- bcrypt, written by bcryptjs today and readable by golang.org/x/crypto/bcrypt: both
    -- speak $2a$ and $2b$, so hashes move between the two implementations unchanged.
    password      text NOT NULL,
    name          text NOT NULL DEFAULT '',
    firm          text NOT NULL DEFAULT '',
    role          text NOT NULL DEFAULT 'admin',
    -- What actually locks a login out. NULL means no expiry.
    active_until  timestamptz,
    totp_enabled  boolean NOT NULL DEFAULT false,
    totp_secret   text,
    -- How many company profiles this admin may create. The switcher's Add control reads it.
    company_limit integer NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE companies (
    id          text PRIMARY KEY DEFAULT gen_ulid(),
    uid         text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        text NOT NULL,
    firm        text NOT NULL DEFAULT '',
    phone       text NOT NULL DEFAULT '',
    gst         text NOT NULL DEFAULT '',
    address     text NOT NULL DEFAULT '',
    -- The logo/letterhead image path, embedded in customer-facing PDFs and the alert page.
    url         text NOT NULL DEFAULT '',
    upi_qr      text NOT NULL DEFAULT '',
    account_no  text NOT NULL DEFAULT '',
    ifsc        text NOT NULL DEFAULT '',
    bank_name   text NOT NULL DEFAULT '',
    is_active   boolean NOT NULL DEFAULT true,
    is_default  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX companies_uid_idx ON companies (uid);
-- One default per user. A partial unique index says that in the schema rather than in every
-- route that sets a default - the Mongo version relies on application code and can drift.
CREATE UNIQUE INDEX companies_one_default_per_user ON companies (uid) WHERE is_default;

CREATE TABLE user_sessions (
    id          text PRIMARY KEY DEFAULT gen_ulid(),
    token       text NOT NULL UNIQUE,
    uid         text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        text NOT NULL DEFAULT 'admin',
    person_id   text,
    -- "feature:action" entries, snapshotted from the Person at login. An array rather than a
    -- join table because it IS a snapshot: changing someone's permissions must not
    -- retroactively change what their live session was allowed to do.
    permissions text[] NOT NULL DEFAULT '{}',
    is_active   boolean NOT NULL DEFAULT true,
    remembered  boolean NOT NULL DEFAULT false,
    -- NULL means never expires, matching Model/UserSession.js.
    expires_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_sessions_token_idx ON user_sessions (token);

-- Which company a given browser TAB is acting as. One login, several tabs, several companies.
CREATE TABLE company_sessions (
    token      text NOT NULL,
    tab_id     text NOT NULL,
    uid        text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id text NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (token, tab_id)
);

-- ---------------------------------------------------------------------------------------
-- Parties
-- ---------------------------------------------------------------------------------------

CREATE TABLE clients (
    id           text PRIMARY KEY DEFAULT gen_ulid(),
    company_id   text REFERENCES companies(id) ON DELETE CASCADE,
    uid          text REFERENCES users(id) ON DELETE SET NULL,
    client_name  text NOT NULL DEFAULT '',
    client_firm  text NOT NULL DEFAULT '',
    client_phone text NOT NULL DEFAULT '',
    client_gst   text NOT NULL DEFAULT '',
    client_address text NOT NULL DEFAULT '',
    opening_balance numeric(14,2) NOT NULL DEFAULT 0,
    -- Which companies this record is shared with (Phase 2). A Postgres array mirrors Mongo's
    -- sharing.companies; a read widens by membership, a write never does (#1).
    shared_company_ids text[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX clients_company_idx ON clients (company_id);

-- ---------------------------------------------------------------------------------------
-- Work
-- ---------------------------------------------------------------------------------------

CREATE TABLE jobs (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    company_id     text REFERENCES companies(id) ON DELETE CASCADE,
    uid            text REFERENCES users(id) ON DELETE SET NULL,
    client_id      text REFERENCES clients(id) ON DELETE SET NULL,
    challan_number text NOT NULL,
    -- The date an admin puts on the job-id. A DATE, not a timestamp: it is a calendar day,
    -- and storing it as a timestamp is what produced the UTC-midnight day-shift bug in the
    -- JavaScript version. The day list is grouped on this column.
    received_date  date,
    total          numeric(14,2) NOT NULL DEFAULT 0,
    advance        numeric(14,2) NOT NULL DEFAULT 0,
    queue          text NOT NULL DEFAULT 'Created',
    progress       text NOT NULL DEFAULT 'Unassigned',
    unlocked       boolean NOT NULL DEFAULT false,
    employee_id    text,
    vendor_id      text,
    queue_order    text[] NOT NULL DEFAULT '{}',
    -- The two customer-alert channels (created/done), as JSONB - {status,statusAt,error,
    -- sentAt,count,rowIds,signature}. Empty until a message goes out; the board derives
    -- the send buttons from them (internal/joblifecycle).
    created_alert  jsonb NOT NULL DEFAULT '{}',
    done_alert     jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_company_idx ON jobs (company_id);
-- The day list groups on this, so it is worth an index rather than a sequential scan that
-- grows with every job ever created.
CREATE INDEX jobs_received_date_idx ON jobs (company_id, received_date);
CREATE UNIQUE INDEX jobs_challan_per_company ON jobs (company_id, challan_number);

CREATE TABLE job_rows (
    id           text PRIMARY KEY DEFAULT gen_ulid(),
    job_id       text NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    row_id       text NOT NULL DEFAULT '',
    position     integer NOT NULL DEFAULT 0,
    material     text NOT NULL DEFAULT '',
    description  text NOT NULL DEFAULT '',
    has_dimensions boolean NOT NULL DEFAULT true,
    length       text NOT NULL DEFAULT '1',
    width        text NOT NULL DEFAULT '1',
    qty          numeric(14,3) NOT NULL DEFAULT 1,
    rate         numeric(14,2) NOT NULL DEFAULT 0,
    -- A row is taxed on one side or the other, never both: cgst+sgst for an intrastate sale,
    -- igst for an interstate one. The CHECK is what the Mongo version could only hope for -
    -- there, a row carrying both is silently taxed twice.
    cgst         numeric(6,2) NOT NULL DEFAULT 0,
    sgst         numeric(6,2) NOT NULL DEFAULT 0,
    igst         numeric(6,2) NOT NULL DEFAULT 0,
    discount     numeric(14,2) NOT NULL DEFAULT 0,
    charges      numeric(14,2) NOT NULL DEFAULT 0,
    queue        text NOT NULL DEFAULT 'Created',
    progress     text NOT NULL DEFAULT 'Assign',
    entry_id     text,
    employee_id  text,
    quotation_id text,
    queue_order  text[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT job_rows_one_tax_side CHECK (igst = 0 OR (cgst = 0 AND sgst = 0))
);
CREATE INDEX job_rows_job_idx ON job_rows (job_id);
-- "Which job-ids still have work on them" is the dashboard's alert, and in Mongo it needs an
-- $elemMatch over a subdocument array. Here the stage is a column and can be indexed.
CREATE INDEX job_rows_open_idx ON job_rows (job_id) WHERE queue <> 'Done';

-- ---------------------------------------------------------------------------------------
-- Customer reviews (routes/Alert.js)
-- ---------------------------------------------------------------------------------------

-- One review per job, left from the public alert link. company_id and uid are denormalised
-- onto the row - every report filters by company, and a review reachable only by joining to the
-- job would be the one collection that cannot be. The parties are snapshotted, like every
-- document here, so a rename or deletion cannot make an old review unreadable. Scores are flat
-- columns with a CHECK rather than a nested object: 1-5 is enforced by the database, not hoped
-- for. The unique index on job_id is the real guard - the page is unauthenticated, so a second
-- submit is a refresh away.
CREATE TABLE job_reviews (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    uid            text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id     text NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    job_id         text NOT NULL UNIQUE REFERENCES jobs(id) ON DELETE CASCADE,
    client_id      text NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    jobcard_id     text NOT NULL DEFAULT '',
    challan_number text NOT NULL DEFAULT '',
    client_name    text NOT NULL DEFAULT '',
    quality        smallint NOT NULL CHECK (quality       BETWEEN 1 AND 5),
    speed          smallint NOT NULL CHECK (speed         BETWEEN 1 AND 5),
    communication  smallint NOT NULL CHECK (communication BETWEEN 1 AND 5),
    satisfaction   smallint NOT NULL CHECK (satisfaction  BETWEEN 1 AND 5),
    overall        smallint NOT NULL CHECK (overall       BETWEEN 1 AND 5),
    comment        text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX job_reviews_company_created_idx ON job_reviews (company_id, created_at DESC);
CREATE INDEX job_reviews_client_idx ON job_reviews (client_id);

-- A day that work was collected under. Historically the only thing that made a day exist;
-- now days come from jobs.received_date and this survives for days that predate that.
CREATE TABLE sheets (
    id         text PRIMARY KEY DEFAULT gen_ulid(),
    company_id text REFERENCES companies(id) ON DELETE CASCADE,
    uid        text REFERENCES users(id) ON DELETE SET NULL,
    date       date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sheets_company_date_idx ON sheets (company_id, date);

-- ---------------------------------------------------------------------------------------
-- Configuration
-- ---------------------------------------------------------------------------------------

CREATE TABLE units (
    id         text PRIMARY KEY DEFAULT gen_ulid(),
    uid        text REFERENCES users(id) ON DELETE SET NULL,
    company_id text REFERENCES companies(id) ON DELETE CASCADE,
    name       text NOT NULL,
    -- name with case, spacing and punctuation removed, so "SQ. Ft", "sq ft" and "SQ.FT"
    -- collapse to one value. Maintained by the store layer, exactly as the Mongoose
    -- pre("validate") hook maintains it - and unique, which is the actual guarantee. An
    -- application-level "does this exist?" check is racy; two requests can both read "no".
    key        text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX units_key_per_company ON units (company_id, key);

-- Products (materials). Shared like clients (#1): a read widens by shared_company_ids, a write
-- never does. priceHistory is a child table rather than a JSON column so a rate change is one
-- INSERT and the history is queryable.
CREATE TABLE materials (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    uid            text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id     text REFERENCES companies(id) ON DELETE CASCADE,
    material_name  text NOT NULL DEFAULT '',
    material_rate  numeric(14,2) NOT NULL DEFAULT 0,
    purchase_rate  numeric(14,2) NOT NULL DEFAULT 0,
    -- Stocked unit, by name (matches how a job row stores it). Blank on products predating it.
    unit           text NOT NULL DEFAULT '',
    hsn            text NOT NULL DEFAULT '',
    tax            numeric(6,2) NOT NULL DEFAULT 0,
    shared_company_ids text[] NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX materials_company_idx ON materials (company_id);

-- People: employees (portal logins) and suppliers, scoped by owner (uid), filtered by `type`.
-- The notify_po_* flags are nullable on purpose - Model/Person.js gives them no default, so an
-- unset flag is ABSENT on the wire, not false; the handler omits a NULL rather than sending it.
CREATE TABLE persons (
    id            text PRIMARY KEY DEFAULT gen_ulid(),
    uid           text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          text NOT NULL DEFAULT '',
    type          text NOT NULL DEFAULT '',
    email         text,
    phone         text NOT NULL DEFAULT '',
    firm          text NOT NULL DEFAULT '',
    address       text NOT NULL DEFAULT '',
    gst           text NOT NULL DEFAULT '',
    opening_balance numeric(14,2) NOT NULL DEFAULT 0,
    is_active     boolean NOT NULL DEFAULT true,
    permissions   text[] NOT NULL DEFAULT '{}',
    notify_po_created   boolean,
    notify_po_updated   boolean,
    notify_po_confirmed boolean,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX persons_uid_idx ON persons (uid);

-- Quotations. company+uid scoped; the list populates client_id into a nested client object.
CREATE TABLE quotations (
    id               text PRIMARY KEY DEFAULT gen_ulid(),
    uid              text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id       text REFERENCES companies(id) ON DELETE CASCADE,
    client_id        text REFERENCES clients(id) ON DELETE SET NULL,
    quotation_number text NOT NULL,
    date             text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quotations_company_idx ON quotations (company_id);
CREATE UNIQUE INDEX quotations_number_per_company ON quotations (company_id, quotation_number);

CREATE TABLE quotation_rows (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    quotation_id   text NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
    position       integer NOT NULL DEFAULT 0,
    material       text NOT NULL DEFAULT '',
    description    text NOT NULL DEFAULT '',
    has_dimensions boolean NOT NULL DEFAULT true,
    length         text NOT NULL DEFAULT '1',
    width          text NOT NULL DEFAULT '1',
    qty            numeric(14,3) NOT NULL DEFAULT 1,
    rate           numeric(14,2) NOT NULL DEFAULT 0,
    cgst           numeric(6,2) NOT NULL DEFAULT 0,
    sgst           numeric(6,2) NOT NULL DEFAULT 0,
    igst           numeric(6,2) NOT NULL DEFAULT 0,
    discount       numeric(14,2) NOT NULL DEFAULT 0,
    charges        numeric(14,2) NOT NULL DEFAULT 0,
    -- Set once this row has started a job, so it cannot be added twice.
    job_id         text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quotation_rows_quotation_idx ON quotation_rows (quotation_id);

-- Purchase invoices (supplier bills). supplier_id references a person of type Supplier and is
-- populated into a nested object on the list.
CREATE TABLE purchase_invoices (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    uid            text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id     text REFERENCES companies(id) ON DELETE CASCADE,
    supplier_id    text REFERENCES persons(id) ON DELETE SET NULL,
    date           text NOT NULL,
    invoice_number text NOT NULL,
    total          numeric(14,2) NOT NULL DEFAULT 0,
    amount         numeric(14,2) NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX purchase_invoices_company_idx ON purchase_invoices (company_id);

CREATE TABLE purchase_invoice_rows (
    id             text PRIMARY KEY DEFAULT gen_ulid(),
    invoice_id     text NOT NULL REFERENCES purchase_invoices(id) ON DELETE CASCADE,
    position       integer NOT NULL DEFAULT 0,
    description    text NOT NULL DEFAULT '',
    material       text NOT NULL DEFAULT '',
    hsn            text NOT NULL DEFAULT '',
    gst            numeric(6,2) NOT NULL DEFAULT 0,
    -- Purchase rows default to BY-QUANTITY (false), unlike job/quotation rows.
    has_dimensions boolean NOT NULL DEFAULT false,
    length         text NOT NULL DEFAULT '1',
    width          text NOT NULL DEFAULT '1',
    rate           numeric(14,2) NOT NULL DEFAULT 0,
    qty            numeric(14,3) NOT NULL DEFAULT 1,
    unit           text NOT NULL DEFAULT '',
    discount       numeric(14,2) NOT NULL DEFAULT 0,
    charges        numeric(14,2) NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX purchase_invoice_rows_invoice_idx ON purchase_invoice_rows (invoice_id);

-- Invoices and their entries. An invoice bills a set of entries (Invoice.entries in Mongo is
-- an ordered array of Entry ids); here an entry carries invoice_id (its `issued` ref) and the
-- list joins them back. invoiceId is the printed document number.
CREATE TABLE invoices (
    id           text PRIMARY KEY DEFAULT gen_ulid(),
    uid          text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id   text REFERENCES companies(id) ON DELETE CASCADE,
    client_id    text REFERENCES clients(id) ON DELETE SET NULL,
    invoice_id   text NOT NULL,
    amount       numeric(14,2) NOT NULL DEFAULT 0,
    total_amount numeric(14,2) NOT NULL DEFAULT 0,
    date         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoices_company_idx ON invoices (company_id);

-- Delivery challans (a simple standalone log; no timestamps in the Mongo model).
CREATE TABLE challans (
    id           text PRIMARY KEY DEFAULT gen_ulid(),
    uid          text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id   text REFERENCES companies(id) ON DELETE CASCADE,
    company_name text NOT NULL DEFAULT '',
    description  text NOT NULL DEFAULT '',
    date         text NOT NULL DEFAULT '',
    type         text NOT NULL DEFAULT '',
    quantity     numeric(14,3) NOT NULL DEFAULT 0,
    amount       numeric(14,2) NOT NULL DEFAULT 0
);
CREATE INDEX challans_company_idx ON challans (company_id);

CREATE TABLE entries (
    id            text PRIMARY KEY DEFAULT gen_ulid(),
    uid           text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id    text REFERENCES companies(id) ON DELETE CASCADE,
    client_id     text REFERENCES clients(id) ON DELETE SET NULL,
    -- The invoice this entry was billed on (Entry.issued). NULL until invoiced.
    invoice_id    text REFERENCES invoices(id) ON DELETE SET NULL,
    description   text NOT NULL DEFAULT '',
    material      text NOT NULL DEFAULT '',
    hsn           text NOT NULL DEFAULT '',
    rate          numeric(14,2) NOT NULL DEFAULT 0,
    qty           numeric(14,3) NOT NULL DEFAULT 0,
    has_dimensions boolean NOT NULL DEFAULT true,
    length        text NOT NULL DEFAULT '0',
    width         text NOT NULL DEFAULT '0',
    date          text NOT NULL DEFAULT '',
    amount        numeric(14,2) NOT NULL DEFAULT 0,
    cgst          numeric(6,2) NOT NULL DEFAULT 0,
    sgst          numeric(6,2) NOT NULL DEFAULT 0,
    igst          numeric(6,2) NOT NULL DEFAULT 0,
    discount      numeric(14,2) NOT NULL DEFAULT 0,
    charges       numeric(14,2) NOT NULL DEFAULT 0,
    advance       numeric(14,2) NOT NULL DEFAULT 0,
    total         numeric(14,2) NOT NULL DEFAULT 0,
    has_issued    boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX entries_invoice_idx ON entries (invoice_id);
CREATE INDEX entries_company_idx ON entries (company_id);

-- Payments received against invoices (Invoice's collected side).
CREATE TABLE invoice_received (
    id          text PRIMARY KEY DEFAULT gen_ulid(),
    uid         text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id  text REFERENCES companies(id) ON DELETE CASCADE,
    client_id   text REFERENCES clients(id) ON DELETE SET NULL,
    invoice_id  text REFERENCES invoices(id) ON DELETE SET NULL,
    bank_id     text,
    date        text NOT NULL DEFAULT '',
    amount      numeric(14,2) NOT NULL DEFAULT 0,
    note        text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoice_received_company_idx ON invoice_received (company_id);

CREATE TABLE material_price_history (
    id            text PRIMARY KEY DEFAULT gen_ulid(),
    material_id   text NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    material_rate numeric(14,2) NOT NULL,
    purchase_rate numeric(14,2) NOT NULL,
    changed_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX material_price_history_material_idx ON material_price_history (material_id, changed_at);

-- Bank accounts for the Batch Receive form's "which bank did this land in" dropdown.
CREATE TABLE banks (
    id              text PRIMARY KEY DEFAULT gen_ulid(),
    uid             text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id      text REFERENCES companies(id) ON DELETE CASCADE,
    name            text NOT NULL,
    -- Signed opening balance: positive is money in the account, negative an overdraft.
    opening_balance numeric(14,2) NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX banks_company_idx ON banks (company_id);

-- Expenses (money out, always tied to a bank). date is a string (YYYY-MM-DD) to match the
-- Mongo model and keep range filters string-comparable across the cashflow reports.
CREATE TABLE expenses (
    id          text PRIMARY KEY DEFAULT gen_ulid(),
    uid         text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_id  text REFERENCES companies(id) ON DELETE CASCADE,
    bank_id     text REFERENCES banks(id) ON DELETE SET NULL,
    date        text NOT NULL DEFAULT '',
    amount      numeric(14,2) NOT NULL DEFAULT 0,
    notes       text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX expenses_company_date_idx ON expenses (company_id, date);
