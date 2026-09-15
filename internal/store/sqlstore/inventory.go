package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type inventory struct{ pool *pgxpool.Pool }

func (s *Store) Inventory() store.Inventory { return &inventory{pool: s.pool} }

func (i *inventory) Data(ctx context.Context, companyID store.ID, from, to string) (store.InventoryData, error) {
	var out store.InventoryData
	c := string(companyID)

	// Products are never date-filtered (a product bought outside the range still owns its rows).
	mrows, err := i.pool.Query(ctx, `SELECT id, material_name, unit, purchase_rate FROM materials WHERE company_id = $1`, c)
	if err != nil {
		return out, fmt.Errorf("inventory materials: %w", err)
	}
	for mrows.Next() {
		var m store.InvMaterial
		if err := mrows.Scan(&m.ID, &m.MaterialName, &m.Unit, &m.PurchaseRate); err != nil {
			mrows.Close()
			return out, err
		}
		out.Materials = append(out.Materials, m)
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return out, err
	}

	// Purchase rows and job rows are date-filtered on the PARENT's created_at (from/to as
	// YYYY-MM-DD; empty = no bound).
	prows, err := i.pool.Query(ctx, `
		SELECT r.id, r.material, r.qty, r.rate, $1
		  FROM purchase_invoice_rows r JOIN purchase_invoices p ON p.id = r.invoice_id
		 WHERE p.company_id = $1
		   AND ($2 = '' OR p.created_at >= to_timestamp($2,'YYYY-MM-DD'))
		   AND ($3 = '' OR p.created_at < to_timestamp($3,'YYYY-MM-DD') + interval '1 day')`, c, from, to)
	if err != nil {
		return out, fmt.Errorf("inventory purchases: %w", err)
	}
	for prows.Next() {
		var r store.InvPurchaseRow
		if err := prows.Scan(&r.ID, &r.Material, &r.Qty, &r.Rate, &r.CompanyID); err != nil {
			prows.Close()
			return out, err
		}
		out.PurchaseRows = append(out.PurchaseRows, r)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return out, err
	}

	jrows, err := i.pool.Query(ctx, `
		SELECT r.id, r.material, r.length, r.width, r.qty, r.has_dimensions, $1
		  FROM job_rows r JOIN jobs j ON j.id = r.job_id
		 WHERE j.company_id = $1
		   AND ($2 = '' OR j.created_at >= to_timestamp($2,'YYYY-MM-DD'))
		   AND ($3 = '' OR j.created_at < to_timestamp($3,'YYYY-MM-DD') + interval '1 day')`, c, from, to)
	if err != nil {
		return out, fmt.Errorf("inventory jobs: %w", err)
	}
	for jrows.Next() {
		var r store.InvJobRow
		var hasDim bool
		if err := jrows.Scan(&r.ID, &r.Material, &r.Length, &r.Width, &r.Qty, &hasDim, &r.CompanyID); err != nil {
			jrows.Close()
			return out, err
		}
		hd := hasDim
		r.HasDimensions = &hd
		out.JobRows = append(out.JobRows, r)
	}
	jrows.Close()
	if err := jrows.Err(); err != nil {
		return out, err
	}

	// Wastage has no timestamp to range on, so it is always counted in full.
	wrows, err := i.pool.Query(ctx, `SELECT id, material_name, length, height, $1 FROM wastages WHERE company_id = $1`, c)
	if err != nil {
		return out, fmt.Errorf("inventory wastage: %w", err)
	}
	for wrows.Next() {
		var r store.InvWastageRow
		if err := wrows.Scan(&r.ID, &r.MaterialName, &r.Length, &r.Height, &r.CompanyID); err != nil {
			wrows.Close()
			return out, err
		}
		out.WastageRows = append(out.WastageRows, r)
	}
	wrows.Close()
	return out, wrows.Err()
}
