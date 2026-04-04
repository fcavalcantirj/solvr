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

// msgTestCleanup removes test data for message tests.
func msgTestCleanup(ctx context.Context, pool *db.Pool, roomSlug string) {
	pool.Exec(ctx, "DELETE FROM messages WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug)       //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug) //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM rooms WHERE slug = $1", roomSlug)                                                  //nolint:errcheck
}

// createTestRoom creates a room for message tests and returns its ID.
func createTestRoom(t *testing.T, ctx context.Context, pool *db.Pool, slug string) uuid.UUID {
	t.Helper()
	msgTestCleanup(ctx, pool, slug)
	_, err := pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash)
		VALUES ($1, 'Message Test Room', 'hash_msg_test')
	`, slug)
	if err != nil {
		t.Fatalf("createTestRoom: %v", err)
	}
	var roomID uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = $1`, slug).Scan(&roomID)
	if err != nil {
		t.Fatalf("createTestRoom get ID: %v", err)
	}
	return roomID
}

func TestMessageRepository_Create(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testmsgcr-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewMessageRepository(pool)

	t.Run("creates message with sequence_num=1 for first message", func(t *testing.T) {
		params := models.CreateMessageParams{
			RoomID:      roomID,
			AuthorType:  "agent",
			AgentName:   "test-agent",
			Content:     "Hello world",
			ContentType: "text",
			Metadata:    json.RawMessage(`{}`),
		}
		msg, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if msg.ID == 0 {
			t.Error("Create() returned ID=0")
		}
		if msg.SequenceNum == nil || *msg.SequenceNum != 1 {
			t.Errorf("SequenceNum = %v, want 1", msg.SequenceNum)
		}
		if msg.Content != "Hello world" {
			t.Errorf("Content = %q, want %q", msg.Content, "Hello world")
		}
	})

	t.Run("creates second message with sequence_num=2", func(t *testing.T) {
		params := models.CreateMessageParams{
			RoomID:      roomID,
			AuthorType:  "agent",
			AgentName:   "test-agent",
			Content:     "Second message",
			ContentType: "text",
			Metadata:    json.RawMessage(`{}`),
		}
		msg, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if msg.SequenceNum == nil || *msg.SequenceNum != 2 {
			t.Errorf("SequenceNum = %v, want 2", msg.SequenceNum)
		}
	})
}

func TestMessageRepository_ListAfter(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testmsgla-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewMessageRepository(pool)

	// Create 3 messages
	var firstID int64
	for i := 0; i < 3; i++ {
		msg, err := repo.Create(ctx, models.CreateMessageParams{
			RoomID:      roomID,
			AuthorType:  "agent",
			AgentName:   "test-agent",
			Content:     "Message " + time.Now().Format("150405.000"),
			ContentType: "text",
			Metadata:    json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if i == 0 {
			firstID = msg.ID
		}
	}

	t.Run("afterID=0 returns all messages", func(t *testing.T) {
		msgs, err := repo.ListAfter(ctx, roomID, 0, 100)
		if err != nil {
			t.Fatalf("ListAfter() error = %v", err)
		}
		if len(msgs) < 3 {
			t.Errorf("ListAfter() returned %d messages, want >= 3", len(msgs))
		}
	})

	t.Run("afterID=firstMsgID returns only subsequent messages", func(t *testing.T) {
		msgs, err := repo.ListAfter(ctx, roomID, firstID, 100)
		if err != nil {
			t.Fatalf("ListAfter() error = %v", err)
		}
		if len(msgs) != 2 {
			t.Errorf("ListAfter() returned %d messages, want 2", len(msgs))
		}
		// Verify ordering is ASC by id
		for i := 1; i < len(msgs); i++ {
			if msgs[i].ID <= msgs[i-1].ID {
				t.Error("ListAfter() messages not ordered by id ASC")
			}
		}
	})
}

func TestMessageRepository_ListRecent(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testmsglr-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewMessageRepository(pool)

	// Create 5 messages
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, models.CreateMessageParams{
			RoomID:      roomID,
			AuthorType:  "agent",
			AgentName:   "test-agent",
			Content:     "Recent msg " + time.Now().Format("150405.000"),
			ContentType: "text",
			Metadata:    json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	t.Run("returns up to limit most recent messages in chronological order", func(t *testing.T) {
		msgs, err := repo.ListRecent(ctx, roomID, 3)
		if err != nil {
			t.Fatalf("ListRecent() error = %v", err)
		}
		if len(msgs) != 3 {
			t.Errorf("ListRecent() returned %d messages, want 3", len(msgs))
		}
		// Verify chronological order (ASC by id after reversing)
		for i := 1; i < len(msgs); i++ {
			if msgs[i].ID <= msgs[i-1].ID {
				t.Error("ListRecent() messages not in chronological order")
			}
		}
	})
}
