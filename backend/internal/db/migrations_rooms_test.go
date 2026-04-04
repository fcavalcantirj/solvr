package db_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// roomsTestCleanup removes all test data by slug prefix or exact name.
// Call at test start and register with t.Cleanup to ensure idempotent teardown.
func roomsTestCleanup(ctx context.Context, pool *db.Pool, slugs ...string) {
	for _, slug := range slugs {
		// Delete dependent rows first (CASCADE should handle this, but be explicit)
		pool.Exec(ctx, "DELETE FROM messages WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", slug) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", slug) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM rooms WHERE slug = $1", slug)                                                 //nolint:errcheck
	}
}

// TestMigrations_RoomsTable tests that the rooms migration creates the table correctly.
func TestMigrations_RoomsTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify rooms table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'rooms'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("Rooms table does not exist: %v", err)
	}

	// Verify all 15 columns exist
	columns := []string{
		"id", "slug", "display_name", "description", "category",
		"tags", "is_private", "owner_id", "token_hash", "message_count",
		"created_at", "updated_at", "last_active_at", "expires_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'rooms' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in rooms table: %v", col, err)
		}
	}

	// Verify 4 indexes exist
	indexes := []string{
		"idx_rooms_owner_id",
		"idx_rooms_expires_at",
		"idx_rooms_active",
		"idx_rooms_deleted",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'rooms' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on rooms table: %v", idx, err)
		}
	}
}

// TestMigrations_RoomsConstraints tests that rooms constraints are enforced correctly.
func TestMigrations_RoomsConstraints(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	testSlugs := []string{"test-constraints", "test-tags-limit"}

	// Pre-cleanup to handle leftover data from previous runs
	roomsTestCleanup(ctx, pool, testSlugs...)

	// Register cleanup to run at test end
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomsTestCleanup(cleanCtx, pool, testSlugs...)
	})

	// Create a valid room first
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('test-constraints', 'Test Room', 'hash123')
	`)
	if err != nil {
		t.Fatalf("Valid room insert failed: %v", err)
	}

	// Test slug UNIQUE: duplicate slug should fail
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('test-constraints', 'Test Room 2', 'hash456')
	`)
	if err == nil {
		t.Error("Expected unique constraint violation for duplicate slug, but got nil error")
	}

	// Test tags array_length: 11 tags (> 10 limit) should fail
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash, tags)
		VALUES ('test-tags-limit', 'Test Tags', 'hash456',
			ARRAY['a','b','c','d','e','f','g','h','i','j','k'])
	`)
	if err == nil {
		t.Error("Expected check constraint violation for >10 tags, but got nil error")
	}

	// Test slug regex: uppercase letters should fail
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('UPPERCASE', 'Test', 'hash789')
	`)
	if err == nil {
		t.Error("Expected check constraint violation for uppercase slug, but got nil error")
	}

	// Test slug regex: too short (2 chars, minimum is 3: pattern requires [a-z0-9]{1,38} in middle)
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('ab', 'Test', 'hash101')
	`)
	if err == nil {
		t.Error("Expected check constraint violation for too-short slug 'ab', but got nil error")
	}
}

// TestMigrations_AgentPresenceTable tests that the agent_presence migration creates the table correctly.
func TestMigrations_AgentPresenceTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify agent_presence table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'agent_presence'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("agent_presence table does not exist: %v", err)
	}

	// Verify all 7 columns exist
	columns := []string{
		"id", "room_id", "agent_name", "card_json",
		"joined_at", "last_seen", "ttl_seconds",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'agent_presence' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in agent_presence table: %v", col, err)
		}
	}

	// Verify 2 indexes exist
	indexes := []string{
		"idx_agent_presence_room_id",
		"idx_agent_presence_last_seen",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'agent_presence' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on agent_presence table: %v", idx, err)
		}
	}

	// Test UNIQUE(room_id, agent_name): duplicate (room_id, agent_name) should fail
	presenceRoom := "presence-test-room"

	// Pre-cleanup to handle leftover data from previous runs
	roomsTestCleanup(ctx, pool, presenceRoom)

	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomsTestCleanup(cleanCtx, pool, presenceRoom)
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('presence-test-room', 'Presence Test Room', 'hash_presence')
	`)
	if err != nil {
		t.Fatalf("Failed to create room for agent_presence test: %v", err)
	}

	var roomID string
	err = pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = 'presence-test-room'`).Scan(&roomID)
	if err != nil {
		t.Fatalf("Failed to get room ID: %v", err)
	}

	// Insert first agent presence record
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_presence (room_id, agent_name, card_json)
		VALUES ($1, 'test-agent', '{"name": "test-agent"}')
	`, roomID)
	if err != nil {
		t.Fatalf("First agent_presence insert failed: %v", err)
	}

	// Insert same (room_id, agent_name) again — should fail unique constraint
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_presence (room_id, agent_name, card_json)
		VALUES ($1, 'test-agent', '{"name": "test-agent-2"}')
	`, roomID)
	if err == nil {
		t.Error("Expected unique constraint violation for duplicate (room_id, agent_name), but got nil error")
	}
}

// TestMigrations_MessagesTable tests that the messages migration creates the table correctly.
func TestMigrations_MessagesTable(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	// Verify messages table exists
	var tableName string
	err = pool.QueryRow(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'messages'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("messages table does not exist: %v", err)
	}

	// Verify all 11 columns exist
	columns := []string{
		"id", "room_id", "author_type", "author_id", "agent_name",
		"content", "content_type", "metadata", "sequence_num",
		"created_at", "deleted_at",
	}
	for _, col := range columns {
		var colName string
		err = pool.QueryRow(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'messages' AND column_name = $1
		`, col).Scan(&colName)
		if err != nil {
			t.Errorf("Column %s does not exist in messages table: %v", col, err)
		}
	}

	// Verify 2 indexes exist
	indexes := []string{
		"idx_messages_room_created",
		"idx_messages_room_active",
	}
	for _, idx := range indexes {
		var idxName string
		err = pool.QueryRow(ctx, `
			SELECT indexname
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'messages' AND indexname = $1
		`, idx).Scan(&idxName)
		if err != nil {
			t.Errorf("Index %s does not exist on messages table: %v", idx, err)
		}
	}
}

