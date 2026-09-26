package api

import (
	"net/http"
	"sort"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

// reportsC1Customers — Customer Growth & Top Customers (doc §8):
// new/lost/net-growth by month, Top-5/Top-10 revenue concentration and the
// TOP customer table. Query: connection (required), from, to.
func (s *Server) reportsC1Customers(w http.ResponseWriter, r *http.Request) {
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
	q := c1.Query{From: from, To: to}
	q.AggregatorIDs = s.c1AggIDs(r, connID)

	spans, err := ag.CustomerSpans(ctx)
	if err != nil {
		errJSON(w, 502, "spans: "+err.Error())
		return
	}
	dataUntil, err := ag.DataUntil(ctx)
	if err != nil {
		errJSON(w, 502, "data until: "+err.Error())
		return
	}
	// "lost" needs a horizon: a customer whose last invoice falls in the final
	// data month may simply not be billed yet — don't count them as lost.
	lastYM := dataUntil.Year()*12 + int(dataUntil.Month()) - 1

	ym := func(t time.Time) int { return t.Year()*12 + int(t.Month()) - 1 }
	type mrow struct {
		Year int `json:"year"`
		M    int `json:"month"`
		New  int `json:"new"`
		Lost int `json:"lost"`
		Net  int `json:"net"`
	}
	rowsByYM := map[int]*mrow{}
	cur := ym(from)
	for cur < ym(to.AddDate(0, 0, -1))+1 {
		rowsByYM[cur] = &mrow{Year: cur / 12, M: cur%12 + 1}
		cur++
	}
	var newInPeriod, lostInPeriod int
	for _, sp := range spans {
		f, l := ym(sp.First), ym(sp.Last)
		if rw, ok := rowsByYM[f]; ok {
			rw.New++
			newInPeriod++
		}
		if l < lastYM { // churned before the final data month
			if rw, ok := rowsByYM[l]; ok {
				rw.Lost++
				lostInPeriod++
			}
		}
	}
	monthly := make([]*mrow, 0, len(rowsByYM))
	for _, rw := range rowsByYM {
		rw.Net = rw.New - rw.Lost
		monthly = append(monthly, rw)
	}
	sort.Slice(monthly, func(i, j int) bool {
		return monthly[i].Year*12+monthly[i].M < monthly[j].Year*12+monthly[j].M
	})

	// activity + concentration over the selected period
	curAct, _, err := ag.CustomerActivity(ctx, q)
	if err != nil {
		errJSON(w, 502, "activity: "+err.Error())
		return
	}
	topAll, err := ag.TopCustomers(ctx, q)
	if err != nil {
		errJSON(w, 502, "top: "+err.Error())
		return
	}
	var totalInvoiced float64
	for _, t := range topAll {
		totalInvoiced += t.Invoiced
	}
	share := func(n int) float64 {
		var s float64
		for i, t := range topAll {
			if i >= n {
				break
			}
			s += t.Invoiced
		}
		if totalInvoiced == 0 {
			return 0
		}
		return round1(s / totalInvoiced * 100)
	}

	type topRow struct {
		c1.CustomerRow
		SharePct float64 `json:"share_pct"`
		CumPct   float64 `json:"cum_pct"`
	}
	limit := 20
	if len(topAll) < limit {
		limit = len(topAll)
	}
	topOut := make([]topRow, 0, limit)
	var cum float64
	for i := 0; i < limit; i++ {
		t := topAll[i]
		t.Invoiced = round2(t.Invoiced)
		t.Paid = round2(t.Paid)
		var sh float64
		if totalInvoiced > 0 {
			sh = round1(t.Invoiced / totalInvoiced * 100)
		}
		cum += sh
		topOut = append(topOut, topRow{CustomerRow: t, SharePct: sh, CumPct: round1(cum)})
	}

	writeJSON(w, 200, map[string]any{
		"info":   ad.Info(),
		"period": map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")},
		"kpi": map[string]any{
			"active":          len(curAct),
			"total_customers": len(spans),
			"new":             newInPeriod,
			"lost":            lostInPeriod,
			"net_growth":      newInPeriod - lostInPeriod,
			"top5_share_pct":  share(5),
			"top10_share_pct": share(10),
		},
		"monthly": monthly,
		"top":     topOut,
	})
}
