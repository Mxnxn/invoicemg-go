package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type registrationTokens struct{ pool *pgxpool.Pool }

func (s *Store) RegistrationTokens() store.RegistrationTokens { return &registrationTokens{pool: s.pool} }

func scanRegToken(row rowScanner) (store.RegistrationToken, error) {
	var t store.RegistrationToken
	var id string
	var usedBy *string
	if err := row.Scan(&id, &t.Token, &t.ExpiresAt, &t.UsedAt, &usedBy, &t.CreatedAt); err != nil {
		return store.RegistrationToken{}, err
	}
	t.ID = store.ID(id)
	if usedBy != nil {
		t.UsedBy = store.ID(*usedBy)
	}
	return t, nil
}

const regTokenCols = `id, token, expires_at, used_at, used_by, created_at`

func (r *registrationTokens) FindByToken(ctx context.Context, token string) (store.RegistrationToken, bool, error) {
	t, err := scanRegToken(r.pool.QueryRow(ctx, `SELECT `+regTokenCols+` FROM registration_tokens WHERE token=$1`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.RegistrationToken{}, false, nil
	}
	if err != nil {
		return store.RegistrationToken{}, false, fmt.Errorf("find registration token: %w", err)
	}
	return t, true, nil
}

func (r *registrationTokens) MarkUsed(ctx context.Context, id, uid store.ID) error {
	_, err := r.pool.Exec(ctx, `UPDATE registration_tokens SET used_at=now(), used_by=$2, updated_at=now() WHERE id=$1`,
		string(id), string(uid))
	if err != nil {
		return fmt.Errorf("mark registration token used: %w", err)
	}
	return nil
}

func (r *registrationTokens) Create(ctx context.Context, token string, expiresAt time.Time) (store.RegistrationToken, error) {
	t := store.RegistrationToken{Token: token, ExpiresAt: expiresAt}
	var id string
	if err := r.pool.QueryRow(ctx, `INSERT INTO registration_tokens (token, expires_at) VALUES ($1,$2) RETURNING id, created_at`,
		token, expiresAt).Scan(&id, &t.CreatedAt); err != nil {
		return store.RegistrationToken{}, fmt.Errorf("create registration token: %w", err)
	}
	t.ID = store.ID(id)
	return t, nil
}

func (r *registrationTokens) List(ctx context.Context) ([]store.RegistrationToken, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+regTokenCols+` FROM registration_tokens ORDER BY created_at DESC LIMIT 25`)
	if err != nil {
		return nil, fmt.Errorf("list registration tokens: %w", err)
	}
	defer rows.Close()
	out := []store.RegistrationToken{}
	for rows.Next() {
		t, err := scanRegToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type passwordResetRequests struct{ pool *pgxpool.Pool }

func (s *Store) PasswordResetRequests() store.PasswordResetRequests {
	return &passwordResetRequests{pool: s.pool}
}

const prrCols = `id, email, uid, status, note, resolved_at, created_at`

func scanPRR(row rowScanner) (store.PasswordResetRequest, error) {
	var p store.PasswordResetRequest
	var id string
	var uid *string
	if err := row.Scan(&id, &p.Email, &uid, &p.Status, &p.Note, &p.ResolvedAt, &p.CreatedAt); err != nil {
		return store.PasswordResetRequest{}, err
	}
	p.ID = store.ID(id)
	if uid != nil {
		p.UID = store.ID(*uid)
	}
	return p, nil
}

func (p *passwordResetRequests) Create(ctx context.Context, email string, uid store.ID) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO password_reset_requests (email, uid) VALUES ($1,$2)`, email, nullID(uid))
	if err != nil {
		return fmt.Errorf("create password reset request: %w", err)
	}
	return nil
}

func (p *passwordResetRequests) HasPending(ctx context.Context, email string) (bool, error) {
	var exists bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM password_reset_requests WHERE email=$1 AND status='pending')`, email).Scan(&exists); err != nil {
		return false, fmt.Errorf("password reset pending: %w", err)
	}
	return exists, nil
}

func (p *passwordResetRequests) ListPending(ctx context.Context) ([]store.PasswordResetRequest, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+prrCols+` FROM password_reset_requests WHERE status='pending' ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("list password reset requests: %w", err)
	}
	defer rows.Close()
	out := []store.PasswordResetRequest{}
	for rows.Next() {
		r, err := scanPRR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *passwordResetRequests) Get(ctx context.Context, id store.ID) (store.PasswordResetRequest, bool, error) {
	r, err := scanPRR(p.pool.QueryRow(ctx, `SELECT `+prrCols+` FROM password_reset_requests WHERE id=$1`, string(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.PasswordResetRequest{}, false, nil
	}
	if err != nil {
		return store.PasswordResetRequest{}, false, fmt.Errorf("get password reset request: %w", err)
	}
	return r, true, nil
}

func (p *passwordResetRequests) Resolve(ctx context.Context, id store.ID, status string) error {
	_, err := p.pool.Exec(ctx, `UPDATE password_reset_requests SET status=$2, resolved_at=now(), updated_at=now() WHERE id=$1`, string(id), status)
	if err != nil {
		return fmt.Errorf("resolve password reset request: %w", err)
	}
	return nil
}
