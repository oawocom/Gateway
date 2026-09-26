package api

import (
	"net/http"
	"sort"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

// reportsC1Dashboard — management dashboard figures straight from the 1C base
// through the version adapter. Query: connection (id, required), from, to
// (YYYY-MM-DD, to exclusive; default = last 12 full months), customer (id),
// due_days (default 30, used when a contract has no payment term).
func (s *Server) reportsC1Dashboard(w http.ResponseWriter, r *http.Request) {
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
	now := time.Now().UTC()
	// informational: the last month with normal activity in the base
	dataUntil, err := ag.DataUntil(ctx)
	if err != nil {
		errJSON(w, 502, "data until: "+err.Error())
		return
	}
	// default period: last 12 calendar months including the current one
	to := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	from := to.AddDate(-1, 0, 0)
	if t, e := time.Parse("2006-01-02", qs.Get("from")); e == nil {
		from = t
	}
	if t, e := time.Parse("2006-01-02", qs.Get("to")); e == nil {
		to = t
	}
	dueDays := 30
	if d, e := parseInt(qs.Get("due_days")); e == nil && d > 0 {
		dueDays = d
	}
	q := c1.Query{From: from, To: to, CustomerID: qs.Get("customer"), DefaultDueDays: dueDays, SettleByContract: qs.Get("settle") == "contract"}

	monthly, err := ag.Monthly(ctx, q)
	if err != nil {
		errJSON(w, 502, "monthly: "+err.Error())
		return
	}
	services, err := ag.RevenueByService(ctx, q)
	if err != nil {
		errJSON(w, 502, "services: "+err.Error())
		return
	}
	tq := q
	tq.Limit = 10
	topCust, err := ag.TopCustomers(ctx, tq)
	if err != nil {
		errJSON(w, 502, "top customers: "+err.Error())
		return
	}
	asOf := now
	if to.Before(now) {
		asOf = to.AddDate(0, 0, -1) // report "as of" the end of the selected period
	}
	open, advances, err := ag.OpenInvoices(ctx, q, asOf)
	if err != nil {
		errJSON(w, 502, "open invoices: "+err.Error())
		return
	}
	// opening balance: same computation as of the day before the period
	openStart, _, err := ag.OpenInvoices(ctx, q, from.AddDate(0, 0, -1))
	if err != nil {
		errJSON(w, 502, "open invoices (start): "+err.Error())
		return
	}
	cur, prev, err := ag.CustomerActivity(ctx, q)
	if err != nil {
		errJSON(w, 502, "customer activity: "+err.Error())
		return
	}

	// ---- KPIs ----
	var invoiced, net, paid float64
	var invCount int64
	for _, m := range monthly {
		invoiced += m.Invoiced
		net += m.Net
		paid += m.Paid
		invCount += m.Invoices
	}
	var outstanding, overdue, advanceTotal float64
	var outstandingStart, overdueStart float64
	for _, o := range openStart {
		outstandingStart += o.Outstanding
		if o.DaysOverdue > 0 {
			overdueStart += o.Outstanding
		}
	}
	// invoices issued inside the period that are still (partly) unpaid
	var outstandingPeriod, overduePeriod float64
	var openPeriodCount int
	buckets := map[string]*bucket{
		"current": {Label: "Vaxtı çatmayıb"}, "1_30": {Label: "1–30 gün"}, "31_60": {Label: "31–60 gün"},
		"61_90": {Label: "61–90 gün"}, "90_plus": {Label: "90+ gün"},
	}
	byCust := map[string]*overdueCust{}
	for _, o := range open {
		outstanding += o.Outstanding
		if !o.Date.Before(from) && o.Date.Before(to) {
			outstandingPeriod += o.Outstanding
			openPeriodCount++
			if o.DaysOverdue > 0 {
				overduePeriod += o.Outstanding
			}
		}
		k := "current"
		switch {
		case o.DaysOverdue <= 0:
		case o.DaysOverdue <= 30:
			k = "1_30"
		case o.DaysOverdue <= 60:
			k = "31_60"
		case o.DaysOverdue <= 90:
			k = "61_90"
		default:
			k = "90_plus"
		}
		buckets[k].Amount += o.Outstanding
		buckets[k].Count++
		if o.DaysOverdue > 0 {
			overdue += o.Outstanding
			c := byCust[o.CustomerID]
			if c == nil {
				c = &overdueCust{CustomerID: o.CustomerID, Customer: o.Customer}
				byCust[o.CustomerID] = c
			}
			c.Overdue += o.Outstanding
			c.Invoices++
			if o.DaysOverdue > c.MaxDays {
				c.MaxDays = o.DaysOverdue
			}
		}
	}
	var nonInvoiced float64
	realAdvances := []c1.Advance{}
	otherPayers := []c1.Advance{}
	for _, a := range advances {
		if a.NonInvoiced {
			nonInvoiced += a.Amount
			otherPayers = append(otherPayers, a)
		} else {
			advanceTotal += a.Amount
			realAdvances = append(realAdvances, a)
		}
	}
	// payments in the period from payers that have no invoices at all
	paidOther, err := ag.PaidByNonInvoiced(ctx, q)
	if err != nil {
		errJSON(w, 502, "non-invoiced payments: "+err.Error())
		return
	}
	paidInvoiced := paid - paidOther
	topOverdue := make([]*overdueCust, 0, len(byCust))
	for _, c := range byCust {
		topOverdue = append(topOverdue, c)
	}
	sort.Slice(topOverdue, func(i, j int) bool { return topOverdue[i].Overdue > topOverdue[j].Overdue })
	if len(topOverdue) > 10 {
		topOverdue = topOverdue[:10]
	}
	newC, lostC := 0, 0
	for c := range cur {
		if !prev[c] {
			newC++
		}
	}
	for c := range prev {
		if !cur[c] {
			lostC++
		}
	}
	collection := 0.0
	if invoiced > 0 {
		collection = paidInvoiced / invoiced * 100
	}
	overdueRatio := 0.0
	if outstanding > 0 {
		overdueRatio = overdue / outstanding * 100
	}

	writeJSON(w, 200, map[string]any{
		"info":   ad.Info(),
		"period": map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "as_of": asOf.Format("2006-01-02"), "data_until": dataUntil.Format("2006-01-02"), "due_days_default": dueDays, "settle_by": map[bool]string{false: "customer", true: "contract"}[q.SettleByContract]},
		"kpi": map[string]any{
			"invoiced":             round2(invoiced),     // hesablanmış (ƏDV daxil)
			"revenue_net":          round2(net),          // gəlir, ƏDV-siz
			"paid":                 round2(paid),         // ümumi daxilolma (dövrdə)
			"paid_invoiced":        round2(paidInvoiced), // invoice-lu müştərilərdən
			"paid_non_invoiced":    round2(paidOther),    // invoice-suz daxilolma (ödəniş sistemləri, B2C)
			"collection_rate":      round1(collection),   // paid_invoiced / invoiced
			"outstanding":          round2(outstanding),  // qalıq — bütün açıq invoice-lar (as_of)
			"overdue":              round2(overdue),
			"overdue_ratio":        round1(overdueRatio),
			"outstanding_start":    round2(outstandingStart), // eyni göstərici dövr əvvəlinə
			"overdue_start":        round2(overdueStart),
			"outstanding_period":   round2(outstandingPeriod), // dövrdə hesablanıb, hələ ödənilməyib
			"overdue_period":       round2(overduePeriod),
			"open_period":          openPeriodCount,
			"advances":             round2(advanceTotal), // invoice-lu müştərilərin artıq ödəməsi (real avans)
			"non_invoiced_balance": round2(nonInvoiced),  // invoice-suz ödəyicilərin tarixi cəmi
			"invoices":             invCount,
			"active_customers":     len(cur),
			"new_customers":        newC,
			"lost_customers":       lostC,
			"open_invoices":        len(open),
		},
		"monthly":       monthly,
		"services":      services,
		"top_customers": topCust,
		"aging":         []*bucket{buckets["current"], buckets["1_30"], buckets["31_60"], buckets["61_90"], buckets["90_plus"]},
		"top_overdue":   topOverdue,
		"advances":      realAdvances,
		"other_payers":  otherPayers,
	})
}

type bucket struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

type overdueCust struct {
	CustomerID string  `json:"customer_id"`
	Customer   string  `json:"customer"`
	Overdue    float64 `json:"overdue"`
	Invoices   int     `json:"invoices"`
	MaxDays    int     `json:"max_days"`
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

func parseInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, errEmpty
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errEmpty
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

var errEmpty = errString("empty")

type errString string

func (e errString) Error() string { return string(e) }
