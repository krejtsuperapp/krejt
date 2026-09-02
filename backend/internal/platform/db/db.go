// Package db — lidhja me PostgreSQL (Aurora në AWS, Postgres lokal në dev) dhe migrimet (§40).
// Migrimet janë SQL të versionuara, të embed-uara në binar, të zbatuara në transaksion, një herë.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// QueryTracer — instrumentim opsional (OpenTelemetry) i çdo query; vendoset para Connect.
var QueryTracer pgx.QueryTracer

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}
	if QueryTracer != nil {
		cfg.ConnConfig.Tracer = QueryTracer
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	var pool *pgxpool.Pool
	// Aurora me auto-pause zgjohet në ~15 s: riprovojmë në vend që të dështojmë në nisje.
	for attempt := 1; attempt <= 6; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = pool.Ping(pctx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt*3) * time.Second):
		}
	}
	return nil, fmt.Errorf("db: connect: %w", err)
}

// migrateLockKey — bllokim këshillimor: vetëm një proces migron njëherësh (disa task-e ECS ose paketa
// testesh paralele kundër së njëjtës bazë do të garonin te CREATE TABLE dhe te vetë migrimet).
const migrateLockKey = 7_263_001

// Migrate zbaton migrimet që mungojnë, me radhë leksikografike (0001_, 0002_, …), secilën në transaksion.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("db: migrate acquire: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return fmt.Errorf("db: migrate lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrateLockKey) }()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("db: migrations table: %w", err)
	}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlText, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, name)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
