// Package sqlstore implements store.Store against PostgreSQL.
//
// This package is the point of internal/store. It was written after mongostore, against the
// same interfaces, and NOT ONE HANDLER CHANGED - which is the whole claim the seam was built
// to make, now tested rather than asserted.
//
// It is also where the differences show. Counting "job-ids with a card short of Done" is an
// $elemMatch over a subdocument array in Mongo and an EXISTS over an indexed column here.
// Grouping jobs by day is an aggregation pipeline there and a GROUP BY here. The queries are
// shorter, the indexes are real, and a row is finally addressable on its own.
package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string, timeout time.Duration) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parsing postgres url: %w", err)
	}
	// A small pool on purpose: this service is meant to run in a few hundred megabytes, and
	// every idle connection costs memory on the database side too. Raise it when a measurement
	// says to, not before.
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Close(_ context.Context) error  { s.pool.Close(); return nil }
func (s *Store) Sessions() store.Sessions       { return &sessions{pool: s.pool} }
func (s *Store) Days() store.Days               { return &days{pool: s.pool} }
func (s *Store) Units() store.Units             { return &units{pool: s.pool} }
func (s *Store) Users() store.Users             { return &users{pool: s.pool} }

// isUniqueViolation recognises SQLSTATE 23505, the Postgres equivalent of Mongo's E11000.
// Both are translated to store.ErrDuplicate so the handler that turns it into "That unit
// already exists." is identical for either backend - which is the seam doing its job.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func noRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// ---------------------------------------------------------------------------------------

type sessions struct{ pool *pgxpool.Pool }

func (s *sessions) FindByToken(ctx context.Context, token string) (store.Session, error) {
	var sess store.Session
	var personID *string
	var expiresAt *time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, uid, role, person_id, permissions, is_active, expires_at
		  FROM user_sessions
		 WHERE token = $1`, token).
		Scan(&sess.SessionID, &sess.UID, &sess.Role, &personID, &sess.Permissions, &sess.IsActive, &expiresAt)

	if noRows(err) {
		return store.Session{}, store.ErrNotFound
	}
	if err != nil {
		return store.Session{}, fmt.Errorf("looking up session: %w", err)
	}

	sess.Token = token
	sess.ExpiresAt = expiresAt
	if personID != nil {
		sess.PersonID = store.ID(*personID)
	}
	return sess, nil
}

func (s *sessions) Deactivate(ctx context.Context, sessionID store.ID) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_sessions SET is_active = false WHERE id = $1`, string(sessionID))
	if err != nil {
		return fmt.Errorf("retiring session: %w", err)
	}
	return nil
}

func (s *sessions) ResolveCompany(ctx context.Context, token, tabID string, uid store.ID) (store.ID, error) {
	// An existing binding for this tab wins.
	if tabID != "" {
		var companyID string
		err := s.pool.QueryRow(ctx,
			`SELECT company_id FROM company_sessions WHERE token = $1 AND tab_id = $2`, token, tabID).
			Scan(&companyID)
		if err == nil {
			return store.ID(companyID), nil
		}
		if !noRows(err) {
			return "", fmt.Errorf("looking up tab binding: %w", err)
		}
	}

	// Otherwise the user's default company, or any company they own. ORDER BY is_default DESC
	// does in one query what the Node helper does in two - and, unlike two queries, cannot
	// return a different company between them if something changes in between.
	var companyID string
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM companies
		 WHERE uid = $1
		 ORDER BY is_default DESC, created_at ASC
		 LIMIT 1`, string(uid)).Scan(&companyID)
	if noRows(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("looking up default company: %w", err)
	}

	// Persist the fallback, or a reload puts the tab on a different company than it was on.
	if tabID != "" {
		_, err = s.pool.Exec(ctx, `
			INSERT INTO company_sessions (token, tab_id, uid, company_id)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (token, tab_id) DO UPDATE SET company_id = EXCLUDED.company_id`,
			token, tabID, string(uid), companyID)
		if err != nil {
			return "", fmt.Errorf("persisting tab binding: %w", err)
		}
	}
	return store.ID(companyID), nil
}

func (s *sessions) BindCompany(ctx context.Context, token, tabID string, uid, companyID store.ID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO company_sessions (token, tab_id, uid, company_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token, tab_id) DO UPDATE SET company_id = EXCLUDED.company_id`,
		token, tabID, string(uid), string(companyID))
	if err != nil {
		return fmt.Errorf("binding tab to company: %w", err)
	}
	return nil
}

// DeactivateOthers retires every other session of uid (all but keepToken), for
// /user/password/change.
func (s *sessions) DeactivateOthers(ctx context.Context, uid store.ID, keepToken string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE user_sessions SET is_active = false WHERE uid = $1 AND token <> $2`,
		string(uid), keepToken)
	if err != nil {
		return fmt.Errorf("deactivate other sessions: %w", err)
	}
	return nil
}

func (s *sessions) LastLoginAt(ctx context.Context, uid store.ID) (*time.Time, error) {
	var at *time.Time
	err := s.pool.QueryRow(ctx, `SELECT max(created_at) FROM user_sessions WHERE uid=$1`, string(uid)).Scan(&at)
	if err != nil {
		return nil, fmt.Errorf("last login: %w", err)
	}
	return at, nil
}
