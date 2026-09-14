package sqlstore

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type units struct{ pool *pgxpool.Pool }

var defaultUnits = []string{"SQ. Ft", "SQ. In", "Qty", "Piece", "mm", "in", "cm", "Feet"}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// normaliseKey is Helpers/InventoryMath.js's rule, duplicated from mongostore rather than
// shared, because it is the key a UNIQUE INDEX is built on in each backend. If it lived in one
// place and someone changed it, both indexes would silently start disagreeing with the data
// already in them. Two copies, each beside the index it feeds, is the safer shape - and the
// tests pin both to the same captured JavaScript output.
func normaliseKey(name string) string {
	return strings.TrimSpace(nonAlphanumeric.ReplaceAllString(strings.ToLower(name), " "))
}

const unitColumns = `id, uid, company_id, name, key, created_at, updated_at`

func scanUnit(row interface {
	Scan(dest ...any) error
}) (store.Unit, error) {
	var u store.Unit
	var uid, companyID *string
	if err := row.Scan(&u.ID, &uid, &companyID, &u.Name, &u.Key, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return store.Unit{}, err
	}
	if uid != nil {
		u.UID = store.ID(*uid)
	}
	if companyID != nil {
		u.CompanyID = store.ID(*companyID)
	}
	// Version is Mongoose's __v. Postgres has no such column and no such concept, so it is
	// reported as 0 - which is what every document Mongo has ever written here also carries,
	// since nothing in this application uses optimistic concurrency.
	return u, nil
}

func (u *units) List(ctx context.Context, companyID store.ID) ([]store.Unit, error) {
	rows, err := u.pool.Query(ctx,
		`SELECT `+unitColumns+` FROM units WHERE company_id = $1 ORDER BY name ASC, id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing units: %w", err)
	}
	defer rows.Close()

	out := []store.Unit{}
	for rows.Next() {
		unit, err := scanUnit(rows)
		if err != nil {
			return nil, fmt.Errorf("reading units: %w", err)
		}
		out = append(out, unit)
	}
	return out, rows.Err()
}

func (u *units) Create(ctx context.Context, uid, companyID store.ID, name string) (store.Unit, error) {
	name = strings.TrimSpace(name)
	// key is set HERE, in the store, exactly as the Mongoose pre("validate") hook sets it
	// there - not in the handler, because the next handler written is the one that forgets
	// and creates the duplicate the index exists to prevent.
	row := u.pool.QueryRow(ctx, `
		INSERT INTO units (uid, company_id, name, key)
		VALUES ($1, $2, $3, $4)
		RETURNING `+unitColumns, string(uid), string(companyID), name, normaliseKey(name))

	unit, err := scanUnit(row)
	if err != nil {
		if isUniqueViolation(err) {
			return store.Unit{}, store.ErrDuplicate
		}
		return store.Unit{}, fmt.Errorf("creating unit: %w", err)
	}
	return unit, nil
}

func (u *units) Rename(ctx context.Context, unitID, companyID store.ID, name string) (store.Unit, error) {
	name = strings.TrimSpace(name)
	row := u.pool.QueryRow(ctx, `
		UPDATE units
		   SET name = $3, key = $4, updated_at = now()
		 WHERE id = $1 AND company_id = $2
		RETURNING `+unitColumns, string(unitID), string(companyID), name, normaliseKey(name))

	unit, err := scanUnit(row)
	if noRows(err) {
		return store.Unit{}, store.ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return store.Unit{}, store.ErrDuplicate
		}
		return store.Unit{}, fmt.Errorf("renaming unit: %w", err)
	}
	return unit, nil
}

func (u *units) Delete(ctx context.Context, unitID, companyID store.ID) error {
	tag, err := u.pool.Exec(ctx, `DELETE FROM units WHERE id = $1 AND company_id = $2`,
		string(unitID), string(companyID))
	if err != nil {
		return fmt.Errorf("deleting unit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (u *units) SeedDefaults(ctx context.Context, uid, companyID store.ID) error {
	// ON CONFLICT DO NOTHING makes this idempotent in ONE statement, and safe against two
	// requests seeding at once - the loser's duplicates are refused by the index and the
	// company still ends up with exactly one of each. The Mongo version has to read the
	// existing units first and then insert the difference, which is the same thing with a
	// race in the middle.
	batch := make([]string, 0, len(defaultUnits))
	args := []any{string(uid), string(companyID)}
	for i, name := range defaultUnits {
		batch = append(batch, fmt.Sprintf("($1, $2, $%d, $%d)", len(args)+1, len(args)+2))
		args = append(args, name, normaliseKey(name))
		_ = i
	}

	_, err := u.pool.Exec(ctx,
		`INSERT INTO units (uid, company_id, name, key) VALUES `+strings.Join(batch, ", ")+
			` ON CONFLICT (company_id, key) DO NOTHING`, args...)
	if err != nil {
		return fmt.Errorf("seeding units: %w", err)
	}
	return nil
}
