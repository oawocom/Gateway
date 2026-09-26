// Package c1 defines the version-independent contract for reading management
// data (customers, contracts, invoices, payments) out of a 1C:Enterprise base.
// Concrete adapters live in sub-packages, one per configuration family; the
// registry picks one from the base's c1meta.Profile. Adapters address 1C by
// metadata names only — physical table/column names come from the profile.
package c1

import (
	"context"
	"time"
)

// Query bounds a read. Zero From/To mean unbounded. Limit 0 means no limit.
type Query struct {
	From, To         time.Time
	CustomerID       string // hex _IDRRef; empty = all
	Limit            int
	DefaultDueDays   int  // used when a contract has no payment term
	SettleByContract bool // allocate payments per contract instead of per customer
}

type Customer struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	VOEN     string `json:"voen"`
	IsGroup  bool   `json:"is_group"`
	Marked   bool   `json:"marked"`
}

type Contract struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	DueDays    int    `json:"due_days"`     // payment term in days, 0 = not set
	DueDaysSet bool   `json:"due_days_set"` // whether the term is enforced in 1C
}

type Invoice struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Date       time.Time `json:"date"`
	Due        time.Time `json:"due"`
	CustomerID string    `json:"customer_id"`
	Customer   string    `json:"customer"`
	ContractID string    `json:"contract_id"`
	Contract   string    `json:"contract"`
	Manager    string    `json:"manager"`
	Amount     float64   `json:"amount"`
}

// InvoiceLine is one service row of an invoice.
type InvoiceLine struct {
	InvoiceID   string  `json:"invoice_id"`
	ServiceID   string  `json:"service_id"`
	Service     string  `json:"service"`
	Category    string  `json:"category"` // parent group of the service in Номенклатура
	Description string  `json:"description"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Amount      float64 `json:"amount"`
	VAT         float64 `json:"vat"`
}

type Payment struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Date       time.Time `json:"date"`
	CustomerID string    `json:"customer_id"`
	Customer   string    `json:"customer"`
	ContractID string    `json:"contract_id"`
	Amount     float64   `json:"amount"`
	Purpose    string    `json:"purpose"`
}

type Counts struct {
	Customers int64 `json:"customers"`
	Contracts int64 `json:"contracts"`
	Invoices  int64 `json:"invoices"`
	Payments  int64 `json:"payments"`
}

// Info describes which adapter was chosen and why.
type Info struct {
	Adapter       string `json:"adapter"`
	ConfigName    string `json:"config_name"`
	ConfigVersion string `json:"config_version"`
	YearOffset    int    `json:"year_offset"`
}

type Adapter interface {
	Info() Info
	Counts(ctx context.Context) (Counts, error)
	Customers(ctx context.Context, q Query) ([]Customer, error)
	Contracts(ctx context.Context, q Query) ([]Contract, error)
	Invoices(ctx context.Context, q Query) ([]Invoice, error)
	InvoiceLines(ctx context.Context, q Query) ([]InvoiceLine, error)
	Payments(ctx context.Context, q Query) ([]Payment, error)
}

// ---- aggregates for management reports (computed in SQL) ----

type MonthRow struct {
	Year     int     `json:"year"`
	Month    int     `json:"month"`
	Invoiced float64 `json:"invoiced"` // sum of invoice totals (incl. VAT) issued in the month
	Net      float64 `json:"net"`      // service lines total excl. VAT (revenue)
	Paid     float64 `json:"paid"`     // customer payments received in the month
	Invoices int64   `json:"invoices"`
}

type ServiceRow struct {
	ServiceID string  `json:"service_id"`
	Service   string  `json:"service"`
	Category  string  `json:"category"`
	Net       float64 `json:"net"` // excl. VAT
	VAT       float64 `json:"vat"`
	Customers int64   `json:"customers"`
	Lines     int64   `json:"lines"`
}

type CustomerRow struct {
	CustomerID string  `json:"customer_id"`
	Customer   string  `json:"customer"`
	VOEN       string  `json:"voen"`
	Invoiced   float64 `json:"invoiced"`
	Paid       float64 `json:"paid"`
	Invoices   int64   `json:"invoices"`
}

// OpenInvoice is an invoice with an unpaid remainder, payments allocated
// oldest-first (FIFO) within the same contract (or customer when no contract).
type OpenInvoice struct {
	Invoice
	Paid        float64 `json:"paid"`
	Outstanding float64 `json:"outstanding"`
	DaysOverdue int     `json:"days_overdue"` // 0 when not yet due
}

// Advance is money received beyond what was invoiced. NonInvoiced marks payers
// with no invoices at all in history (payment aggregators, B2C collections):
// such money is not a customer prepayment and is reported separately.
type Advance struct {
	NonInvoiced bool    `json:"non_invoiced"`
	CustomerID  string  `json:"customer_id"`
	Customer    string  `json:"customer"`
	ContractID  string  `json:"contract_id"`
	Contract    string  `json:"contract"`
	Amount      float64 `json:"amount"`
}

type Aggregates interface {
	Monthly(ctx context.Context, q Query) ([]MonthRow, error)
	RevenueByService(ctx context.Context, q Query) ([]ServiceRow, error)
	TopCustomers(ctx context.Context, q Query) ([]CustomerRow, error)
	// OpenInvoices ignores q.From/q.To: outstanding is a state, not a flow.
	OpenInvoices(ctx context.Context, q Query, asOf time.Time) ([]OpenInvoice, []Advance, error)
	// CustomerActivity returns distinct invoiced customers in [From,To) and in
	// the equally long period before it (for new/lost/active KPIs).
	CustomerActivity(ctx context.Context, q Query) (current, previous map[string]bool, err error)
	// PaidByNonInvoiced sums payments in [From,To) from payers that have no
	// invoice at all in the base's history (settlement key per Query).
	PaidByNonInvoiced(ctx context.Context, q Query) (float64, error)
	// DataUntil is the date of the latest posted invoice or payment in the base.
	DataUntil(ctx context.Context) (time.Time, error)
}
