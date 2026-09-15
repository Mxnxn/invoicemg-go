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
