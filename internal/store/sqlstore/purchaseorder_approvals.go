package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (p *purchaseOrders) PendingApprovals(ctx context.Context, uid, companyID, actorID store.ID) ([]store.PurchaseOrder, []store.PurchaseOrder, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+poSelectCols+`
		  FROM purchase_orders po LEFT JOIN persons s ON s.id = po.supplier_id
		 WHERE po.uid=$1 AND po.company_id=$2 AND po.approval_state='draft' AND po.purchase_invoice_id IS NULL
		 ORDER BY po.created_at DESC`, string(uid), string(companyID))
	if err != nil {
		return nil, nil, fmt.Errorf("pending approvals: %w", err)
	}
	candidates := []store.PurchaseOrder{}
	ids := []string{}
	for rows.Next() {
		po, err := scanPO(rows)
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		candidates = append(candidates, po)
		ids = append(ids, string(po.ID))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// isAwaitingApproval's rows clause: only POs that actually have rows.
	pending := []store.PurchaseOrder{}
	if len(ids) > 0 {
		rowsByPO, err := p.rowsFor(ctx, ids)
		if err != nil {
			return nil, nil, err
		}
		for i := range candidates {
			candidates[i].Rows = rowsByPO[candidates[i].ID]
			if len(candidates[i].Rows) > 0 {
				pending = append(pending, candidates[i])
			}
		}
	}

	// Dismissals for this actor over the pending set.
	dismissed := map[store.ID]time.Time{}
	if len(pending) > 0 {
		pids := make([]string, len(pending))
		for i, po := range pending {
			pids[i] = string(po.ID)
		}
		dr, err := p.pool.Query(ctx, `SELECT purchase_order_id, po_updated_at FROM po_approval_dismissals WHERE actor_id=$1 AND purchase_order_id = ANY($2)`,
			string(actorID), pids)
		if err != nil {
			return nil, nil, fmt.Errorf("pending dismissals: %w", err)
		}
		for dr.Next() {
			var id string
			var at time.Time
			if err := dr.Scan(&id, &at); err != nil {
				dr.Close()
				return nil, nil, err
			}
			dismissed[store.ID(id)] = at
		}
		dr.Close()
		if err := dr.Err(); err != nil {
			return nil, nil, err
		}
	}

	visible := []store.PurchaseOrder{}
	for _, po := range pending {
		at, ok := dismissed[po.ID]
		if !ok || po.UpdatedAt.After(at) {
			visible = append(visible, po)
		}
	}
	return pending, visible, nil
}

func (p *purchaseOrders) DismissApprovals(ctx context.Context, uid, companyID, actorID store.ID, actorType string, all bool, poID store.ID) (int, int, int, error) {
	pending, visible, err := p.PendingApprovals(ctx, uid, companyID, actorID)
	if err != nil {
		return 0, 0, 0, err
	}

	// Prune dismissals for orders no longer pending (approved/invoiced since).
	pendingIDs := make([]string, len(pending))
	for i, po := range pending {
		pendingIDs[i] = string(po.ID)
	}
	if _, err := p.pool.Exec(ctx, `DELETE FROM po_approval_dismissals WHERE actor_id=$1 AND NOT (purchase_order_id = ANY($2))`,
		string(actorID), pendingIDs); err != nil {
		return 0, 0, 0, fmt.Errorf("prune dismissals: %w", err)
	}

	targets := []store.PurchaseOrder{}
	for _, po := range visible {
		if all || po.ID == poID {
			targets = append(targets, po)
		}
	}
	if len(targets) == 0 {
		return 0, len(visible), len(pending), nil
	}

	for _, po := range targets {
		if _, err := p.pool.Exec(ctx, `
			INSERT INTO po_approval_dismissals (actor_id, purchase_order_id, po_updated_at)
			VALUES ($1,$2,$3)
			ON CONFLICT (actor_id, purchase_order_id) DO UPDATE SET po_updated_at=EXCLUDED.po_updated_at`,
			string(actorID), string(po.ID), po.UpdatedAt); err != nil {
			return 0, 0, 0, fmt.Errorf("upsert dismissal: %w", err)
		}
	}

	freshPending, freshVisible, err := p.PendingApprovals(ctx, uid, companyID, actorID)
	if err != nil {
		return 0, 0, 0, err
	}
	return len(targets), len(freshVisible), len(freshPending), nil
}
