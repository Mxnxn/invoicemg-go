package purchaseorder

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubPO struct {
	numbers     []string
	created     store.PurchaseOrder
	gotCreate   store.POWrite
	gotPONumber string
	list        []store.PurchaseOrder

	detail        store.PurchaseOrder
	detailHistory []store.POHistoryRow
	detailNotes   []store.PONote
	detailFound   bool

	updated   store.PurchaseOrder
	updateRes store.POUpdateResult
	gotUpdate store.POUpdate

	actionPO      store.PurchaseOrder
	actionStatus  store.POActionStatus
	note          store.PONote
	noteFound     bool
	noteForbidden bool
	gotNoteText   string

	pending        []store.PurchaseOrder
	visible        []store.PurchaseOrder
	dismissTargets int
	dismissVisible int
	dismissTotal   int
	gotDismissAll  bool
	gotDismissID   store.ID

	convertInvoiceID store.ID
	gotConvertNumber string

	public          store.PublicPO
	publicFound     bool
	recordSendFound bool
	gotSendKind     string
}

func (s *stubPO) Numbers(context.Context, store.ID, store.ID) ([]string, error) { return s.numbers, nil }
func (s *stubPO) Create(_ context.Context, _, _ store.ID, poNumber string, _ store.NoteActor, in store.POWrite) (store.PurchaseOrder, error) {
	s.gotPONumber = poNumber
	s.gotCreate = in
	return s.created, nil
}
func (s *stubPO) List(context.Context, store.ID, store.ID, store.ID) ([]store.PurchaseOrder, error) {
	return s.list, nil
}
func (s *stubPO) Detail(context.Context, store.ID, store.ID, store.ID) (store.PurchaseOrder, []store.POHistoryRow, []store.PONote, bool, error) {
	return s.detail, s.detailHistory, s.detailNotes, s.detailFound, nil
}
func (s *stubPO) Update(_ context.Context, _, _, _ store.ID, _ store.NoteActor, in store.POUpdate) (store.PurchaseOrder, store.POUpdateResult, error) {
	s.gotUpdate = in
	return s.updated, s.updateRes, nil
}
func (s *stubPO) Approve(context.Context, store.ID, store.ID, store.ID, store.NoteActor) (store.PurchaseOrder, store.POActionStatus, error) {
	return s.actionPO, s.actionStatus, nil
}
func (s *stubPO) Revoke(context.Context, store.ID, store.ID, store.ID, store.NoteActor) (store.PurchaseOrder, store.POActionStatus, error) {
	return s.actionPO, s.actionStatus, nil
}
func (s *stubPO) AddNote(_ context.Context, _, _, _ store.ID, _ store.NoteActor, text string) (store.PONote, bool, error) {
	s.gotNoteText = text
	return s.note, s.noteFound, nil
}
func (s *stubPO) EditNote(_ context.Context, _, _ store.ID, _ store.NoteActor, text string) (store.PONote, bool, bool, error) {
	s.gotNoteText = text
	return s.note, s.noteFound, s.noteForbidden, nil
}
func (s *stubPO) PendingApprovals(context.Context, store.ID, store.ID, store.ID) ([]store.PurchaseOrder, []store.PurchaseOrder, error) {
	return s.pending, s.visible, nil
}
func (s *stubPO) DismissApprovals(_ context.Context, _, _, _ store.ID, _ string, all bool, poID store.ID) (int, int, int, error) {
	s.gotDismissAll = all
	s.gotDismissID = poID
	return s.dismissTargets, s.dismissVisible, s.dismissTotal, nil
}
func (s *stubPO) Convert(_ context.Context, _, _, _ store.ID, _ store.NoteActor, invoiceNumber, date string) (store.PurchaseOrder, store.ID, store.POActionStatus, error) {
	s.gotConvertNumber = invoiceNumber
	return s.actionPO, s.convertInvoiceID, s.actionStatus, nil
}
func (s *stubPO) PublicView(context.Context, store.ID, store.ID) (store.PublicPO, bool, error) {
	return s.public, s.publicFound, nil
}
func (s *stubPO) RecordSend(_ context.Context, _, _, _ store.ID, kind, _ string) (store.PurchaseOrder, bool, error) {
	s.gotSendKind = kind
	return s.actionPO, s.recordSendFound, nil
}

