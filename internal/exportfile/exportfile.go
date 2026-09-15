// Package exportfile owns the naming and safe resolution of generated .xlsx exports, the Go
// port of Helpers/ExportFiles.js. Every export is named "<companyId>_<millis>_<label>.xlsx",
// so a download can be authorised from the filename alone (the owner is in the name) and a
// caller-controlled name can never escape the exports directory or read another company's file.
package exportfile

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	labelUnsafe = regexp.MustCompile(`[^\w-]+`)
	labelTrim   = regexp.MustCompile(`^_+|_+$`)
)

// Name builds the base filename (no extension) for a company's export from a free-text label.
// Dots are dropped along with separators, so a label like "../../etc/passwd" cannot smuggle a
// path in, and the label is capped at 60 chars. The caller appends ".xlsx".
func Name(companyID, label string) string {
	safe := labelUnsafe.ReplaceAllString(label, "_")
	safe = labelTrim.ReplaceAllString(safe, "")
	if len(safe) > 60 {
		safe = safe[:60]
	}
	if safe == "" {
		safe = "export"
	}
	return fmt.Sprintf("%s_%d_%s", companyID, time.Now().UnixMilli(), safe)
}

// ownedBy is the companyId prefix check: an export belongs to the company whose id starts its name.
func ownedBy(fileName, companyID string) bool {
	return companyID != "" && strings.HasPrefix(fileName, companyID+"_")
}

// ResolvePath returns the absolute path of an export only when rawName is a plain .xlsx filename
// that stays inside dir and belongs to companyID; otherwise "". Callers answer 404 on "" so the
// response cannot probe which files exist. basename strips any directory part (including one
// smuggled in as %2F), and a containment check is the belt-and-braces second guard.
func ResolvePath(dir, rawName, companyID string) string {
	if rawName == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	fileName := filepath.Base(rawName)
	if fileName == "" || fileName == "." || strings.HasPrefix(fileName, ".") {
		return ""
	}
	if strings.ToLower(filepath.Ext(fileName)) != ".xlsx" {
		return ""
	}
	if !ownedBy(fileName, companyID) {
		return ""
	}
	resolved := filepath.Join(dir, fileName)
	if resolved != filepath.Join(dir, filepath.Base(fileName)) {
		return ""
	}
	if !strings.HasPrefix(resolved, dir+string(filepath.Separator)) {
		return ""
	}
	return resolved
}
