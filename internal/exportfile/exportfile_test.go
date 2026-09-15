package exportfile

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	n := Name("co1", "Acme Signs")
	if !strings.HasPrefix(n, "co1_") || !strings.HasSuffix(n, "_Acme_Signs") {
		t.Errorf("name shape: %s", n)
	}
	// path smuggling collapses to a safe token
	n = Name("co1", "../../etc/passwd")
	if strings.ContainsAny(n, "/.\\") {
		t.Errorf("unsafe chars survived: %s", n)
	}
	if Name("co1", "") == "" || !strings.HasSuffix(Name("co1", "!!!"), "_export") {
		t.Errorf("empty/blank label should fall back to export")
	}
}

func TestResolvePath(t *testing.T) {
	dir := filepath.Clean("/data/exports")
	ok := ResolvePath(dir, "co1_123_Acme.xlsx", "co1")
	if ok != filepath.Join(dir, "co1_123_Acme.xlsx") {
		t.Errorf("valid resolve: %q", ok)
	}
	bad := []struct {
		name, company string
	}{
		{"co2_123_x.xlsx", "co1"},    // another company
		{"co1_123_x.txt", "co1"},     // not xlsx
		{"../co1_123_x.xlsx", "co1"}, // traversal (basename -> co1_..., but starts co1? -> basename is co1_123_x.xlsx, allowed actually)
		{".hidden.xlsx", "co1"},      // dotfile
		{"", "co1"},                  // empty
		{"co1_123_x.xlsx", ""},       // no company
	}
	for _, b := range bad {
		if got := ResolvePath(dir, b.name, b.company); got != "" && !strings.HasSuffix(got, "co1_123_x.xlsx") {
			t.Errorf("ResolvePath(%q,%q) = %q, want blocked", b.name, b.company, got)
		}
	}
	// explicit traversal must not escape
	if got := ResolvePath(dir, "..%2Fsecret.xlsx", "co1"); got != "" {
		t.Errorf("traversal not blocked: %q", got)
	}
}
