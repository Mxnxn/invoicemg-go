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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/alerts"
	"github.com/mxnxn/invoicemg-go/internal/analytics"
	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/bank"
	"github.com/mxnxn/invoicemg-go/internal/batchreceive"
	"github.com/mxnxn/invoicemg-go/internal/challan"
	"github.com/mxnxn/invoicemg-go/internal/client"
	"github.com/mxnxn/invoicemg-go/internal/company"
	"github.com/mxnxn/invoicemg-go/internal/config"
	"github.com/mxnxn/invoicemg-go/internal/days"
	"github.com/mxnxn/invoicemg-go/internal/entry"
	"github.com/mxnxn/invoicemg-go/internal/expense"
	"github.com/mxnxn/invoicemg-go/internal/gstreport"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/inventory"
	"github.com/mxnxn/invoicemg-go/internal/invoice"
	"github.com/mxnxn/invoicemg-go/internal/ledger"
	"github.com/mxnxn/invoicemg-go/internal/lifecycle"
	"github.com/mxnxn/invoicemg-go/internal/lookups"
	"github.com/mxnxn/invoicemg-go/internal/material"
	"github.com/mxnxn/invoicemg-go/internal/person"
	"github.com/mxnxn/invoicemg-go/internal/purchaseinvoice"
	"github.com/mxnxn/invoicemg-go/internal/purchasereport"
	"github.com/mxnxn/invoicemg-go/internal/quotation"
	"github.com/mxnxn/invoicemg-go/internal/sheet"
	"github.com/mxnxn/invoicemg-go/internal/statistics"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/store/mongostore"
	"github.com/mxnxn/invoicemg-go/internal/store/sqlstore"
	"github.com/mxnxn/invoicemg-go/internal/supplierpayment"
	"github.com/mxnxn/invoicemg-go/internal/trash"
	"github.com/mxnxn/invoicemg-go/internal/units"
	"github.com/mxnxn/invoicemg-go/internal/userinfo"
	"github.com/mxnxn/invoicemg-go/internal/users"
	"github.com/mxnxn/invoicemg-go/internal/wastage"
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
		Handler: routes(db, cfg.UploadsDir),
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

