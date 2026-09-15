// Package config reads the handful of settings this service needs from the environment.
//
// No config file and no framework: twelve-factor, so the same binary runs under docker
// compose locally and anywhere else later without a rebuild.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	StoreMongo    = "mongo"
	StorePostgres = "postgres"
)

type Config struct {
	// Where to listen. ":5002" by default, deliberately NOT 5000/5001 - the Node API owns
	// those, and this service is meant to run beside it, not instead of it.
	Addr string

	// Which backend to talk to: "mongo" or "postgres".
	//
	// Two modes, and the choice is not a preference - it decides what this service IS.
	//
	//   mongo     the sideways stack. Same documents as the live Node API, so both can answer
	//             the same request and be diffed against each other.
	//   postgres  the all-in-one local stack. Its OWN database, seeded from deploy/postgres -
	//             a preview of life after cutover, sharing nothing with the Node app.
	//
	// Pointing the postgres mode at anything real would give you two services writing two
	// databases that disagree, which is exactly what the sideways phase exists to avoid.
	Store string

	// The same MongoDB the Node API uses, when Store is "mongo".
	MongoURI string
	MongoDB  string

	// Where Postgres is, when Store is "postgres".
	PostgresURL string

	// "legacy" or "rest" - see internal/httpx/style.go.
	//
	// Defaults to legacy, because the default deployment of this service is beside the Node
	// API, where answering a real HTTP status would break the client. The all-in-one stack
	// sets rest explicitly.
	APIStyle string

	// How long to wait for Mongo before giving up at startup.
	ConnectTimeout time.Duration

	// Where uploaded company logos are stored and served from (routes/UserInfo /upload and the
	// public /uploads mount). Defaults to "uploads" beside the binary.
	UploadsDir string

	// Where generated .xlsx exports are written and streamed from. Defaults to "exports".
	ExportsDir string
}

func Load() (Config, error) {
	c := Config{
		Addr:           env("ADDR", ":5002"),
		Store:          env("STORE", StoreMongo),
		APIStyle:       env("API_STYLE", "legacy"),
		UploadsDir:     env("UPLOADS_DIR", "uploads"),
		ExportsDir:     env("EXPORTS_DIR", "exports"),
		PostgresURL:    env("POSTGRES_URL", ""),
		MongoURI:       env("MONGO_URI", "mongodb://127.0.0.1:27018"),
		MongoDB:        env("MONGO_DB", "invoicemg_new"),
		ConnectTimeout: 10 * time.Second,
	}

	if raw := os.Getenv("CONNECT_TIMEOUT_SECONDS"); raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("CONNECT_TIMEOUT_SECONDS must be a whole number of seconds, got %q", raw)
		}
		c.ConnectTimeout = time.Duration(secs) * time.Second
	}

	if c.APIStyle != "legacy" && c.APIStyle != "rest" {
		return Config{}, fmt.Errorf(`API_STYLE must be "legacy" or "rest", got %q`, c.APIStyle)
	}

	switch c.Store {
	case StoreMongo:
		if c.MongoURI == "" {
			return Config{}, fmt.Errorf("MONGO_URI must be set when STORE=mongo")
		}
	case StorePostgres:
		if c.PostgresURL == "" {
			return Config{}, fmt.Errorf("POSTGRES_URL must be set when STORE=postgres")
		}
	default:
		return Config{}, fmt.Errorf("STORE must be %q or %q, got %q", StoreMongo, StorePostgres, c.Store)
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
