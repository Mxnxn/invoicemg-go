package shared

import (
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func TestDuplicateMaterials_GroupsAndFlags(t *testing.T) {
	co := &stubCompanies{list: []store.Company{
		{ID: "c1", Name: "Alpha"},
		{ID: "c2", Firm: "Beta LLP"}, // no name -> label falls back to firm
	}}
	m := &stubMaterials{dups: []store.MaterialDuplicate{
		// "Art Card" / "art-card" both normalise to "art card" (punctuation -> space), so they are
		// the same product typed twice - two rates -> a price mismatch across two companies.
		{ID: "m1", MaterialName: "Art Card", MaterialRate: 10, CompanyID: "c1", SharedWith: 2},
		{ID: "m2", MaterialName: "art-card", MaterialRate: 12, CompanyID: "c2", SharedWith: 1},
		// Glue exists in only one company -> not a duplicate, dropped.
		{ID: "m3", MaterialName: "Glue", MaterialRate: 3, CompanyID: "c1", SharedWith: 1},
	}}
	h := New(co, &stubClients{}, m)
	rec := httptest.NewRecorder()
	h.DuplicateMaterials(rec, req(store.Session{UID: "u1", CompanyID: "c1", Role: "admin"}))

	if co.gotList != "u1" {
		t.Errorf("List called for %q, want the owner u1 (full set, not the toggle scope)", co.gotList)
	}
	data := decodeData(t, rec)
	rows := data["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (only the duplicated product)", len(rows))
	}
	row := rows[0].(map[string]any)
	// Heading is the longest spelling; on a length tie the first (by company name) wins.
	if row["name"] != "Art Card" {
		t.Errorf("name = %v, want Art Card", row["name"])
	}
	if row["priceMismatch"] != true {
		t.Errorf("priceMismatch = %v, want true (10 vs 12)", row["priceMismatch"])
	}
	if row["lowestRate"] != float64(10) || row["highestRate"] != float64(12) {
		t.Errorf("rate span = %v..%v, want 10..12", row["lowestRate"], row["highestRate"])
	}
	copies := row["copies"].([]any)
	if len(copies) != 2 {
		t.Fatalf("copies = %d, want 2", len(copies))
	}
	// Copies sort by owning company name: Alpha before Beta LLP.
	first := copies[0].(map[string]any)
	if first["companyName"] != "Alpha" || first["sharedWith"] != float64(2) {
		t.Errorf("copy0 = %v, want Alpha with sharedWith 2", first)
	}
	companies := row["companies"].([]any)
	if len(companies) != 2 || companies[0] != "Alpha" || companies[1] != "Beta LLP" {
		t.Errorf("companies = %v, want [Alpha, Beta LLP]", companies)
	}
}

func TestDuplicateMaterials_EmptyIsArray(t *testing.T) {
	co := &stubCompanies{list: []store.Company{{ID: "c1", Name: "Alpha"}}}
	h := New(co, &stubClients{}, &stubMaterials{dups: []store.MaterialDuplicate{
		{ID: "m1", MaterialName: "Solo", MaterialRate: 1, CompanyID: "c1"},
	}})
	rec := httptest.NewRecorder()
	h.DuplicateMaterials(rec, req(store.Session{UID: "u1", CompanyID: "c1"}))
	if got := rec.Body.String(); !containsRowsArray(got) {
		t.Errorf(`expected "rows":[] when nothing duplicates, got %s`, got)
	}
}