func routes(db store.Store, uploadsDir string) http.Handler {
	_ = os.MkdirAll(uploadsDir, 0o755)
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
	userHandler := users.New(db.Users(), db.Sessions())
	alertHandler := alerts.New(db.Alerts())
	bankHandler := bank.New(db.Banks())
	batchReceiveHandler := batchreceive.New(db.BatchReceives())
	supplierPaymentHandler := supplierpayment.New(db.SupplierPayments())
	trashHandler := trash.New(db.Trash())
	inventoryHandler := inventory.New(db.Inventory(), db.Companies())
	statsHandler := statistics.New(db.Statistics())
	ledgerHandler := ledger.New(db.Ledger())
	challanHandler := challan.New(db.Challans())
	expenseHandler := expense.New(db.Expenses())
	analyticsHandler := analytics.New(db.Analytics())
	gstHandler := gstreport.New(db.Analytics())
	wastageHandler := wastage.New(db.Wastages())
	entryHandler := entry.New(db.Entries())
	sheetHandler := sheet.New(db.Sheets())
	clientHandler := client.New(db.Clients(), db.BatchReceives())
	materialHandler := material.New(db.Materials())
	personHandler := person.New(db.People(), db.Users())
	quotationHandler := quotation.New(db.Quotations())
	purchaseInvoiceHandler := purchaseinvoice.New(db.PurchaseInvoices())
	invoiceHandler := invoice.New(db.Invoices(), db.Companies(), db.Users())
	purchaseReportHandler := purchasereport.New(db.PurchaseReport())
	lookupHandler := lookups.New(db.Lookups(), db.Users())
	lifecycleHandler := lifecycle.New(db.Jobs(), db.JobNotes())
	companyHandler := company.New(db.Companies(), db.Users(), db.Sessions())
	userinfoHandler := userinfo.New(db.Users(), db.Companies(), uploadsDir)
	whatsappHandler := whatsapp.New(os.Getenv("WHATSAPP_VERIFY_TOKEN"), db.Companies())

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
	mux.HandleFunc("POST /person/login", personHandler.Login)
	mux.Handle("POST /user/logout", authed(userHandler.Logout))
	mux.HandleFunc("POST /sessions", userHandler.Login)

	// Reads. GET under REST, so they are cacheable, safe to retry, and visible as reads in
	// any log - none of which is true of a POST.
	mux.Handle("POST /sheet/only", admin(dayHandler.Only))
	mux.Handle("GET /days", admin(dayHandler.Only))
	mux.Handle("POST /sheet/open-jobs", admin(dayHandler.OpenJobs))
	mux.Handle("POST /entry/add", admin(entryHandler.Add))
	mux.Handle("POST /entry/update", admin(entryHandler.Update))
	mux.Handle("POST /entry/get", admin(entryHandler.Get))
	mux.Handle("POST /entry/getall", admin(entryHandler.GetAll))
	mux.Handle("POST /sheet/get", admin(sheetHandler.Get))
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
	mux.Handle("POST /bank/create", feature("batch_receive", bankHandler.Create))

	// Customers, behind the customers feature. Both reads WIDEN by sharing (#1); the writes use
	// company-only scope. The populate-heavy /client/get is not ported.
	mux.Handle("POST /client/getall", feature("customers", clientHandler.Getall))
	mux.Handle("GET /clients", feature("customers", clientHandler.Getall))
	mux.Handle("POST /client/only", feature("customers", clientHandler.Only))
	mux.Handle("GET /clients/only", feature("customers", clientHandler.Only))
	mux.Handle("POST /client/add", feature("customers", clientHandler.Add, auth.RequireCreate("customers")))
	mux.Handle("POST /client/update", feature("customers", clientHandler.Update))
	mux.Handle("POST /client/remove", feature("customers", clientHandler.Remove, auth.RequireDelete("customers")))
	mux.Handle("POST /client/get", feature("customers", clientHandler.Get))
	mux.Handle("POST /client/batchUpdate", feature("customers", clientHandler.BatchUpdate))
	mux.Handle("POST /client/batchReceiveUpdate", feature("customers", clientHandler.BatchReceiveUpdate))
	mux.Handle("POST /client/batchReceiveDelete", feature("customers", clientHandler.BatchReceiveDelete))

	// The shell bootstrap: the company switcher, the active-company letterhead, and the admin
	// profile. /company/* need only a session; /userinfo/get is admin-only.
	mux.Handle("POST /company/list", authed(companyHandler.List))
	mux.Handle("GET /companies", authed(companyHandler.List))
	mux.Handle("POST /company/active", authed(companyHandler.Active))
	mux.Handle("POST /company/create", admin(companyHandler.Create))
	mux.Handle("POST /company/update", admin(companyHandler.Update))
	mux.Handle("POST /company/switch", authed(companyHandler.Switch))
	mux.Handle("POST /company/deactivate", admin(companyHandler.Deactivate))
	mux.Handle("POST /userinfo/get", admin(userinfoHandler.Get))
	mux.Handle("POST /userinfo/add", admin(userinfoHandler.Add))
	mux.Handle("POST /userinfo/update", admin(userinfoHandler.Update))
	mux.Handle("POST /userinfo/set-template", admin(userinfoHandler.SetTemplate))
	mux.Handle("POST /userinfo/upload", admin(userinfoHandler.Upload))

	// Products, behind the products feature as routes/Material.js is. Read widens by sharing
	// (#1); the writes and /material/get are not ported.
	mux.Handle("POST /material/getall", feature("products", materialHandler.Getall))
	mux.Handle("GET /materials", feature("products", materialHandler.Getall))
	mux.Handle("POST /material/add", feature("products", materialHandler.Add, auth.RequireCreate("products")))
	mux.Handle("POST /material/update", feature("products", materialHandler.Update))
	mux.Handle("POST /material/remove", feature("products", materialHandler.Remove, auth.RequireDelete("products")))
	mux.Handle("POST /material/get", feature("products", materialHandler.Get))

	// People (employees + suppliers), admin-only as routes/Person.js is. Only /list is ported.
	mux.Handle("POST /person/list", admin(personHandler.List))
	mux.Handle("GET /people", admin(personHandler.List))
	mux.Handle("POST /person/create", admin(personHandler.Create))
	mux.Handle("POST /person/update", admin(personHandler.Update))
	mux.Handle("POST /person/delete", admin(personHandler.Delete))

	// Quotations, behind the quotations feature. client_id is populated. Only /list is ported.
	mux.Handle("POST /quotation/list", feature("quotations", quotationHandler.List))
	mux.Handle("POST /quotation/row/add-to-job", feature("quotations", quotationHandler.AddRowToJob))
	mux.Handle("GET /quotations", feature("quotations", quotationHandler.List))
	mux.Handle("POST /quotation/next-quotation-number", feature("quotations", quotationHandler.NextNumber))
	mux.Handle("POST /quotation/get", feature("quotations", quotationHandler.Get))
	mux.Handle("POST /quotation/create", feature("quotations", quotationHandler.Create, auth.RequireCreate("quotations")))
	mux.Handle("POST /quotation/update", feature("quotations", quotationHandler.Update))
	mux.Handle("POST /quotation/delete", feature("quotations", quotationHandler.Delete, auth.RequireDelete("quotations")))
	mux.Handle("POST /quotation/row/delete", feature("quotations", quotationHandler.RowDelete, auth.RequireDelete("quotations")))

	// Purchase invoices (supplier bills), behind the purchase_invoices feature. supplier_id is
	// populated. Only /list is ported.
	mux.Handle("POST /purchase-invoice/list", feature("purchase_invoices", purchaseInvoiceHandler.List))
	mux.Handle("GET /purchase-invoices", feature("purchase_invoices", purchaseInvoiceHandler.List))
	mux.Handle("POST /purchase-invoice/create", feature("purchase_invoices", purchaseInvoiceHandler.Create, auth.RequireCreate("purchase_invoices")))
	mux.Handle("POST /purchase-invoice/update", feature("purchase_invoices", purchaseInvoiceHandler.Update))
	mux.Handle("POST /purchase-invoice/delete", feature("purchase_invoices", purchaseInvoiceHandler.Delete, auth.RequireDelete("purchase_invoices")))

	// Invoices, behind the invoices feature. Each row carries the issuer letterhead, the client,
	// the populated entries and computed totals. Only /getAll is ported.
	mux.Handle("POST /invoice/getAll", feature("invoices", invoiceHandler.List))
	mux.Handle("POST /invoice/get", feature("invoices", invoiceHandler.Get))
	mux.Handle("POST /invoice/getClientInvoices", feature("invoices", invoiceHandler.GetClientInvoices))
	mux.Handle("GET /invoices", feature("invoices", invoiceHandler.List))
	mux.Handle("POST /invoice/next-invoice-number", feature("invoices", invoiceHandler.NextNumber))
	mux.Handle("POST /invoice/entries-jobs", feature("invoices", invoiceHandler.EntriesJobs))
	mux.Handle("POST /invoice/getReceived", feature("invoices", invoiceHandler.GetReceived))
	mux.Handle("POST /invoice/save", feature("invoices", invoiceHandler.Save, auth.RequireCreate("invoices")))
	mux.Handle("POST /invoice/paid", feature("invoices", invoiceHandler.Paid))
	mux.Handle("POST /invoice/remove", feature("invoices", invoiceHandler.Remove, auth.RequireDelete("invoices")))

	// Lifecycle name/rate lookups for the Job and Quotation forms. Authenticated only (no
	// feature gate), as routes/Lifecycle.js registers them before its feature guard.
	mux.Handle("POST /lifecycle/lookups/clients", authed(lookupHandler.Clients))
	mux.Handle("POST /lifecycle/lookups/materials", authed(lookupHandler.Materials))
	mux.Handle("POST /lifecycle/lookups/people", authed(lookupHandler.People))

	// The Jobs board. Behind the lifecycle feature. Only /jobs/list is ported.
	mux.Handle("POST /lifecycle/jobs/list", feature("lifecycle", lifecycleHandler.List))
	mux.Handle("POST /lifecycle/jobs/next-challan-number", feature("lifecycle", lifecycleHandler.NextChallan))
	mux.Handle("POST /lifecycle/jobs/by-entry", feature("lifecycle", lifecycleHandler.ByEntry))
	mux.Handle("POST /lifecycle/jobs/create", feature("lifecycle", lifecycleHandler.Create))
	mux.Handle("POST /lifecycle/jobs/update", feature("lifecycle", lifecycleHandler.Update))
	mux.Handle("POST /lifecycle/jobs/assign", feature("lifecycle", lifecycleHandler.Assign))
	mux.Handle("POST /lifecycle/jobs/progress", feature("lifecycle", lifecycleHandler.Progress))
	mux.Handle("POST /lifecycle/jobs/queue", feature("lifecycle", lifecycleHandler.Queue))
	mux.Handle("POST /lifecycle/jobs/set-queue", feature("lifecycle", lifecycleHandler.SetQueue))
	mux.Handle("POST /lifecycle/jobs/rows/assign", feature("lifecycle", lifecycleHandler.RowAssign))
	mux.Handle("POST /lifecycle/jobs/rows/queue", feature("lifecycle", lifecycleHandler.RowQueue))
	mux.Handle("POST /lifecycle/jobs/rows/queue-order", feature("lifecycle", lifecycleHandler.RowQueueOrder))
	mux.Handle("POST /lifecycle/jobs/rows/progress", feature("lifecycle", lifecycleHandler.RowProgress))
	mux.Handle("POST /lifecycle/jobs/convert-to-entries", feature("lifecycle", lifecycleHandler.ConvertToEntries, auth.RequireAdmin))
	mux.Handle("POST /lifecycle/notes/list", feature("lifecycle", lifecycleHandler.NotesList))
	mux.Handle("POST /lifecycle/notes/create", feature("lifecycle", lifecycleHandler.NoteCreate))
	mux.Handle("POST /lifecycle/notes/update", feature("lifecycle", lifecycleHandler.NoteUpdate))
	mux.Handle("POST /lifecycle/history/list", feature("lifecycle", lifecycleHandler.HistoryList))
	mux.Handle("GET /jobs", feature("lifecycle", lifecycleHandler.List))

	// Delivery challans, behind the challan feature. Only /getAll is ported.
	mux.Handle("POST /challan/getAll", feature("challan", challanHandler.GetAll))

	// Expenses, behind the batch_receive feature. Only /list is ported.
	mux.Handle("POST /batch-receive/list", feature("batch_receive", batchReceiveHandler.List))
	mux.Handle("POST /batch-receive/lookups/open-jobs", feature("batch_receive", batchReceiveHandler.OpenJobs))
	mux.Handle("POST /batch-receive/create", feature("batch_receive", batchReceiveHandler.Create, auth.RequireCreate("batch_receive")))
	mux.Handle("POST /batch-receive/delete", feature("batch_receive", batchReceiveHandler.Delete, auth.RequireDelete("batch_receive")))
	mux.Handle("POST /supplier-payment/list", feature("purchase_invoices", supplierPaymentHandler.List))
	mux.Handle("POST /supplier-payment/lookups/open-invoices", feature("purchase_invoices", supplierPaymentHandler.OpenInvoices))
	mux.Handle("POST /supplier-payment/create", feature("purchase_invoices", supplierPaymentHandler.Create, auth.RequireCreate("purchase_invoices")))
	mux.Handle("POST /supplier-payment/delete", feature("purchase_invoices", supplierPaymentHandler.Delete, auth.RequireDelete("purchase_invoices")))
	mux.Handle("POST /expense/list", feature("batch_receive", expenseHandler.List))
	mux.Handle("GET /expenses", feature("batch_receive", expenseHandler.List))

	// Analytics (the reporting dashboards). Only /revenue is ported so far; the other tabs
	// still 404 until their aggregations are ported.
	mux.Handle("POST /analytics/revenue", feature("analytics", analyticsHandler.Revenue))
	mux.Handle("POST /analytics/top-sales", feature("analytics", analyticsHandler.TopSales))
	mux.Handle("POST /analytics/top-credits", feature("analytics", analyticsHandler.TopCredits))
	mux.Handle("POST /analytics/top-paid", feature("analytics", analyticsHandler.TopPaid))
	mux.Handle("POST /analytics/avg-payment-time", feature("analytics", analyticsHandler.AvgPaymentTime))
	mux.Handle("POST /analytics/avg-pending-time", feature("analytics", analyticsHandler.AvgPendingTime))
	mux.Handle("POST /analytics/payables", feature("analytics", analyticsHandler.Payables))
	mux.Handle("POST /analytics/aging", feature("analytics", analyticsHandler.Aging))
	mux.Handle("POST /analytics/unbilled", feature("analytics", analyticsHandler.Unbilled))
	mux.Handle("POST /analytics/reviews", feature("analytics", analyticsHandler.Reviews))
	mux.Handle("POST /analytics/payout-weekday", feature("analytics", analyticsHandler.PayoutWeekday))
	mux.Handle("POST /analytics/cashflow", feature("analytics", analyticsHandler.Cashflow))

	mux.Handle("POST /gst-report", feature("gst_report", gstHandler.Report))

	mux.Handle("POST /trash/get", feature("trash", trashHandler.Get))
	mux.Handle("POST /trash/setPassword", feature("trash", trashHandler.SetPassword))

	mux.Handle("POST /inventory/report", feature("products", inventoryHandler.Report))

	mux.Handle("POST /stats/get", feature("dashboard", statsHandler.Get))
	mux.Handle("GET /stats/clients", feature("dashboard", statsHandler.Clients))

	mux.Handle("POST /ledger/client", feature("ledger", ledgerHandler.Client))
	mux.Handle("POST /ledger/dues", feature("ledger", ledgerHandler.Dues))
	mux.Handle("POST /purchase-report/dues", feature("purchase_invoices", purchaseReportHandler.Dues))
	mux.Handle("POST /purchase-report/supplier", feature("purchase_invoices", purchaseReportHandler.Supplier))

	mux.Handle("POST /wastage/getall", feature("challan", wastageHandler.GetAll))
	mux.Handle("POST /wastage/materials", authed(wastageHandler.Materials))
	mux.Handle("POST /wastage/add", feature("challan", wastageHandler.Add, auth.RequireCreate("challan")))

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

	// Public company-logo serving, the Go side of Node's `app.use("/uploads", express.static(...))`.
	// Deliberately unauthenticated (logos are embedded in customer-facing PDFs), basename-only so
	// a crafted name cannot escape the directory, and dotfiles are denied.
	mux.HandleFunc("GET /uploads/{fname}", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.PathValue("fname"))
		if name == "" || name == "." || strings.HasPrefix(name, ".") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(uploadsDir, name))
	})
	mux.Handle("POST /whatsapp/config", feature("whatsapp", whatsappHandler.Config))
	mux.Handle("POST /whatsapp/config/update", feature("whatsapp", whatsappHandler.ConfigUpdate, auth.RequireAdmin))
	mux.Handle("POST /whatsapp/send", feature("whatsapp", whatsappHandler.Send))

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
