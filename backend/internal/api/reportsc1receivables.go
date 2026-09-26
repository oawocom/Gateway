package api

import (
	"net/http"
	"sort"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

// reportsC1Receivables — Debitor Borcları (doc §5): outstanding KPIs, aging
// buckets and the invoice-level table. Query: connection (required), as_of,
// customer. (Manager/contract filters are intentionally absent: payments are
// settled FIFO per customer, so slicing invoices by manager would misstate
// which of them are actually paid.)
func (s *Server) reportsC1Receivables(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	connID := qs.Get("connection")
	if connID == "" {
		errJSON(w, 400, "connection tələb olunur")
		return
	}
	prof, cfg, err := s.loadProfile(r, connID)
	if err != nil {
		errJSON(w, 502, "profil yüklənmədi: "+err.Error())
		return
	}
	db, err := sqldb.New("mssql", *cfg).Open()
	if err != nil {
		errJSON(w, 502, "1C bazasına qoşulmaq alınmadı: "+err.Error())
		return
	}
	defer db.Close()
	ad, err := c1.Select(db, prof)
	if err != nil {
		errJSON(w, 400, err.Error())
		return
	}
	ag, ok := ad.(c1.Aggregates)
	if !ok {
		errJSON(w, 400, "bu adapter aqreqat hesabatları dəstəkləmir")
		return
	}

	ctx := r.Context()
	asOf := time.Now().UTC()
	if t, e := time.Parse("2006-01-02", qs.Get("as_of")); e == nil {
		asOf = t.AddDate(0, 0, 1) // inclusive day
	}
	q := c1.Query{
		CustomerID:     qs.Get("customer"),
		
	}
	s.applyC1Rules(r, &q)
	q.AggregatorIDs = s.c1AggIDs(r, connID)

	open, _, err := ag.OpenInvoices(ctx, q, asOf)
	if err != nil {
		errJSON(w, 502, "open invoices: "+err.Error())
		return
	}

	type bkt struct {
		Key    string  `json:"key"`
		Label  string  `json:"label"`
		Amount float64 `json:"amount"`
		Count  int     `json:"count"`
	}
	order := []*bkt{
		{Key: "current", Label: "Vaxtı çatmayıb"},
		{Key: "1_30", Label: "1–30 gün"},
		{Key: "31_60", Label: "31–60 gün"},
		{Key: "61_90", Label: "61–90 gün"},
		{Key: "90_plus", Label: "90+ gün"},
	}
	byKey := map[string]*bkt{}
	for _, b := range order {
		byKey[b.Key] = b
	}
	keyOf := func(days int) string {
		switch {
		case days <= 0:
			return "current"
		case days <= 30:
			return "1_30"
		case days <= 60:
			return "31_60"
		case days <= 90:
			return "61_90"
		default:
			return "90_plus"
		}
	}

	var outstanding, overdue float64
	for _, o := range open {
		k := keyOf(o.DaysOverdue)
		byKey[k].Amount += o.Outstanding
		byKey[k].Count++
		outstanding += o.Outstanding
		if o.DaysOverdue > 0 {
			overdue += o.Outstanding
		}
	}
	for _, b := range order {
		b.Amount = round2(b.Amount)
	}

	// collection rate over the trailing 12 full months: paid / invoiced
	now := time.Now().UTC()
	mTo := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	mq := q
	mq.From, mq.To = mTo.AddDate(-1, 0, 0), mTo
	months, err := ag.Monthly(ctx, mq)
	if err != nil {
		errJSON(w, 502, "monthly: "+err.Error())
		return
	}
	var invoiced12, paid12 float64
	for _, m := range months {
		invoiced12 += m.Invoiced
		paid12 += m.Paid
	}

	kpi := map[string]any{
		"outstanding":   round2(outstanding),
		"overdue":       round2(overdue),
		"open_invoices": len(open),
	}
	if outstanding > 0 {
		kpi["overdue_ratio_pct"] = round1(overdue / outstanding * 100)
	}
	if invoiced12 > 0 {
		kpi["collection_rate_pct"] = round1(paid12 / invoiced12 * 100)
	}

	// invoice-level table, worst first
	sort.Slice(open, func(i, j int) bool {
		if open[i].DaysOverdue != open[j].DaysOverdue {
			return open[i].DaysOverdue > open[j].DaysOverdue
		}
		return open[i].Outstanding > open[j].Outstanding
	})
	type invRow struct {
		c1.OpenInvoice
		Bucket string `json:"bucket"`
	}
	rowsOut := make([]invRow, 0, len(open))
	for _, o := range open {
		o.Amount = round2(o.Amount)
		o.Paid = round2(o.Paid)
		o.Outstanding = round2(o.Outstanding)
		rowsOut = append(rowsOut, invRow{OpenInvoice: o, Bucket: keyOf(o.DaysOverdue)})
	}

	writeJSON(w, 200, map[string]any{
		"info":     ad.Info(),
		"as_of":    asOf.AddDate(0, 0, -1).Format("2006-01-02"),
		"kpi":      kpi,
		"aging":    order,
		"invoices": rowsOut,
	})
}