func TestNextNumber(t *testing.T) {
	poClock = func() time.Time { return time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { poClock = func() time.Time { return time.Now().UTC() } }()
	s := &stubPO{numbers: []string{"MG/25-26/PO-00003"}}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	New(s).NextNumber(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["data"].(map[string]any)["poNumber"] != "MG/25-26/PO-00004" {
		t.Errorf("poNumber = %v, want MG/25-26/PO-00004", body["data"])
	}
}

func TestCreate_Validation(t *testing.T) {
	s := &stubPO{}
	for _, v := range []url.Values{{"date": {"2026-09-01"}}, {"supplier_id": {"s1"}}} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
		New(s).Create(rec, r)
		var body map[string]any
		json.Unmarshal(rec.Body.Bytes(), &body)
		if body["code"] != float64(422) {
			t.Errorf("%v -> %v, want 422", v, body["code"])
		}
	}
}

func TestCreate_ComputesTotalAndNumber(t *testing.T) {
	poClock = func() time.Time { return time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { poClock = func() time.Time { return time.Now().UTC() } }()
	s := &stubPO{numbers: nil, created: store.PurchaseOrder{ID: "p1", PoNumber: "MG/25-26/PO-00001"}}
	rows := `[{"material":"Steel","qty":"2","rate":"100","gst":"18"}]`
	v := url.Values{"supplier_id": {"s1"}, "date": {"2026-09-01"}, "rows": {rows}}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	New(s).Create(rec, r)

	if s.gotPONumber != "MG/25-26/PO-00001" {
		t.Errorf("poNumber = %q", s.gotPONumber)
	}
	// 2*100 = 200, +18% = 236.
	if s.gotCreate.Total != 236 {
		t.Errorf("total = %v, want 236", s.gotCreate.Total)
	}
	if len(s.gotCreate.Rows) != 1 || s.gotCreate.Rows[0].Material != "Steel" {
		t.Errorf("rows = %+v", s.gotCreate.Rows)
	}
}

func TestUpdate_NotFoundAndConverted(t *testing.T) {
	// not found
	s := &stubPO{detailFound: false}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"po_id": {"p1"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	New(s).Update(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(404) {
		t.Errorf("not found -> %v, want 404", body["code"])
	}

	// converted
	s = &stubPO{detailFound: true, detail: store.PurchaseOrder{ID: "p1", PurchaseInvoiceID: "inv1"}}
	rec = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"po_id": {"p1"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	New(s).Update(rec, r)
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(409) {
		t.Errorf("converted -> %v, want 409", body["code"])
	}
}

func TestUpdate_ComputesFingerprintAndChanges(t *testing.T) {
	// Current: approved PO with one row at rate 100.
	current := store.PurchaseOrder{
		ID: "p1", SupplierID: "s1", Date: "2026-09-01", Total: 100,
		Approval: store.POApproval{State: "approved"},
		Rows:     []store.PORow{{Material: "Steel", Qty: 1, Rate: 100}},
	}
	s := &stubPO{detailFound: true, detail: current, updateRes: store.POUpdateResult{Found: true, RevokedApproval: true}, updated: current}
	// Submit a rate change 100 -> 150.
	rows := `[{"material":"Steel","qty":"1","rate":"150"}]`
	v := url.Values{"po_id": {"p1"}, "rows": {rows}}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	New(s).Update(rec, r)

	if !s.gotUpdate.SetRows || s.gotUpdate.Total != 150 {
		t.Errorf("update total = %v, want 150", s.gotUpdate.Total)
	}
	if len(s.gotUpdate.Changes) == 0 {
		t.Error("expected a rate change to be diffed")
	}
	// New fingerprint must differ from the (empty) approved one so the store can revoke.
	if s.gotUpdate.NewFingerprint == "" {
		t.Error("expected a computed fingerprint")
	}
}
