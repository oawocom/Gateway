package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"gateway/internal/api"
	"gateway/internal/auth"
	"gateway/internal/crypto"
	"gateway/internal/scheduler"
	"gateway/internal/syncer"
	"gateway/internal/tenantdb"
	"gateway/migrations"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatal("ENCRYPTION_KEY is required (64 hex chars)")
	}
	box, err := crypto.New(encKey)
	if err != nil {
		log.Fatalf("encryption: %v", err)
	}

	// master migrations
	src, err := iofs.New(migrations.Files, "master")
	if err != nil {
		log.Fatalf("migrations source: %v", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate up: %v", err)
	}
	log.Println("master migrations applied")

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	tenants := tenantdb.New(dbURL)
	migrateTenants(pool, tenants)
	seedAdmin(pool)

	sync := &syncer.Syncer{Box: box}

	srv := &api.Server{
		Master:  pool,
		JWT:     auth.NewJWT(secret),
		Tenants: tenants,
		Box:     box,
		Sync:    sync,
	}

	sched := &scheduler.Scheduler{Master: pool, Tenants: tenants, Sync: sync}
	go sched.Run(context.Background())

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}

// migrateTenants applies pending tenant migrations to every existing tenant DB.
func migrateTenants(pool *pgxpool.Pool, tenants *tenantdb.Manager) {
	rows, err := pool.Query(context.Background(), "SELECT db_name FROM tenants WHERE status IN ('active','suspended')")
	if err != nil {
		log.Printf("tenant migrate: list: %v", err)
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
		if err := tenants.Migrate(db); err != nil {
			log.Printf("tenant migrate %s: %v", db, err)
		} else {
			log.Printf("tenant migrated: %s", db)
		}
	}
}

func seedAdmin(pool *pgxpool.Pool) {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	pw := os.Getenv("ADMIN_PASSWORD")
	if email == "" || pw == "" {
		return
	}
	ctx := context.Background()
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM platform_admins WHERE lower(email)=$1)", email).Scan(&exists); err != nil {
		log.Printf("seed admin check: %v", err)
		return
	}
	if exists {
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		log.Printf("seed admin hash: %v", err)
		return
	}
	if _, err := pool.Exec(ctx, "INSERT INTO platform_admins (email, password_hash) VALUES ($1,$2)", email, hash); err != nil {
		log.Printf("seed admin insert: %v", err)
		return
	}
	log.Printf("platform admin seeded: %s", email)
}
