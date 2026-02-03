package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestMigrations_ConfigTable tests that the config migration creates the table correctly.
func TestMigrations_ConfigTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify config table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'config'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Config table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"key", "value", "created_at", "updated_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'config' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in config table: %v", col, err)
		}
	}

	// Verify key is primary key
	var constraintName string
	err = pool.QueryRow(ctx, `
		SELECT constraint_name
		FROM information_schema.table_constraints
		WHERE table_schema = 'public'
		AND table_name = 'config'
		AND constraint_type = 'PRIMARY KEY'
	`).Scan(&constraintName)
	if err != nil {
		t.Error("Primary key constraint does not exist on config table")
	}

	// Verify value column is JSONB type
	var dataType string
	err = pool.QueryRow(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'config' AND column_name = 'value'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("Could not get data type for value column: %v", err)
	}
	if dataType != "jsonb" {
		t.Errorf("Value column should be jsonb, got %s", dataType)
	}
}

// TestConfig_InsertAndQuery tests that we can insert and query config values.
func TestConfig_InsertAndQuery(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	testKey := "test.config.key"
	testValue := `{"enabled": true, "count": 42, "name": "test"}`

	// Clean up any existing test key first
	_, _ = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)

	// Test INSERT
	_, err = pool.Exec(ctx, `
		INSERT INTO config (key, value)
		VALUES ($1, $2::jsonb)
	`, testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to insert into config table: %v", err)
	}

	// Test SELECT - verify we can query the inserted value
	var retrievedValue string
	err = pool.QueryRow(ctx, `
		SELECT value::text FROM config WHERE key = $1
	`, testKey).Scan(&retrievedValue)
	if err != nil {
		t.Fatalf("Failed to query config table: %v", err)
	}

	// Test JSONB query - verify we can query JSON fields
	var enabled bool
	err = pool.QueryRow(ctx, `
		SELECT (value->>'enabled')::boolean FROM config WHERE key = $1
	`, testKey).Scan(&enabled)
	if err != nil {
		t.Fatalf("Failed to query JSONB field: %v", err)
	}
	if !enabled {
		t.Error("Expected enabled=true from JSONB query")
	}

	// Clean up test data
	_, err = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)
	if err != nil {
		t.Fatalf("Failed to delete test data: %v", err)
	}
}

// TestConfig_Update tests that we can update config values.
func TestConfig_Update(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	testKey := "test.config.update"
	initialValue := `{"enabled": true, "count": 42}`

	// Clean up any existing test key first
	_, _ = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)

	// Insert initial value
	_, err = pool.Exec(ctx, `
		INSERT INTO config (key, value)
		VALUES ($1, $2::jsonb)
	`, testKey, initialValue)
	if err != nil {
		t.Fatalf("Failed to insert initial config: %v", err)
	}

	// Update value
	newValue := `{"enabled": false, "count": 100}`
	_, err = pool.Exec(ctx, `
		UPDATE config SET value = $2::jsonb, updated_at = NOW() WHERE key = $1
	`, testKey, newValue)
	if err != nil {
		t.Fatalf("Failed to update config table: %v", err)
	}

	// Verify update
	var enabled bool
	err = pool.QueryRow(ctx, `
		SELECT (value->>'enabled')::boolean FROM config WHERE key = $1
	`, testKey).Scan(&enabled)
	if err != nil {
		t.Fatalf("Failed to query updated JSONB field: %v", err)
	}
	if enabled {
		t.Error("Expected enabled=false after update")
	}

	var count int
	err = pool.QueryRow(ctx, `
		SELECT (value->>'count')::int FROM config WHERE key = $1
	`, testKey).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count field: %v", err)
	}
	if count != 100 {
		t.Errorf("Expected count=100 after update, got %d", count)
	}

	// Clean up test data
	_, err = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)
	if err != nil {
		t.Fatalf("Failed to delete test data: %v", err)
	}
}

// TestConfig_PrimaryKeyConstraint tests that the primary key constraint works.
func TestConfig_PrimaryKeyConstraint(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	testKey := "test.config.pk"
	testValue := `{"test": true}`

	// Clean up any existing test key first
	_, _ = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)

	// Insert first value
	_, err = pool.Exec(ctx, `
		INSERT INTO config (key, value) VALUES ($1, $2::jsonb)
	`, testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to insert config: %v", err)
	}

	// Try to insert duplicate key - should fail
	_, err = pool.Exec(ctx, `
		INSERT INTO config (key, value) VALUES ($1, $2::jsonb)
	`, testKey, `{"different": true}`)
	if err == nil {
		t.Error("Expected error when inserting duplicate key, but got none")
	}

	// Clean up test data
	_, _ = pool.Exec(ctx, `DELETE FROM config WHERE key = $1`, testKey)
}
