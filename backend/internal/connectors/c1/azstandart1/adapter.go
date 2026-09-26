// Package azstandart1 is the c1 adapter for "AzStandart" — Бухгалтерия
// предприятия для Азербайджана, редакция 1.x (COMPLEX SERVİCES). It knows the
// configuration's METADATA names; every physical table/column is resolved from
// the base's own c1meta.Profile at construction time. Read-only.
package azstandart1

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/c1meta"
)

func init() {
	c1.Register("azstandart1",
		func(p *c1meta.Profile) bool {
			return p.ConfigName == "AzStandart" && c1.MajorVersion(p.ConfigVersion) == 1
		},
		New)
}

// metadata names used by this configuration family
const (
	mCustomers = "Справочник.Контрагенты"
	mContracts = "Справочник.ДоговорыКонтрагентов"
	mNomen     = "Справочник.Номенклатура"
	mUsers     = "Справочник.Пользователи"
	mInvoice   = "Документ.РеализацияТоваровУслуг"
	mInvoiceVT = "Документ.РеализацияТоваровУслуг.Услуги"
	mPayment   = "Документ.ПлатежноеПоручениеВходящее"
)

type adapter struct {
	db     *sql.DB
	p      *c1meta.Profile
	offset int // 1C stores dates shifted by this many years (usually 2000)

	tCust, tContr, tNomen, tUsers, tInv, tInvVT, tPay string

	// resolved physical columns
	custINN, custFull          string
	contrDueDays, contrDueSet  string
	invCust, invContr, invResp string
	invSum                     string
	vtNomen, vtSum, vtVAT      string
	vtQty, vtPrice, vtContent  string
	payCust, payContr, paySum  string
	payPurpose                 string
}

// New resolves every table and column this adapter needs; fails loudly if the
// base does not expose an expected metadata object (never guesses).
func New(db *sql.DB, p *c1meta.Profile) (c1.Adapter, error) {
	a := &adapter{db: db, p: p, offset: 2000}
	if err := db.QueryRow(`SELECT TOP 1 Offset FROM _YearOffset`).Scan(&a.offset); err != nil {
		return nil, fmt.Errorf("_YearOffset: %w", err)
	}
	var err error
	need := func(meta string) string {
		t := p.Table(meta)
		if t == "" {
			t = p.TabularSections[meta]
		}
		if t == "" && err == nil {
			err = fmt.Errorf("metadata obyekti tapılmadı: %s", meta)
		}
		return t
	}
	a.tCust, a.tContr, a.tNomen, a.tUsers = need(mCustomers), need(mContracts), need(mNomen), need(mUsers)
	a.tInv, a.tInvVT, a.tPay = need(mInvoice), need(mInvoiceVT), need(mPayment)
	if err != nil {
		return nil, err
	}
	col := func(table, object, attr string) string {
		if err != nil {
			return ""
		}
		var c string
		c, err = p.Resolve(db, table, object, attr)
		return c
	}
	a.custINN = col(a.tCust, mCustomers, "ИНН")
	a.custFull = col(a.tCust, mCustomers, "НаименованиеПолное")
	a.contrDueDays = col(a.tContr, mContracts, "СрокОплаты")
	a.contrDueSet = col(a.tContr, mContracts, "УстановленСрокОплаты")
	a.invCust = col(a.tInv, mInvoice, "Контрагент")
	a.invContr = col(a.tInv, mInvoice, "ДоговорКонтрагента")
	a.invResp = col(a.tInv, mInvoice, "Ответственный")
	a.invSum = col(a.tInv, mInvoice, "СуммаДокумента")
	a.vtNomen = col(a.tInvVT, mInvoice, "Номенклатура")
	a.vtSum = col(a.tInvVT, mInvoice, "Сумма")
	a.vtVAT = col(a.tInvVT, mInvoice, "СуммаНДС")
	a.vtQty = col(a.tInvVT, mInvoice, "Количество")
	a.vtPrice = col(a.tInvVT, mInvoice, "Цена")
	a.vtContent = col(a.tInvVT, mInvoice, "Содержание")
	a.payCust = col(a.tPay, mPayment, "Контрагент")
	a.payContr = col(a.tPay, mPayment, "ДоговорКонтрагента")
	a.paySum = col(a.tPay, mPayment, "СуммаДокумента")
	a.payPurpose = col(a.tPay, mPayment, "НазначениеПлатежа")
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *adapter) Info() c1.Info {
	return c1.Info{Adapter: "azstandart1", ConfigName: a.p.ConfigName, ConfigVersion: a.p.ConfigVersion, YearOffset: a.offset}
}

// ---- helpers ----

func (a *adapter) shift(t time.Time) time.Time   { return t.AddDate(a.offset, 0, 0) }
func (a *adapter) unshift(t time.Time) time.Time { return t.AddDate(-a.offset, 0, 0) }

func id(b []byte) string { return hex.EncodeToString(b) }

func idParam(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		return nil, fmt.Errorf("bad id")
	}
	return b, nil
}

