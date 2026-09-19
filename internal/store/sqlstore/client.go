package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type clients struct{ pool *pgxpool.Pool }

// clientNotifyColumn maps a notify-preference wire field to its column. A whitelist: the result
// is one of two literals, so building the query around it cannot inject SQL.
func clientNotifyColumn(field string) (string, bool) {
	switch field {
	case "notifyOnCreate":
		return "notify_on_create", true
	case "notifyOnUpdate":
		return "notify_on_update", true
	default:
		return "", false
	}
}

func (s *Store) Clients() store.Clients { return &clients{pool: s.pool} }

func (c *clients) Visible(ctx context.Context, uid, companyID store.ID) ([]store.Client, error) {
	// visibleScope in SQL: own company, a legacy null company, or shared with this company via
	// the array. Bounded by uid so sharing never crosses admins (#1). id is the tiebreak (#19).
	rows, err := c.pool.Query(ctx, `
		SELECT id, uid, company_id, client_name, client_firm, client_phone, client_gst,
		       client_address, opening_balance, shared_company_ids
		  FROM clients
		 WHERE uid = $1
		   AND (company_id = $2 OR company_id IS NULL OR $2 = ANY (shared_company_ids))
		 ORDER BY id ASC`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()

	out := make([]store.Client, 0)
	for rows.Next() {
		var cl store.Client
		var companyIDCol *string
		var shared []string
		if err := rows.Scan(&cl.ID, &cl.UID, &companyIDCol, &cl.ClientName, &cl.ClientFirm,
			&cl.ClientPhone, &cl.ClientGST, &cl.ClientAddress, &cl.OpeningBalance, &shared); err != nil {
			return nil, fmt.Errorf("reading clients: %w", err)
		}
		if companyIDCol != nil {
			cl.CompanyID = store.ID(*companyIDCol)
		}
		// The column is NOT NULL, so the array is always present here - every Postgres client
		// has a sharing list, unlike a legacy Mongo record. Convert to []store.ID (possibly
		// empty), and set the pointer so the row sends {companies:[...]} like a stamped record.
		ids := make([]store.ID, 0, len(shared))
		for _, id := range shared {
			ids = append(ids, store.ID(id))
		}
		cl.Sharing = &ids
		out = append(out, cl)
	}
	return out, rows.Err()
}

func (c *clients) Create(ctx context.Context, companyID, uid store.ID, legacyID int64, in store.ClientWrite) (store.Client, store.Dup, error) {
	if dup, err := c.dupCheck(ctx, companyID, "", in.ClientGST, in.ClientPhone); err != nil || dup != store.DupNone {
		return store.Client{}, dup, err
	}
	var id string
	err := c.pool.QueryRow(ctx, `
		INSERT INTO clients (company_id, uid, client_id, client_name, client_firm, client_phone, client_gst, client_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		string(companyID), string(uid), legacyID, in.ClientName, in.ClientFirm, in.ClientPhone, in.ClientGST, in.ClientAddress).Scan(&id)
	if err != nil {
		return store.Client{}, store.DupNone, fmt.Errorf("insert client: %w", err)
	}
	lid := legacyID
	return store.Client{
		ID: store.ID(id), UID: uid, CompanyID: companyID, LegacyID: &lid,
		ClientName: in.ClientName, ClientFirm: in.ClientFirm, ClientPhone: in.ClientPhone,
		ClientGST: in.ClientGST, ClientAddress: in.ClientAddress,
	}, store.DupNone, nil
}

func (c *clients) Update(ctx context.Context, companyID, clientID store.ID, in store.ClientWrite) (store.Client, store.Dup, bool, error) {
	if dup, err := c.dupCheck(ctx, companyID, clientID, in.ClientGST, in.ClientPhone); err != nil || dup != store.DupNone {
		return store.Client{}, dup, false, err
	}
	var out store.Client
	var legacy *int64
	var companyCol *string
	err := c.pool.QueryRow(ctx, `
		UPDATE clients SET client_name=$3, client_firm=$4, client_phone=$5, client_gst=$6, client_address=$7, updated_at=now()
		 WHERE id=$1 AND company_id=$2
		 RETURNING id, uid, company_id, client_id, client_name, client_firm, client_phone, client_gst, client_address`,
		string(clientID), string(companyID), in.ClientName, in.ClientFirm, in.ClientPhone, in.ClientGST, in.ClientAddress).
		Scan(&out.ID, &out.UID, &companyCol, &legacy, &out.ClientName, &out.ClientFirm, &out.ClientPhone, &out.ClientGST, &out.ClientAddress)
	if noRows(err) {
		return store.Client{}, store.DupNone, false, nil
	}
	if err != nil {
		return store.Client{}, store.DupNone, false, fmt.Errorf("update client: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	out.LegacyID = legacy
	return out, store.DupNone, true, nil
}

func (c *clients) Delete(ctx context.Context, companyID, clientID store.ID) (bool, error) {
	tag, err := c.pool.Exec(ctx, `DELETE FROM clients WHERE id=$1 AND company_id=$2`, string(clientID), string(companyID))
	if err != nil {
		return false, fmt.Errorf("delete client: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// dupCheck reports a per-company GST or phone collision (GST first, matching Node). excludeID,
// when set, is the row being updated - so a client keeping its own GST/phone is not a collision.
func (c *clients) dupCheck(ctx context.Context, companyID, excludeID store.ID, gst, phone string) (store.Dup, error) {
	var n int
	if err := c.pool.QueryRow(ctx, `SELECT count(*) FROM clients WHERE company_id=$1 AND client_gst=$2 AND id <> $3`,
		string(companyID), gst, string(excludeID)).Scan(&n); err != nil {
		return store.DupNone, fmt.Errorf("gst dup check: %w", err)
	}
	if n > 0 {
		return store.DupGST, nil
	}
	if err := c.pool.QueryRow(ctx, `SELECT count(*) FROM clients WHERE company_id=$1 AND client_phone=$2 AND id <> $3`,
		string(companyID), phone, string(excludeID)).Scan(&n); err != nil {
		return store.DupNone, fmt.Errorf("phone dup check: %w", err)
	}
	if n > 0 {
		return store.DupPhone, nil
	}
	return store.DupNone, nil
}

func (c *clients) EnsureSupplier(ctx context.Context, uid store.ID, in store.ClientWrite) (bool, error) {
	// Match an existing supplier on GST, or on phone when no GST - the same key Node uses.
	var match string
	var arg string
	if in.ClientGST != "" {
		match, arg = "gst", in.ClientGST
	} else {
		match, arg = "phone", in.ClientPhone
	}
	var n int
	q := fmt.Sprintf(`SELECT count(*) FROM persons WHERE uid=$1 AND type='Supplier' AND %s=$2`, match)
	if err := c.pool.QueryRow(ctx, q, string(uid), arg).Scan(&n); err != nil {
		return false, fmt.Errorf("supplier match: %w", err)
	}
	if n > 0 {
		return false, nil
	}
	if _, err := c.pool.Exec(ctx, `
		INSERT INTO persons (uid, type, name, firm, phone, gst, address)
		VALUES ($1, 'Supplier', $2, $3, $4, $5, $6)`,
		string(uid), in.ClientName, in.ClientFirm, in.ClientPhone, in.ClientGST, in.ClientAddress); err != nil {
		return false, fmt.Errorf("insert supplier: %w", err)
	}
	return true, nil
}

func (c *clients) SetNotifyPreference(ctx context.Context, companyID, clientID store.ID, field string, value *bool) (*bool, *bool, bool, error) {
	col, ok := clientNotifyColumn(field)
	if !ok {
		return nil, nil, false, fmt.Errorf("unknown notify field %q", field)
	}
	var onCreate, onUpdate *bool
	// value is *bool: nil -> NULL ("ask each time"), &true/&false -> the boolean.
	err := c.pool.QueryRow(ctx,
		`UPDATE clients SET `+col+` = $3, updated_at = now()
		  WHERE id = $1 AND company_id = $2
	  RETURNING notify_on_create, notify_on_update`,
		string(clientID), string(companyID), value).Scan(&onCreate, &onUpdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("set client notify: %w", err)
	}
	return onCreate, onUpdate, true, nil
}

func (c *clients) NotifyPreferences(ctx context.Context, companyID store.ID, answeredOnly bool) ([]store.ClientNotify, error) {
	q := `SELECT id, client_name, client_firm, client_phone, notify_on_create, notify_on_update
	        FROM clients WHERE company_id = $1`
	if answeredOnly {
		// A boolean column is true/false/null, so IS NOT NULL is Node's $in:[true,false].
		q += ` AND (notify_on_create IS NOT NULL OR notify_on_update IS NOT NULL)`
	}
	q += ` ORDER BY client_firm ASC, id ASC` // id tiebreak (#19); Node sorts clientFirm only
	rows, err := c.pool.Query(ctx, q, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing notify preferences: %w", err)
	}
	defer rows.Close()
	out := make([]store.ClientNotify, 0)
	for rows.Next() {
		var cn store.ClientNotify
		if err := rows.Scan(&cn.ID, &cn.ClientName, &cn.ClientFirm, &cn.ClientPhone,
			&cn.NotifyOnCreate, &cn.NotifyOnUpdate); err != nil {
			return nil, fmt.Errorf("reading notify preferences: %w", err)
		}
		out = append(out, cn)
	}
	return out, rows.Err()
}

func (c *clients) SharedList(ctx context.Context, companyIDs []store.ID) ([]store.SharedClient, error) {
	ids := make([]string, 0, len(companyIDs))
	for _, id := range companyIDs {
		ids = append(ids, string(id))
	}
	rows, err := c.pool.Query(ctx, `
		SELECT id, client_name, client_firm, client_phone, client_gst, company_id
		  FROM clients
		 WHERE company_id = ANY ($1)
		 ORDER BY client_firm ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("shared customers: %w", err)
	}
	defer rows.Close()
	out := make([]store.SharedClient, 0)
	for rows.Next() {
		var sc store.SharedClient
		var companyIDCol *string
		if err := rows.Scan(&sc.ID, &sc.ClientName, &sc.ClientFirm, &sc.ClientPhone,
			&sc.ClientGST, &companyIDCol); err != nil {
			return nil, fmt.Errorf("reading shared customers: %w", err)
		}
		if companyIDCol != nil {
			sc.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
