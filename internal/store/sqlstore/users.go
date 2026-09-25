package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type users struct{ pool *pgxpool.Pool }

func (u *users) FindByEmail(ctx context.Context, email string) (store.User, error) {
	var user store.User
	var activeUntil *time.Time
	var totpSecret *string

	err := u.pool.QueryRow(ctx, `
		SELECT id, email, password, name, firm, role, company_limit, active_until, totp_enabled, totp_secret
		  FROM users
		 WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Firm,
			&user.Role, &user.CompanyLimit, &activeUntil, &user.TotpEnabled, &totpSecret)

	if noRows(err) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("looking up user: %w", err)
	}

	user.ActiveUntil = activeUntil
	if totpSecret != nil {
		user.TotpSecret = *totpSecret
	}
	return user, nil
}

func (u *users) FindByID(ctx context.Context, uid store.ID) (store.User, error) {
	var user store.User
	var activeUntil *time.Time
	var totpSecret *string

	err := u.pool.QueryRow(ctx, `
		SELECT id, email, password, name, firm, role, company_limit, active_until, totp_enabled, totp_secret
		  FROM users
		 WHERE id = $1`, string(uid)).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Firm,
			&user.Role, &user.CompanyLimit, &activeUntil, &user.TotpEnabled, &totpSecret)

	if noRows(err) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("looking up user: %w", err)
	}
	user.ActiveUntil = activeUntil
	if totpSecret != nil {
		user.TotpSecret = *totpSecret
	}
	return user, nil
}

func (u *users) UpdateProfile(ctx context.Context, uid store.ID, email, name string) (store.User, bool, error) {
	ct, err := u.pool.Exec(ctx,
		`UPDATE users SET email = $1, name = $2 WHERE id = $3`, email, name, string(uid))
	if err != nil {
		return store.User{}, false, fmt.Errorf("updating user profile: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return store.User{}, false, nil
	}
	user, err := u.FindByID(ctx, uid)
	if err != nil {
		return store.User{}, false, err
	}
	return user, true, nil
}

func (u *users) CreateSession(ctx context.Context, s store.NewSession) (store.Session, error) {
	// permissions is a text[] and must never be NULL - the column is NOT NULL DEFAULT '{}',
	// and a nil slice would fail the insert rather than become an empty array.
	perms := s.Permissions
	if perms == nil {
		perms = []string{}
	}

	var created store.Session
	var personID *string
	var expiresAt *time.Time

	var personArg any
	if s.PersonID != "" {
		v := string(s.PersonID)
		personArg = v
	}

	err := u.pool.QueryRow(ctx, `
		INSERT INTO user_sessions (token, uid, role, person_id, permissions, remembered, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, uid, role, person_id, permissions, is_active, expires_at`,
		s.Token, string(s.UID), s.Role, personArg, perms, s.Remembered, s.ExpiresAt).
		Scan(&created.SessionID, &created.UID, &created.Role, &personID,
			&created.Permissions, &created.IsActive, &expiresAt)
	if err != nil {
		return store.Session{}, fmt.Errorf("creating session: %w", err)
	}

	created.Token = s.Token
	created.ExpiresAt = expiresAt
	if personID != nil {
		created.PersonID = store.ID(*personID)
	}
	return created, nil
}

func (u *users) UpdatePassword(ctx context.Context, uid store.ID, passwordHash string) (bool, error) {
	tag, err := u.pool.Exec(ctx, `UPDATE users SET password = $2 WHERE id = $1`, string(uid), passwordHash)
	if err != nil {
		return false, fmt.Errorf("update password: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (u *users) SetTotpSecret(ctx context.Context, uid store.ID, secret string) error {
	_, err := u.pool.Exec(ctx, `UPDATE users SET totp_secret=$2 WHERE id=$1`, string(uid), secret)
	if err != nil {
		return fmt.Errorf("set totp secret: %w", err)
	}
	return nil
}

func (u *users) SetTotpEnabled(ctx context.Context, uid store.ID, enabled bool) error {
	_, err := u.pool.Exec(ctx, `UPDATE users SET totp_enabled=$2 WHERE id=$1`, string(uid), enabled)
	if err != nil {
		return fmt.Errorf("set totp enabled: %w", err)
	}
	return nil
}

func (u *users) ClearTotp(ctx context.Context, uid store.ID) error {
	_, err := u.pool.Exec(ctx, `UPDATE users SET totp_enabled=false, totp_secret=NULL WHERE id=$1`, string(uid))
	if err != nil {
		return fmt.Errorf("clear totp: %w", err)
	}
	return nil
}

func (u *users) Register(ctx context.Context, email, passwordHash, name string, activeUntil time.Time) (store.ID, bool, error) {
	var id string
	err := u.pool.QueryRow(ctx, `INSERT INTO users (email, password, name, active_until) VALUES ($1,$2,$3,$4) RETURNING id`,
		email, passwordHash, name, activeUntil).Scan(&id)
	if isUniqueViolation(err) {
		return "", true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("register user: %w", err)
	}
	return store.ID(id), false, nil
}