func top(n int) string {
	if n > 0 {
		return fmt.Sprintf("TOP %d ", n)
	}
	return ""
}

// docFilter builds the standard document predicate + args for a query on alias d.
func (a *adapter) docFilter(q c1.Query, custCol string, args *[]any) string {
	w := "d._Posted=0x01 AND d._Marked=0x00"
	if !q.From.IsZero() {
		*args = append(*args, a.shift(q.From))
		w += fmt.Sprintf(" AND d._Date_Time >= @p%d", len(*args))
	}
	if !q.To.IsZero() {
		*args = append(*args, a.shift(q.To))
		w += fmt.Sprintf(" AND d._Date_Time < @p%d", len(*args))
	}
	if q.CustomerID != "" {
		if b, err := idParam(q.CustomerID); err == nil {
			*args = append(*args, b)
			w += fmt.Sprintf(" AND d.[%s] = @p%d", custCol, len(*args))
		}
	}
	return w
}

// ---- Adapter ----

func (a *adapter) Counts(ctx context.Context) (c1.Counts, error) {
	var c c1.Counts
	q := fmt.Sprintf(`SELECT
		(SELECT COUNT(*) FROM [%s] WHERE _Folder=0x01 AND _Marked=0x00),
		(SELECT COUNT(*) FROM [%s] WHERE _Marked=0x00),
		(SELECT COUNT(*) FROM [%s] WHERE _Posted=0x01 AND _Marked=0x00),
		(SELECT COUNT(*) FROM [%s] WHERE _Posted=0x01 AND _Marked=0x00)`,
		a.tCust, a.tContr, a.tInv, a.tPay)
	err := a.db.QueryRowContext(ctx, q).Scan(&c.Customers, &c.Contracts, &c.Invoices, &c.Payments)
	return c, err
}

