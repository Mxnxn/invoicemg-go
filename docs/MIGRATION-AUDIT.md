# Migration audit — Node/Mongo → Go

**Date:** 14 September 2026
**Subject:** `InvoiceMG-Mxnxn` (Express + Mongoose + MongoDB) → `InvoiceMG-Go` (Go + Postgres/Mongo)
**Status at time of writing:** 6 of 209 routes ported and verified.

## How the numbers were produced

Route counts come from a signature scan of `invoice-mg-api/routes/*.js`: each file is split into
per-route handler bodies and each body tested against a pattern per problem. It is a heuristic,
not a parser — it reads **193 of the 209** routes the docs inventory declares, because a handful
are mounted in shapes the splitter does not match.

**Treat every count as a floor, never a ceiling.** Where a count is quoted below it is the number
of routes the scan positively identified; the true number is that or higher.

Everything marked *Solved* is implemented **and tested** in `InvoiceMG-Go`, not merely planned.

---

## The listing

| # | Problem | Severity | Scope | Status | Suggested solution |
|---|---------|----------|-------|--------|--------------------|
| 1 | Tenant isolation is hand-written, and the rule is asymmetric | Critical | 131 routes | Partly solved | Keep the rule in one helper; Postgres RLS with split read/write policies at the SQL step |
| 2 | Money is computed in floats, in the browser | Critical | 16 routes · 58 web files | Open | Server computes; money as a JSON **string**; integer paise where the client must calculate |
| 3 | Multi-write routes have no transaction | Critical | 34 routes | Open | Transaction-scoped store in Go; single-node replica set if staying on Mongo |
| 4 | Document numbering is a read-then-write race | Critical | 7 routes | Open | Counter row locked in the document's own transaction; `$inc` counter as the Mongo fix |
| 5 | Dates serialise differently in Go | High | 73 routes | **Solved** | `httpx.Time` — never `time.Time` on a response struct |
| 6 | 82 `populate()` calls to hand-roll | High | 52 routes | Open | One batched lookup per id set, never per row; `LEFT JOIN` after the SQL step |
| 7 | `lean()` skips the `toJSON` transform | High | 31 routes | Open | Audit each against its non-lean siblings; reproduce, do not unify |
| 8 | Permission gates and the flat-key rule | High | 45 routes | **Solved** | `auth.HasPermission` — flat key grants all actions; `superadmin` listed explicitly |
| 9 | Mongoose setters normalise text on write | High | 13 models | **Solved** | `internal/textcase`, applied in the store, never in handlers |
| 10 | TOTP is not implemented in Go | High | 8 routes | Open | Port `Helpers/Totp.js` before any cutover including login; match the time window |
| 11 | WhatsApp sends must stay idempotent | High | 6 routes | Open | Port the compare-and-set as a real conditional update and check rows affected |
| 12 | Delete means move to Trash | High | 4 routes | Open | Decide `deleted_at` + view vs parallel table **before** porting the delete routes |
| 13 | Reporting is where Mongo degrades | High | ~40 routes | Open | The genuine argument for SQL; not a CRUD-latency problem |
| 14 | A malformed id answers 500, not 404 | Medium | 88 routes | **Solved** | `store.ErrBadID` — reproduce the bug; fix in both services together or not at all |
| 15 | The response envelope is inconsistent per route | Medium | All routes | Mitigated | `scripts/parity.js` — a route is done when the diff is empty, not when it looks right |
| 16 | Tests that only pass in UTC | Medium | Date logic | Open | Pin the timezone inside the test rather than inheriting it |
| 17 | XLSX exports will not be byte-identical | Low | 2 routes | Assessed | Verify by opening the file; `excelize` covers every feature in use |
| 18 | File uploads write paths the database stores | Low | 3 routes | Open | Match the existing filename scheme exactly, or orphan every logo |

Four criticals. Two of them — **#3 and #4** — are live defects in production today, not migration
risks: they will corrupt data whether or not anyone ports anything.

---

# 1. Tenant isolation is hand-written, and the rule is asymmetric

**Severity:** Critical · **Scope:** 131 routes · **Status:** Partly solved

## What it is

Every query that reads or writes company-owned data must filter on `company_id`. 131 routes do
this by hand. One omission returns another business's data — no error, no crash. At 1,000 admins
that is 1,000 separate businesses' financial records in one database, so a leak shows one printing
firm its competitor's customers, rates and invoices.

## The part that is easy to miss

The rule is **not** `company_id = <one value>`. From `Helpers/CompanyScope.js:42`:

