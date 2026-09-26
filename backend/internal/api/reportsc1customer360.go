package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

func (s *Server) c1Adapter(w http.ResponseWriter, r *http.Request, connID string) (c1.Adapter, c1.Aggregates, func(), bool) {
	prof, cfg, err := s.loadProfile(r, connID)
	if err != nil {
		errJSON(w, 502, "profil yüklənmədi: "+err.Error())
		return nil, nil, nil, false
	}
	db, err := sqldb.New("mssql", *cfg).Open()
	if err != nil {
		errJSON(w, 502, "1C bazasına qoşulmaq alınmadı: "+err.Error())
		return nil, nil, nil, false
	}
	ad, err := c1.Select(db, prof)
	if err != nil {
		db.Close()
		errJSON(w, 400, err.Error())
		return nil, nil, nil, false
	}
	ag, ok := ad.(c1.Aggregates)
	if !ok {
		db.Close()
		errJSON(w, 400, "bu adapter aqreqat hesabatları dəstəkləmir")
		return nil, nil, nil, false
	}
	return ad, ag, func() { db.Close() }, true
}

// reportsC1CustomerSearch — GET /reports/c1/customersearch?connection&q
// Lightweight name/VOEN search over the customer reference for pickers.
func (s *Server) reportsC1CustomerSearch(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	connID := qs.Get("connection")
	if connID == "" {
		errJSON(w, 400, "connection tələb olunur")
		return
	}
	ad, _, closeDB, ok := s.c1Adapter(w, r, connID)
	if !ok {
		return
	}
	defer closeDB()
	all, err := ad.Customers(r.Context(), c1.Query{})
	if err != nil {
		errJSON(w, 502, err.Error())
		return
	}
	needle := strings.ToLower(strings.TrimSpace(qs.Get("q")))
	type hit struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		VOEN string `json:"voen"`
	}
	out := []hit{}
	for _, c := range all {
		if c.IsGroup || c.Marked {
			continue
		}
		if needle == "" || strings.Contains(strings.ToLower(c.Name), needle) ||
			strings.Contains(strings.ToLower(c.FullName), needle) || strings.Contains(c.VOEN, needle) {
			out = append(out, hit{ID: c.ID, Name: c.Name, VOEN: c.VOEN})
			if len(out) >= 20 {
				break
			}
		}
	}
	writeJSON(w, 200, map[string]any{"customers": out})
}

