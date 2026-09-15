package sqlstore

import (
	"context"
	"fmt"

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
