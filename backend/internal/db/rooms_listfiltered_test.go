package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// containsRoomID reports whether the listing includes a room with the given id.
func containsRoomID(rooms []models.RoomWithStats, id uuid.UUID) bool {
	for _, r := range rooms {
		if r.ID == id {
			return true
		}
	}
	return false
}

// TestRoomRepository_ListFiltered covers the public-discovery listing behaviour
// required by the "focused public discovery page" mission: Recent vs Active-now
// sorting, free-text search, exclusion of expired / empty-abandoned / archived
// rooms, an include-archived escape hatch, and a last-message preview.
func TestRoomRepository_ListFiltered(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomlf"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// mkRoom creates a public room with the given display name and returns it.
	mkRoom := func(t *testing.T, name string) *models.Room {
		t.Helper()
		room, err := repo.Create(ctx, models.CreateRoomParams{
			Slug:        prefix + "-" + uuid.New().String()[:8],
			DisplayName: name,
			OwnerID:     uuid.Nil,
		})
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		return room
	}

	t.Run("sort=active orders rooms with live agents first", func(t *testing.T) {
		token := "activesort" + time.Now().Format("150405.000")
		idle := mkRoom(t, "Idle "+token)
		active := mkRoom(t, "Active "+token)
		// Give the active room a live agent (non-expired presence).
		presentID := admitPresenceMember(ctx, t, pool, active.ID, "agent_lf_present_"+uuid.New().String()[:8])
		if _, err := pool.Exec(ctx, `
			INSERT INTO agent_presence (room_id, agent_id, agent_name, card_json, last_seen, ttl_seconds)
			VALUES ($1, $2, 'presence-agent', '{}', NOW(), 300)
		`, active.ID, presentID); err != nil {
			t.Fatalf("insert presence error = %v", err)
		}
		// Make the idle room the more recent one so only the sort key decides order.
		if _, err := pool.Exec(ctx, `UPDATE rooms SET last_active_at = NOW() WHERE id = $1`, idle.ID); err != nil {
			t.Fatalf("bump idle error = %v", err)
		}

		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Sort: "active", Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if len(rooms) != 2 {
			t.Fatalf("expected 2 rooms for token, got %d", len(rooms))
		}
		if rooms[0].ID != active.ID {
			t.Errorf("sort=active: first room = %q, want the room with a live agent", rooms[0].DisplayName)
		}
		if rooms[0].LiveAgentCount != 1 {
			t.Errorf("active room LiveAgentCount = %d, want 1", rooms[0].LiveAgentCount)
		}
	})

	t.Run("sort=recent (default) orders by last activity", func(t *testing.T) {
		token := "recentsort" + time.Now().Format("150405.000")
		older := mkRoom(t, "Older "+token)
		newer := mkRoom(t, "Newer "+token)
		// Keep "older" within the empty-abandoned window (< 1h) so only the recency
		// sort — not the abandoned filter — decides whether it appears.
		if _, err := pool.Exec(ctx, `UPDATE rooms SET last_active_at = NOW() - INTERVAL '10 minutes' WHERE id = $1`, older.ID); err != nil {
			t.Fatalf("age older error = %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE rooms SET last_active_at = NOW() WHERE id = $1`, newer.ID); err != nil {
			t.Fatalf("bump newer error = %v", err)
		}

		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if len(rooms) != 2 || rooms[0].ID != newer.ID {
			t.Fatalf("sort=recent: expected newer room first, got %+v", rooms)
		}
	})

	t.Run("search filters by name", func(t *testing.T) {
		token := "searchtok" + time.Now().Format("150405.000")
		match := mkRoom(t, "Findable "+token)
		mkRoom(t, "Unrelated room name")

		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if !containsRoomID(rooms, match.ID) {
			t.Error("search did not return the matching room")
		}
		for _, r := range rooms {
			if r.DisplayName == "Unrelated room name" {
				t.Error("search returned a non-matching room")
			}
		}
	})

	t.Run("excludes expired rooms", func(t *testing.T) {
		token := "expiredtok" + time.Now().Format("150405.000")
		expired := mkRoom(t, "Expired "+token)
		if _, err := pool.Exec(ctx, `UPDATE rooms SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, expired.ID); err != nil {
			t.Fatalf("expire error = %v", err)
		}
		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if containsRoomID(rooms, expired.ID) {
			t.Error("ListFiltered returned an expired room")
		}
	})

	t.Run("excludes empty abandoned rooms but keeps content or fresh rooms", func(t *testing.T) {
		token := "abandontok" + time.Now().Format("150405.000")
		abandoned := mkRoom(t, "Abandoned "+token)
		withMsg := mkRoom(t, "HasMessage "+token)
		fresh := mkRoom(t, "Fresh "+token)

		// abandoned: no messages, no presence, activity 2h ago -> excluded
		if _, err := pool.Exec(ctx, `UPDATE rooms SET message_count = 0, last_active_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, abandoned.ID); err != nil {
			t.Fatalf("age abandoned error = %v", err)
		}
		// withMsg: has a message + old activity -> kept (content is valuable)
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, agent_name, content)
			VALUES ($1, 'agent', 'a', 'hello there')
		`, withMsg.ID); err != nil {
			t.Fatalf("insert message error = %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE rooms SET message_count = 1, last_active_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, withMsg.ID); err != nil {
			t.Fatalf("age withMsg error = %v", err)
		}
		// fresh: no messages but just created -> not abandoned, kept

		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if containsRoomID(rooms, abandoned.ID) {
			t.Error("ListFiltered returned an empty abandoned room")
		}
		if !containsRoomID(rooms, withMsg.ID) {
			t.Error("ListFiltered dropped an old room that has messages")
		}
		if !containsRoomID(rooms, fresh.ID) {
			t.Error("ListFiltered dropped a freshly created empty room")
		}
	})

	t.Run("archived excluded by default, included via flag and via search", func(t *testing.T) {
		token := "archivedtok" + time.Now().Format("150405.000")
		archived := mkRoom(t, "Archived "+token)
		// Give it a message so the empty-abandoned rule cannot also hide it.
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, agent_name, content)
			VALUES ($1, 'agent', 'a', 'archived content')
		`, archived.ID); err != nil {
			t.Fatalf("insert message error = %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE rooms SET message_count = 1, archived_at = NOW() WHERE id = $1`, archived.ID); err != nil {
			t.Fatalf("archive error = %v", err)
		}

		// Default browse (no search, no include flag) hides archived rooms.
		def, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 100})
		if err != nil {
			t.Fatalf("ListFiltered(default) error = %v", err)
		}
		if containsRoomID(def, archived.ID) {
			t.Error("default listing exposed an archived room")
		}

		// include-archived surfaces it.
		inc, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 100, IncludeArchived: true})
		if err != nil {
			t.Fatalf("ListFiltered(include) error = %v", err)
		}
		if !containsRoomID(inc, archived.ID) {
			t.Error("include-archived did not surface the archived room")
		}

		// search surfaces it too.
		found, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 100, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered(search) error = %v", err)
		}
		if !containsRoomID(found, archived.ID) {
			t.Error("search did not surface the archived room")
		}
	})

	t.Run("populates last_message_preview from the latest message", func(t *testing.T) {
		token := "previewtok" + time.Now().Format("150405.000")
		room := mkRoom(t, "Preview "+token)
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, agent_name, content, created_at)
			VALUES ($1, 'agent', 'a', 'an older message', NOW() - INTERVAL '5 minutes')
		`, room.ID); err != nil {
			t.Fatalf("insert old message error = %v", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, agent_name, content, created_at)
			VALUES ($1, 'agent', 'a', 'the latest message', NOW())
		`, room.ID); err != nil {
			t.Fatalf("insert latest message error = %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE rooms SET message_count = 2 WHERE id = $1`, room.ID); err != nil {
			t.Fatalf("count error = %v", err)
		}

		rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Limit: 50, Query: token})
		if err != nil {
			t.Fatalf("ListFiltered() error = %v", err)
		}
		if len(rooms) != 1 {
			t.Fatalf("expected 1 room, got %d", len(rooms))
		}
		if rooms[0].LastMessagePreview == nil || *rooms[0].LastMessagePreview != "the latest message" {
			t.Errorf("LastMessagePreview = %v, want %q", rooms[0].LastMessagePreview, "the latest message")
		}
	})

	t.Run("List delegates to ListFiltered with recent defaults", func(t *testing.T) {
		token := "delegatetok" + time.Now().Format("150405.000")
		want := mkRoom(t, "Delegate "+token)
		rooms, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if !containsRoomID(rooms, want.ID) {
			t.Error("List() did not return a freshly created public room")
		}
	})
}