func (a *adapter) Customers(ctx context.Context, q c1.Query) ([]c1.Customer, error) {
	sqlq := fmt.Sprintf(`SELECT %s_IDRRef, _Code, _Description, [%s], [%s], _Folder, _Marked
		FROM [%s] ORDER BY _Description`, top(q.Limit), a.custFull, a.custINN, a.tCust)
	rows, err := a.db.QueryContext(ctx, sqlq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.Customer{}
	for rows.Next() {
		var c c1.Customer
		var rid, folder, marked []byte
		if err := rows.Scan(&rid, &c.Code, &c.Name, &c.FullName, &c.VOEN, &folder, &marked); err != nil {
			return nil, err
		}
		c.ID = id(rid)
		c.Code, c.Name, c.FullName, c.VOEN = strings.TrimSpace(c.Code), strings.TrimSpace(c.Name), strings.TrimSpace(c.FullName), strings.TrimSpace(c.VOEN)
		c.IsGroup = len(folder) > 0 && folder[0] == 0x00
		c.Marked = len(marked) > 0 && marked[0] == 0x01
		out = append(out, c)
	}
	return out, rows.Err()
}

func (a *adapter) Contracts(ctx context.Context, q c1.Query) ([]c1.Contract, error) {
	var args []any
	w := "_Marked=0x00"
	if q.CustomerID != "" {
		b, err := idParam(q.CustomerID)
		if err != nil {
			return nil, err
		}
		args = append(args, b)
		w += " AND _OwnerIDRRef=@p1"
	}
	sqlq := fmt.Sprintf(`SELECT %s_IDRRef, _OwnerIDRRef, _Code, _Description, [%s], [%s]
		FROM [%s] WHERE %s ORDER BY _Description`, top(q.Limit), a.contrDueDays, a.contrDueSet, a.tContr, w)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.Contract{}
	for rows.Next() {
		var c c1.Contract
		var rid, owner, set []byte
		var days float64
		if err := rows.Scan(&rid, &owner, &c.Code, &c.Name, &days, &set); err != nil {
			return nil, err
		}
		c.ID, c.CustomerID = id(rid), id(owner)
		c.Code, c.Name = strings.TrimSpace(c.Code), strings.TrimSpace(c.Name)
		c.DueDays = int(days)
		c.DueDaysSet = len(set) > 0 && set[0] == 0x01
		out = append(out, c)
	}
	return out, rows.Err()
}

func (a *adapter) Invoices(ctx context.Context, q c1.Query) ([]c1.Invoice, error) {
	var args []any
	w := a.docFilter(q, a.invCust, &args)
	args = append(args, q.DefaultDueDays)
	dueArg := len(args)
	sqlq := fmt.Sprintf(`SELECT %sd._IDRRef, d._Number, d._Date_Time,
		DATEADD(day, CASE WHEN c.[%s]=0x01 AND c.[%s]>0 THEN CAST(c.[%s] AS int) ELSE @p%d END, d._Date_Time) AS due,
		d.[%s], ISNULL(k._Description,''), d.[%s], ISNULL(c._Description,''), ISNULL(u._Description,''), d.[%s]
		FROM [%s] d
		LEFT JOIN [%s] k ON k._IDRRef=d.[%s]
		LEFT JOIN [%s] c ON c._IDRRef=d.[%s]
		LEFT JOIN [%s] u ON u._IDRRef=d.[%s]
		WHERE %s ORDER BY d._Date_Time DESC`,
		top(q.Limit), a.contrDueSet, a.contrDueDays, a.contrDueDays, dueArg,
		a.invCust, a.invContr, a.invSum,
		a.tInv, a.tCust, a.invCust, a.tContr, a.invContr, a.tUsers, a.invResp, w)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.Invoice{}
	for rows.Next() {
		var v c1.Invoice
		var rid, cust, contr []byte
		if err := rows.Scan(&rid, &v.Number, &v.Date, &v.Due, &cust, &v.Customer, &contr, &v.Contract, &v.Manager, &v.Amount); err != nil {
			return nil, err
		}
		v.ID, v.CustomerID, v.ContractID = id(rid), id(cust), id(contr)
		v.Number = strings.TrimSpace(v.Number)
		v.Customer, v.Contract, v.Manager = strings.TrimSpace(v.Customer), strings.TrimSpace(v.Contract), strings.TrimSpace(v.Manager)
		v.Date, v.Due = a.unshift(v.Date), a.unshift(v.Due)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (a *adapter) InvoiceLines(ctx context.Context, q c1.Query) ([]c1.InvoiceLine, error) {
	var args []any
	w := a.docFilter(q, a.invCust, &args)
	sqlq := fmt.Sprintf(`SELECT %sl.[%s_IDRRef], l.[%s], ISNULL(n._Description,''), ISNULL(g._Description,''),
		l.[%s], l.[%s], l.[%s], l.[%s], l.[%s]
		FROM [%s] l
		JOIN [%s] d ON d._IDRRef = l.[%s_IDRRef]
		LEFT JOIN [%s] n ON n._IDRRef = l.[%s]
		LEFT JOIN [%s] g ON g._IDRRef = n._ParentIDRRef
		WHERE %s ORDER BY d._Date_Time DESC%s`,
		top(q.Limit), a.tInv, a.vtNomen, a.vtContent, a.vtQty, a.vtPrice, a.vtSum, a.vtVAT,
		a.tInvVT, a.tInv, a.tInv, a.tNomen, a.vtNomen, a.tNomen, w, a.lineNoSuffix())
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.InvoiceLine{}
	for rows.Next() {
		var l c1.InvoiceLine
		var inv, svc []byte
		if err := rows.Scan(&inv, &svc, &l.Service, &l.Category, &l.Description, &l.Qty, &l.Price, &l.Amount, &l.VAT); err != nil {
			return nil, err
		}
		l.InvoiceID, l.ServiceID = id(inv), id(svc)
		l.Service, l.Category, l.Description = strings.TrimSpace(l.Service), strings.TrimSpace(l.Category), strings.TrimSpace(l.Description)
		out = append(out, l)
	}
	return out, rows.Err()
}

// lineNoSuffix: the VT line-number column is _LineNo<N> where N = VT number + 1
// in 8.2 storage; we read it from sys.columns once instead of assuming.
func (a *adapter) lineNoSuffix() string {
	var col string
	if a.db.QueryRow(`SELECT TOP 1 name FROM sys.columns WHERE object_id=OBJECT_ID(@p1) AND name LIKE '\_LineNo%' ESCAPE '\'`, a.tInvVT).Scan(&col) == nil && col != "" {
		return ", l.[" + col + "]"
	}
	return ""
}

func (a *adapter) Payments(ctx context.Context, q c1.Query) ([]c1.Payment, error) {
	var args []any
	w := a.docFilter(q, a.payCust, &args)
	sqlq := fmt.Sprintf(`SELECT %sd._IDRRef, d._Number, d._Date_Time, d.[%s], ISNULL(k._Description,''), d.[%s], d.[%s], ISNULL(CAST(d.[%s] AS nvarchar(500)),'')
		FROM [%s] d
		LEFT JOIN [%s] k ON k._IDRRef=d.[%s]
		WHERE %s ORDER BY d._Date_Time DESC`,
		top(q.Limit), a.payCust, a.payContr, a.paySum, a.payPurpose,
		a.tPay, a.tCust, a.payCust, w)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.Payment{}
	for rows.Next() {
		var v c1.Payment
		var rid, cust, contr []byte
		if err := rows.Scan(&rid, &v.Number, &v.Date, &cust, &v.Customer, &contr, &v.Amount, &v.Purpose); err != nil {
			return nil, err
		}
		v.ID, v.CustomerID, v.ContractID = id(rid), id(cust), id(contr)
		v.Number, v.Customer, v.Purpose = strings.TrimSpace(v.Number), strings.TrimSpace(v.Customer), strings.TrimSpace(v.Purpose)
		v.Date = a.unshift(v.Date)
		out = append(out, v)
	}
	return out, rows.Err()
}