- **Writes** are always exactly `req.auth.companyId`. No exceptions.
- **Reads** may span every company the same admin owns, when `sharing.reportsAcrossCompanies`
  is on for the acting company.

That asymmetry is currently enforced by a comment in that file:

> "READS ONLY. No create, update or delete path may call this… widening a write is a different
> project with 227 call sites to audit."

A rule held up by a code comment, across 227 call sites.

## Suggested solution

**Now — keep the rule in one place.** `internal/store` already never exposes a table or collection
to a handler; every method takes `companyID` and builds the filter itself, so a handler cannot
forget it because it never writes a query. Extend that: a single helper that every store method
calls to build its tenant predicate, so there is exactly one implementation of the rule rather
than one per method.

**At the Postgres step — row-level security**, with policies that mirror the asymmetry:

```sql
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs FORCE ROW LEVEL SECURITY;

-- Reads may span the owner's companies when sharing is on.
CREATE POLICY jobs_read ON jobs FOR SELECT
    USING (company_id = ANY (string_to_array(current_setting('app.company_ids', true), ',')));

-- Writes never do, in either direction.
CREATE POLICY jobs_write ON jobs FOR ALL
    USING      (company_id = current_setting('app.company_id', true))
    WITH CHECK (company_id = current_setting('app.company_id', true));
```

`FOR SELECT` versus `FOR ALL`, and `USING` versus `WITH CHECK`, express exactly the read/write
split the comment asks people to remember. That is the real argument for RLS here — stronger than
"someone might forget a `WHERE`", which the store layer already prevents.

## Costs, so the trade is fair

- Every query must run inside a transaction, or `SET LOCAL` has nothing to scope to.
- Two settings per request, not one — `app.company_ids` needs computing, which is a query of its own.
- **Table owners bypass RLS silently.** Without `FORCE ROW LEVEL SECURITY` *and* a non-owner
  application role, the policies do nothing and nothing warns you.
- **`SET LOCAL`, never `SET`.** A plain `SET` persists on the pooled connection and the next
  request inherits that tenant. This produces cross-tenant leaks that look like RLS bugs.
- Tests must connect as the **application role**. A suite connecting as the owner proves nothing.
- Admin and migration scripts need `BYPASSRLS` handled deliberately.

## Do not

