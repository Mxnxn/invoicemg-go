# InvoiceMG — Go service

A second implementation of the InvoiceMG API, built to run **beside** the Node one and take
routes over gradually. Local only for now: nothing here is deployed, and the droplet still
runs `InvoiceMG-Mxnxn/docker-compose.prod.yml` unchanged.

## Why it is shaped like this

Three constraints decided the whole design.

**It must run sideways.** A live billing system does not get a flag day. So this service
speaks the same wire protocol as the Node API — the same `{code, message, status}` envelope
over HTTP 200, the same `SESSION-TOKEN` and `TAB-ID` headers — and reads the same MongoDB.
nginx can then send one path here and leave the other 207 with Node, and roll that back by
editing one line. The React client never learns there are two services.

**Which means it cannot use Postgres yet.** The plan is SQL. But a Go service writing Postgres
while Node writes Mongo means the two disagree about your data the moment anyone creates a
job — the sideways phase and a second database are mutually exclusive. So the storage layer
sits behind the interfaces in `internal/store`, with `internal/store/mongostore` as a
temporary tenant. Moving to SQL at cutover is writing a second implementation of one package,
not rewriting handlers.

> **The rule that keeps that promise:** nothing outside `internal/store/...` may import the
> mongo driver. If a `bson.M` or a `primitive.ObjectID` appears in a handler, the seam has
> leaked and the Postgres swap stops being a swap.

**Parity beats tidiness.** Several things here look wrong and are deliberate. Reads are `POST`,
because the frontend posts FormData to them. An auth failure is HTTP **200** with `code: 401`
in the body, because the client branches on `data.code` and never on `res.status` — a real 401
reads to it as a successful request that returned no data, i.e. a blank screen with no error.
`/sheet/only` sends no `status` field while `/sheet/open-jobs` does, because that is what Node
sends, and a client checking `status` on a route that never sent one gets `undefined`.

## What is in it

| | |
|---|---|
| `cmd/api` | wiring, routing, graceful shutdown, `/healthz` |
| `internal/httpx` | the response envelope — the wire contract, in one place |
| `internal/auth` | `TokenHelper` + `RoleHelper`, reproduced exactly |
| `internal/store` | the interfaces — **the Postgres seam** |
| `internal/store/mongostore` | the only package that imports the mongo driver |
| `internal/days` | first slice: `POST /sheet/only`, `POST /sheet/open-jobs` |

The first slice is two **reads**, on purpose. A strangler's first route should be one where
being wrong costs a wrong number on a screen, not a corrupted invoice — both services are
live against the same documents, so anything that writes has to be right the first time.

## Running it

Go is not required on the host; everything goes through Docker.

```sh
# from InvoiceMG-Go/
docker compose up --build        # starts on :5002, tests run during the image build
```

It expects the InvoiceMG-Mxnxn stack's mongo on host port 27018 (`docker compose up` in that
repo). Check it is alive:

```sh
curl localhost:5002/healthz
```

### Tests

```sh
docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine go test ./...
```

## Comparing an answer against Node

The point of running sideways is that you can diff. With a real session token:

```sh
TOKEN=...   # a row in the usersessions collection
for PORT in 5001 5002; do
  curl -s -X POST "localhost:$PORT/sheet/only" -H "SESSION-TOKEN: $TOKEN" > "/tmp/only.$PORT.json"
done
diff <(jq -S . /tmp/only.5001.json) <(jq -S . /tmp/only.5002.json) && echo "identical"
```

A route is ready to move across when that diff is empty for every session and company you can
find, not when it looks right once.

## Moving a route across

1. Implement it here, behind the same middleware.
2. Diff both services' answers until they are byte-identical.
3. Point nginx at `:5002` for that path only.
4. Watch. Roll back by reverting one nginx line — no deploy, no data migration.

Delete the Node handler only once the route has been served from here long enough that you
would have noticed.

## The frontend copy (`web/`)

`web/` is a copy of `InvoiceMG-Mxnxn/Invoice-mg`, taken so this repository can eventually be
the whole application. Until cutover it is **not** the source of truth, and the risk is not
staleness - it is that both copies get hand-edited and quietly diverge.

> Edit the frontend in `InvoiceMG-Mxnxn/Invoice-mg`. Bring this copy forward with
> `pwsh scripts/sync-web.ps1 -Apply`. Never hand-edit `web/`.

At cutover that reverses: `web/` becomes the source of truth, the sync script is deleted, and
the old repo is archived. The client needs no change either way - it talks to whatever nginx
points it at.

## Progress

`ROUTES.md` is generated from the Node source by `Docs/routeInventory.js`, so it cannot drift
from what the API actually exposes. A route is ticked only when `scripts/parity.js` says its
answer is identical - implemented is not done.

```sh
node scripts/parity.js --token <SESSION-TOKEN>              # every ticked route
node scripts/parity.js --token <TOKEN> --route "post /unit/list"
```

Mint a throwaway token against the dev database rather than using your own session.

## Not done yet

- 207 of 209 routes.
- Writes. Everything here is read-only, which is why it is safe to run against live data.
- Postgres. See above — it arrives at cutover, behind `internal/store`.
- CI. Deliberately absent: this is local until a route actually moves across.
