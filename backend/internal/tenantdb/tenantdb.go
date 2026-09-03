package tenantdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gateway/migrations"
)

type Manager struct {
	masterDSN string
	mu        sync.Mutex
	pools     map[string]*pgxpool.Pool
}

func New(masterDSN string) *Manager {
	return &Manager{masterDSN: masterDSN, pools: map[string]*pgxpool.Pool{}}
}

func (m *Manager) dsnFor(dbName string) (string, error) {
	u, err := url.Parse(m.masterDSN)
	if err != nil {
		return "", err
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

// Provision creates the tenant database and applies tenant migrations.
func (m *Manager) Provision(ctx context.Context, dbName string) error {
	conn, err := pgx.Connect(ctx, m.masterDSN)
	if err != nil {
		return fmt.Errorf("connect master: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return m.Migrate(dbName)
}

// Migrate applies tenant migrations (safe to run repeatedly).
func (m *Manager) Migrate(dbName string) error {
	dsn, err := m.dsnFor(dbName)
	if err != nil {
		return err
	}
	src, err := iofs.New(migrations.Files, "tenant")
	if err != nil {
		return err
	}
	mig, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return err
	}
	defer mig.Close()
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// Pool returns a cached connection pool for a tenant database.
func (m *Manager) Pool(ctx context.Context, dbName string) (*pgxpool.Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.pools[dbName]; ok {
		return p, nil
	}
	dsn, err := m.dsnFor(dbName)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 4
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	m.pools[dbName] = p
	return p, nil
}

// Drop closes the cached pool and drops the tenant database permanently.
func (m *Manager) Drop(ctx context.Context, dbName string) error {
	m.mu.Lock()
	if p, ok := m.pools[dbName]; ok {
		p.Close()
		delete(m.pools, dbName)
	}
	m.mu.Unlock()

	conn, err := pgx.Connect(ctx, m.masterDSN)
	if err != nil {
		return fmt.Errorf("connect master: %w", err)
	}
	defer conn.Close(ctx)

	// terminate remaining connections, then drop
	conn.Exec(ctx,
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()",
		dbName)
	if _, err := conn.Exec(ctx,
		"DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)"); err != nil {
		return fmt.Errorf("drop database: %w", err)
	}
	return nil
}
