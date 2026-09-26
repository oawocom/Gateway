package api

import (
	"net/http"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

// reportsC1Revenue — Revenue Analytics (doc §4): yearly comparison, service
// share, MoM/YoY KPIs and the detailed service table with previous-period
// comparison. Query: connection (required), from, to, customer, service,
// manager, contract.
func (s *Server) reportsC1Revenue(w http.ResponseWriter, r *http.Request) {
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
	to := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	from := to.AddDate(-1, 0, 0)
	if t, e := time.Parse("2006-01-02", qs.Get("from")); e == nil {
		from = t
	}
	if t, e := time.Parse("2006-01-02", qs.Get("to")); e == nil {
		to = t
	}
	q := c1.Query{
		From: from, To: to,
		CustomerID: qs.Get("customer"),
		ServiceID:  qs.Get("service"),
		ManagerID:  qs.Get("manager"),
		ContractID: qs.Get("contract"),
	}
	q.AggregatorIDs = s.c1AggIDs(r, connID)

	years, err := ag.YearlyRevenue(ctx, q)
	if err != nil {
		errJSON(w, 502, "yearly: "+err.Error())
		return
	}
	monthly, err := ag.NetByMonth(ctx, q)
	if err != nil {
		errJSON(w, 502, "monthly: "+err.Error())
		return
	}
	net, customers, err := ag.NetStats(ctx, q)
	if err != nil {
		errJSON(w, 502, "stats: "+err.Error())
		return
	}
	// previous equal-length period and same period a year earlier
	length := to.Sub(from)
	pq := q
	pq.From, pq.To = from.Add(-length), from
	prevNet, _, err := ag.NetStats(ctx, pq)
	if err != nil {
		errJSON(w, 502, "prev stats: "+err.Error())
		return
	}
	yq := q
	yq.From, yq.To = from.AddDate(-1, 0, 0), to.AddDate(-1, 0, 0)
	yoyNet, _, err := ag.NetStats(ctx, yq)
	if err != nil {
		errJSON(w, 502, "yoy stats: "+err.Error())
		return
	}

	services, err := ag.RevenueByService(ctx, q)
	if err != nil {
		errJSON(w, 502, "services: "+err.Error())
		return
	}
	prevServices, err := ag.RevenueByService(ctx, pq)
	if err != nil {
		errJSON(w, 502, "prev services: "+err.Error())
		return
	}
	prevBySvc := map[string]float64{}
	for _, p := range prevServices {
		prevBySvc[p.ServiceID] = p.Net
	}
	type svcRow struct {
		c1.ServiceRow
		PrevNet   float64  `json:"prev_net"`
		ChangePct *float64 `json:"change_pct"` // nil when no previous base
		SharePct  float64  `json:"share_pct"`
		AvgPerCst float64  `json:"avg_per_customer"`
	}
	svcOut := make([]svcRow, 0, len(services))
	for _, sv := range services {
		row := svcRow{ServiceRow: sv, PrevNet: round2(prevBySvc[sv.ServiceID])}
		if net > 0 {
			row.SharePct = round1(sv.Net / net * 100)
		}
		if sv.Customers > 0 {
			row.AvgPerCst = round2(sv.Net / float64(sv.Customers))
		}
		if p := prevBySvc[sv.ServiceID]; p != 0 {
			v := round1((sv.Net - p) / p * 100)
			row.ChangePct = &v
		}
		svcOut = append(svcOut, row)
	}

	// filter dropdown options: all services ever billed + managers
	oq := c1.Query{}
	allSvc, err := ag.RevenueByService(ctx, oq)
	if err != nil {
		errJSON(w, 502, "service options: "+err.Error())
		return
	}
	svcOpts := make([]c1.Ref, 0, len(allSvc))
	for _, sv := range allSvc {
		name := sv.Service
		if sv.Category != "" {
			name = sv.Category + " › " + sv.Service
		}
		svcOpts = append(svcOpts, c1.Ref{ID: sv.ServiceID, Name: name})
	}
	managers, err := ag.Managers(ctx)
	if err != nil {
		errJSON(w, 502, "managers: "+err.Error())
		return
	}

	mom := momPct(monthly)
	kpi := map[string]any{
		"net":              round2(net),
		"prev_net":         round2(prevNet),
		"yoy_net":          round2(yoyNet),
		"active_customers": customers,
	}
	if prevNet != 0 {
		kpi["period_growth_pct"] = round1((net - prevNet) / prevNet * 100)
	}
	if yoyNet != 0 {
		kpi["yoy_pct"] = round1((net - yoyNet) / yoyNet * 100)
	}
	if mom != nil {
		kpi["mom_pct"] = *mom
	}
	if customers > 0 {
		kpi["avg_per_customer"] = round2(net / float64(customers))
	}

	writeJSON(w, 200, map[string]any{
		"info":     ad.Info(),
		"period":   map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")},
		"kpi":      kpi,
		"years":    years,
		"monthly":  monthly,
		"services": svcOut,
		"options":  map[string]any{"services": svcOpts, "managers": managers},
	})
}

// momPct: last full month vs the month before it, from the in-period series.
func momPct(m []c1.MonthNet) *float64 {
	if len(m) < 2 {
		return nil
	}
	a, b := m[len(m)-2].Net, m[len(m)-1].Net
	if a == 0 {
		return nil
	}
	v := round1((b - a) / a * 100)
	return &v
}
