package api

import (
	"net/http"

	"gateway/internal/connectors/c1"
)

type c1MenuItem struct {
	ConnectionID string         `json:"connection_id"`
	Connection   string         `json:"connection"`
	Adapter      string         `json:"adapter"`
	ConfigName   string         `json:"config_name"`
	Reports      []c1.ReportDef `json:"reports"`
}

// reportsC1Menu lists 1C connections whose configuration has a supported
// adapter, together with the reports that adapter provides. The sidebar
// menu is built from this — reports appear only after a 1C base is
// connected and recognized.
func (s *Server) reportsC1Menu(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(),
		`SELECT id, name FROM connections WHERE connector_type = 'mssql' ORDER BY created_at`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	type conn struct{ id, name string }
	var conns []conn
	for rows.Next() {
		var c conn
		if rows.Scan(&c.id, &c.name) == nil {
			conns = append(conns, c)
		}
	}
	rows.Close()

	items := []c1MenuItem{}
	for _, c := range conns {
		prof, _, err := s.loadProfile(r, c.id)
		if err != nil {
			continue // unreachable base or not a 1C base — no menu entry
		}
		name, reports, ok := c1.Match(prof)
		if !ok {
			continue
		}
		items = append(items, c1MenuItem{
			ConnectionID: c.id,
			Connection:   c.name,
			Adapter:      name,
			ConfigName:   prof.ConfigName,
			Reports:      reports,
		})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
