package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestMigrations_UsersTable tests that the users migration creates the table correctly.
func TestMigrations_UsersTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify users table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'users'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Users table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "username", "display_name", "email",
		"auth_provider", "auth_provider_id", "avatar_url", "bio",
		"role", "created_at", "updated_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'users' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in users table: %v", col, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_users_auth_provider",
		"idx_users_username",
		"idx_users_email",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'users' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on users table: %v", idx, err)
		}
	}
}

// TestMigrations_AgentsTable tests that the agents migration creates the table correctly.
func TestMigrations_AgentsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify agents table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'agents'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Agents table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "display_name", "human_id", "bio",
		"specialties", "avatar_url", "api_key_hash", "moltbook_id",
		"created_at", "updated_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'agents' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in agents table: %v", col, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_agents_human_id",
		"idx_agents_moltbook_id",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'agents' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on agents table: %v", idx, err)
		}
	}
}

// TestMigrations_PostsTable tests that the posts migration creates the table correctly.
func TestMigrations_PostsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify posts table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'posts'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Posts table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "type", "title", "description", "tags",
		"posted_by_type", "posted_by_id", "status",
		"upvotes", "downvotes", "success_criteria", "weight",
		"accepted_answer_id", "evolved_into",
		"created_at", "updated_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'posts' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in posts table: %v", col, err)
		}
	}

	// Verify indexes exist
	indexes := []string{
		"idx_posts_type",
		"idx_posts_status",
		"idx_posts_tags",
		"idx_posts_created",
		"idx_posts_author",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'posts' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on posts table: %v", idx, err)
		}
	}
}

// TestMigrations_ApproachesTable tests that the approaches migration creates the tables correctly.
func TestMigrations_ApproachesTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify approaches table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'approaches'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Approaches table does not exist: %v", err)
	}

	// Verify approaches columns exist
	columns := []string{
		"id", "problem_id", "author_type", "author_id",
		"angle", "method", "assumptions", "differs_from",
		"status", "outcome", "solution",
		"created_at", "updated_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'approaches' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in approaches table: %v", col, err)
		}
	}

	// Verify progress_notes table exists
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'progress_notes'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Progress_notes table does not exist: %v", err)
	}

	// Verify progress_notes columns exist
	pnColumns := []string{"id", "approach_id", "content", "created_at"}
	for _, col := range pnColumns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'progress_notes' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in progress_notes table: %v", col, err)
		}
	}
}

// TestMigrations_AnswersTable tests that the answers migration creates the table correctly.
func TestMigrations_AnswersTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify answers table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'answers'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Answers table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "question_id", "author_type", "author_id",
		"content", "is_accepted", "upvotes", "downvotes",
		"created_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'answers' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in answers table: %v", col, err)
		}
	}
}

// TestMigrations_ResponsesTable tests that the responses migration creates the table correctly.
func TestMigrations_ResponsesTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify responses table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'responses'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Responses table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "idea_id", "author_type", "author_id",
		"content", "response_type", "upvotes", "downvotes", "created_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'responses' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in responses table: %v", col, err)
		}
	}
}

// TestMigrations_CommentsTable tests that the comments migration creates the table correctly.
func TestMigrations_CommentsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify comments table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'comments'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Comments table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "target_type", "target_id",
		"author_type", "author_id", "content",
		"created_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'comments' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in comments table: %v", col, err)
		}
	}

	// Verify index exists
	var idxName string
	err = pool.QueryRow(ctx, `
		SELECT indexname
		FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = 'comments' AND indexname = 'idx_comments_target'
	`).Scan(&idxName)
	if err != nil {
		t.Error("Index idx_comments_target does not exist on comments table")
	}
}

// TestMigrations_VotesTable tests that the votes migration creates the table correctly.
func TestMigrations_VotesTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify votes table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'votes'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Votes table does not exist: %v", err)
	}

	// Verify columns exist
	columns := []string{
		"id", "target_type", "target_id",
		"voter_type", "voter_id", "direction",
		"confirmed", "created_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'votes' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in votes table: %v", col, err)
		}
	}

	// Verify unique constraint exists
	var constraintName string
	err = pool.QueryRow(ctx, `
		SELECT constraint_name
		FROM information_schema.table_constraints
		WHERE table_schema = 'public'
		AND table_name = 'votes'
		AND constraint_type = 'UNIQUE'
	`).Scan(&constraintName)
	if err != nil {
		t.Error("Unique constraint does not exist on votes table")
	}
}

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

// TestMigrations_AllTablesExist verifies all expected tables from migrations exist.
func TestMigrations_AllTablesExist(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	expectedTables := []string{
		"users",
		"agents",
		"posts",
		"approaches",
		"progress_notes",
		"answers",
		"responses",
		"comments",
		"votes",
		"notifications",
		"webhooks",
		"rate_limits",
		"audit_log",
		"flags",
		"refresh_tokens",
		"config",
	}

	for _, table := range expectedTables {
		var tableName string
		err = pool.QueryRow(ctx, `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, table).Scan(&tableName)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}
}
