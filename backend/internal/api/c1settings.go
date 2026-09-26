package api

import (
	"encoding/json"
	"net/http"
)

// Per-connection 1C settings, kept as JSONB in the tenant DB. Today it holds
// the payment-channel (aggregator) contragents: payers like "KAPİTAL BANK"
// that collect subscriber money — their payments are B2C inflow, not
// customer advances, and they must not appear in customer reports.

type c1Settings struct {
	Aggregators []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"aggregators"`
}

func (s *Server) loadC1Settings(r *http.Request, connID string) c1Settings {
	var out c1Settings
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		return out
	}
	var raw []byte
	if tdb.QueryRow(r.Context(), `SELECT data FROM c1_settings WHERE connection_id=$1`, connID).Scan(&raw) == nil {
		json.Unmarshal(raw, &out)
	}
	return out
}

// c1AggIDs: aggregator ids for report queries.
func (s *Server) c1AggIDs(r *http.Request, connID string) []string {
	st := s.loadC1Settings(r, connID)
	ids := make([]string, 0, len(st.Aggregators))
	for _, a := range st.Aggregators {
		ids = append(ids, a.ID)
	}
	return ids
}

// GET /reports/c1/settings?connection
func (s *Server) c1SettingsGet(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connection")
	if connID == "" {
		errJSON(w, 400, "connection tələb olunur")
		return
	}
	st := s.loadC1Settings(r, connID)
	if st.Aggregators == nil {
		st.Aggregators = []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{}
	}
	writeJSON(w, 200, st)
}

// PUT /reports/c1/settings?connection
func (s *Server) c1SettingsPut(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connection")
	if connID == "" {
		errJSON(w, 400, "connection tələb olunur")
		return
	}
	var st c1Settings
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	raw, _ := json.Marshal(st)
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	if _, err := tdb.Exec(r.Context(), `
		INSERT INTO c1_settings (connection_id, data) VALUES ($1,$2)
		ON CONFLICT (connection_id) DO UPDATE SET data=EXCLUDED.data, updated_at=now()`,
		connID, raw); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
