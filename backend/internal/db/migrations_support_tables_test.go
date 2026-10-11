package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestMigrations_NotificationsTable tests that the notifications migration creates the table correctly.
func TestMigrations_NotificationsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify notifications table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'notifications'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Notifications table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "user_id", "agent_id", "type",
		"title", "body", "link", "read_at", "created_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'notifications' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in notifications table: %v", col, err)
		}
	}
}

// TestMigrations_WebhooksTable tests that the webhooks migration creates the table correctly.
func TestMigrations_WebhooksTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify webhooks table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'webhooks'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Webhooks table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "agent_id", "url", "events",
		"secret_hash", "status", "created_at", "updated_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'webhooks' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in webhooks table: %v", col, err)
		}
	}
}

// TestMigrations_RateLimitsTable tests that the rate_limits migration creates the table correctly.
func TestMigrations_RateLimitsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify rate_limits table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'rate_limits'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Rate_limits table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{"key", "count", "window_start"}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'rate_limits' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in rate_limits table: %v", col, err)
		}
	}
}

// TestMigrations_AuditLogTable tests that the audit_log migration creates the table correctly.
func TestMigrations_AuditLogTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify audit_log table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'audit_log'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Audit_log table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "admin_id", "admin_type", "action",
		"target_type", "target_id", "details", "created_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'audit_log' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in audit_log table: %v", col, err)
		}
	}
}

// TestMigrations_FlagsTable tests that the flags migration creates the table correctly.
func TestMigrations_FlagsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify flags table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'flags'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Flags table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "target_type", "target_id",
		"reporter_type", "reporter_id", "reason",
		"status", "resolved_by", "resolved_at", "created_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'flags' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in flags table: %v", col, err)
		}
	}
}

// TestMigrations_RefreshTokensTable tests that the refresh_tokens migration creates the table correctly.
func TestMigrations_RefreshTokensTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify refresh_tokens table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'refresh_tokens'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Refresh_tokens table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "user_id", "token_hash", "expires_at",
		"created_at", "revoked_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'refresh_tokens' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in refresh_tokens table: %v", col, err)
		}
	}
}

// TestMigrations_PinsTable tests that the pins migration creates the table correctly.
func TestMigrations_PinsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify pins table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'pins'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Pins table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "cid", "status", "name",
		"origins", "meta", "delegates",
		"owner_id", "owner_type", "size_bytes",
		"created_at", "updated_at", "pinned_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'pins' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in pins table: %v", col, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_pins_cid",
		"idx_pins_owner",
		"idx_pins_status",
		"idx_pins_created_at",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'pins' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on pins table: %v", idx, err)
		}
	}

	// Verify check constraints exist
	constraints := []string{
		"pins_status_check",
		"pins_owner_type_check",
	}
	for _, con := range constraints {
		var conName string
		err = pool.QueryRow(ctx, `
			SELECT constraint_name
			FROM information_schema.table_constraints
			WHERE table_schema = 'public'
			AND table_name = 'pins'
			AND constraint_name = $1
		`, con).Scan(&conName)
		if err != nil {
			t.Errorf("Constraint %s does not exist on pins table: %v", con, err)
		}
	}

	// Verify unique constraint on (cid, owner_id)
	var uniqueConName string
	err = pool.QueryRow(ctx, `
		SELECT constraint_name
		FROM information_schema.table_constraints
		WHERE table_schema = 'public'
		AND table_name = 'pins'
		AND constraint_name = 'pins_cid_owner_unique'
	`).Scan(&uniqueConName)
	if err != nil {
		t.Error("Unique constraint pins_cid_owner_unique does not exist on pins table")
	}
}
