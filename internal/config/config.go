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

type Config struct {
	// Where to listen. ":5002" by default, deliberately NOT 5000/5001 - the Node API owns
	// those, and this service is meant to run beside it, not instead of it.
	Addr string

	// The same MongoDB the Node API uses. During the sideways phase both services read the
	// same documents; see README.md for why that rules out Postgres until cutover.
	MongoURI string
	MongoDB  string

	// How long to wait for Mongo before giving up at startup.
	ConnectTimeout time.Duration
}

func Load() (Config, error) {
	c := Config{
		Addr:           env("ADDR", ":5002"),
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

	if c.MongoURI == "" {
		return Config{}, fmt.Errorf("MONGO_URI must be set")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
