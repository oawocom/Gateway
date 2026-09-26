package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"gateway/internal/connectors/sqldb"
)

type monthVal struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Value float64 `json:"value"`
}

type monthProfit struct {
	Year    int     `json:"year"`
	Month   int     `json:"month"`
	Revenue float64 `json:"revenue"`
	Cost    float64 `json:"cost"`
	Profit  float64 `json:"profit"`
}

type monthCash struct {
	Year    int     `json:"year"`
	Month   int     `json:"month"`
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
}

var (
	reAccTable   = regexp.MustCompile(`^_Acc[0-9]+$`)
	reAccRgTable = regexp.MustCompile(`^_AccRg[0-9]+$`)
	reFldPlain   = regexp.MustCompile(`^_Fld([0-9]+)$`)
	reIdent      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// detect1C finds the accounts table (chart containing code '601'),
// the main accounting register and its amount column.
func detect1C(db *sql.DB) (accTable, rgTable, amtCol string, err error) {
	rows, err := db.Query(`SELECT name FROM sys.tables WHERE name LIKE '\_Acc%' ESCAPE '\'`)
	if err != nil {
		return "", "", "", err
	}
	var accs, rgs []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			if reAccTable.MatchString(n) {
				accs = append(accs, n)
			} else if reAccRgTable.MatchString(n) {
				rgs = append(rgs, n)
			}
		}
	}
	rows.Close()
	if len(accs) == 0 || len(rgs) == 0 {
		return "", "", "", fmt.Errorf("1C mühasibat strukturu tapılmadı")
	}
	// main register: has _AccountDtRRef
	for _, rg := range rgs {
		var n int
		q := `SELECT count(*) FROM sys.columns WHERE object_id=OBJECT_ID(@p1) AND name='_AccountDtRRef'`
		if db.QueryRow(q, rg).Scan(&n) == nil && n > 0 {
			rgTable = rg
			break
		}
	}
	if rgTable == "" {
		return "", "", "", fmt.Errorf("mühasibat registri tapılmadı")
	}
	// accounts table: contains code '601' AND actually joins with the register
	// (configs may hold several charts of accounts, e.g. _Acc18 and _Acc19)
	for _, a := range accs {
		var n int
		if db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM [%s] WHERE _Code='601'`, a)).Scan(&n) != nil || n == 0 {
			continue
		}
		q := fmt.Sprintf(`SELECT count(*) FROM (SELECT TOP 1 r._Period FROM [%s] r JOIN [%s] a ON a._IDRRef = r._AccountDtRRef) x`, rgTable, a)
		if db.QueryRow(q).Scan(&n) == nil && n > 0 {
			accTable = a
			break
		}
	}
	if accTable == "" {
		return "", "", "", fmt.Errorf("registrlə uyğun hesablar planı tapılmadı")
	}
	// amount column: lowest-numbered plain _FldNNN numeric column
	rows, err = db.Query(`SELECT c.name FROM sys.columns c
		JOIN sys.types t ON t.user_type_id = c.user_type_id
		WHERE c.object_id = OBJECT_ID(@p1) AND t.name IN ('numeric','decimal','money')`, rgTable)
	if err != nil {
		return "", "", "", err
	}
	best := 1 << 30
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			if m := reFldPlain.FindStringSubmatch(n); m != nil {
				if v, _ := strconv.Atoi(m[1]); v < best {
					best, amtCol = v, n
				}
			}
		}
	}
	rows.Close()
	if amtCol == "" {
		return "", "", "", fmt.Errorf("məbləğ sütunu tapılmadı")
	}
	return accTable, rgTable, amtCol, nil
}

func scanMonths(rows *sql.Rows) []monthVal {
	out := []monthVal{}
	for rows.Next() {
		var v monthVal
		if rows.Scan(&v.Year, &v.Month, &v.Value) == nil {
			out = append(out, v)
		}
	}
	rows.Close()
	return out
}

func (s *Server) reports1C(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	months := 12
	if m, err := strconv.Atoi(r.URL.Query().Get("months")); err == nil && m >= 3 && m <= 36 {
		months = m
	}
	rows, err := tdb.Query(r.Context(),
		`SELECT name, config_enc FROM connections WHERE connector_type='mssql' ORDER BY created_at`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	type cand struct{ name, enc string }
	var cands []cand
	for rows.Next() {
		var c cand
		if rows.Scan(&c.name, &c.enc) == nil {
			cands = append(cands, c)
		}
	}
	rows.Close()

	// months (1C stores dates with +2000 year offset). GETDATE arithmetic
	// works because both sides carry the same offset base within data range;
	// we additionally shift the cutoff into the offset calendar:
	for _, c := range cands {
		var cfg sqldb.Config
		if s.Sync.LoadConfig(c.enc, &cfg) != nil {
			continue
		}
		db, err := sqldb.New("mssql", cfg).Open()
		if err != nil {
			continue
		}
		acc, rg, amt, derr := detect1C(db)
		if derr != nil {
			db.Close()
			continue
		}
		sales, e1 := monthlyOffset(db, acc, rg, amt, "ct", []string{"601"}, months)
		cost, e2 := monthlyOffset(db, acc, rg, amt, "dt", []string{"701"}, months)
		cashIn, e3 := monthlyOffset(db, acc, rg, amt, "dt", []string{"221", "223"}, months)
		cashOut, e4 := monthlyOffset(db, acc, rg, amt, "ct", []string{"221", "223"}, months)
		recvDt, e5 := monthlyOffset(db, acc, rg, amt, "dt", []string{"211"}, 240)
		recvCt, e6 := monthlyOffset(db, acc, rg, amt, "ct", []string{"211"}, 240)
		db.Close()
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil {
			continue
		}

		key := func(v monthVal) int { return v.Year*100 + v.Month }
		cm := map[int]float64{}
		for _, v := range cost {
			cm[key(v)] = v.Value
		}
		profit := make([]monthProfit, 0, len(sales))
		for _, v := range sales {
			profit = append(profit, monthProfit{v.Year, v.Month, v.Value, cm[key(v)], v.Value - cm[key(v)]})
		}

		om := map[int]float64{}
		allKeys := map[int]monthVal{}
		for _, v := range cashOut {
			om[key(v)] = v.Value
		}
		for _, v := range cashIn {
			allKeys[key(v)] = v
		}
		for _, v := range cashOut {
			if _, ok := allKeys[key(v)]; !ok {
				allKeys[key(v)] = monthVal{v.Year, v.Month, 0}
			}
		}
		im := map[int]float64{}
		for _, v := range cashIn {
			im[key(v)] = v.Value
		}
		cash := []monthCash{}
		keys := make([]int, 0, len(allKeys))
		for k := range allKeys {
			keys = append(keys, k)
		}
		sortInts(keys)
		for _, k := range keys {
			v := allKeys[k]
			cash = append(cash, monthCash{v.Year, v.Month, im[k], om[k]})
		}

		// receivables: cumulative balance of 211 (Dt - Ct), last N months
		bm := map[int]float64{}
		for _, v := range recvDt {
			bm[key(v)] += v.Value
		}
		for _, v := range recvCt {
			bm[key(v)] -= v.Value
		}
		bkeys := make([]int, 0, len(bm))
		for k := range bm {
			bkeys = append(bkeys, k)
		}
		sortInts(bkeys)
		recv := []monthVal{}
		run := 0.0
		for _, k := range bkeys {
			run += bm[k]
			recv = append(recv, monthVal{k / 100, k % 100, run})
		}
		if len(recv) > months {
			recv = recv[len(recv)-months:]
		}

		writeJSON(w, 200, map[string]any{
			"available":   true,
			"connection":  c.name,
			"sales":       sales,
			"profit":      profit,
			"cash":        cash,
			"receivables": recv,
		})
		return
	}
	writeJSON(w, 200, map[string]any{"available": false})
}

// monthlyOffset wraps monthlyTurnover with the 1C +2000-year date offset:
// the cutoff is computed in the shifted calendar.
func monthlyOffset(db *sql.DB, acc, rg, amt, side string, codes []string, months int) ([]monthVal, error) {
	if !reIdent.MatchString(acc) || !reIdent.MatchString(rg) || !reIdent.MatchString(amt) {
		return nil, fmt.Errorf("bad ident")
	}
	col := "_AccountDtRRef"
	if side == "ct" {
		col = "_AccountCtRRef"
	}
	cond := ""
	for i, c := range codes {
		if i > 0 {
			cond += " OR "
		}
		cond += "a._Code LIKE '" + regexp.MustCompile(`[^0-9.]`).ReplaceAllString(c, "") + "%'"
	}
	// last N months of AVAILABLE data (not calendar): robust for stale bases
	q := fmt.Sprintf(`
		SELECT TOP %d YEAR(r._Period)-2000, MONTH(r._Period), coalesce(SUM(r.[%s]),0)
		FROM [%s] r JOIN [%s] a ON a._IDRRef = r.[%s]
		WHERE (%s) AND r._Active = 0x01 AND r._Period >= '2000-01-01'
		GROUP BY YEAR(r._Period), MONTH(r._Period)
		ORDER BY 1 DESC, 2 DESC`, months, amt, rg, acc, col, cond)
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	out := scanMonths(rows)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
