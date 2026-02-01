// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTest is used for testing transaction rollback behavior.
var ErrTest = errors.New("test error")

// Pool wraps pgxpool.Pool with helper methods.
type Pool struct {
	pool *pgxpool.Pool
}

// Tx represents a database transaction interface.
type Tx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// txWrapper wraps pgx.Tx to implement Tx interface.
type txWrapper struct {
	tx pgx.Tx
}

func (t *txWrapper) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, arguments...)
}

func (t *txWrapper) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t *txWrapper) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t *txWrapper) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *txWrapper) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// NewPool creates a new database connection pool with configured settings.
// Configuration per SPEC.md requirements:
// - MaxConns: 10
// - MinConns: 2
// - MaxConnIdleTime: 30s
// - HealthCheckPeriod: 30s
func NewPool(ctx context.Context, databaseURL string) (*Pool, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool settings per requirements
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnIdleTime = 30 * time.Second
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection works
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{pool: pool}, nil
}

// Ping verifies the database connection is alive.
func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Query executes a query that returns rows.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return rows, nil
}

// QueryRow executes a query that returns at most one row.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

// Exec executes a query that doesn't return rows.
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := p.pool.Exec(ctx, sql, args...)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("exec failed: %w", err)
	}
	return tag, nil
}

// BeginTx starts a new transaction.
func (p *Pool) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return &txWrapper{tx: tx}, nil
}

// WithTx executes a function within a transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func (p *Pool) WithTx(ctx context.Context, fn func(tx Tx) error) error {
	tx, err := p.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			log.Printf("failed to rollback transaction: %v", err)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the connection pool.
func (p *Pool) Close() {
	log.Println("closing database connection pool")
	p.pool.Close()
}

// Config returns the pool configuration for inspection.
func (p *Pool) Config() *pgxpool.Config {
	return p.pool.Config()
}
