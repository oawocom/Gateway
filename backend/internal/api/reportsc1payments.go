package api

import (
	"net/http"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/sqldb"
)

// reportsC1Payments — Payments / Collection (doc §6): accrued vs paid by
// month with the collection rate. Query: connection (required), from, to.
func (s *Server) reportsC1Payments(w http.ResponseWriter, r *http.Request) {
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
	q := c1.Query{From: from, To: to, CustomerID: qs.Get("customer")}
	q.AggregatorIDs = s.c1AggIDs(r, connID)

	months, err := ag.Monthly(ctx, q)
	if err != nil {
		errJSON(w, 502, "monthly: "+err.Error())
		return
	}
	nonInv, err := ag.PaidByNonInvoiced(ctx, q)
	if err != nil {
		errJSON(w, 502, "non-invoiced: "+err.Error())
		return
	}

	type mrow struct {
		Year          int      `json:"year"`
		Month         int      `json:"month"`
		Invoiced      float64  `json:"invoiced"`
		Paid          float64  `json:"paid"`
		Gap           float64  `json:"gap"`
		CollectionPct *float64 `json:"collection_pct"`
	}
	rowsOut := make([]mrow, 0, len(months))
	var invoiced, paid float64
	for _, m := range months {
		row := mrow{Year: m.Year, Month: m.Month,
			Invoiced: round2(m.Invoiced), Paid: round2(m.Paid), Gap: round2(m.Invoiced - m.Paid)}
		if m.Invoiced > 0 {
			v := round1(m.Paid / m.Invoiced * 100)
			row.CollectionPct = &v
		}
		rowsOut = append(rowsOut, row)
		invoiced += m.Invoiced
		paid += m.Paid
	}

	kpi := map[string]any{
		"invoiced":           round2(invoiced),
		"paid":               round2(paid),
		"gap":                round2(invoiced - paid),
		"paid_non_invoiced":  round2(nonInv),
	}
	if invoiced > 0 {
		kpi["collection_pct"] = round1(paid / invoiced * 100)
	}

	writeJSON(w, 200, map[string]any{
		"info":    ad.Info(),
		"period":  map[string]any{"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")},
		"kpi":     kpi,
		"monthly": rowsOut,
	})
}
