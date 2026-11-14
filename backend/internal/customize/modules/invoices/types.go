// Package invoices lets selected users apply for invoices on their own on-site
// paid orders and lets administrators review and issue them.
//
// Invoiceable orders are payment orders of this site (balance recharges and plan
// purchases) that completed, were paid in CNY and carry no refund. Several such
// orders form one application; an order belongs to at most one live application
// (pending, processing or issued), enforced by a partial unique index.
package invoices

import (
	"time"

	"github.com/shopspring/decimal"
)

// ID is the extension switch that gates every entry point of this module.
const ID = "invoices"

// Application statuses. Pending and processing applications can still change;
// issued ones can only be voided. Rejected, cancelled and voided applications
// release their orders so they can be invoiced again.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusIssued     = "issued"
	StatusRejected   = "rejected"
	StatusCancelled  = "cancelled"
	StatusVoided     = "voided"
)

// Invoice kinds: electronic ordinary invoice (普票) and VAT special invoice (专票).
const (
	InvoiceNormal  = "normal"
	InvoiceSpecial = "special"
)

// Title kinds.
const (
	TitlePersonal = "personal"
	TitleCompany  = "company"
)

// Access modes.
const (
	AccessAll       = "all"
	AccessAllowlist = "allowlist"
)

// Limits that keep a single request bounded.
const (
	MaxTitlesPerUser        = 10
	MaxOrdersPerApplication = 200
	MaxInvoiceableListed    = 500
	MaxFileBytes            = 10 << 20
)

// Settings are the administrator's rules for applications.
type Settings struct {
	MinAmount       decimal.Decimal `json:"min_amount"`
	AllowSpecial    bool            `json:"allow_special"`
	OrderWindowDays int             `json:"order_window_days"`
	AccessMode      string          `json:"access_mode"`
	ItemName        string          `json:"item_name"`
	Notice          string          `json:"notice"`
	UpdatedAt       *time.Time      `json:"updated_at,omitempty"`
}

// Title is a saved invoice title (抬头) of one user.
type Title struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"-"`
	TitleType   string    `json:"title_type"`
	Name        string    `json:"name"`
	TaxNo       string    `json:"tax_no"`
	Address     string    `json:"address"`
	Phone       string    `json:"phone"`
	BankName    string    `json:"bank_name"`
	BankAccount string    `json:"bank_account"`
	Email       string    `json:"email"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TitleInput is the user-editable part of a title.
type TitleInput struct {
	TitleType   string `json:"title_type"`
	Name        string `json:"name"`
	TaxNo       string `json:"tax_no"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	BankName    string `json:"bank_name"`
	BankAccount string `json:"bank_account"`
	Email       string `json:"email"`
	IsDefault   bool   `json:"is_default"`
}

// Order is an invoiceable payment order as shown to its owner.
type Order struct {
	ID          int64           `json:"id"`
	OutTradeNo  string          `json:"out_trade_no"`
	OrderType   string          `json:"order_type"`
	PaymentType string          `json:"payment_type"`
	Amount      decimal.Decimal `json:"amount"`
	PaidAt      *time.Time      `json:"paid_at"`
}

// ApplicationOrder is an order captured by an application.
type ApplicationOrder struct {
	OrderID    int64           `json:"order_id"`
	OutTradeNo string          `json:"out_trade_no"`
	OrderType  string          `json:"order_type"`
	Amount     decimal.Decimal `json:"amount"`
	PaidAt     *time.Time      `json:"paid_at"`
	// Refunded is set for administrators when the order received a refund after
	// it was captured; an issued invoice then usually needs to be voided.
	Refunded bool `json:"refunded,omitempty"`
}

// Application is one invoice request with its title snapshot.
type Application struct {
	ID           int64              `json:"id"`
	UserID       int64              `json:"user_id"`
	UserEmail    string             `json:"user_email,omitempty"`
	InvoiceType  string             `json:"invoice_type"`
	TitleType    string             `json:"title_type"`
	TitleName    string             `json:"title_name"`
	TaxNo        string             `json:"tax_no"`
	Address      string             `json:"address"`
	Phone        string             `json:"phone"`
	BankName     string             `json:"bank_name"`
	BankAccount  string             `json:"bank_account"`
	Email        string             `json:"email"`
	ItemName     string             `json:"item_name"`
	Amount       decimal.Decimal    `json:"amount"`
	OrderCount   int                `json:"order_count"`
	Remark       string             `json:"remark"`
	Status       string             `json:"status"`
	InvoiceNo    string             `json:"invoice_no"`
	RejectReason string             `json:"reject_reason"`
	AdminNote    string             `json:"admin_note,omitempty"`
	HasFile      bool               `json:"has_file"`
	FileName     string             `json:"file_name,omitempty"`
	RefundAlert  bool               `json:"refund_alert,omitempty"`
	IssuedAt     *time.Time         `json:"issued_at"`
	ClosedAt     *time.Time         `json:"closed_at"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	Orders       []ApplicationOrder `json:"orders,omitempty"`
}

// ApplyInput is a user's request to invoice some of their orders.
type ApplyInput struct {
	OrderIDs    []int64 `json:"order_ids"`
	TitleID     int64   `json:"title_id"`
	InvoiceType string  `json:"invoice_type"`
	Remark      string  `json:"remark"`
}

// ApplicationFilter narrows application lists.
type ApplicationFilter struct {
	UserID   int64
	Status   string
	Query    string
	Page     int
	PageSize int
}

// AccessEntry is an allowlisted user.
type AccessEntry struct {
	UserID    int64            `json:"user_id"`
	Email     string           `json:"email"`
	Username  string           `json:"username"`
	Note      string           `json:"note"`
	MinAmount *decimal.Decimal `json:"min_amount"`
	CreatedAt time.Time        `json:"created_at"`
}

// File is an issued invoice document.
type File struct {
	Name string
	Data []byte
}

// live reports whether an application still holds its orders.
func live(status string) bool {
	return status == StatusPending || status == StatusProcessing || status == StatusIssued
}
