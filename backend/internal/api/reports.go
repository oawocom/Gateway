package api

import (
	"net/http"
)

type reportEntity struct {
	Connection string `json:"connection"`
	Entity     string `json:"entity"`
	Records    int64  `json:"records"`
}

type reportDay struct {
	Day     string `json:"day"`
	Records int64  `json:"records"`
	Syncs   int64  `json:"syncs"`
	Errors  int64  `json:"errors"`
}

type reportConn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	LastSync string `json:"last_sync"`
	LastErr  string `json:"last_error"`
}

// reportsSummary aggregates synced data for the Reports page.
func (s *Server) reportsSummary(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	ctx := r.Context()

	entities := []reportEntity{}
	rows, err := tdb.Query(ctx, `
		SELECT c.name, se.entity_name, se.record_count
		FROM synced_entities se JOIN connections c ON c.id = se.connection_id
		ORDER BY se.record_count DESC LIMIT 20`)
	if err == nil {
		for rows.Next() {
			var e reportEntity
			if rows.Scan(&e.Connection, &e.Entity, &e.Records) == nil {
				entities = append(entities, e)
			}
		}
		rows.Close()
	}

	daily := []reportDay{}
	rows, err = tdb.Query(ctx, `
		SELECT to_char(created_at::date, 'YYYY-MM-DD'),
		       coalesce(sum(records_synced),0),
		       count(*),
		       count(*) FILTER (WHERE status = 'error')
		FROM sync_log
		WHERE created_at > now() - interval '14 days'
		GROUP BY 1 ORDER BY 1`)
	if err == nil {
		for rows.Next() {
			var d reportDay
			if rows.Scan(&d.Day, &d.Records, &d.Syncs, &d.Errors) == nil {
				daily = append(daily, d)
			}
		}
		rows.Close()
	}

	conns := []reportConn{}
	rows, err = tdb.Query(ctx, `
		SELECT name, connector_type, status,
		       coalesce(to_char(last_sync_at, 'YYYY-MM-DD HH24:MI'), ''),
		       coalesce(last_error, '')
		FROM connections ORDER BY name`)
	if err == nil {
		for rows.Next() {
			var c reportConn
			if rows.Scan(&c.Name, &c.Type, &c.Status, &c.LastSync, &c.LastErr) == nil {
				conns = append(conns, c)
			}
		}
		rows.Close()
	}

	writeJSON(w, 200, map[string]any{
		"entities":    entities,
		"daily":       daily,
		"connections": conns,
	})
}
