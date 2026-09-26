package azstandart1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gateway/internal/connectors/c1"
)

// All aggregates are document-based (invoices, service lines, payments) so the
// figures reconcile with each other: invoiced − paid = outstanding.

func (a *adapter) Monthly(ctx context.Context, q c1.Query) ([]c1.MonthRow, error) {
	var args []any
	wInv := a.docFilter(q, a.invCust, &args)
	nInv := len(args)
	wPay := a.docFilter(q, a.payCust, &args)
	sqlq := fmt.Sprintf(`
		SELECT y, m, SUM(invoiced), SUM(net), SUM(paid), SUM(cnt) FROM (
			SELECT YEAR(d._Date_Time) y, MONTH(d._Date_Time) m, d.[%s] invoiced, 0 net, 0 paid, 1 cnt
			  FROM [%s] d WHERE %s
			UNION ALL
			SELECT YEAR(d._Date_Time), MONTH(d._Date_Time), 0, l.[%s], 0, 0
			  FROM [%s] l JOIN [%s] d ON d._IDRRef = l.[%s_IDRRef] WHERE %s
			UNION ALL
			SELECT YEAR(d._Date_Time), MONTH(d._Date_Time), 0, 0, d.[%s], 0
			  FROM [%s] d WHERE %s
		) x GROUP BY y, m ORDER BY y, m`,
		a.invSum, a.tInv, wInv,
		a.vtSum, a.tInvVT, a.tInv, a.tInv, wInv,
		a.paySum, a.tPay, wPay)
	_ = nInv
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.MonthRow{}
	for rows.Next() {
		var r c1.MonthRow
		if err := rows.Scan(&r.Year, &r.Month, &r.Invoiced, &r.Net, &r.Paid, &r.Invoices); err != nil {
			return nil, err
		}
		r.Year -= a.offset
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *adapter) RevenueByService(ctx context.Context, q c1.Query) ([]c1.ServiceRow, error) {
	var args []any
	w := a.docFilter(q, a.invCust, &args)
	sqlq := fmt.Sprintf(`
		SELECT l.[%s], ISNULL(n._Description,''), ISNULL(g._Description,''),
		       SUM(l.[%s]), SUM(l.[%s]), COUNT(DISTINCT d.[%s]), COUNT(*)
		FROM [%s] l
		JOIN [%s] d ON d._IDRRef = l.[%s_IDRRef]
		LEFT JOIN [%s] n ON n._IDRRef = l.[%s]
		LEFT JOIN [%s] g ON g._IDRRef = n._ParentIDRRef
		WHERE %s
		GROUP BY l.[%s], n._Description, g._Description
		ORDER BY SUM(l.[%s]) DESC`,
		a.vtNomen, a.vtSum, a.vtVAT, a.invCust,
		a.tInvVT, a.tInv, a.tInv, a.tNomen, a.vtNomen, a.tNomen, w, a.vtNomen, a.vtSum)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.ServiceRow{}
	for rows.Next() {
		var r c1.ServiceRow
		var sid []byte
		if err := rows.Scan(&sid, &r.Service, &r.Category, &r.Net, &r.VAT, &r.Customers, &r.Lines); err != nil {
			return nil, err
		}
		r.ServiceID = id(sid)
		r.Service, r.Category = strings.TrimSpace(r.Service), strings.TrimSpace(r.Category)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *adapter) TopCustomers(ctx context.Context, q c1.Query) ([]c1.CustomerRow, error) {
	var args []any
	wInv := a.docFilter(q, a.invCust, &args)
	wPay := a.docFilter(q, a.payCust, &args)
	sqlq := fmt.Sprintf(`
		SELECT %sx.cust, ISNULL(k._Description,''), ISNULL(k.[%s],''), SUM(x.invoiced), SUM(x.paid), SUM(x.cnt)
		FROM (
			SELECT d.[%s] cust, d.[%s] invoiced, 0 paid, 1 cnt FROM [%s] d WHERE %s
			UNION ALL
			SELECT d.[%s], 0, d.[%s], 0 FROM [%s] d WHERE %s
		) x LEFT JOIN [%s] k ON k._IDRRef = x.cust
		GROUP BY x.cust, k._Description, k.[%s]
		ORDER BY SUM(x.invoiced) DESC`,
		top(q.Limit), a.custINN,
		a.invCust, a.invSum, a.tInv, wInv,
		a.payCust, a.paySum, a.tPay, wPay,
		a.tCust, a.custINN)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c1.CustomerRow{}
	for rows.Next() {
		var r c1.CustomerRow
		var cid []byte
		if err := rows.Scan(&cid, &r.Customer, &r.VOEN, &r.Invoiced, &r.Paid, &r.Invoices); err != nil {
			return nil, err
		}
		r.CustomerID = id(cid)
		r.Customer, r.VOEN = strings.TrimSpace(r.Customer), strings.TrimSpace(r.VOEN)
		out = append(out, r)
	}
	return out, rows.Err()
}

// OpenInvoices: FIFO allocation of all payments to invoices per settlement key
// (customer by default, see Query.SettleByContract), over the whole
// history up to asOf — an outstanding balance is a state, not a flow. Two
// plain queries (invoices, payments per key) and the allocation in Go: fast
// and independent of the SQL Server version.
func (a *adapter) OpenInvoices(ctx context.Context, q c1.Query, asOf time.Time) ([]c1.OpenInvoice, []c1.Advance, error) {
	state := c1.Query{CustomerID: q.CustomerID, To: asOf.AddDate(0, 0, 1), DefaultDueDays: q.DefaultDueDays}
	// settlement key: by customer (default — payments are matched to the
	// customer's invoices oldest-first, as account 211 does), or by contract
	// when the base keeps one contract per customer and pays against it.
	keyInv, keyPay := fmt.Sprintf("d.[%s]", a.invCust), fmt.Sprintf("d.[%s]", a.payCust)
	if q.SettleByContract {
		nullKey := "0x00000000000000000000000000000000"
		keyInv = fmt.Sprintf("CASE WHEN d.[%s] = %s THEN d.[%s] ELSE d.[%s] END", a.invContr, nullKey, a.invCust, a.invContr)
		keyPay = fmt.Sprintf("CASE WHEN d.[%s] = %s THEN d.[%s] ELSE d.[%s] END", a.payContr, nullKey, a.payCust, a.payContr)
	}

	// 1) invoices, oldest first within each settlement key
	var args []any
	w := a.docFilter(state, a.invCust, &args)
	args = append(args, q.DefaultDueDays)
	sqlq := fmt.Sprintf(`SELECT d._IDRRef, d._Number, d._Date_Time,
		DATEADD(day, CASE WHEN c.[%s]=0x01 AND c.[%s]>0 THEN CAST(c.[%s] AS int) ELSE @p%d END, d._Date_Time),
		d.[%s], ISNULL(k._Description,''), d.[%s], ISNULL(c._Description,''), ISNULL(u._Description,''), d.[%s], %s
		FROM [%s] d
		LEFT JOIN [%s] k ON k._IDRRef = d.[%s]
		LEFT JOIN [%s] c ON c._IDRRef = d.[%s]
		LEFT JOIN [%s] u ON u._IDRRef = d.[%s]
		WHERE %s ORDER BY %s, d._Date_Time, d._IDRRef`,
		a.contrDueSet, a.contrDueDays, a.contrDueDays, len(args),
		a.invCust, a.invContr, a.invSum, keyInv,
		a.tInv, a.tCust, a.invCust, a.tContr, a.invContr, a.tUsers, a.invResp, w, keyInv)
	rows, err := a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, nil, err
	}
	type inv struct {
		c1.OpenInvoice
		key string
	}
	var invs []inv
	for rows.Next() {
		var v inv
		var rid, cust, contr, key []byte
		if err := rows.Scan(&rid, &v.Number, &v.Date, &v.Due, &cust, &v.Customer, &contr, &v.Contract, &v.Manager, &v.Amount, &key); err != nil {
			rows.Close()
			return nil, nil, err
		}
		v.ID, v.CustomerID, v.ContractID, v.key = id(rid), id(cust), id(contr), id(key)
		v.Number, v.Customer, v.Contract, v.Manager = strings.TrimSpace(v.Number), strings.TrimSpace(v.Customer), strings.TrimSpace(v.Contract), strings.TrimSpace(v.Manager)
		v.Date, v.Due = a.unshift(v.Date), a.unshift(v.Due)
		invs = append(invs, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// 2) payments per settlement key
	args = args[:0]
	w = a.docFilter(state, a.payCust, &args)
	sqlq = fmt.Sprintf(`SELECT %s, MIN(d.[%s]), SUM(d.[%s]) FROM [%s] d WHERE %s GROUP BY %s`,
		keyPay, a.payCust, a.paySum, a.tPay, w, keyPay)
	rows, err = a.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, nil, err
	}
	paid := map[string]float64{}
	payCust := map[string]string{}
	for rows.Next() {
		var key, cust []byte
		var sum float64
		if err := rows.Scan(&key, &cust, &sum); err != nil {
			rows.Close()
			return nil, nil, err
		}
		paid[id(key)] = sum
		payCust[id(key)] = id(cust)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// 3) FIFO allocation
	open := []c1.OpenInvoice{}
	remaining := map[string]float64{}
	for k, v := range paid {
		remaining[k] = v
	}
	for _, v := range invs {
		left := remaining[v.key]
		switch {
		case left >= v.Amount:
			v.Paid = v.Amount
			remaining[v.key] = left - v.Amount
		case left > 0:
			v.Paid = left
			remaining[v.key] = 0
		}
		v.Outstanding = v.Amount - v.Paid
		if v.Outstanding <= 0.005 {
			continue
		}
		if asOf.After(v.Due) {
			v.DaysOverdue = int(asOf.Sub(v.Due).Hours() / 24)
		}
		open = append(open, v.OpenInvoice)
	}

	// 4) advances: payment left over after all invoices of the key are covered;
	// keys that never had an invoice are flagged NonInvoiced
	hasInv := map[string]bool{}
	for _, v := range invs {
		hasInv[v.key] = true
	}
	adv := []c1.Advance{}
	for k, left := range remaining {
		if left > 0.005 {
			adv = append(adv, c1.Advance{CustomerID: payCust[k], ContractID: k, Amount: left, NonInvoiced: !hasInv[k]})
		}
	}
	if len(adv) > 0 {
		if err := a.fillNames(ctx, adv); err != nil {
			return nil, nil, err
		}
	}
	return open, adv, nil
}

// fillNames resolves customer and contract names for advances (keys may be a
// contract or a customer id).
func (a *adapter) fillNames(ctx context.Context, adv []c1.Advance) error {
	names := func(table string) (map[string]string, error) {
		rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`SELECT _IDRRef, _Description FROM [%s]`, table))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		m := map[string]string{}
		for rows.Next() {
			var b []byte
			var n string
			if rows.Scan(&b, &n) == nil {
				m[id(b)] = strings.TrimSpace(n)
			}
		}
		return m, rows.Err()
	}
	cust, err := names(a.tCust)
	if err != nil {
		return err
	}
	contr, err := names(a.tContr)
	if err != nil {
		return err
	}
	for i := range adv {
		adv[i].Customer = cust[adv[i].CustomerID]
		if n, ok := contr[adv[i].ContractID]; ok {
			adv[i].Contract = n
		} else {
			adv[i].ContractID = ""
		}
	}
	sortAdv(adv)
	return nil
}

func sortAdv(adv []c1.Advance) {
	for i := 1; i < len(adv); i++ {
		for j := i; j > 0 && adv[j].Amount > adv[j-1].Amount; j-- {
			adv[j], adv[j-1] = adv[j-1], adv[j]
		}
	}
}

func (a *adapter) CustomerActivity(ctx context.Context, q c1.Query) (map[string]bool, map[string]bool, error) {
	set := func(qq c1.Query) (map[string]bool, error) {
		var args []any
		w := a.docFilter(qq, a.invCust, &args)
		rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`SELECT DISTINCT d.[%s] FROM [%s] d WHERE %s`, a.invCust, a.tInv, w), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		m := map[string]bool{}
		for rows.Next() {
			var b []byte
			if rows.Scan(&b) == nil {
				m[id(b)] = true
			}
		}
		return m, rows.Err()
	}
	cur, err := set(q)
	if err != nil {
		return nil, nil, err
	}
	if q.From.IsZero() || q.To.IsZero() {
		return cur, map[string]bool{}, nil
	}
	span := q.To.Sub(q.From)
	prev, err := set(c1.Query{From: q.From.Add(-span), To: q.From, CustomerID: q.CustomerID})
	return cur, prev, err
}

