package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) dataSummary(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(), `
		SELECT se.connection_id, c.name, se.entity_name, se.record_count, se.last_synced_at
		FROM synced_entities se JOIN connections c ON c.id = se.connection_id
		ORDER BY c.name, se.entity_name`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	type row struct {
		ConnectionID   string     `json:"connection_id"`
		ConnectionName string     `json:"connection_name"`
		EntityName     string     `json:"entity_name"`
		RecordCount    int64      `json:"record_count"`
		LastSyncedAt   *time.Time `json:"last_synced_at"`
	}
	list := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ConnectionID, &x.ConnectionName, &x.EntityName, &x.RecordCount, &x.LastSyncedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"entities": list})
}

func (s *Server) dataRecords(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connection_id")
	entity := r.URL.Query().Get("entity")
	if connID == "" || entity == "" {
		errJSON(w, 400, "connection_id and entity required")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const per = 50
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	var total int64
	tdb.QueryRow(r.Context(),
		"SELECT count(*) FROM records WHERE connection_id=$1 AND entity_name=$2", connID, entity).Scan(&total)

	rows, err := tdb.Query(r.Context(), `
		SELECT external_id, data, synced_at FROM records
		WHERE connection_id=$1 AND entity_name=$2
		ORDER BY id DESC LIMIT $3 OFFSET $4`, connID, entity, per, (page-1)*per)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	type rec struct {
		ExternalID string          `json:"external_id"`
		Data       json.RawMessage `json:"data"`
		SyncedAt   time.Time       `json:"synced_at"`
	}
	list := []rec{}
	for rows.Next() {
		var x rec
		if err := rows.Scan(&x.ExternalID, &x.Data, &x.SyncedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"records": list, "total": total, "page": page, "per_page": per})
}

func (s *Server) syncLog(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(), `
		SELECT sl.id, coalesce(c.name,''), coalesce(sl.entity_name,''), sl.status, coalesce(sl.message,''), sl.records_synced, sl.created_at
		FROM sync_log sl LEFT JOIN connections c ON c.id = sl.connection_id
		ORDER BY sl.id DESC LIMIT 100`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	type row struct {
		ID             int64     `json:"id"`
		ConnectionName string    `json:"connection_name"`
		EntityName     string    `json:"entity_name"`
		Status         string    `json:"status"`
		Message        string    `json:"message"`
		RecordsSynced  int64     `json:"records_synced"`
		CreatedAt      time.Time `json:"created_at"`
	}
	list := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.ConnectionName, &x.EntityName, &x.Status, &x.Message, &x.RecordsSynced, &x.CreatedAt); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"log": list})
}

// ---------------- automations ----------------

type automationRow struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	ConnectionID    string     `json:"connection_id"`
	ConnectionName  string     `json:"connection_name"`
	EntityName      string     `json:"entity_name"`
	IntervalMinutes int        `json:"interval_minutes"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at"`
	LastStatus      *string    `json:"last_status"`
}

func (s *Server) listAutomations(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	rows, err := tdb.Query(r.Context(), `
		SELECT a.id, a.name, a.connection_id, c.name, a.entity_name, a.interval_minutes, a.enabled, a.last_run_at, a.last_status
		FROM automations a JOIN connections c ON c.id = a.connection_id
		ORDER BY a.created_at DESC`)
	if err != nil {
		errJSON(w, 500, "db error")
		return
	}
	defer rows.Close()
	list := []automationRow{}
	for rows.Next() {
		var x automationRow
		if err := rows.Scan(&x.ID, &x.Name, &x.ConnectionID, &x.ConnectionName, &x.EntityName, &x.IntervalMinutes, &x.Enabled, &x.LastRunAt, &x.LastStatus); err != nil {
			errJSON(w, 500, "db error")
			return
		}
		list = append(list, x)
	}
	writeJSON(w, 200, map[string]any{"automations": list})
}

func (s *Server) createAutomation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		ConnectionID    string `json:"connection_id"`
		EntityName      string `json:"entity_name"`
		IntervalMinutes int    `json:"interval_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.ConnectionID == "" || strings.TrimSpace(req.EntityName) == "" || req.IntervalMinutes < 5 {
		errJSON(w, 400, "ad, bağlantı, entity və interval (min 5 dəq) tələb olunur")
		return
	}
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	var id string
	err = tdb.QueryRow(r.Context(), `
		INSERT INTO automations (name, connection_id, entity_name, interval_minutes)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Name, req.ConnectionID, req.EntityName, req.IntervalMinutes).Scan(&id)
	if err != nil {
		errJSON(w, 500, "could not create automation")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) updateAutomation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled         *bool `json:"enabled"`
		IntervalMinutes *int  `json:"interval_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid json")
		return
	}
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	if req.Enabled != nil {
		tdb.Exec(r.Context(), "UPDATE automations SET enabled=$1 WHERE id=$2", *req.Enabled, id)
	}
	if req.IntervalMinutes != nil && *req.IntervalMinutes >= 5 {
		tdb.Exec(r.Context(), "UPDATE automations SET interval_minutes=$1 WHERE id=$2", *req.IntervalMinutes, id)
	}
	writeJSON(w, 200, map[string]string{"id": id})
}

func (s *Server) deleteAutomation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	tag, err := tdb.Exec(r.Context(), "DELETE FROM automations WHERE id=$1", id)
	if err != nil || tag.RowsAffected() == 0 {
		errJSON(w, 404, "automation not found")
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "deleted": "true"})
}

// ---------------- stats ----------------

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	ctx := r.Context()
	var connections, automations, syncsThisMonth, errorsThisMonth, totalRecords int64
	tdb.QueryRow(ctx, "SELECT count(*) FROM connections").Scan(&connections)
	tdb.QueryRow(ctx, "SELECT count(*) FROM automations WHERE enabled").Scan(&automations)
	tdb.QueryRow(ctx, "SELECT count(*) FROM sync_log WHERE created_at >= date_trunc('month', now())").Scan(&syncsThisMonth)
	tdb.QueryRow(ctx, "SELECT count(*) FROM sync_log WHERE status='error' AND created_at >= date_trunc('month', now())").Scan(&errorsThisMonth)
	tdb.QueryRow(ctx, "SELECT count(*) FROM records").Scan(&totalRecords)
	writeJSON(w, 200, map[string]int64{
		"connections":       connections,
		"automations":       automations,
		"syncs_this_month":  syncsThisMonth,
		"errors_this_month": errorsThisMonth,
		"total_records":     totalRecords,
	})
}
