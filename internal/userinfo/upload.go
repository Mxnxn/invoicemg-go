package userinfo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

const maxUpload = 50 << 20 // 50 MiB, matching the Node multer limit.

var (
	unsafeChars = regexp.MustCompile(`[^\w.-]+`)
	leadingDots = regexp.MustCompile(`^\.+`)
	imageTypes  = map[string]bool{"image/png": true, "image/jpg": true, "image/jpeg": true}
)

// safeName mirrors the multer diskStorage filename: basename, then a safe charset, no leading
// dots, capped at 80 chars, defaulting to "upload".
func safeName(original string) string {
	base := filepath.Base(original)
	if base == "." || base == "/" || base == "" {
		base = "upload"
	}
	s := unsafeChars.ReplaceAllString(base, "_")
	s = leadingDots.ReplaceAllString(s, "")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "upload"
	}
	return s
}

// Upload is POST /userinfo/upload: store the company logo (a png/jpg/jpeg under `userDP`),
// replace the previous one, and return the refreshed profile. A missing or non-image file is
// Node's 200 { status:false, "Invalid request." } (the multer fileFilter drops it, leaving no
// file), and a missing company is a 404.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	file, header, err := r.FormFile("userDP")
	if err != nil || header == nil || !imageTypes[header.Header.Get("Content-Type")] {
		if file != nil {
			file.Close()
		}
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), safeName(header.Filename))
	if err := os.MkdirAll(h.uploadsDir, 0o755); err != nil {
		httpx.Internal(w, err)
		return
	}
	dst, err := os.Create(filepath.Join(h.uploadsDir, filename))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		httpx.Internal(w, err)
		return
	}
	dst.Close()

	// Delete the previous logo, best-effort (Node's fs.unlink with a logged error).
	if prev, found, _ := h.companies.FindActive(r.Context(), sess.UID, sess.CompanyID); found && prev.URL != "" {
		if old := filepath.Base(prev.URL); old != "" && old != "." {
			_ = os.Remove(filepath.Join(h.uploadsDir, old))
		}
	}

	company, found, err := h.companies.Update(r.Context(), sess.UID, sess.CompanyID, store.CompanyPatch{URL: &filename})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	user, _ := h.users.FindByID(r.Context(), sess.UID)
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Upload successful", Data: buildProfile(user, company)})
}
