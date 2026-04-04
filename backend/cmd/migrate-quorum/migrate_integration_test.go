// Package main provides integration tests for the Quorum-to-Solvr migration CLI tool.
// Tests run against a real local Docker PostgreSQL instance (port 5433).
// Tests are skipped automatically when DATABASE_URL is not set.
//
// Run with:
//
//	DATABASE_URL="postgres://solvr:solvr_dev@localhost:5433/solvr?sslmode=disable" go test ./cmd/migrate-quorum/... -run TestIntegration_ -v -count=1
package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaQuorumMigrationDB implements migrationDB using a quorum_test schema
// within the Solvr DB for integration testing. This avoids needing a second
// DB connection for the Quorum source.
type schemaQuorumMigrationDB struct {
	pool      *db.Pool
	rawPool   *pgxpool.Pool
	quorumRooms  []quorumRoom
	quorumMsgs   map[uuid.UUID][]quorumMessage
}

func (d *schemaQuorumMigrationDB) ListQuorumRooms(ctx context.Context) ([]quorumRoom, error) {
	return d.quorumRooms, nil
}

func (d *schemaQuorumMigrationDB) ListQuorumMessages(ctx context.Context, roomID uuid.UUID) ([]quorumMessage, error) {
	return d.quorumMsgs[roomID], nil
}

func (d *schemaQuorumMigrationDB) FindSolvrUserByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := d.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find user by email %s: %w", email, err)
	}
	return id, nil
}

func (d *schemaQuorumMigrationDB) CheckSlugConflict(ctx context.Context, slug string, roomID uuid.UUID) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rooms WHERE slug = $1 AND id != $2)`,
		slug, roomID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check slug conflict: %w", err)
	}
	return exists, nil
}

func (d *schemaQuorumMigrationDB) BeginTx(ctx context.Context) (txInterface, error) {
	return d.pool.BeginTx(ctx)
}

func (d *schemaQuorumMigrationDB) InsertRoom(ctx context.Context, tx txInterface, r roomInsert) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO rooms (id, slug, display_name, description, category, tags, is_private,
		                   owner_id, token_hash, message_count, created_at, updated_at,
		                   last_active_at, expires_at, deleted_at)
		VALUES ($1, $2, $3, NULL, NULL, '{}', $4, $5, $6, 0, $7, $7, $7, NULL, NULL)
		ON CONFLICT (id) DO NOTHING`,
		r.ID, r.Slug, r.DisplayName, r.IsPrivate, r.OwnerID, r.TokenHash, r.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert room %s: %w", r.Slug, err)
	}
	return tag.RowsAffected() > 0, nil
}

func (d *schemaQuorumMigrationDB) InsertAgent(ctx context.Context, tx txInterface, a agentInsert) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO agents (id, display_name, human_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING`,
		a.ID, a.DisplayName, a.HumanID,
	)
	if err != nil {
		return false, fmt.Errorf("insert agent %s: %w", a.ID, err)
	}
	return tag.RowsAffected() > 0, nil
}

func (d *schemaQuorumMigrationDB) InsertMessagesWithSequence(ctx context.Context, tx txInterface, roomID uuid.UUID, msgs []messageInsert) (int, error) {
	count := 0
	for _, msg := range msgs {
		_, err := tx.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, author_id, agent_name, content,
			                      content_type, metadata, sequence_num, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)`,
			msg.RoomID, msg.AuthorType, msg.AuthorID, msg.AgentName, msg.Content,
			msg.ContentType, msg.Metadata, msg.SequenceNum, msg.CreatedAt,
		)
		if err != nil {
			return count, fmt.Errorf("insert message seq=%d: %w", msg.SequenceNum, err)
		}
		count++
	}
	return count, nil
}

