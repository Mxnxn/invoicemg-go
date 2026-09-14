package auth

import (
	"reflect"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func sessionWith(role string, perms ...string) store.Session {
	return store.Session{Role: role, Permissions: perms}
}

func TestAdminsBypassEverything(t *testing.T) {
	for _, role := range []string{"admin", "superadmin"} {
		s := sessionWith(role)
		for _, action := range Actions {
			if !HasPermission(s, "invoices", action) {
				t.Errorf("%s was refused invoices:%s", role, action)
			}
		}
	}
}

// superadmin is the one that has been wrong before: sessions used to be stamped "admin" for
// everyone, so once the role became real a superadmin matched neither branch and was refused
// every gated route.
func TestUnknownRoleGetsNothing(t *testing.T) {
	if HasPermission(sessionWith("viewer", "invoices:view"), "invoices", "view") {
		t.Fatal("a role that is neither admin nor employee was granted access")
	}
}

// The compatibility rule that matters most. Every employee predating the action split has
// flat keys on their Person record and in their live session snapshot. Reading a flat key as
// view-only would revoke write access from all of them the moment this service answered.
func TestFlatKeyGrantsEveryAction(t *testing.T) {
	s := sessionWith("employee", "invoices")
	for _, action := range Actions {
		if !HasPermission(s, "invoices", action) {
			t.Errorf("flat key did not grant invoices:%s", action)
		}
	}
	if HasPermission(s, "quotations", ActionView) {
		t.Error("a flat key for one feature leaked into another")
	}
}

func TestScopedKeyGrantsOnlyThatAction(t *testing.T) {
	s := sessionWith("employee", "invoices:view")
	if !HasPermission(s, "invoices", ActionView) {
		t.Error("invoices:view did not grant view")
	}
	if HasPermission(s, "invoices", ActionCreate) {
		t.Error("invoices:view granted create")
	}
	if HasPermission(s, "invoices", ActionDelete) {
		t.Error("invoices:view granted delete")
	}
}

// An empty action defaults to view, matching the Node signature's default parameter.
func TestEmptyActionMeansView(t *testing.T) {
	s := sessionWith("employee", "invoices:view")
	if !HasPermission(s, "invoices", "") {
		t.Error("an empty action was not treated as view")
	}
}

func TestEmployeeWithNoPermissionsGetsNothing(t *testing.T) {
	if HasPermission(sessionWith("employee"), "invoices", ActionView) {
		t.Fatal("an employee with no permissions was granted access")
	}
}

// A trailing colon is a flat key with an empty action - "invoices:" must not be read as
// granting an action literally named "".
func TestTrailingColonIsTreatedAsFlat(t *testing.T) {
	s := sessionWith("employee", "invoices:")
	for _, action := range Actions {
		if !HasPermission(s, "invoices", action) {
			t.Errorf("invoices: did not grant %s", action)
		}
	}
}

func TestNormaliseExpandsFlatKeys(t *testing.T) {
	got := NormalisePermissions([]string{"invoices"})
	want := []string{"invoices:view", "invoices:create", "invoices:delete"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Unknown actions are dropped rather than stored, so a typo cannot become a permanent grant.
func TestNormaliseDropsUnknownActions(t *testing.T) {
	got := NormalisePermissions([]string{"invoices:approve", "invoices:view", ":orphan", ""})
	want := []string{"invoices:view"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormaliseDeduplicates(t *testing.T) {
	got := NormalisePermissions([]string{"invoices:view", "invoices:view", "invoices"})
	want := []string{"invoices:view", "invoices:create", "invoices:delete"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormaliseOfNothingIsEmptyNotNil(t *testing.T) {
	got := NormalisePermissions(nil)
	if got == nil {
		t.Fatal("nil would marshal as JSON null; Node sends []")
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}
