package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// reportsBankBalances — GET /reports/bank/balances: account balances from the
// synced "accounts" records of bank connections (PASHA, Kapital). No live
// bank call — the dashboard reads what the last sync brought.
func (s *Server) reportsBankBalances(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(), `
		SELECT c.id, c.name, c.connector_type, r.data, r.synced_at
		FROM records r JOIN connections c ON c.id = r.connection_id
		WHERE c.connector_type IN ('pasha_bank','kapital_bank') AND r.entity_name = 'accounts'
		ORDER BY c.name`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()

	str := func(m map[string]any, keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	num := func(m map[string]any, keys ...string) (float64, bool) {
		for _, k := range keys {
			switch v := m[k].(type) {
			case float64:
				return v, true
			case string:
				var f float64
				if json.Unmarshal([]byte(v), &f) == nil {
					return f, true
				}
			}
		}
		return 0, false
	}

	type acc struct {
		Connection string    `json:"connection"`
		Bank       string    `json:"bank"`
		Name       string    `json:"name"`
		IBAN       string    `json:"iban"`
		Currency   string    `json:"currency"`
		Balance    float64   `json:"balance"`
		SyncedAt   time.Time `json:"synced_at"`
	}
	accounts := []acc{}
	totals := map[string]float64{}
	var lastSync time.Time
	for rows.Next() {
		var connID, connName, ctype string
		var raw []byte
		var syncedAt time.Time
		if rows.Scan(&connID, &connName, &ctype, &raw, &syncedAt) != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		bal, ok := num(m, "availableBalance", "currentBalance", "balance", "amount")
		if !ok {
			continue
		}
		cur := str(m, "currency", "currencyCode", "ccy")
		if cur == "" {
			cur = "AZN"
		}
		a := acc{
			Connection: connName,
			Bank:       map[string]string{"pasha_bank": "PASHA Bank", "kapital_bank": "Kapital Bank"}[ctype],
			Name:       str(m, "accountName", "name", "title"),
			IBAN:       str(m, "iban", "accountNo", "accountNumber"),
			Currency:   cur,
			Balance:    round2(bal),
			SyncedAt:   syncedAt,
		}
		accounts = append(accounts, a)
		totals[cur] += bal
		if syncedAt.After(lastSync) {
			lastSync = syncedAt
		}
	}
	for k, v := range totals {
		totals[k] = round2(v)
	}
	out := map[string]any{"accounts": accounts, "totals": totals}
	if !lastSync.IsZero() {
		out["last_sync"] = lastSync
	}
	writeJSON(w, 200, out)
}
