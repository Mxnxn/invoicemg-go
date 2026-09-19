package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type wastages struct{ pool *pgxpool.Pool }

func (s *Store) Wastages() store.Wastages { return &wastages{pool: s.pool} }

func (w *wastages) List(ctx context.Context, companyID store.ID) ([]store.Wastage, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT id, uid, company_id, material_name, rate, purchase_rate, cost_total, length, height, total, date, created_at, updated_at
		  FROM wastages WHERE company_id = $1 ORDER BY created_at DESC, id DESC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing wastages: %w", err)
	}
	defer rows.Close()
	out := make([]store.Wastage, 0)
	for rows.Next() {
		var wa store.Wastage
		var companyIDCol *string
		if err := rows.Scan(&wa.ID, &wa.UID, &companyIDCol, &wa.MaterialName, &wa.Rate, &wa.PurchaseRate,
			&wa.CostTotal, &wa.Length, &wa.Height, &wa.Total, &wa.Date, &wa.CreatedAt, &wa.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading wastages: %w", err)
		}
		if companyIDCol != nil {
			wa.CompanyID = store.ID(*companyIDCol)
		}
		out = append(out, wa)
	}
	return out, rows.Err()
}

func (w *wastages) Create(ctx context.Context, companyID, uid store.ID, in store.WastageWrite) (store.Wastage, error) {
	var out store.Wastage
	var companyCol *string
	err := w.pool.QueryRow(ctx, `
		INSERT INTO wastages (uid, company_id, material_name, rate, purchase_rate, cost_total, length, height, total, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, uid, company_id, material_name, rate, purchase_rate, cost_total, length, height, total, date, created_at, updated_at`,
		string(uid), string(companyID), in.MaterialName, in.Rate, in.PurchaseRate, in.CostTotal, in.Length, in.Height, in.Total, in.Date).
		Scan(&out.ID, &out.UID, &companyCol, &out.MaterialName, &out.Rate, &out.PurchaseRate, &out.CostTotal,
			&out.Length, &out.Height, &out.Total, &out.Date, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return store.Wastage{}, fmt.Errorf("insert wastage: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	return out, nil
}

func (w *wastages) MaterialsSummary(ctx context.Context, companyID store.ID) ([]store.MaterialAvg, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT min(material_name) AS name, avg(material_rate), avg(purchase_rate)
		  FROM materials WHERE company_id = $1
		 GROUP BY lower(material_name)
		 ORDER BY lower(material_name) ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("wastage materials summary: %w", err)
	}
	defer rows.Close()
	out := make([]store.MaterialAvg, 0)
	for rows.Next() {
		var m store.MaterialAvg
		if err := rows.Scan(&m.MaterialName, &m.MaterialRate, &m.PurchaseRate); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (w *wastages) Delete(ctx context.Context, companyID, wastageID store.ID) (bool, error) {
	tag, err := w.pool.Exec(ctx, `DELETE FROM wastages WHERE id = $1 AND company_id = $2`,
		string(wastageID), string(companyID))
	if err != nil {
		return false, fmt.Errorf("delete wastage: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
