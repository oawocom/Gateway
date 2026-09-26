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
	From, To       time.Time
	CustomerID     string // hex _IDRRef; empty = all
	Limit          int
	DefaultDueDays int // used when a contract has no payment term
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