func (d *schemaQuorumMigrationDB) UpdateRoomMessageCount(ctx context.Context, tx txInterface, roomID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE rooms
		SET message_count = (SELECT COUNT(*) FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
		WHERE id = $1`,
		roomID,
	)
	return err
}

// txWrapper wraps db.Tx to implement txInterface.
// db.Tx has the same methods but is a different type.
type integTxWrapper struct {
	tx interface {
		Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
		Commit(ctx context.Context) error
		Rollback(ctx context.Context) error
	}
}

func (w *integTxWrapper) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return w.tx.Exec(ctx, sql, arguments...)
}
func (w *integTxWrapper) Commit(ctx context.Context) error {
	return w.tx.Commit(ctx)
}
func (w *integTxWrapper) Rollback(ctx context.Context) error {
	return w.tx.Rollback(ctx)
}

// setupIntegrationDB connects to local Docker PostgreSQL.
// Skips test if DATABASE_URL is not set.
func setupIntegrationDB(t *testing.T) *db.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// seedIntegrationData seeds the test with representative prod-like data.
// Returns the seeded user UUIDs and room/message data for the migrationDB.
func seedIntegrationData(t *testing.T, pool *db.Pool) (felipeID, marceloID uuid.UUID, mDB migrationDB) {
	t.Helper()
	ctx := context.Background()

	// Create test users (Felipe + Marcelo) with unique emails to avoid conflicts.
	// Use a unique suffix per test run to avoid collisions.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	felipeEmail := fmt.Sprintf("integ-felipe-%s@test.solvr.dev", suffix)
	marceloEmail := fmt.Sprintf("integ-marcelo-%s@test.solvr.dev", suffix)

	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, username, display_name, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, $2, 'Felipe Test', 'github', $3, $4)
		RETURNING id`,
		felipeEmail,
		fmt.Sprintf("felipe_integ_%s", suffix[:8]),
		fmt.Sprintf("gh_felipe_%s", suffix[:8]),
		fmt.Sprintf("FELI%s", suffix[:4]),
	).Scan(&felipeID)
	if err != nil {
		t.Fatalf("failed to create felipe test user: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, username, display_name, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, $2, 'Marcelo Test', 'github', $3, $4)
		RETURNING id`,
		marceloEmail,
		fmt.Sprintf("marcelo_integ_%s", suffix[:8]),
		fmt.Sprintf("gh_marcelo_%s", suffix[:8]),
		fmt.Sprintf("MARC%s", suffix[:4]),
	).Scan(&marceloID)
	if err != nil {
		t.Fatalf("failed to create marcelo test user: %v", err)
	}

	// Seed quorum room/message data in memory (same as what Quorum DB would provide).
	baseTime := time.Now().Add(-24 * time.Hour)

	room1ID := uuid.New()
	room2ID := uuid.New()
	room3ID := uuid.New()
	room4ID := uuid.New()
	room5ID := uuid.New()
	skippedRoom1ID := uuid.New()
	skippedRoom2ID := uuid.New()

	quorumRooms := []quorumRoom{
		{ID: room1ID, Slug: "composio-integration", DisplayName: "Composio Integration", TokenHash: "hash1", CreatedAt: baseTime},
		{ID: room2ID, Slug: "solvr-usage-analysys", DisplayName: "Solvr Usage Analysis", TokenHash: "hash2", CreatedAt: baseTime.Add(time.Hour)},
		{ID: room3ID, Slug: "ballona-trade-v0", DisplayName: "Ballona Trade", TokenHash: "hash3", CreatedAt: baseTime.Add(2 * time.Hour)},
		{ID: room4ID, Slug: "mackjack-ops", DisplayName: "MackJack Ops", TokenHash: "hash4", CreatedAt: baseTime.Add(3 * time.Hour)},
		{ID: room5ID, Slug: "jack-mack-msv-trading", DisplayName: "Jack Mack MSV Trading", TokenHash: "hash5", CreatedAt: baseTime.Add(4 * time.Hour)},
		// 2 skipped rooms
		{ID: skippedRoom1ID, Slug: "some-test-room", DisplayName: "Some Test Room", TokenHash: "hash6", CreatedAt: baseTime},
		{ID: skippedRoom2ID, Slug: "another-random-room", DisplayName: "Another Room", TokenHash: "hash7", CreatedAt: baseTime},
	}

	quorumMsgs := map[uuid.UUID][]quorumMessage{
		room1ID: {
			{AgentName: "ClaudiusThePirateEmperor", Content: "# Welcome to composio", CreatedAt: baseTime},
			{AgentName: "Jack", Content: "Running composio integration", CreatedAt: baseTime.Add(time.Minute)},
			{AgentName: "ClaudiusThePirateEmperor", Content: "**Done** with integration", CreatedAt: baseTime.Add(2 * time.Minute)},
		},
		room2ID: {
			{AgentName: "ClaudiusThePirateEmperor", Content: "Analyzing solvr usage", CreatedAt: baseTime},
			{AgentName: "Mack", Content: "Usage stats look good", CreatedAt: baseTime.Add(time.Minute)},
		},
		room3ID: {
			{AgentName: "Jack", Content: "Trade bot v0 init", CreatedAt: baseTime},
			{AgentName: "Mack", Content: "Confirmed", CreatedAt: baseTime.Add(time.Minute)},
			{AgentName: "Jack", Content: "Done", CreatedAt: baseTime.Add(2 * time.Minute)},
		},
		room4ID: {
			{AgentName: "Jack", Content: "Ops check", CreatedAt: baseTime},
			{AgentName: "Mack", Content: "All clear", CreatedAt: baseTime.Add(time.Minute)},
		},
		room5ID: {
			{AgentName: "Jack", Content: "MSV trading session", CreatedAt: baseTime},
		},
	}

	// Build slugOwnerEmail overrides with the integration test emails.
	savedSlugOwnerEmail := make(map[string]string)
	for k, v := range slugOwnerEmail {
		savedSlugOwnerEmail[k] = v
	}
	// Patch the global slugOwnerEmail to use our test user emails.
	slugOwnerEmail["composio-integration"] = felipeEmail
	slugOwnerEmail["solvr-usage-analysys"] = felipeEmail
	slugOwnerEmail["ballona-trade-v0"] = marceloEmail
	slugOwnerEmail["mackjack-ops"] = marceloEmail
	slugOwnerEmail["jack-mack-msv-trading"] = marceloEmail

	t.Cleanup(func() {
		// Restore original slugOwnerEmail after test.
		for k, v := range savedSlugOwnerEmail {
			slugOwnerEmail[k] = v
		}
		// Clean up test data.
		cleanupIntegrationData(t, pool, felipeID, marceloID,
			[]uuid.UUID{room1ID, room2ID, room3ID, room4ID, room5ID})
	})

	mDB = &schemaQuorumMigrationDB{
		pool:        pool,
		quorumRooms: quorumRooms,
		quorumMsgs:  quorumMsgs,
	}

	return felipeID, marceloID, mDB
}

// cleanupIntegrationData removes test data after integration tests.
func cleanupIntegrationData(t *testing.T, pool *db.Pool, felipeID, marceloID uuid.UUID, roomIDs []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	// Delete messages for test rooms.
	for _, roomID := range roomIDs {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE room_id = $1`, roomID)
	}
	// Delete test rooms.
	for _, roomID := range roomIDs {
		_, _ = pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, roomID)
	}
	// Delete agents created for these rooms.
	for _, agentID := range []string{"agent_ClaudiusThePirateEmperor", "agent_Jack", "agent_Mack"} {
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	}
	// Delete test users.
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1 OR id = $2`, felipeID, marceloID)
}

// TestIntegration_CountAccuracy verifies that after migration exactly 5 rooms
// and the correct number of messages are present in Solvr.
func TestIntegration_CountAccuracy(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	felipeID, marceloID, mDB := seedIntegrationData(t, pool)
	_ = felipeID
	_ = marceloID

	m := &migrator{db: mDB, dryRun: false}
	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if result.RoomsMigrated != 5 {
		t.Errorf("expected 5 rooms migrated, got %d", result.RoomsMigrated)
	}

	// Count actual rooms in Solvr DB (only the 5 allowed slugs).
	var roomCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms
		WHERE slug IN ('composio-integration', 'solvr-usage-analysis', 'ballona-trade-v0',
		               'mackjack-ops', 'jack-mack-msv-trading')
	`).Scan(&roomCount)
	if err != nil {
		t.Fatalf("count rooms: %v", err)
	}
	if roomCount != 5 {
		t.Errorf("expected 5 rooms in DB, got %d", roomCount)
	}

	// Total messages: 3+2+3+2+1 = 11
	expectedMessages := 11
	if result.MessagesMigrated != expectedMessages {
		t.Errorf("expected %d messages migrated, got %d", expectedMessages, result.MessagesMigrated)
	}

	// Verify message_count on each room matches actual count.
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.message_count, COUNT(m.id) as actual_count
		FROM rooms r
		LEFT JOIN messages m ON m.room_id = r.id AND m.deleted_at IS NULL
		WHERE r.slug IN ('composio-integration', 'solvr-usage-analysis', 'ballona-trade-v0',
		                 'mackjack-ops', 'jack-mack-msv-trading')
		GROUP BY r.id, r.message_count
	`)
	if err != nil {
		t.Fatalf("query room message counts: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var roomID uuid.UUID
		var storedCount, actualCount int
		if err := rows.Scan(&roomID, &storedCount, &actualCount); err != nil {
			t.Fatalf("scan room counts: %v", err)
		}
		if storedCount != actualCount {
			t.Errorf("room %s: message_count=%d but actual count=%d", roomID, storedCount, actualCount)
		}
	}
}

// TestIntegration_OwnerMapping verifies each room's owner_id matches the correct
// Solvr user UUID resolved from email.
func TestIntegration_OwnerMapping(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	felipeID, marceloID, mDB := seedIntegrationData(t, pool)

	m := &migrator{db: mDB, dryRun: false}
	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Felipe's rooms.
	rows, err := pool.Query(ctx, `
		SELECT slug, owner_id FROM rooms
		WHERE slug IN ('composio-integration', 'solvr-usage-analysis')
	`)
	if err != nil {
		t.Fatalf("query felipe rooms: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slug string
		var ownerID uuid.UUID
		if err := rows.Scan(&slug, &ownerID); err != nil {
			t.Fatalf("scan room: %v", err)
		}
		if ownerID != felipeID {
			t.Errorf("room %s: expected Felipe's UUID %s, got %s", slug, felipeID, ownerID)
		}
	}
	rows.Close()

	// Marcelo's rooms.
	rows2, err := pool.Query(ctx, `
		SELECT slug, owner_id FROM rooms
		WHERE slug IN ('ballona-trade-v0', 'mackjack-ops', 'jack-mack-msv-trading')
	`)
	if err != nil {
		t.Fatalf("query marcelo rooms: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var slug string
		var ownerID uuid.UUID
		if err := rows2.Scan(&slug, &ownerID); err != nil {
			t.Fatalf("scan room: %v", err)
		}
		if ownerID != marceloID {
			t.Errorf("room %s: expected Marcelo's UUID %s, got %s", slug, marceloID, ownerID)
		}
	}
}

// TestIntegration_SequenceNumbers verifies messages have sequence_num 1..N with no gaps.
func TestIntegration_SequenceNumbers(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	_, _, mDB := seedIntegrationData(t, pool)

	m := &migrator{db: mDB, dryRun: false}
	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// For each migrated room, check sequence numbers are 1..N.
	rows, err := pool.Query(ctx, `
		SELECT r.id, r.slug, m.sequence_num
		FROM rooms r
		JOIN messages m ON m.room_id = r.id
		WHERE r.slug IN ('composio-integration', 'solvr-usage-analysis', 'ballona-trade-v0',
		                 'mackjack-ops', 'jack-mack-msv-trading')
		ORDER BY r.slug, m.sequence_num
	`)
	if err != nil {
		t.Fatalf("query messages with sequence: %v", err)
	}
	defer rows.Close()

	// Track sequence numbers per room.
	roomSeqs := make(map[string][]int)
	for rows.Next() {
		var roomID uuid.UUID
		var slug string
		var seqNum int
		if err := rows.Scan(&roomID, &slug, &seqNum); err != nil {
			t.Fatalf("scan sequence: %v", err)
		}
		roomSeqs[slug] = append(roomSeqs[slug], seqNum)
	}

	// Verify each room's sequences are 1..N with no gaps.
	for slug, seqs := range roomSeqs {
		for i, seq := range seqs {
			expected := i + 1
			if seq != expected {
				t.Errorf("room %s: sequence[%d]=%d, want %d (no gaps)", slug, i, seq, expected)
			}
		}
	}
}

// TestIntegration_SkippedRoomsExcluded verifies that rooms not in allowedSlugs
// do NOT appear in Solvr rooms table.
func TestIntegration_SkippedRoomsExcluded(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	_, _, mDB := seedIntegrationData(t, pool)

	m := &migrator{db: mDB, dryRun: false}
	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// The skipped rooms should NOT be in Solvr.
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms
		WHERE slug IN ('some-test-room', 'another-random-room')
	`).Scan(&count)
	if err != nil {
		t.Fatalf("count skipped rooms: %v", err)
	}
	if count > 0 {
		t.Errorf("expected 0 skipped rooms in DB, found %d", count)
	}

	// Only the 5 allowed slugs should be present.
	var allowedCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms
		WHERE slug IN ('composio-integration', 'solvr-usage-analysis', 'ballona-trade-v0',
		               'mackjack-ops', 'jack-mack-msv-trading')
	`).Scan(&allowedCount)
	if err != nil {
		t.Fatalf("count allowed rooms: %v", err)
	}
	if allowedCount != 5 {
		t.Errorf("expected exactly 5 allowed rooms, found %d", allowedCount)
	}
}

// TestIntegration_Idempotent verifies running migration twice produces same counts.
func TestIntegration_Idempotent(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	_, _, mDB := seedIntegrationData(t, pool)

	m := &migrator{db: mDB, dryRun: false}

	// First run.
	result1, err := m.run(ctx)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}
	if result1.RoomsMigrated != 5 {
		t.Errorf("first run: expected 5 rooms migrated, got %d", result1.RoomsMigrated)
	}

	// Second run — should succeed with 0 new rooms (all ON CONFLICT DO NOTHING).
	result2, err := m.run(ctx)
	if err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}
	if result2.RoomsMigrated != 0 {
		t.Errorf("second run: expected 0 new rooms (idempotent), got %d", result2.RoomsMigrated)
	}

	// DB should still have exactly 5 rooms, not 10.
	var roomCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms
		WHERE slug IN ('composio-integration', 'solvr-usage-analysis', 'ballona-trade-v0',
		               'mackjack-ops', 'jack-mack-msv-trading')
	`).Scan(&roomCount)
	if err != nil {
		t.Fatalf("count rooms after idempotent run: %v", err)
	}
	if roomCount != 5 {
		t.Errorf("expected 5 rooms after idempotent second run, got %d", roomCount)
	}
}

// TestIntegration_SlugTransform verifies "solvr-usage-analysis" exists (not the typo).
func TestIntegration_SlugTransform(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	_, _, mDB := seedIntegrationData(t, pool)

	m := &migrator{db: mDB, dryRun: false}
	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Target slug should be "solvr-usage-analysis" (fixed typo).
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM rooms WHERE slug = 'solvr-usage-analysis'`).Scan(&count)
	if err != nil {
		t.Fatalf("check slug transform: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 room with slug 'solvr-usage-analysis', got %d", count)
	}

	// Original typo slug should NOT exist.
	var typoCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM rooms WHERE slug = 'solvr-usage-analysys'`).Scan(&typoCount)
	if err != nil {
		t.Fatalf("check typo slug: %v", err)
	}
	if typoCount > 0 {
		t.Errorf("typo slug 'solvr-usage-analysys' should not exist in Solvr, but found %d", typoCount)
	}
}