// TestMigrations_MessagesConstraints tests that messages constraints are enforced correctly.
func TestMigrations_MessagesConstraints(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	msgRoom := "msg-test-room"

	// Pre-cleanup to handle leftover data from previous runs
	roomsTestCleanup(ctx, pool, msgRoom)

	// Create a room for FK
	_, err = pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ('msg-test-room', 'Msg Test', 'hash_msg')
	`)
	if err != nil {
		t.Fatalf("Failed to create room for messages constraint test: %v", err)
	}

	var roomID string
	err = pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = 'msg-test-room'`).Scan(&roomID)
	if err != nil {
		t.Fatalf("Failed to get room ID: %v", err)
	}

	// Cleanup at end
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomsTestCleanup(cleanCtx, pool, msgRoom)
	})

	// Test author_type CHECK: 'bot' is not in allowed values ('human', 'agent', 'system')
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, author_type, agent_name, content)
		VALUES ($1, 'bot', 'test-agent', 'hello')
	`, roomID)
	if err == nil {
		t.Error("Expected check constraint violation for author_type 'bot', but got nil error")
	}

	// Test author_type valid values: 'human' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, author_type, agent_name, content)
		VALUES ($1, 'human', 'test-human', 'hello from human')
	`, roomID)
	if err != nil {
		t.Errorf("author_type 'human' should be allowed, but got error: %v", err)
	}

	// Test author_type valid values: 'agent' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, author_type, agent_name, content)
		VALUES ($1, 'agent', 'test-agent', 'hello from agent')
	`, roomID)
	if err != nil {
		t.Errorf("author_type 'agent' should be allowed, but got error: %v", err)
	}

	// Test author_type valid values: 'system' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, author_type, agent_name, content)
		VALUES ($1, 'system', 'system', 'room created')
	`, roomID)
	if err != nil {
		t.Errorf("author_type 'system' should be allowed, but got error: %v", err)
	}

	// Test content_type CHECK: 'html' is not allowed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, agent_name, content, content_type)
		VALUES ($1, 'test-agent', 'hello', 'html')
	`, roomID)
	if err == nil {
		t.Error("Expected check constraint violation for content_type 'html', but got nil error")
	}

	// Test content_type valid values: 'text' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, agent_name, content, content_type)
		VALUES ($1, 'test-agent', 'hello text', 'text')
	`, roomID)
	if err != nil {
		t.Errorf("content_type 'text' should be allowed, but got error: %v", err)
	}

	// Test content_type valid values: 'markdown' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, agent_name, content, content_type)
		VALUES ($1, 'test-agent', '# hello', 'markdown')
	`, roomID)
	if err != nil {
		t.Errorf("content_type 'markdown' should be allowed, but got error: %v", err)
	}

	// Test content_type valid values: 'json' should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, agent_name, content, content_type)
		VALUES ($1, 'test-agent', '{}', 'json')
	`, roomID)
	if err != nil {
		t.Errorf("content_type 'json' should be allowed, but got error: %v", err)
	}

	// Test content length CHECK: string > 65536 chars should be rejected
	longContent := strings.Repeat("x", 65537)
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, agent_name, content)
		VALUES ($1, 'test-agent', $2)
	`, roomID, longContent)
	if err == nil {
		t.Error("Expected check constraint violation for content > 65536 chars, but got nil error")
	}
}
