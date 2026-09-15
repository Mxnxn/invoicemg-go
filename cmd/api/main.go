// Command api serves the InvoiceMG API in Go.
//
// It runs in one of two modes, chosen by STORE:
//
//	mongo     beside the Node API, against the same documents, so the two can be diffed
//	          request for request. :5002 while Node keeps :5001.
//	postgres  the all-in-one local stack (docker-compose.local.yml), against its own
//	          database - a preview of life after cutover, sharing nothing with the Node app.
//
// Nothing in this repository touches the production compose file or the deploy workflow.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/alerts"
	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/bank"
	"github.com/mxnxn/invoicemg-go/internal/client"
	"github.com/mxnxn/invoicemg-go/internal/company"
	"github.com/mxnxn/invoicemg-go/internal/config"
	"github.com/mxnxn/invoicemg-go/internal/days"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/invoice"
	"github.com/mxnxn/invoicemg-go/internal/lookups"
	"github.com/mxnxn/invoicemg-go/internal/material"
	"github.com/mxnxn/invoicemg-go/internal/person"
	"github.com/mxnxn/invoicemg-go/internal/purchaseinvoice"
	"github.com/mxnxn/invoicemg-go/internal/quotation"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/store/mongostore"
	"github.com/mxnxn/invoicemg-go/internal/store/sqlstore"
	"github.com/mxnxn/invoicemg-go/internal/units"
	"github.com/mxnxn/invoicemg-go/internal/userinfo"
	"github.com/mxnxn/invoicemg-go/internal/users"
	"github.com/mxnxn/invoicemg-go/internal/whatsapp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("invoicemg-go: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			log.Printf("closing the database: %v", err)
		}
	}()

	// Set once, before anything can serve a request. See internal/httpx/style.go.
	httpx.SetStyle(cfg.APIStyle)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: routes(db),
		// A request that has not finished in this long is not going to. The Node app has no
		// equivalent, which is why one slow query there can hold a connection open
		// indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("invoicemg-go listening on %s, store=%s, api-style=%s", cfg.Addr, cfg.Store, httpx.Style())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		// Finish what is in flight before exiting, so a restart or a Ctrl-C does not cut a
		// response in half.
		log.Print("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// openStore is the ONLY place either backend is named. Everything below it takes a
// store.Store, which is why adding Postgres changed no handler.
func openStore(ctx context.Context, cfg config.Config) (store.Store, error) {
	switch cfg.Store {
	case config.StorePostgres:
		log.Printf("using postgres")
		return sqlstore.Open(ctx, cfg.PostgresURL, cfg.ConnectTimeout)
	default:
		log.Printf("using mongo at %s/%s", cfg.MongoURI, cfg.MongoDB)
		return mongostore.Open(ctx, cfg.MongoURI, cfg.MongoDB, cfg.ConnectTimeout)
	}
}

func routes(db store.Store) http.Handler {
	mux := http.NewServeMux()

	guard := auth.New(db.Sessions())
	admin := func(h http.HandlerFunc) http.Handler {
		return auth.Chain(h, guard.Require, auth.RequireAdmin)
	}
	// authed is just a valid session, no role or feature gate - what routes/Company.js applies
	// to /list and /active (the writes add requireAdmin on top).
	authed := func(h http.HandlerFunc) http.Handler { return auth.Chain(h, guard.Require) }
	// Units live inside the Products manager, so they carry the products permission - and the
	// write routes carry the write-level gate ON TOP of it, exactly as routes/Unit.js layers
	// requireCreate and requireDelete under its router-wide requireFeature.
	feature := func(key string, h http.HandlerFunc, extra ...func(http.Handler) http.Handler) http.Handler {
		mw := append([]func(http.Handler) http.Handler{guard.Require, auth.RequireFeature(key)}, extra...)
		return auth.Chain(h, mw...)
	}

	dayHandler := days.New(db.Days())
	unitHandler := units.New(db.Units())
	userHandler := users.New(db.Users())
	alertHandler := alerts.New(db.Alerts())
	bankHandler := bank.New(db.Banks())
	clientHandler := client.New(db.Clients())
	materialHandler := material.New(db.Materials())
	personHandler := person.New(db.People())
	quotationHandler := quotation.New(db.Quotations())
	purchaseInvoiceHandler := purchaseinvoice.New(db.PurchaseInvoices())
	invoiceHandler := invoice.New(db.Invoices(), db.Companies(), db.Users())
	lookupHandler := lookups.New(db.Lookups(), db.Users())
	companyHandler := company.New(db.Companies(), db.Users())
	userinfoHandler := userinfo.New(db.Users(), db.Companies())
	whatsappHandler := whatsapp.New(os.Getenv("WHATSAPP_VERIFY_TOKEN"))

	// Every route is registered TWICE, under two surfaces.
	//
	// The legacy surface is what the Node API exposes and what the React client calls today:
	// POST for everything, including reads, with the outcome in the body. It is preserved
	// exactly, because while both services are live the same client must be able to call
	// either one.
	//
	// The REST surface is the same handlers under the verb that actually describes what they
	// do, with the id in the path. It is what the all-in-one stack is for - there the frontend
	// is ours to move, one call at a time, with the legacy path still answering until it has.
	//
	// Registering both costs one line each and means the client migration is not a flag day.

	// Unauthenticated - this is what issues a session. A login CREATES a session, so POST is
	// right under either surface; only the path shape differs.
	mux.HandleFunc("POST /user/login", userHandler.Login)
	mux.HandleFunc("POST /sessions", userHandler.Login)

	// Reads. GET under REST, so they are cacheable, safe to retry, and visible as reads in
	// any log - none of which is true of a POST.
	mux.Handle("POST /sheet/only", admin(dayHandler.Only))
	mux.Handle("GET /days", admin(dayHandler.Only))
	mux.Handle("POST /sheet/open-jobs", admin(dayHandler.OpenJobs))
	mux.Handle("GET /jobs/open", admin(dayHandler.OpenJobs))

	mux.Handle("POST /unit/list", feature("products", unitHandler.List))
	mux.Handle("GET /units", feature("products", unitHandler.List))

	mux.Handle("POST /unit/create", feature("products", unitHandler.Create, auth.RequireCreate("products")))
	mux.Handle("POST /units", feature("products", unitHandler.Create, auth.RequireCreate("products")))

	// PATCH rather than PUT: the request carries the fields to change, not a whole
	// replacement unit. PUT would promise that anything omitted is cleared, which is not what
	// this does.
	mux.Handle("POST /unit/update", feature("products", unitHandler.Update))
	mux.Handle("PATCH /units/{id}", feature("products", unitHandler.Update))

	mux.Handle("POST /unit/delete", feature("products", unitHandler.Delete, auth.RequireDelete("products")))
	mux.Handle("DELETE /units/{id}", feature("products", unitHandler.Delete, auth.RequireDelete("products")))

	// Bank accounts, behind the batch_receive feature as routes/Bank.js is. Only the read is
	// ported; create/update/remove/report are not.
	mux.Handle("POST /bank/list", feature("batch_receive", bankHandler.List))
	mux.Handle("GET /banks", feature("batch_receive", bankHandler.List))

	// Customers, behind the customers feature. Both reads WIDEN by sharing (#1); the writes
	// (add/update/remove) and the populate-heavy /client/get are not ported.
	mux.Handle("POST /client/getall", feature("customers", clientHandler.Getall))
	mux.Handle("GET /clients", feature("customers", clientHandler.Getall))
	mux.Handle("POST /client/only", feature("customers", clientHandler.Only))
	mux.Handle("GET /clients/only", feature("customers", clientHandler.Only))

	// The shell bootstrap: the company switcher, the active-company letterhead, and the admin
	// profile. /company/* need only a session; /userinfo/get is admin-only.
	mux.Handle("POST /company/list", authed(companyHandler.List))
	mux.Handle("GET /companies", authed(companyHandler.List))
	mux.Handle("POST /company/active", authed(companyHandler.Active))
	mux.Handle("POST /userinfo/get", admin(userinfoHandler.Get))

	// Products, behind the products feature as routes/Material.js is. Read widens by sharing
	// (#1); the writes and /material/get are not ported.
	mux.Handle("POST /material/getall", feature("products", materialHandler.Getall))
	mux.Handle("GET /materials", feature("products", materialHandler.Getall))

	// People (employees + suppliers), admin-only as routes/Person.js is. Only /list is ported.
	mux.Handle("POST /person/list", admin(personHandler.List))
	mux.Handle("GET /people", admin(personHandler.List))

	// Quotations, behind the quotations feature. client_id is populated. Only /list is ported.
	mux.Handle("POST /quotation/list", feature("quotations", quotationHandler.List))
	mux.Handle("GET /quotations", feature("quotations", quotationHandler.List))

	// Purchase invoices (supplier bills), behind the purchase_invoices feature. supplier_id is
	// populated. Only /list is ported.
	mux.Handle("POST /purchase-invoice/list", feature("purchase_invoices", purchaseInvoiceHandler.List))
	mux.Handle("GET /purchase-invoices", feature("purchase_invoices", purchaseInvoiceHandler.List))

	// Invoices, behind the invoices feature. Each row carries the issuer letterhead, the client,
	// the populated entries and computed totals. Only /getAll is ported.
	mux.Handle("POST /invoice/getAll", feature("invoices", invoiceHandler.List))
	mux.Handle("GET /invoices", feature("invoices", invoiceHandler.List))

	// Lifecycle name/rate lookups for the Job and Quotation forms. Authenticated only (no
	// feature gate), as routes/Lifecycle.js registers them before its feature guard.
	mux.Handle("POST /lifecycle/lookups/clients", authed(lookupHandler.Clients))
	mux.Handle("POST /lifecycle/lookups/materials", authed(lookupHandler.Materials))
	mux.Handle("POST /lifecycle/lookups/people", authed(lookupHandler.People))

	// The public customer link from a WhatsApp message (routes/Alert.js). Unauthenticated -
	// the recipient is a customer with no login; the pair of ids is what authorises it, since
	// both must resolve to the same job. This is the exact path baked into links already
	// sent, so it is served as-is rather than under a REST alias nothing would call. The
	// review GET/POST are not ported yet.
	mux.HandleFunc("GET /alert/{job_id}/job/{jobcard_id}", alertHandler.Detail)
	mux.HandleFunc("GET /alert/{job_id}/job/{jobcard_id}/review", alertHandler.Review)
	mux.HandleFunc("POST /alert/{job_id}/job/{jobcard_id}/review", alertHandler.CreateReview)

	// Meta's webhook verification handshake. Unauthenticated - Meta carries no session - and
	// it answers with a real status and a bare body, not the envelope (#23). The POST callback
	// (HMAC over the raw body) is not ported yet.
	mux.HandleFunc("GET /whatsapp/webhook", whatsappHandler.Verify)

	mux.HandleFunc("GET /healthz", health(db))

	// Anything this service has not taken over yet.
	//
	// In the sideways stack nginx should never send these here, so the log line is how a
	// misrouted path announces itself. In the all-in-one stack there is no Node to fall back
	// to, so this is ALSO what the frontend gets for a screen that has not been ported - the
	// message says which, rather than leaving a blank panel with no explanation.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("not implemented here: %s %s", r.Method, r.URL.Path)
		httpx.Write(w, httpx.Envelope{
			Code:    404,
			Message: "This route has not been ported to the Go service yet.",
			Status:  httpx.False(),
		})
	})

	return logging(mux)
}

func health(db store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		// A service that is up but cannot reach its database is not healthy, and a TCP check
		// would call it healthy - the same reasoning as the Node container's healthcheck.
		if err := db.Ping(ctx); err != nil {
			httpx.Internal(w, err)
			return
		}
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "ok", Status: httpx.True()})
	}
}

// logging records method, path and duration. One line per request, because the first question
// asked of this service will be whether it is actually faster than the one it is replacing.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(100*time.Microsecond))
	})
}
