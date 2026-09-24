package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type materials struct{ pool *pgxpool.Pool }

func (s *Store) Materials() store.Materials { return &materials{pool: s.pool} }

func (m *materials) Visible(ctx context.Context, companyID store.ID) ([]store.Material, error) {
	// visibleScope: own company or shared with it (#1). id is the tiebreak (#19).
	rows, err := m.pool.Query(ctx, `
		SELECT id, uid, company_id, material_name, material_rate, purchase_rate, unit, hsn, tax,
		       shared_company_ids, created_at, updated_at
		  FROM materials
		 WHERE company_id = $1 OR $1 = ANY (shared_company_ids)
		 ORDER BY id ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing materials: %w", err)
	}
	defer rows.Close()

	out := make([]store.Material, 0)
	ids := make([]string, 0)
	byID := map[string]int{}
	for rows.Next() {
		var mt store.Material
		var companyIDCol *string
		var shared []string
		if err := rows.Scan(&mt.ID, &mt.UID, &companyIDCol, &mt.MaterialName, &mt.MaterialRate,
			&mt.PurchaseRate, &mt.Unit, &mt.Hsn, &mt.Tax, &shared, &mt.CreatedAt, &mt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading materials: %w", err)
		}
		if companyIDCol != nil {
			mt.CompanyID = store.ID(*companyIDCol)
		}
		sids := make([]store.ID, 0, len(shared))
		for _, id := range shared {
			sids = append(sids, store.ID(id))
		}
		mt.Sharing = &sids
		mt.PriceHistory = make([]store.PriceHistoryEntry, 0)
		byID[string(mt.ID)] = len(out)
		ids = append(ids, string(mt.ID))
		out = append(out, mt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	// One batched lookup for every product's price history (#6: never a query per row). Ordered
	// oldest-first within each product to match the append order Mongo's array preserves.
	hRows, err := m.pool.Query(ctx, `
		SELECT material_id, material_rate, purchase_rate, changed_at
		  FROM material_price_history
		 WHERE material_id = ANY ($1)
		 ORDER BY changed_at ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("listing price history: %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var mid string
		var e store.PriceHistoryEntry
		if err := hRows.Scan(&mid, &e.MaterialRate, &e.PurchaseRate, &e.ChangedAt); err != nil {
			return nil, fmt.Errorf("reading price history: %w", err)
		}
		if i, ok := byID[mid]; ok {
			out[i].PriceHistory = append(out[i].PriceHistory, e)
		}
	}
	return out, hRows.Err()
}

func (m *materials) Create(ctx context.Context, companyID, uid store.ID, in store.MaterialWrite) (store.Material, error) {
	var out store.Material
	var companyCol *string
	err := m.pool.QueryRow(ctx, `
		INSERT INTO materials (uid, company_id, material_name, material_rate, purchase_rate, hsn, tax)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, uid, company_id, material_name, material_rate, purchase_rate, unit, hsn, tax, created_at, updated_at`,
		string(uid), string(companyID), in.MaterialName, in.MaterialRate, in.PurchaseRate, in.Hsn, in.Tax).
		Scan(&out.ID, &out.UID, &companyCol, &out.MaterialName, &out.MaterialRate, &out.PurchaseRate,
			&out.Unit, &out.Hsn, &out.Tax, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return store.Material{}, fmt.Errorf("insert material: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	out.PriceHistory = make([]store.PriceHistoryEntry, 0)
	return out, nil
}

func (m *materials) Update(ctx context.Context, companyID, materialID store.ID, in store.MaterialWrite) (store.Material, bool, error) {
	// Read the current rates first, to decide whether a history row is needed and with what old
	// values. Scoped to the company so a shared-in product can't be edited through this path.
	var oldMat, oldPur float64
	err := m.pool.QueryRow(ctx, `SELECT material_rate, purchase_rate FROM materials WHERE id=$1 AND company_id=$2`,
		string(materialID), string(companyID)).Scan(&oldMat, &oldPur)
	if noRows(err) {
		return store.Material{}, false, nil
	}
	if err != nil {
		return store.Material{}, false, fmt.Errorf("read material: %w", err)
	}
	priceChanged := oldMat != in.MaterialRate || oldPur != in.PurchaseRate

	if priceChanged {
		if _, err := m.pool.Exec(ctx, `
			INSERT INTO material_price_history (material_id, material_rate, purchase_rate)
			VALUES ($1, $2, $3)`, string(materialID), oldMat, oldPur); err != nil {
			return store.Material{}, false, fmt.Errorf("push price history: %w", err)
		}
	}

	if _, err := m.pool.Exec(ctx, `
		UPDATE materials SET material_name=$3, material_rate=$4, purchase_rate=$5, hsn=$6, tax=$7, updated_at=now()
		 WHERE id=$1 AND company_id=$2`,
		string(materialID), string(companyID), in.MaterialName, in.MaterialRate, in.PurchaseRate, in.Hsn, in.Tax); err != nil {
		return store.Material{}, false, fmt.Errorf("update material: %w", err)
	}

	out, err := m.one(ctx, companyID, materialID)
	if err != nil {
		return store.Material{}, false, err
	}
	return out, true, nil
}

func (m *materials) Delete(ctx context.Context, companyID, materialID store.ID) error {
	if _, err := m.pool.Exec(ctx, `DELETE FROM materials WHERE id=$1 AND company_id=$2`, string(materialID), string(companyID)); err != nil {
		return fmt.Errorf("delete material: %w", err)
	}
	return nil
}

// one reads a single company-scoped product with its price history (oldest-first).
func (m *materials) one(ctx context.Context, companyID, materialID store.ID) (store.Material, error) {
	var out store.Material
	var companyCol *string
	err := m.pool.QueryRow(ctx, `
		SELECT id, uid, company_id, material_name, material_rate, purchase_rate, unit, hsn, tax, created_at, updated_at
		  FROM materials WHERE id=$1 AND company_id=$2`, string(materialID), string(companyID)).
		Scan(&out.ID, &out.UID, &companyCol, &out.MaterialName, &out.MaterialRate, &out.PurchaseRate,
			&out.Unit, &out.Hsn, &out.Tax, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return store.Material{}, fmt.Errorf("read material: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	out.PriceHistory = make([]store.PriceHistoryEntry, 0)
	hRows, err := m.pool.Query(ctx, `
		SELECT material_rate, purchase_rate, changed_at FROM material_price_history
		 WHERE material_id=$1 ORDER BY changed_at ASC, id ASC`, string(materialID))
	if err != nil {
		return store.Material{}, fmt.Errorf("read price history: %w", err)
	}
	defer hRows.Close()
	for hRows.Next() {
		var e store.PriceHistoryEntry
		if err := hRows.Scan(&e.MaterialRate, &e.PurchaseRate, &e.ChangedAt); err != nil {
			return store.Material{}, err
		}
		out.PriceHistory = append(out.PriceHistory, e)
	}
	return out, hRows.Err()
}

func (m *materials) Get(ctx context.Context, companyID, materialID store.ID) (store.Material, bool, error) {
	out, err := m.one(ctx, companyID, materialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Material{}, false, nil
		}
		return store.Material{}, false, err
	}
	return out, true, nil
}

func (m *materials) SetUnit(ctx context.Context, companyID, materialID store.ID, unit string) (string, string, bool, bool, error) {
	var name, saved string
	err := m.pool.QueryRow(ctx, `
		UPDATE materials SET unit = $3, updated_at = now()
		 WHERE id = $1 AND company_id = $2
	 RETURNING material_name, unit`, string(materialID), string(companyID), unit).Scan(&name, &saved)
	if err == nil {
		return name, saved, true, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, false, fmt.Errorf("set unit: %w", err)
	}
	// Not owned by this company. If the id exists at all it is a product shared IN - the case
	// the handler names distinctly from a plain miss.
	var exists bool
	if err := m.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE id = $1)`,
		string(materialID)).Scan(&exists); err != nil {
		return "", "", false, false, fmt.Errorf("set unit borrow check: %w", err)
	}
	return "", "", false, exists, nil
}

func (m *materials) OwnedSharing(ctx context.Context, companyID, materialID store.ID) ([]store.ID, bool, error) {
	var shared []string
	err := m.pool.QueryRow(ctx,
		`SELECT shared_company_ids FROM materials WHERE id = $1 AND company_id = $2`,
		string(materialID), string(companyID)).Scan(&shared)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("material owned sharing: %w", err)
	}
	out := make([]store.ID, 0, len(shared))
	for _, id := range shared {
		out = append(out, store.ID(id))
	}
	return out, true, nil
}

func (m *materials) SetSharing(ctx context.Context, companyID, materialID store.ID, companies []store.ID) ([]store.ID, bool, error) {
	arr := make([]string, 0, len(companies))
	for _, id := range companies {
		arr = append(arr, string(id))
	}
	var stored []string
	err := m.pool.QueryRow(ctx,
		`UPDATE materials SET shared_company_ids = $1, updated_at = now()
		  WHERE id = $2 AND company_id = $3
	  RETURNING shared_company_ids`,
		arr, string(materialID), string(companyID)).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("material set sharing: %w", err)
	}
	out := make([]store.ID, 0, len(stored))
	for _, id := range stored {
		out = append(out, store.ID(id))
	}
	return out, true, nil
}

func (m *materials) DuplicatesSource(ctx context.Context, companyIDs []store.ID) ([]store.MaterialDuplicate, error) {
	ids := make([]string, 0, len(companyIDs))
	for _, id := range companyIDs {
		ids = append(ids, string(id))
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, material_name, material_rate, purchase_rate, hsn, unit, company_id,
		       coalesce(cardinality(shared_company_ids), 0)
		  FROM materials
		 WHERE company_id = ANY ($1)
		 ORDER BY id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("duplicate materials source: %w", err)
	}
	defer rows.Close()
	out := make([]store.MaterialDuplicate, 0)
	for rows.Next() {
		var d store.MaterialDuplicate
		var companyIDCol *string
		if err := rows.Scan(&d.ID, &d.MaterialName, &d.MaterialRate, &d.PurchaseRate,
			&d.Hsn, &d.Unit, &companyIDCol, &d.SharedWith); err != nil {
			return nil, fmt.Errorf("reading duplicate materials: %w", err)
		}
		if companyIDCol != nil {
			d.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (m *materials) SharedList(ctx context.Context, companyIDs []store.ID) ([]store.SharedMaterial, error) {
	ids := make([]string, 0, len(companyIDs))
	for _, id := range companyIDs {
		ids = append(ids, string(id))
	}
	rows, err := m.pool.Query(ctx, `
		SELECT id, material_name, material_rate, purchase_rate, hsn, unit, company_id
		  FROM materials
		 WHERE company_id = ANY ($1)
		 ORDER BY material_name ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("shared materials: %w", err)
	}
	defer rows.Close()
	out := make([]store.SharedMaterial, 0)
	for rows.Next() {
		var sm store.SharedMaterial
		var companyIDCol *string
		if err := rows.Scan(&sm.ID, &sm.MaterialName, &sm.MaterialRate, &sm.PurchaseRate,
			&sm.Hsn, &sm.Unit, &companyIDCol); err != nil {
			return nil, fmt.Errorf("reading shared materials: %w", err)
		}
		if companyIDCol != nil {
			sm.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}
