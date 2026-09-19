package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type people struct{ pool *pgxpool.Pool }

func (s *Store) People() store.People { return &people{pool: s.pool} }

func (p *people) List(ctx context.Context, uid store.ID, personType string) ([]store.Person, error) {
	// Optional type filter: $2 = '' means no filter (the People vs Suppliers tabs pass a type).
	// id is the tiebreak (#19).
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, type, email, phone, firm, address, gst, opening_balance, is_active,
		       permissions, notify_po_created, notify_po_updated, notify_po_confirmed, created_at
		  FROM persons
		 WHERE uid = $1 AND ($2 = '' OR type = $2)
		 ORDER BY id ASC`, string(uid), personType)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	defer rows.Close()

	out := make([]store.Person, 0)
	for rows.Next() {
		var pr store.Person
		var email *string
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Type, &email, &pr.Phone, &pr.Firm, &pr.Address,
			&pr.Gst, &pr.OpeningBalance, &pr.IsActive, &pr.Permissions,
			&pr.NotifyPoCreated, &pr.NotifyPoUpdated, &pr.NotifyPoConfirmed, &pr.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading people: %w", err)
		}
		if email != nil {
			pr.Email = *email
		}
		if pr.Permissions == nil {
			pr.Permissions = []string{}
		}
		out = append(out, pr)
	}
	return out, rows.Err()
}

func (p *people) Create(ctx context.Context, uid store.ID, in store.PersonWrite) (store.Person, bool, error) {
	perms := in.Permissions
	if perms == nil {
		perms = []string{}
	}
	var id string
	err := p.pool.QueryRow(ctx, `
		INSERT INTO persons (uid, name, type, email, phone, firm, address, gst, password, permissions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		string(uid), in.Name, in.Type, in.Email, in.Phone, in.Firm, in.Address, in.Gst, in.PasswordHash, perms).Scan(&id)
	if isUniqueViolation(err) {
		return store.Person{}, true, nil
	}
	if err != nil {
		return store.Person{}, false, fmt.Errorf("insert person: %w", err)
	}
	created, err := p.one(ctx, uid, store.ID(id))
	return created, false, err
}

func (p *people) Update(ctx context.Context, uid, personID store.ID, patch store.PersonPatch) (store.Person, bool, bool, error) {
	// Load the current row (owner-scoped) so only the submitted fields change and the password
	// stays put unless a new one is given.
	cur, err := p.one(ctx, uid, personID)
	if noRows(err) {
		return store.Person{}, false, false, nil
	}
	if err != nil {
		return store.Person{}, false, false, err
	}

	name, typ := cur.Name, cur.Type
	if patch.Name != nil {
		name = *patch.Name
	}
	if patch.Type != nil {
		typ = *patch.Type
	}
	var email *string
	if cur.Email != "" {
		e := cur.Email
		email = &e
	}
	if patch.EmailSet {
		email = patch.Email // nil (NULL) when submitted empty
	}
	phone, firm, address, gst := cur.Phone, cur.Firm, cur.Address, cur.Gst
	if patch.Phone != nil {
		phone = *patch.Phone
	}
	if patch.Firm != nil {
		firm = *patch.Firm
	}
	if patch.Address != nil {
		address = *patch.Address
	}
	if patch.Gst != nil {
		gst = *patch.Gst
	}
	isActive := cur.IsActive
	if patch.IsActive != nil {
		isActive = *patch.IsActive
	}
	perms := cur.Permissions
	if patch.Permissions != nil {
		perms = *patch.Permissions
	}
	if perms == nil {
		perms = []string{}
	}

	// The password column is updated only when a new hash is supplied ($9 IS NOT NULL keeps the
	// existing value otherwise), so an edit that omits the password never clears it.
	_, err = p.pool.Exec(ctx, `
		UPDATE persons SET name=$3, type=$4, email=$5, phone=$6, firm=$7, address=$8, gst=$9,
		       is_active=$10, permissions=$11, password=COALESCE($12, password), updated_at=now()
		 WHERE id=$1 AND uid=$2`,
		string(personID), string(uid), name, typ, email, phone, firm, address, gst, isActive, perms, patch.PasswordHash)
	if isUniqueViolation(err) {
		return store.Person{}, true, false, nil
	}
	if err != nil {
		return store.Person{}, false, false, fmt.Errorf("update person: %w", err)
	}
	updated, err := p.one(ctx, uid, personID)
	if err != nil {
		return store.Person{}, false, false, err
	}
	return updated, false, true, nil
}

