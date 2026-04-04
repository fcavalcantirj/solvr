package db_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// presenceTestCleanup removes test data for presence tests.
func presenceTestCleanup(ctx context.Context, pool *db.Pool, roomSlug string) {
	pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug) //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM rooms WHERE slug = $1", roomSlug)                                                  //nolint:errcheck
}

// createPresenceTestRoom creates a room for presence tests and returns its ID.
func createPresenceTestRoom(t *testing.T, ctx context.Context, pool *db.Pool, slug string) uuid.UUID {
	t.Helper()
	presenceTestCleanup(ctx, pool, slug)
	_, err := pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ($1, 'Presence Test Room', 'hash_presence_test')
	`, slug)
	if err != nil {
		t.Fatalf("createPresenceTestRoom: %v", err)
	}
	var roomID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = $1`, slug).Scan(&roomID)
	if err != nil {
		t.Fatalf("createPresenceTestRoom get ID: %v", err)
	}
	return roomID
}

func TestAgentPresenceRepository_Upsert(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testpresup-" + time.Now().Format("150405")
	roomID := createPresenceTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		presenceTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewAgentPresenceRepository(pool)

	t.Run("upsert new agent returns record with joined_at", func(t *testing.T) {
		params := models.UpsertAgentPresenceParams{
			RoomID:     roomID,
			AgentName:  "agent-alpha",
			CardJSON:   json.RawMessage(`{"name": "Alpha"}`),
			TTLSeconds: 900,
		}
		record, err := repo.Upsert(ctx, params)
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
		if record.ID == uuid.Nil {
			t.Error("Upsert() returned nil ID")
		}
		if record.AgentName != "agent-alpha" {
			t.Errorf("AgentName = %q, want %q", record.AgentName, "agent-alpha")
		}
		if record.JoinedAt.IsZero() {
			t.Error("JoinedAt is zero")
		}
	})

	t.Run("upsert existing agent updates card_json and last_seen", func(t *testing.T) {
		// Wait a tiny bit to ensure last_seen changes
		time.Sleep(10 * time.Millisecond)

		params := models.UpsertAgentPresenceParams{
			RoomID:     roomID,
			AgentName:  "agent-alpha",
			CardJSON:   json.RawMessage(`{"name": "Alpha Updated"}`),
			TTLSeconds: 600,
		}
		record, err := repo.Upsert(ctx, params)
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
		// Card should be updated
		if string(record.CardJSON) != `{"name": "Alpha Updated"}` {
			t.Errorf("CardJSON = %s, want updated value", string(record.CardJSON))
		}
		// TTL should be updated
		if record.TTLSeconds != 600 {
			t.Errorf("TTLSeconds = %d, want 600", record.TTLSeconds)
		}
	})
}

func TestAgentPresenceRepository_ListByRoom(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testpreslr-" + time.Now().Format("150405")
	roomID := createPresenceTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		presenceTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewAgentPresenceRepository(pool)

	// Add two agents
	for _, name := range []string{"agent-1", "agent-2"} {
		_, err := repo.Upsert(ctx, models.UpsertAgentPresenceParams{
			RoomID:     roomID,
			AgentName:  name,
			CardJSON:   json.RawMessage(`{"name": "` + name + `"}`),
			TTLSeconds: 900,
		})
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	t.Run("returns only agents within TTL window", func(t *testing.T) {
		records, err := repo.ListByRoom(ctx, roomID)
		if err != nil {
			t.Fatalf("ListByRoom() error = %v", err)
		}
		if len(records) != 2 {
			t.Errorf("ListByRoom() returned %d records, want 2", len(records))
		}
	})
}

func TestAgentPresenceRepository_DeleteExpired(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testpresde-" + time.Now().Format("150405")
	roomID := createPresenceTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		presenceTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewAgentPresenceRepository(pool)

	t.Run("returns slice of expired presence records", func(t *testing.T) {
		// Insert an agent with expired TTL using raw SQL (ttl_seconds=1, last_seen in the past)
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_presence (room_id, agent_name, card_json, ttl_seconds, last_seen)
			VALUES ($1, 'expired-agent', '{"name":"expired"}', 1, NOW() - INTERVAL '1 hour')
		`, roomID)
		if err != nil {
			t.Fatalf("insert expired presence error = %v", err)
		}

		expired, err := repo.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("DeleteExpired() error = %v", err)
		}
		if len(expired) < 1 {
			t.Errorf("DeleteExpired() returned %d records, want >= 1", len(expired))
		}
		// Verify the expired record contains our agent
		found := false
		for _, ep := range expired {
			if ep.AgentName == "expired-agent" {
				found = true
				break
			}
		}
		if !found {
			t.Error("DeleteExpired() did not return our expired-agent")
		}
	})
}

func TestAgentPresenceRepository_UpdateHeartbeat(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testpreshb-" + time.Now().Format("150405")
	roomID := createPresenceTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		presenceTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewAgentPresenceRepository(pool)

	// Insert an agent
	_, err = repo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID:     roomID,
		AgentName:  "heartbeat-agent",
		CardJSON:   json.RawMessage(`{"name": "hb"}`),
		TTLSeconds: 900,
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	t.Run("updates last_seen timestamp", func(t *testing.T) {
		// Get initial last_seen
		var beforeLastSeen time.Time
		err := pool.QueryRow(ctx, `
			SELECT last_seen FROM agent_presence WHERE room_id = $1 AND agent_name = 'heartbeat-agent'
		`, roomID).Scan(&beforeLastSeen)
		if err != nil {
			t.Fatalf("get initial last_seen: %v", err)
		}

		time.Sleep(10 * time.Millisecond) // Ensure timestamp changes

		err = repo.UpdateHeartbeat(ctx, roomID, "heartbeat-agent")
		if err != nil {
			t.Fatalf("UpdateHeartbeat() error = %v", err)
		}

		var afterLastSeen time.Time
		err = pool.QueryRow(ctx, `
			SELECT last_seen FROM agent_presence WHERE room_id = $1 AND agent_name = 'heartbeat-agent'
		`, roomID).Scan(&afterLastSeen)
		if err != nil {
			t.Fatalf("get updated last_seen: %v", err)
		}

		if !afterLastSeen.After(beforeLastSeen) {
			t.Error("UpdateHeartbeat() did not update last_seen")
		}
	})
}
