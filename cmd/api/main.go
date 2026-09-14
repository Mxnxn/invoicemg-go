// Command api is the Go service that runs BESIDE the Node API, not instead of it.
//
// Local only for now: it listens on :5002 while Node keeps :5001, reads the same MongoDB, and
// nothing in this repository touches the production compose file or the deploy workflow. See
// README.md for how a route moves across.
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

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/config"
	"github.com/mxnxn/invoicemg-go/internal/days"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store/mongostore"
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

	db, err := mongostore.Open(ctx, cfg.MongoURI, cfg.MongoDB, cfg.ConnectTimeout)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			log.Printf("closing mongo: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: routes(db),
		// A request that has not finished in this long is not going to. The Node app has no
		// equivalent, which is why one slow Mongo query there can hold a connection open
		// indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("invoicemg-go listening on %s, mongo %s/%s", cfg.Addr, cfg.MongoURI, cfg.MongoDB)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		// Finish what is in flight before exiting, so a deploy or a Ctrl-C does not cut a
		// response in half.
		log.Print("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

type pinger interface {
	Ping(ctx context.Context) error
}

func routes(db *mongostore.Store) http.Handler {
	mux := http.NewServeMux()

	guard := auth.New(db.Sessions())
	admin := func(h http.HandlerFunc) http.Handler {
		return auth.Chain(h, guard.Require, auth.RequireAdmin)
	}

	dayHandler := days.New(db.Days())

	// Method and path together, which Go 1.22's mux understands. The Node app mounts
	// everything as POST, including reads, and that is preserved: the frontend posts FormData
	// to all of these, and changing a verb would mean changing the client.
	mux.Handle("POST /sheet/only", admin(dayHandler.Only))
	mux.Handle("POST /sheet/open-jobs", admin(dayHandler.OpenJobs))

	mux.HandleFunc("GET /healthz", health(db))

	// Anything this service has not taken over yet. Returning a clear signal rather than a
	// bare 404 page matters while both services are live: if nginx sends a path here by
	// mistake, this says so in the log instead of looking like an application error.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("not handled here: %s %s", r.Method, r.URL.Path)
		httpx.Write(w, httpx.Envelope{
			Code:    404,
			Message: "This route is still served by the Node API.",
			Status:  httpx.False(),
		})
	})

	return logging(mux)
}

func health(db pinger) http.HandlerFunc {
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
