package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// getTestDatabaseURL returns the database URL from environment or skips the test.
func getTestDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping database integration test")
	}
	return url
}

func TestNewPool_Success(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v, want nil", err)
	}
	defer pool.Close()

	if pool == nil {
		t.Fatal("NewPool() returned nil pool")
	}
}

func TestNewPool_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewPool(ctx, "invalid://not-a-valid-url")
	if err == nil {
		t.Fatal("NewPool() with invalid URL should return error")
	}
}

func TestNewPool_EmptyURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewPool(ctx, "")
	if err == nil {
		t.Fatal("NewPool() with empty URL should return error")
	}
}

func TestPool_Ping(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	err = pool.Ping(ctx)
	if err != nil {
		t.Fatalf("Pool.Ping() error = %v, want nil", err)
	}
}

func TestPool_Query(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT 1 AS value")
	if err != nil {
		t.Fatalf("Pool.Query() error = %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("Pool.Query() returned no rows")
	}

	var value int
	err = rows.Scan(&value)
	if err != nil {
		t.Fatalf("rows.Scan() error = %v", err)
	}

	if value != 1 {
		t.Fatalf("got value = %d, want 1", value)
	}
}

func TestPool_QueryRow(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	var value int
	err = pool.QueryRow(ctx, "SELECT 42 AS value").Scan(&value)
	if err != nil {
		t.Fatalf("Pool.QueryRow().Scan() error = %v", err)
	}

	if value != 42 {
		t.Fatalf("got value = %d, want 42", value)
	}
}

func TestPool_Exec(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a temporary table for testing
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE test_exec (
			id SERIAL PRIMARY KEY,
			name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Pool.Exec() CREATE TABLE error = %v", err)
	}

	// Insert a row
	tag, err := pool.Exec(ctx, "INSERT INTO test_exec (name) VALUES ($1)", "test")
	if err != nil {
		t.Fatalf("Pool.Exec() INSERT error = %v", err)
	}

	if tag.RowsAffected() != 1 {
		t.Fatalf("Pool.Exec() RowsAffected() = %d, want 1", tag.RowsAffected())
	}

	// Insert multiple rows
	tag, err = pool.Exec(ctx, "INSERT INTO test_exec (name) VALUES ($1), ($2)", "a", "b")
	if err != nil {
		t.Fatalf("Pool.Exec() multiple INSERT error = %v", err)
	}

	if tag.RowsAffected() != 2 {
		t.Fatalf("Pool.Exec() RowsAffected() = %d, want 2", tag.RowsAffected())
	}
}

func TestPool_BeginTx_Commit(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a temp table outside transaction
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE test_tx (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	if err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}

	// Start transaction
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Pool.BeginTx() error = %v", err)
	}

	// Insert in transaction
	_, err = tx.Exec(ctx, "INSERT INTO test_tx (value) VALUES ($1)", 100)
	if err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}

	// Commit
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("tx.Commit() error = %v", err)
	}

	// Verify data persisted
	var value int
	err = pool.QueryRow(ctx, "SELECT value FROM test_tx WHERE value = 100").Scan(&value)
	if err != nil {
		t.Fatalf("SELECT after commit error = %v", err)
	}

	if value != 100 {
		t.Fatalf("got value = %d, want 100", value)
	}
}

func TestPool_BeginTx_Rollback(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a temp table outside transaction
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE test_tx_rollback (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	if err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}

	// Start transaction
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Pool.BeginTx() error = %v", err)
	}

	// Insert in transaction
	_, err = tx.Exec(ctx, "INSERT INTO test_tx_rollback (value) VALUES ($1)", 999)
	if err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}

	// Rollback
	err = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("tx.Rollback() error = %v", err)
	}

	// Verify data NOT persisted
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_tx_rollback WHERE value = 999").Scan(&count)
	if err != nil {
		t.Fatalf("SELECT after rollback error = %v", err)
	}

	if count != 0 {
		t.Fatalf("got count = %d, want 0 (rollback should have reverted)", count)
	}
}

func TestPool_WithTx_Success(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a temp table
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE test_with_tx (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	if err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}

	// Use WithTx for automatic commit on success
	err = pool.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO test_with_tx (value) VALUES ($1)", 42)
		return err
	})
	if err != nil {
		t.Fatalf("Pool.WithTx() error = %v", err)
	}

	// Verify data persisted
	var value int
	err = pool.QueryRow(ctx, "SELECT value FROM test_with_tx WHERE value = 42").Scan(&value)
	if err != nil {
		t.Fatalf("SELECT after WithTx error = %v", err)
	}

	if value != 42 {
		t.Fatalf("got value = %d, want 42", value)
	}
}

func TestPool_WithTx_Error(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Create a temp table
	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE test_with_tx_error (
			id SERIAL PRIMARY KEY,
			value INT
		)
	`)
	if err != nil {
		t.Fatalf("CREATE TABLE error = %v", err)
	}

	// Use WithTx that returns an error - should rollback
	testErr := db.ErrTest
	err = pool.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO test_with_tx_error (value) VALUES ($1)", 999)
		if err != nil {
			return err
		}
		return testErr // Return error to trigger rollback
	})
	if err != testErr {
		t.Fatalf("Pool.WithTx() error = %v, want %v", err, testErr)
	}

	// Verify data NOT persisted due to rollback
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_with_tx_error WHERE value = 999").Scan(&count)
	if err != nil {
		t.Fatalf("SELECT after WithTx error = %v", err)
	}

	if count != 0 {
		t.Fatalf("got count = %d, want 0 (error should have triggered rollback)", count)
	}
}

func TestPool_Close(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	// Close the pool
	pool.Close()

	// After close, Ping should fail
	err = pool.Ping(ctx)
	if err == nil {
		t.Fatal("Pool.Ping() after Close() should return error")
	}
}

func TestPoolConfig(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	config := pool.Config()
	if config == nil {
		t.Fatal("Pool.Config() returned nil")
	}

	// Verify configuration per requirements:
	// MaxConns = 10, MinConns = 2, MaxConnIdleTime = 30s, HealthCheckPeriod = 30s
	if config.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", config.MaxConns)
	}

	if config.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", config.MinConns)
	}

	if config.MaxConnIdleTime != 30*time.Second {
		t.Errorf("MaxConnIdleTime = %v, want 30s", config.MaxConnIdleTime)
	}

	if config.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s", config.HealthCheckPeriod)
	}
}