Do not restructure the store today *for* RLS. It is Postgres-only and unavailable until the SQL
step. Restructure for transactions (#3), which is worth it on its own, and RLS arrives free later.

---

# 2. Money is computed in floats, in the browser

**Severity:** Critical · **Scope:** 16 API routes, 58 frontend files · **Status:** Open

## What it is

`Helpers/RoundOff.js:12` and its frontend twin do `Math.round(amt * 100) / 100` on IEEE-754
floats, and **58 frontend files** compute money — `rowTotal`, `quotationGrandTotal`,
`invoiceTotals`, and the rest. This is GST compliance software: a paisa that does not reconcile
is a filing that does not reconcile.

Note the two `RoundOff` helpers are not the same function. The API's returns a single number; the
frontend's `RoundOffWithAmount` returns a `[total, roundOff]` pair. Same name, different shape.

## The part that is easy to miss

**JSON has no decimal type.** Moving to `numeric(14,2)` in Postgres does not fix this on its own —
`152340.50` still arrives in the browser as a float64 the moment it crosses the wire as a JSON
number.

## Suggested solution

In order, because each is cheaper before the one after it:

1. **The server computes, the client displays.** Move the arithmetic into Go, where the type is
   exact, and have the client render what it is given.
2. **Money travels as a JSON string** — `"152340.50"`, not `152340.5`. Six routes today, every
   route later; this is the cheapest it will ever be.
3. **Where the client genuinely must compute** — live totals as someone types — use integer paise
   or `dinero.js`. Never floats.

## On TypeScript

TypeScript does **not** fix this. It is compile-time only; `number` is still float64 at runtime and
`0.1 + 0.2 !== 0.3` in TypeScript exactly as in JavaScript.

What it does add is a branded type:

```ts
type Paise = number & { readonly __brand: unique symbol };
```

which makes it a *compile error* to add a rupee float to a paise integer. Worth doing — for the
half of the problem types can actually solve. It is not a substitute for items 1–3.

---

# 3. Multi-write routes have no transaction

**Severity:** Critical · **Scope:** 34 routes · **Status:** Open — and live in production

## What it is

34 routes perform two or more writes with no atomicity. `/invoice/save` converts job rows to
entries, creates the invoice, updates totals and stamps the rows. A failure halfway leaves a job
half-billed.

This is not hypothetical. `JOB/26-27/000004` went half-billed and then could not be invoiced at all
after its invoice was deleted — investigated earlier in this project and traced to exactly this
class of partial write.

## Suggested solution

**In Go on Postgres:** wrap each multi-write route in a transaction. The store methods take a
transaction handle rather than the pool. This is the same restructure RLS needs, so the two land
together — but the justification is atomicity, which stands alone.

**If staying on Mongo:** convert the standalone deployment to a **single-node replica set**. It is
cheap, it is the only way to get multi-document transactions, and it also enables change streams —
which is what a later CDC-based migration to Postgres reads from. Worth doing even with no
migration planned.

## Priority

This and #4 are the two problems on this list that are damaging data **today**. They should be
fixed in the Node app regardless of what happens with Go.

---

# 4. Document numbering is a read-then-write race

**Severity:** Critical · **Scope:** 7 routes · **Status:** Open — and live in production

## What it is

`Helpers/DocumentNumbering.js:96` — `nextDocumentNumber(existingNumbers, format, now)` — takes the
list of numbers already used and returns the next one. The caller then reads, computes and writes.
Two invoices raised in the same second both read the same list and take the same number.

For a GST document, a duplicate invoice number is not a cosmetic bug.

## Suggested solution

**Do not port the race.**

- **Postgres:** a counter row per company and document type, selected `FOR UPDATE` inside the same
  transaction that writes the document, plus a unique index — `(company_id, invoice_number)` — as
  the backstop.
- **Mongo, today:** `findOneAndUpdate` with `$inc` on a counters document. Atomic, single file,
  worth doing immediately.

## Do not use a bare sequence

A Postgres `SEQUENCE` is the obvious answer and the wrong one. GST numbering must be **gapless**
within a financial year, and sequences deliberately leave gaps on rollback — a rolled-back invoice
would burn a number and produce a gap a tax audit will ask about. The counter row inside the
document's own transaction is the correct shape precisely because it rolls back with the document.

---

# 5. Dates serialise differently in Go

**Severity:** High · **Scope:** 73 routes, 18 `Date` fields across 10 models · **Status:** Solved

## What it is

JavaScript's `Date.prototype.toJSON` always writes three decimal places. Go's `time.Time` marshals
as RFC3339Nano, which trims trailing zeros:

```
Node   {"createdAt":"2026-09-14T12:34:56.000Z"}
Go     {"createdAt":"2026-09-14T12:34:56Z"}
```

They agree whenever the millisecond part is non-zero and differ whenever it is zero — which is
**every date stored as midnight**, so most dates a person typed rather than a machine stamped.

A client that only displays the string will not care. One that compares it, keys on it, sorts it
lexically, or sends it back will, and the failure surfaces far from the cause.

## Solution — implemented

`internal/httpx/time.go`. `httpx.Time` emits the JavaScript form; the zero time serialises as
`null`, matching an unset Mongoose field rather than a date in the year 1; parsing is liberal
(accepts both forms and any offset) while output is strict.

Every expected value in `time_test.go` is what `node -e` actually printed for the same instant —
not what the behaviour was assumed to be.

**Remaining discipline:** never put a bare `time.Time` on a response struct.

---

# 6. 82 `populate()` calls to hand-roll

**Severity:** High · **Scope:** 52 routes · **Status:** Open

## What it is

Mongoose's `.populate()` folds a second query into a nested object. Go has no equivalent, so each
of the 82 calls is written by hand — and each is a chance to shape that object differently: a
different key, a missing field, or `{}` where Node sends `null`.

The performance half matters more. Ported naively, a `populate()` inside a loop becomes an N+1:
one query per row. That is where a "CRUD must stay fast" promise dies — not in the language.

## Suggested solution

**One batched lookup per set of ids, never one per row.** The pattern is already in
`internal/store/mongostore/mongostore.go` — `clientNames` collects every `client_id` from the job
list and resolves them in a single query.

At the Postgres step these collapse into a `LEFT JOIN` and stop being a separate query at all;
`internal/store/sqlstore/days.go` shows the same data fetched in one statement that took three
round trips against Mongo.

**Check the shape against Node before ticking the route.** `scripts/parity.js` reports the exact
path of any nested field that differs.

---

# 7. `lean()` skips the `toJSON` transform

**Severity:** High · **Scope:** 31 routes · **Status:** Open

## What it is

`Model/Job.js:182` defines a `toJSON` transform that fills `receivedDate` from `createdAt` when it
is blank. Mongoose runs that transform on **documents** and not on `.lean()` queries, which return
plain objects straight from the driver.

So the same field is populated or empty **depending on how the route fetched it**. This is
per-route behaviour wearing the costume of per-model behaviour, and reading the model tells you the
wrong thing about 31 of the routes.

## Suggested solution

Audit each of the 31 against its non-lean siblings *before* porting, and reproduce whatever that
route actually returns today.

**Do not unify it.** Making the behaviour consistent is the obvious instinct and it silently
changes what the client receives on routes nobody asked you to change. If it should be unified,
that is its own task with its own testing, in both services at once.

---

# 8. Permission gates and the flat-key rule

**Severity:** High · **Scope:** 45 routes · **Status:** Solved

## What it is

Permissions are stored as `"feature:action"` — `"invoices:create"`. But every employee predating
that split has **flat keys** (`"invoices"`) on their Person record *and* in the permissions
snapshot copied into their live session at login.

Read a flat key as view-only and you revoke write access from every existing employee the moment
the Go service answers a request. No migration runs; no error appears; people simply cannot work.

## Solution — implemented

`internal/auth/permissions.go`, a direct port of `Helpers/Permissions.js`:

- A flat key grants **every** action on that feature.
- A trailing colon (`"invoices:"`) is treated as flat, not as an action named `""`.
- `superadmin` is listed explicitly alongside `admin`. This has been wrong before in the Node app:
  sessions used to stamp `"admin"` for everyone, and once the role became real a superadmin matched
  neither branch and was refused every gated route.
- `NormalisePermissions` expands flat keys on write and drops unknown actions, so a typo cannot
  become a permanent grant.

11 tests cover these rules.

---

# 9. Mongoose setters normalise text on write

**Severity:** High · **Scope:** 13 models, every write path · **Status:** Solved

## What it is

`Helpers/TextCase.js` applies three rules as Mongoose **setters**, so they run on every write path
— routes, maintenance scripts, the WhatsApp CLI:

- `docNumber` — identifiers uppercased (`mg/26-27/qt-1` → `MG/26-27/QT-1`)
- `titleCase` — names, each word capitalised
- `sentenceCase` — descriptions, first letter only

A Go route that skips these writes unnormalised text into the same collection as Node-written rows.
Nothing errors. The list looks wrong, sorts wrong, and a lookup by number misses.

## Solution — implemented

`internal/textcase`, applied **in the store layer**, not in handlers — the next handler written is
the one that forgets.

Every expected value in `textcase_test.go` is captured from the real Node helper, which is how a
genuine divergence was caught before it shipped: Node's `titleCase` regex is `[a-z]`, ASCII-only,
so it leaves `"élan vital"` as `"élan Vital"`. A Unicode-correct Go implementation would have
produced `"Élan Vital"` and stored the first accented customer name two different ways.

**The lesson generalises:** capture the behaviour, do not reason about it.

---

# 10. TOTP is not implemented in Go

**Severity:** High · **Scope:** 8 routes · **Status:** Open

## What it is

Any account with two-factor authentication enabled **cannot sign in** to the Go service. It
currently refuses them with an explicit message, which is the only safe behaviour — signing someone
in without their second factor would be worse than not signing them in at all.

## Suggested solution

Port `Helpers/Totp.js` before any cutover that includes `/user/login`. It is standard RFC 6238:
a small well-tested Go library, or about fifty lines of HMAC-SHA1.

**Check the time-window tolerance matches.** Node's implementation accepts codes from adjacent
30-second windows; a Go version with a different tolerance rejects codes a few seconds either side
of where Node accepts them, which users experience as "2FA is broken" rather than as a bug report.

---

# 11. WhatsApp sends must stay idempotent

**Severity:** High · **Scope:** 6 routes · **Status:** Open

## What it is

`Helpers/JobDoneAlert.js:105` claims a send by **compare-and-set**: the update applies only if
`alertedRowIds` is still exactly what was read. Two clicks landing together cannot both send.

Reimplement that as read-then-write and a customer is told their job is ready twice. These sends
are outbound and irreversible — there is no undo, and a retry after a timeout can produce a second
message for one click.

## Suggested solution

Port it as a genuine conditional update and **check the affected row count**:

```sql
UPDATE jobs SET alerted_row_ids = $new, alert_count = alert_count + 1
 WHERE id = $1 AND alerted_row_ids = $old;
-- 0 rows affected means somebody else claimed it. Do not send.
```

The Mongo equivalent is `findOneAndUpdate` with the old array in the filter. Either way the
decision to send must depend on the write succeeding, never the other way round.

---

# 12. Delete means move to Trash

**Severity:** High · **Scope:** 4 routes · **Status:** Open

## What it is

Deletions snapshot the record into `Trash` rather than removing it. A Go route that issues a real
`DELETE` destroys data the old system kept, and the user expects a restore that no longer exists.

Note that Trash now has exactly **one** live producer — `/lifecycle/jobs/delete`. The other was
`/entry/remove`, pruned when the dead Entry-authoring code was removed. `docs/03-api-reference.md`
claimed `/entry/remove` was the only route that wrote to Trash; that was stale and has been
corrected.

## Suggested solution

In Postgres a `deleted_at` column plus a filtered view is cleaner than a parallel table, and makes
restore a single `UPDATE`. But it changes restore semantics — the current design snapshots the
whole document, so a restore is unaffected by later schema changes, while `deleted_at` restores
whatever the row has become.

**Decide this before porting the delete routes,** not after. Either answer is defensible; changing
your mind later means migrating the trash.

---

# 13. Reporting is where Mongo degrades

**Severity:** High · **Scope:** ~40 routes · **Status:** Open — this is the SQL decision

## What it is

Analytics (14 routes), Ledger, GstReport, Statistics, PurchaseReport, Inventory and Dues all read
across collections. `$lookup` is the Mongo answer and it does not scale the way a join does.

At 1,000 admins: roughly 100,000 jobs per month, ~3.6M job rows per year, **~18M rows after five
years**. That is comfortable for CRUD on either database. It is where `$lookup` starts to hurt, and
where the Mongo workarounds — denormalise, pre-aggregate — begin buying speed with consistency bugs.

## What this is not

**It is not a CRUD latency problem, and scale should not be the argument for moving.** Measured on
a laptop, through Docker, against Mongo: p50 5.1 ms serial, **846 req/s** at 50 concurrent, against
a realistic peak of 20–50 req/s. Neither database will struggle with this application's volume.

## Suggested solution

This is the honest case for Postgres, and the only problem on this list that gets **worse with
time** rather than staying constant. Everything else here is a one-off cost.

Recommended sequencing — two moderate risks instead of one large one, and never a flag day:

1. Port to Go **on Mongo**. Reversible per route, no data migration, production keeps running.
2. Migrate Mongo → Postgres as its own project, with Go already in place and `internal/store`
   already proven to absorb the swap.

For the migration itself the current consensus favours **CDC over dual-write** — read MongoDB's
oplog/change streams into Postgres and pivot reads when the lag is zero. Dual-write cannot be made
consistent for financial data.

---

# 14. A malformed id answers 500, not 404

**Severity:** Medium · **Scope:** 88 routes · **Status:** Solved (as a deliberate reproduction)

## What it is

Mongoose throws a `CastError` when a string cannot be cast to an ObjectID. That lands in each
route's catch block, so `unit_id=nonsense` returns **500 Internal Error** where a well-behaved API
returns 404. 88 routes accept an id, so this recurs across nearly half the surface.

This was found by testing rather than assumed: the first Go implementation of `/unit/update`
returned 404 for a malformed id, and a comment in the code asserted that Node did the same. It does
not.

## Suggested solution

**Reproduce the bug, do not improve on it.** While both services are live, the same client must get
the same answer from either — a Go service that "fixed" this makes the pair inconsistent in a way
that is harder to reason about than the original bug.

`store.ErrBadID` exists for exactly this and is documented as a deliberately preserved defect,
distinct from `ErrNotFound`.

**If you want it fixed:** change both services in the same commit, after the port, with the
frontend checked for anything branching on 500 vs 404.

---

# 15. The response envelope is inconsistent per route

**Severity:** Medium · **Scope:** All routes · **Status:** Mitigated by tooling

## What it is

There is no single envelope. `/sheet/only` sends `{code, message, data}` with no `status` field;
`/sheet/open-jobs` sends `{code, message, status, data}`. This is per-route history, not a rule.

A client checking `status` on a route that never sent one gets `undefined`, which is falsy, which
reads as failure.

The whole contract is unusual and worth restating: **business errors come back as HTTP 200** with
the real outcome in `code`. The React client branches on `data.code` and never on `res.status`, so
a Go handler answering a real 401 would be read as a *successful request that returned no data* —
a blank screen with no error.

## Suggested solution

Match each route exactly, quirk included. This is what `scripts/parity.js` is for: it compares both
services' answers, ignoring object key order (JSON objects are unordered) but **not** array order
(these lists are deliberately sorted), and treats a missing key and `undefined` as the same while
`null` is different.

**A route is done when the diff is empty — not when it looks right.**

For the eventual clean surface, `API_STYLE=rest` puts the outcome back in the status line while the
body keeps `code`, so the client can be moved one call at a time. The default stays `legacy`
because the live client depends on it.

---

# 16. Tests that only pass in UTC

**Severity:** Medium · **Scope:** Date logic generally · **Status:** Open

## What it is

`Helpers/RemindWindow.test.js` passes on CI, which runs UTC, and fails on a development machine in
any other timezone. It asserts behaviour that is only true in one zone — so it is green where it
matters least and red where the work actually happens, which trains people to ignore it.

## Suggested solution

Pin the timezone **inside** the test rather than inheriting it from the environment, so it asserts
the same thing everywhere. Any date logic ported to Go needs the same treatment — otherwise the
trap moves rather than closes.

Related and already fixed: `Helpers/ConvertJobRows.js:69` fell back to a full 24-character
`toISOString()` when `receivedDate` was blank, creating a sheet whose date could never equal any
job's `YYYY-MM-DD`. No rows were affected; it was a trap rather than an active fault.

---

# 17. XLSX exports will not be byte-identical

**Severity:** Low · **Scope:** 2 routes · **Status:** Assessed

## What it is

`Excel/DataToExcel.js` and `Excel/LedgerExcel.js` build workbooks with `exceljs`. Go's `excelize`
produces a valid, equivalent workbook — never a byte-identical one, because it is a different
library writing different internal XML. The parity harness will always report these as different,
and that is correct rather than a defect.

## Capability check — already performed

A workbook was generated with `excelize` and its XML inspected. Every feature these exports
actually use is present: merged cells (44 uses), alignment (28), fonts (18), borders (10), fills
(7), the embedded logo (6), frozen panes (3), column widths and a custom number format.

`excelize` additionally supports charts and pivot tables, which `exceljs` cannot do at all, and a
**streaming writer** that produced a 50,000-row file at 96 MB peak RSS — which would take the
export off the request path, where it currently blocks the event loop for its whole duration.

## Suggested solution

Verify these two routes by **opening the file**, not by diffing it. Bump the Go version when
porting them: `excelize` ≥ 2.11 requires Go 1.25, which the project is now on.

## Not a risk: PDFs

PDF generation is entirely client-side via `@react-pdf/renderer`. The server never produces one, so
there is nothing to port.

---

# 18. File uploads write paths the database stores

**Severity:** Low · **Scope:** 3 routes · **Status:** Open

## What it is

Three routes accept real files through `multer` — `routes/Company.js:27` (logo and UPI QR, used by
`/company/create` and `/company/update`) and `routes/UserInfo.js:109` (`/userinfo/upload`).

Multipart handling in Go is straightforward. The risk is not the parsing: **the database stores the
generated filenames**, and those files are served from the `/uploads` static mount. A different
naming scheme orphans every existing logo and QR code — invoices simply render without them.

## Suggested solution

Match the existing filename generation exactly, and verify by uploading through Go and loading an
invoice that renders an image written by Node. Port these last; they are the only binary path that
matters.

---

## Recommended order of work

Sequenced so the expensive-to-retrofit decisions land while the surface is still small.

| Step | Work | Why in this position |
|------|------|----------------------|
| 1 | Fix numbering (#4) and atomicity (#3) **in the Node app** | Live defects, damaging data now. Worth fixing even if the port never happens. |
| 2 | Transaction-scoped store in Go | Touches every query. An afternoon at 9 tables, a rewrite at 200. Justified by #3 alone. |
| 3 | Money representation on the wire (#2) | Cheapest before 200 routes serialise it as a JSON number. |
| 4 | Port on Mongo, route by route, behind `scripts/parity.js` | No data migration; reversible per route; production keeps running. |
| 5 | Migrate to Postgres; add RLS (#1) | Two moderate risks instead of one large one. RLS is an afternoon once transactions exist. |

## References

- `scripts/parity.js` — the diff harness. A route is done when this is silent.
- `ROUTES.md` — generated from the Node source; cannot drift from what the API exposes.
- `README.md` — why the service is shaped the way it is, and how a route moves across.
- PostgreSQL row-level security: <https://www.postgresql.org/docs/current/ddl-rowsecurity.html>