func (p *people) Delete(ctx context.Context, uid, personID store.ID) (bool, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM persons WHERE id=$1 AND uid=$2`, string(personID), string(uid))
	if err != nil {
		return false, fmt.Errorf("delete person: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// one reads a single owner-scoped person (no password), shaped like the list rows.
func (p *people) one(ctx context.Context, uid, personID store.ID) (store.Person, error) {
	var pr store.Person
	var email *string
	err := p.pool.QueryRow(ctx, `
		SELECT id, name, type, email, phone, firm, address, gst, opening_balance, is_active,
		       permissions, notify_po_created, notify_po_updated, notify_po_confirmed, created_at
		  FROM persons WHERE id=$1 AND uid=$2`, string(personID), string(uid)).
		Scan(&pr.ID, &pr.Name, &pr.Type, &email, &pr.Phone, &pr.Firm, &pr.Address, &pr.Gst,
			&pr.OpeningBalance, &pr.IsActive, &pr.Permissions,
			&pr.NotifyPoCreated, &pr.NotifyPoUpdated, &pr.NotifyPoConfirmed, &pr.CreatedAt)
	if err != nil {
		return store.Person{}, err
	}
	if email != nil {
		pr.Email = *email
	}
	if pr.Permissions == nil {
		pr.Permissions = []string{}
	}
	return pr, nil
}

// FindEmployeeByEmail loads an Employee's credentials by exact email for the portal login.
func (p *people) FindEmployeeByEmail(ctx context.Context, email string) (store.EmployeeAuth, bool, error) {
	var a store.EmployeeAuth
	var emailCol, passwordCol *string
	err := p.pool.QueryRow(ctx, `
		SELECT id, uid, name, email, password, is_active, permissions
		  FROM persons WHERE email = $1 AND type = 'Employee' LIMIT 1`, email).
		Scan(&a.ID, &a.UID, &a.Name, &emailCol, &passwordCol, &a.IsActive, &a.Permissions)
	if noRows(err) {
		return store.EmployeeAuth{}, false, nil
	}
	if err != nil {
		return store.EmployeeAuth{}, false, fmt.Errorf("employee login lookup: %w", err)
	}
	if emailCol != nil {
		a.Email = *emailCol
	}
	if passwordCol != nil {
		a.PasswordHash = *passwordCol
	}
	return a, true, nil
}

// notifyColumn maps a NotifyPo* wire name to its column. A whitelist, not interpolation of
// caller input: the result is one of three literals, so the query text is never caller-shaped.
func notifyColumn(field string) (string, bool) {
	switch field {
	case "notifyPoCreated":
		return "notify_po_created", true
	case "notifyPoUpdated":
		return "notify_po_updated", true
	case "notifyPoConfirmed":
		return "notify_po_confirmed", true
	default:
		return "", false
	}
}

func (p *people) SetNotifyField(ctx context.Context, uid, personID store.ID, field string, value *bool) (store.Person, bool, error) {
	col, ok := notifyColumn(field)
	if !ok {
		return store.Person{}, false, fmt.Errorf("unknown notify field %q", field)
	}
	// value is *bool: nil encodes as NULL ("clear"), &true/&false as the boolean.
	tag, err := p.pool.Exec(ctx,
		`UPDATE persons SET `+col+` = $3, updated_at = now() WHERE id = $1 AND uid = $2`,
		string(personID), string(uid), value)
	if err != nil {
		return store.Person{}, false, fmt.Errorf("set notify field: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.Person{}, false, nil
	}
	pr, err := p.one(ctx, uid, personID)
	if err != nil {
		return store.Person{}, false, fmt.Errorf("reloading person: %w", err)
	}
	return pr, true, nil
}
