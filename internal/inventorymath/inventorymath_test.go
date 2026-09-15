package inventorymath

import "testing"

func boolPtr(b bool) *bool { return &b }

// Values captured from the real Helpers/InventoryMath.buildInventory.
func TestBuild_MatchesNode(t *testing.T) {
	rows, notCounted := Build(Input{
		Materials: []Material{
			{ID: "m1", MaterialName: "Vinyl Sticker", Unit: "SQ. Ft", PurchaseRate: 30},
			{ID: "m2", MaterialName: "Design Charge", Unit: "Qty"},
			{ID: "m3", MaterialName: "NoUnit", Unit: ""},
		},
		PurchaseRows: []PurchaseRow{{ID: "p1", Material: "Vinyl Sticker", Qty: 100, Rate: 30, CompanyID: "co1"}},
		JobRows: []JobRow{
			{ID: "j1", Material: "Vinyl Sticker", Length: "2", Width: "3", Qty: 5, HasDimensions: boolPtr(true), CompanyID: "co1"},
			{ID: "j2", Material: "Design Charge", Qty: 2, CompanyID: "co1"},
		},
		WastageRows: []WastageRow{{ID: "w1", MaterialName: "Vinyl Sticker", Length: 1, Height: 2, CompanyID: "co1"}},
		Labels:      map[string]string{"co1": "Manan Graphics"},
	})

	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// sorted: Design Charge, Vinyl Sticker
	dc, vinyl := rows[0], rows[1]
	if dc.Material != "Design Charge" || dc.Consumed != 2 || dc.InStock != -2 || !dc.Negative {
		t.Errorf("design charge wrong: %+v", dc)
	}
	if vinyl.Material != "Vinyl Sticker" || vinyl.Purchased != 100 || vinyl.Consumed != 30 || vinyl.Wasted != 2 || vinyl.InStock != 68 || vinyl.PurchaseValue != 3000 {
		t.Errorf("vinyl wrong: %+v", vinyl)
	}
	if len(vinyl.Companies) != 1 || vinyl.Companies[0] != "Manan Graphics" {
		t.Errorf("companies wrong: %v", vinyl.Companies)
	}
	if len(notCounted) != 1 || notCounted[0].Ref != "m3" || notCounted[0].Reason != "Product has no unit" {
		t.Errorf("notCounted wrong: %+v", notCounted)
	}
}
