package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gateway/internal/syncer"
	"gateway/internal/tenantdb"
)

// Scheduler runs enabled automations for all active tenants.
type Scheduler struct {
	Master  *pgxpool.Pool
	Tenants *tenantdb.Manager
	Sync    *syncer.Syncer
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	log.Println("scheduler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	rows, err := s.Master.Query(ctx, "SELECT db_name FROM tenants WHERE status='active'")
	if err != nil {
		log.Printf("scheduler: list tenants: %v", err)
		return
	}
	var dbs []string
	for rows.Next() {
		var db string
		if rows.Scan(&db) == nil {
			dbs = append(dbs, db)
		}
	}
	rows.Close()

	for _, db := range dbs {
		s.runTenant(ctx, db)
	}
}

func (s *Scheduler) runTenant(ctx context.Context, dbName string) {
	tdb, err := s.Tenants.Pool(ctx, dbName)
	if err != nil {
		log.Printf("scheduler %s: pool: %v", dbName, err)
		return
	}
	rows, err := tdb.Query(ctx, `
		SELECT a.id, a.connection_id, a.entity_name, c.connector_type, c.config_enc
		FROM automations a JOIN connections c ON c.id = a.connection_id
		WHERE a.enabled
		  AND (a.last_run_at IS NULL OR a.last_run_at + (a.interval_minutes || ' minutes')::interval <= now())`)
	if err != nil {
		return
	}
	type job struct{ id, connID, entity, ctype, enc string }
	var jobs []job
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.connID, &j.entity, &j.ctype, &j.enc) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()

	for _, j := range jobs {
		// claim first to avoid re-running on next tick
		tdb.Exec(ctx, "UPDATE automations SET last_run_at=now(), last_status='running' WHERE id=$1", j.id)
		n, err := s.Sync.SyncEntity(ctx, tdb, j.connID, j.ctype, j.enc, j.entity)
		status := "success"
		if err != nil {
			status = "error"
			log.Printf("scheduler %s: automation %s (%s): %v", dbName, j.id, j.entity, err)
		} else {
			log.Printf("scheduler %s: automation %s synced %d records of %s", dbName, j.id, n, j.entity)
		}
		tdb.Exec(ctx, "UPDATE automations SET last_status=$1 WHERE id=$2", status, j.id)
	}
}