func (a *adapter) PaidByNonInvoiced(ctx context.Context, q c1.Query) (float64, error) {
	keyInv, keyPay := fmt.Sprintf("d.[%s]", a.invCust), fmt.Sprintf("d.[%s]", a.payCust)
	if q.SettleByContract {
		nullKey := "0x00000000000000000000000000000000"
		keyInv = fmt.Sprintf("CASE WHEN d.[%s] = %s THEN d.[%s] ELSE d.[%s] END", a.invContr, nullKey, a.invCust, a.invContr)
		keyPay = fmt.Sprintf("CASE WHEN d.[%s] = %s THEN d.[%s] ELSE d.[%s] END", a.payContr, nullKey, a.payCust, a.payContr)
	}
	var args []any
	wPay := a.docFilter(q, a.payCust, &args)
	sqlq := fmt.Sprintf(`SELECT ISNULL(SUM(d.[%s]),0) FROM [%s] d WHERE %s AND NOT EXISTS (
		SELECT 1 FROM [%s] i WHERE i._Posted=0x01 AND i._Marked=0x00 AND %s = %s)`,
		a.paySum, a.tPay, wPay, a.tInv, keyPay, strings.Replace(keyInv, "d.[", "i.[", -1))
	var v float64
	err := a.db.QueryRowContext(ctx, sqlq, args...).Scan(&v)
	return v, err
}
