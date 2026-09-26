package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"gateway/internal/auth"
	"gateway/internal/connectors/c1"
)

// Tenant-level settings, stored in the tenant DB "settings" key/value table.
// Known keys:
//   report_rules — {"default_due_days": int, "settle": "customer"|"contract"}

var knownSettingKeys = map[string]bool{"report_rules": true}

// GET /settings — all settings as {key: value}.
func (s *Server) settingsGet(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(), `SELECT key, value FROM settings`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v []byte
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	writeJSON(w, 200, out)
}

// PUT /settings/{key} — owner/admin; body is the JSON value.
func (s *Server) settingsPut(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !knownSettingKeys[key] {
		errJSON(w, 400, "naməlum parametr açarı")
		return
	}
	var v json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	if _, err := tdb.Exec(r.Context(), `
		INSERT INTO settings (key, value) VALUES ($1,$2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, []byte(v)); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// c1Rules loads report_rules and applies them to a base query. Handlers call
// this instead of hardcoding due days / settle mode; explicit query params
// still win on pages that expose the choice.
func (s *Server) c1Rules(r *http.Request) (dueDays int, settleByContract bool) {
	dueDays = 14
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		return
	}
	var raw []byte
	if tdb.QueryRow(r.Context(), `SELECT value FROM settings WHERE key='report_rules'`).Scan(&raw) != nil {
		return
	}
	var v struct {
		DefaultDueDays int    `json:"default_due_days"`
		Settle         string `json:"settle"`
	}
	if json.Unmarshal(raw, &v) == nil {
		if v.DefaultDueDays >= 0 && v.DefaultDueDays <= 365 {
			dueDays = v.DefaultDueDays
		}
		settleByContract = v.Settle == "contract"
	}
	return
}

// applyC1Rules fills rule-driven fields unless the request overrides them.
func (s *Server) applyC1Rules(r *http.Request, q *c1.Query) {
	due, settle := s.c1Rules(r)
	qs := r.URL.Query()
	if qs.Get("due") == "" {
		q.DefaultDueDays = due
	}
	if qs.Get("settle") == "" {
		q.SettleByContract = settle
	}
}

// PUT /tenant/name — owner only: rename the workspace (master DB).
func (s *Server) tenantRename(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(*auth.Claims)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) < 2 || len(req.Name) > 80 {
		errJSON(w, 400, "ad 2-80 simvol olmalıdır")
		return
	}
	if _, err := s.Master.Exec(r.Context(),
		"UPDATE tenants SET name=$1 WHERE id=$2", req.Name, claims.TenantID); err != nil {
		errJSON(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]string{"name": req.Name})
}
