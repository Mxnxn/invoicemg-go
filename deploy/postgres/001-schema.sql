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