// reportsC1Customer360 — Customer 360° (doc §9): header, KPIs, payment
// history and the Financial/Contracts/Services/Invoices tabs, all for one
// customer. Query: connection, customer (both required).
func (s *Server) reportsC1Customer360(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	connID, custID := qs.Get("connection"), qs.Get("customer")
	if connID == "" || custID == "" {
		errJSON(w, 400, "connection və customer tələb olunur")
		return
	}
	ad, ag, closeDB, ok := s.c1Adapter(w, r, connID)
	if !ok {
		return
	}
	defer closeDB()
	ctx := r.Context()

	dataUntil, err := ag.DataUntil(ctx)
	if err != nil {
		errJSON(w, 502, "data until: "+err.Error())
		return
	}
	monthStart := time.Date(dataUntil.Year(), dataUntil.Month(), 1, 0, 0, 0, 0, time.UTC)

	cq := c1.Query{CustomerID: custID}

	// header: name/VOEN from the reference, manager from the latest invoice
	custs, err := ad.Customers(ctx, c1.Query{})
	if err != nil {
		errJSON(w, 502, "customers: "+err.Error())
		return
	}
	var name, voen string
	for _, c := range custs {
		if c.ID == custID {
			name, voen = c.Name, c.VOEN
			break
		}
	}
	if name == "" {
		errJSON(w, 404, "müştəri tapılmadı")
		return
	}

	invoices, err := ad.Invoices(ctx, c1.Query{CustomerID: custID, Limit: 200})
	if err != nil {
		errJSON(w, 502, "invoices: "+err.Error())
		return
	}
	manager, firstInv, lastInv := "", time.Time{}, time.Time{}
	if len(invoices) > 0 {
		manager = invoices[0].Manager // newest first
		lastInv = invoices[0].Date
		firstInv = invoices[len(invoices)-1].Date
	}
	// spans give the true first invoice even beyond the 200-row window
	if spans, err := ag.CustomerSpans(ctx); err == nil {
		for _, sp := range spans {
			if sp.CustomerID == custID {
				firstInv, lastInv = sp.First, sp.Last
				break
			}
		}
	}
	status := "Passiv"
	if !lastInv.IsZero() && dataUntil.Sub(lastInv) <= 62*24*time.Hour {
		status = "Aktiv"
	}

	// KPIs
	totalNet, _, err := ag.NetStats(ctx, cq)
	if err != nil {
		errJSON(w, 502, "net: "+err.Error())
		return
	}
	mq := cq
	mq.From, mq.To = monthStart.AddDate(-1, 0, 0), monthStart.AddDate(0, 1, 0)
	monthly, err := ag.Monthly(ctx, mq)
	if err != nil {
		errJSON(w, 502, "monthly: "+err.Error())
		return
	}
	var lastMonthNet, sum12 float64
	for _, m := range monthly {
		sum12 += m.Net
		if m.Year == monthStart.Year() && m.Month == int(monthStart.Month()) {
			lastMonthNet = m.Net
		}
	}
	dueDays, settleRule := s.c1Rules(r)
	openInv, _, err := ag.OpenInvoices(ctx, c1.Query{CustomerID: custID, DefaultDueDays: dueDays, SettleByContract: settleRule}, time.Now().UTC())
	if err != nil {
		errJSON(w, 502, "open: "+err.Error())
		return
	}
	var outstanding, overdue float64
	openByID := map[string]c1.OpenInvoice{}
	for _, o := range openInv {
		outstanding += o.Outstanding
		if o.DaysOverdue > 0 {
			overdue += o.Outstanding
		}
		openByID[o.ID] = o
	}

	// services: active = billed in the last 3 full data months; table over 12m
	sq := cq
	sq.From, sq.To = monthStart.AddDate(0, -2, 0), monthStart.AddDate(0, 1, 0)
	activeSvc, err := ag.RevenueByService(ctx, sq)
	if err != nil {
		errJSON(w, 502, "active services: "+err.Error())
		return
	}
	svc12, err := ag.RevenueByService(ctx, mq)
	if err != nil {
		errJSON(w, 502, "services: "+err.Error())
		return
	}
	activeIDs := map[string]bool{}
	for _, sv := range activeSvc {
		activeIDs[sv.ServiceID] = true
	}
	type svcRow struct {
		c1.ServiceRow
		Active     bool    `json:"active"`
		MonthlyAvg float64 `json:"monthly_avg"`
	}
	svcOut := make([]svcRow, 0, len(svc12))
	for _, sv := range svc12 {
		sv.Net = round2(sv.Net)
		svcOut = append(svcOut, svcRow{ServiceRow: sv, Active: activeIDs[sv.ServiceID], MonthlyAvg: round2(sv.Net / 12)})
	}

	contracts, err := ad.Contracts(ctx, cq)
	if err != nil {
		errJSON(w, 502, "contracts: "+err.Error())
		return
	}

	// invoice tab rows with payment status from the FIFO allocation
	type invRow struct {
		c1.Invoice
		Paid        float64 `json:"paid"`
		Outstanding float64 `json:"outstanding"`
		Status      string  `json:"status"` // paid | partial | open | overdue
	}
	invOut := make([]invRow, 0, len(invoices))
	for _, v := range invoices {
		v.Amount = round2(v.Amount)
		row := invRow{Invoice: v, Paid: v.Amount, Status: "paid"}
		if o, ok := openByID[v.ID]; ok {
			row.Paid = round2(o.Paid)
			row.Outstanding = round2(o.Outstanding)
			if o.DaysOverdue > 0 {
				row.Status = "overdue"
			} else if o.Paid > 0 {
				row.Status = "partial"
			} else {
				row.Status = "open"
			}
		}
		invOut = append(invOut, row)
	}
	sort.Slice(invOut, func(i, j int) bool { return invOut[i].Date.After(invOut[j].Date) })

	activeCount := len(activeIDs)
	writeJSON(w, 200, map[string]any{
		"info": ad.Info(),
		"header": map[string]any{
			"id": custID, "name": name, "voen": voen, "status": status,
			"first_invoice": nilIfZero(firstInv), "last_invoice": nilIfZero(lastInv),
			"manager": manager, "data_until": dataUntil.Format("2006-01-02"),
		},
		"kpi": map[string]any{
			"last_month_net":  round2(lastMonthNet),
			"monthly_avg_net": round2(sum12 / 12),
			"total_net":       round2(totalNet),
			"outstanding":     round2(outstanding),
			"overdue":         round2(overdue),
			"active_services": activeCount,
		},
		"monthly":   monthly,
		"services":  svcOut,
		"contracts": contracts,
		"invoices":  invOut,
	})
}

func nilIfZero(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02")
}
