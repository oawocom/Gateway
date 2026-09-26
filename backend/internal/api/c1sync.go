package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gateway/internal/connectors/c1"
	"gateway/internal/connectors/c1meta"
	"gateway/internal/connectors/sqldb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- background full-sync jobs for recognized 1C connections ----
// One job per connection; the UI polls the state for a step progress bar,
// so the customer never picks raw 1C tables and never blocks on a request.

type c1Step struct {
	Entity  string `json:"entity"`
	Status  string `json:"status"` // pending | running | done | error
	Records int    `json:"records"`
	Message string `json:"message,omitempty"`
}

type c1Job struct {
	mu      sync.Mutex
	status  string // running | done | error
	steps   []c1Step
	started time.Time
}

var (
	c1JobsMu sync.Mutex
	c1Jobs   = map[string]*c1Job{}
)

func (j *c1Job) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	steps := make([]c1Step, len(j.steps))
	copy(steps, j.steps)
	done := 0
	for _, st := range steps {
		if st.Status == "done" || st.Status == "error" {
			done++
		}
	}
	pct := 0
	if len(steps) > 0 {
		pct = done * 100 / len(steps)
	}
	return map[string]any{"status": j.status, "steps": steps, "percent": pct, "started_at": j.started}
}

func (j *c1Job) set(i int, status string, records int, msg string) {
	j.mu.Lock()
	j.steps[i].Status = status
	j.steps[i].Records = records
	j.steps[i].Message = msg
	j.mu.Unlock()
}

func (j *c1Job) finish(status string) {
	j.mu.Lock()
	j.status = status
	j.mu.Unlock()
}

// c1SyncStart — POST /connections/{id}/c1sync: kicks off the background job.
func (s *Server) c1SyncStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c1JobsMu.Lock()
	if j, ok := c1Jobs[id]; ok {
		j.mu.Lock()
		running := j.status == "running"
		j.mu.Unlock()
		if running {
			c1JobsMu.Unlock()
			writeJSON(w, 200, map[string]any{"started": false, "message": "sinxronizasiya artıq gedir"})
			return
		}
	}
	c1JobsMu.Unlock()

	prof, cfg, err := s.loadProfile(r, id)
	if err != nil {
		errJSON(w, 502, "profil yüklənmədi: "+err.Error())
		return
	}
	tdb, _, err := s.tenantPool(r)
	if err != nil {
		errJSON(w, 500, "tenant db error")
		return
	}
	entities := []string{"Müştərilər", "Müqavilələr", "Invoice-lar", "Ödənişlər"}
	job := &c1Job{status: "running", started: time.Now()}
	for _, e := range entities {
		job.steps = append(job.steps, c1Step{Entity: e, Status: "pending"})
	}
	c1JobsMu.Lock()
	c1Jobs[id] = job
	c1JobsMu.Unlock()

	go s.runC1Sync(job, tdb, id, prof, cfg)
	writeJSON(w, 200, map[string]any{"started": true})
}

// c1SyncStatus — GET /connections/{id}/c1sync: current job state for the UI.
func (s *Server) c1SyncStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c1JobsMu.Lock()
	job, ok := c1Jobs[id]
	c1JobsMu.Unlock()
	if !ok {
		writeJSON(w, 200, map[string]any{"status": "idle"})
		return
	}
	writeJSON(w, 200, job.snapshot())
}

func (s *Server) runC1Sync(job *c1Job, tdb *pgxpool.Pool, connID string, prof *c1meta.Profile, cfg *sqldb.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	fail := func(msg string) {
		job.finish("error")
		tdb.Exec(ctx, `UPDATE connections SET last_error=$1 WHERE id=$2`, msg, connID)
	}
	db, err := sqldb.New("mssql", *cfg).Open()
	if err != nil {
		fail("1C bazasına qoşulmaq alınmadı: " + err.Error())
		return
	}
	defer db.Close()
	ad, err := c1.Select(db, prof)
	if err != nil {
		fail(err.Error())
		return
	}

	q := c1.Query{} // unbounded: the full base
	type fetcher func() (int, func(i int) (string, any), error)
	fetchers := []fetcher{
		func() (int, func(int) (string, any), error) {
			rows, err := ad.Customers(ctx, q)
			return len(rows), func(i int) (string, any) { return rows[i].ID, rows[i] }, err
		},
		func() (int, func(int) (string, any), error) {
			rows, err := ad.Contracts(ctx, q)
			return len(rows), func(i int) (string, any) { return rows[i].ID, rows[i] }, err
		},
		func() (int, func(int) (string, any), error) {
			rows, err := ad.Invoices(ctx, q)
			return len(rows), func(i int) (string, any) { return rows[i].ID, rows[i] }, err
		},
		func() (int, func(int) (string, any), error) {
			rows, err := ad.Payments(ctx, q)
			return len(rows), func(i int) (string, any) { return rows[i].ID, rows[i] }, err
		},
	}

	okAll := true
	for i, f := range fetchers {
		entity := job.steps[i].Entity
		job.set(i, "running", 0, "")
		n, at, err := f()
		if err != nil {
			okAll = false
			job.set(i, "error", 0, err.Error())
			logC1Sync(ctx, tdb, connID, entity, "error", err.Error(), 0)
			continue
		}
		saved, err := upsertRecords(ctx, tdb, connID, entity, n, at)
		if err != nil {
			okAll = false
			job.set(i, "error", saved, err.Error())
			logC1Sync(ctx, tdb, connID, entity, "error", err.Error(), saved)
			continue
		}
		job.set(i, "done", saved, "")
		logC1Sync(ctx, tdb, connID, entity, "success", "", saved)
	}
	if okAll {
		tdb.Exec(ctx, `UPDATE connections SET last_sync_at=now(), last_error=NULL WHERE id=$1`, connID)
		job.finish("done")
	} else {
		tdb.Exec(ctx, `UPDATE connections SET last_error=$1 WHERE id=$2`, "tam sinxronizasiyada xəta", connID)
		job.finish("error")
	}
}

func upsertRecords(ctx context.Context, tdb *pgxpool.Pool, connID, entity string, n int, at func(int) (string, any)) (int, error) {
	saved := 0
	for i := 0; i < n; i++ {
		extID, rec := at(i)
		if extID == "" {
			extID = fmt.Sprintf("row_%d", i)
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		if _, err := tdb.Exec(ctx, `
			INSERT INTO records (connection_id, entity_name, external_id, data, synced_at)
			VALUES ($1,$2,$3,$4,now())
			ON CONFLICT (connection_id, entity_name, external_id)
			DO UPDATE SET data=EXCLUDED.data, synced_at=now()`,
			connID, entity, extID, data); err != nil {
			return saved, err
		}
		saved++
	}
	// refresh the per-entity counter shown in Data mərkəzi
	tdb.Exec(ctx, `
		INSERT INTO synced_entities (connection_id, entity_name, record_count, last_synced_at)
		VALUES ($1,$2,(SELECT count(*) FROM records WHERE connection_id=$1 AND entity_name=$2),now())
		ON CONFLICT (connection_id, entity_name)
		DO UPDATE SET record_count=(SELECT count(*) FROM records WHERE connection_id=$1 AND entity_name=$2), last_synced_at=now()`,
		connID, entity)
	return saved, nil
}

func logC1Sync(ctx context.Context, tdb *pgxpool.Pool, connID, entity, status, msg string, n int) {
	tdb.Exec(ctx, `INSERT INTO sync_log (connection_id, entity_name, status, message, records_synced) VALUES ($1,$2,$3,$4,$5)`,
		connID, entity, status, msg, n)
}
