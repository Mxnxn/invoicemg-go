package lifecycle

import "github.com/mxnxn/invoicemg-go/internal/httpx"

type jobDTO struct {
	ID              string         `json:"_id"`
	ChallanNumber   string         `json:"challanNumber"`
	ReceivedDate    string         `json:"receivedDate"`
	Total           float64        `json:"total"`
	Advance         float64        `json:"advance"`
	Queue           string         `json:"queue"`
	Progress        string         `json:"progress"`
	QueueOrder      []string       `json:"queueOrder"`
	Unlocked        bool           `json:"unlocked"`
	ClientID        *jobClientDTO  `json:"client_id"`
	EmployeeID      *personDTO     `json:"employee_id"`
	VendorID        *personDTO     `json:"vendor_id"`
	Rows            []rowDTO       `json:"rows"`
	CreatedAt       httpx.Time     `json:"createdAt"`
	UpdatedAt       httpx.Time     `json:"updatedAt"`
	Version         int            `json:"__v"`
	InvoiceState    string         `json:"invoiceState"`
	InvoicedRows    int            `json:"invoicedRows"`
	InvoiceNumbers  []string       `json:"invoiceNumbers"`
	ReadyForInvoice bool           `json:"readyForInvoice"`
	Lock            lockStateDTO   `json:"lock"`
	Alert           legacyAlertDTO `json:"alert"`
	Alerts          alertsDTO      `json:"alerts"`
}

type jobClientDTO struct {
	ID             string `json:"_id"`
	ClientName     string `json:"clientName"`
	ClientFirm     string `json:"clientFirm"`
	ClientPhone    string `json:"clientPhone"`
	ClientAddress  string `json:"clientAddress"`
	NotifyOnCreate *bool  `json:"notifyOnCreate"`
	NotifyOnUpdate *bool  `json:"notifyOnUpdate"`
}

type personDTO struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type rowDTO struct {
	ID            string           `json:"_id"`
	RowID         string           `json:"rowId"`
	Material      string           `json:"material"`
	Description   string           `json:"description"`
	Qty           float64          `json:"qty"`
	HasDimensions bool             `json:"hasDimensions"`
	Length        string           `json:"length"`
	Width         string           `json:"width"`
	Rate          float64          `json:"rate"`
	Cgst          float64          `json:"cgst"`
	Sgst          float64          `json:"sgst"`
	Igst          float64          `json:"igst"`
	Discount      float64          `json:"discount"`
	Charges       float64          `json:"charges"`
	Queue         string           `json:"queue"`
	Progress      string           `json:"progress"`
	QueueOrder    []string         `json:"queueOrder"`
	EmployeeID    *personDTO       `json:"employee_id"`
	QuotationID   *quotationRefDTO `json:"quotation_id"`
	EntryID       *entryRefDTO     `json:"entry_id"`
	Invoiced      bool             `json:"invoiced"`
	CreatedAt     httpx.Time       `json:"createdAt"`
	UpdatedAt     httpx.Time       `json:"updatedAt"`
}

type quotationRefDTO struct {
	ID              string `json:"_id"`
	QuotationNumber string `json:"quotationNumber"`
}

type entryRefDTO struct {
	ID        string  `json:"_id"`
	HasIssued bool    `json:"has_issued"`
	Total     float64 `json:"total"`
	Advance   float64 `json:"advance"`
}

type lockStateDTO struct {
	Invoiced      bool `json:"invoiced"`
	Unlocked      bool `json:"unlocked"`
	CanEditValues bool `json:"canEditValues"`
	CanEditQueue  bool `json:"canEditQueue"`
	CanDeleteJob  bool `json:"canDeleteJob"`
	CanDeleteRow  bool `json:"canDeleteRow"`
}

// legacyAlertDTO is the singular `alert` (JobDoneAlert.alertState).
type legacyAlertDTO struct {
	SentBefore  bool        `json:"sentBefore"`
	SentAt      *httpx.Time `json:"sentAt"`
	AlertCount  int         `json:"alertCount"`
	PendingRows int         `json:"pendingRows"`
	CanSend     bool        `json:"canSend"`
	IsUpdate    bool        `json:"isUpdate"`
}

type alertsDTO struct {
	Created channelStateDTO `json:"created"`
	Done    channelStateDTO `json:"done"`
}

// channelStateDTO is one channel of the plural `alerts` (JobAlertState.alertState).
type channelStateDTO struct {
	SentBefore  bool        `json:"sentBefore"`
	SentAt      *httpx.Time `json:"sentAt"`
	Count       int         `json:"count"`
	Status      *string     `json:"status"`
	StatusAt    *httpx.Time `json:"statusAt"`
	Error       string      `json:"error"`
	CanSend     bool        `json:"canSend"`
	IsUpdate    bool        `json:"isUpdate"`
	Changed     bool        `json:"changed"`
	Signature   string      `json:"signature"`
	PendingRows int         `json:"pendingRows"`
	DoneRowIDs  []string    `json:"doneRowIds,omitempty"`
}
