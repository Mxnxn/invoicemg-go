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
		SELECT id, email, password, name, firm, role, company_limit, is_active, active_until, totp_enabled, totp_secret
		  FROM users
		 WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Firm,
			&user.Role, &user.CompanyLimit, &user.IsActive, &activeUntil, &user.TotpEnabled, &totpSecret)

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
		SELECT id, email, password, name, firm, role, company_limit, is_active, active_until, totp_enabled, totp_secret
		  FROM users
		 WHERE id = $1`, string(uid)).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Firm,
			&user.Role, &user.CompanyLimit, &user.IsActive, &activeUntil, &user.TotpEnabled, &totpSecret)

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

func (u *users) SetActive(ctx context.Context, uid store.ID, isActive *bool, activeUntil *time.Time, setActiveUntil bool) (store.User, bool, error) {
	set := "updated_at=now()"
	args := []any{string(uid)}
	n := 2
	if isActive != nil {
		set += fmt.Sprintf(", is_active=$%d", n)
		args = append(args, *isActive)
		n++
	}
	if setActiveUntil {
		set += fmt.Sprintf(", active_until=$%d", n)
		args = append(args, activeUntil)
		n++
	}
	tag, err := u.pool.Exec(ctx, "UPDATE users SET "+set+" WHERE id=$1", args...)
	if err != nil {
		return store.User{}, false, fmt.Errorf("set active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.User{}, false, nil
	}
	usr, err := u.FindByID(ctx, uid)
	return usr, true, err
}

func (u *users) SetCompanyLimit(ctx context.Context, uid store.ID, limit int) (store.User, bool, error) {
	tag, err := u.pool.Exec(ctx, `UPDATE users SET company_limit=$2, updated_at=now() WHERE id=$1`, string(uid), limit)
	if err != nil {
		return store.User{}, false, fmt.Errorf("set company limit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.User{}, false, nil
	}
	usr, err := u.FindByID(ctx, uid)
	return usr, true, err
}

func (u *users) AdminList(ctx context.Context) ([]store.User, error) {
	rows, err := u.pool.Query(ctx, `
		SELECT id, email, name, firm, role, company_limit, is_active, active_until
		  FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("admin list: %w", err)
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var usr store.User
		var activeUntil *time.Time
		if err := rows.Scan(&usr.ID, &usr.Email, &usr.Name, &usr.Firm, &usr.Role, &usr.CompanyLimit, &usr.IsActive, &activeUntil); err != nil {
			return nil, err
		}
		usr.ActiveUntil = activeUntil
		out = append(out, usr)
	}
	return out, rows.Err()
}
