package db_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestRoomEventRepository_CreateAndQuery(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-events", false)
	repo := db.NewRoomEventRepository(pool)

	mk := func(typ, issue, actor string) {
		if _, err := repo.Create(ctx, models.CreateRoomEventParams{
			RoomID: room.ID, EventType: typ, Issue: issue, Actor: actor,
			Payload: json.RawMessage(`{"k":"v"}`),
		}); err != nil {
			t.Fatalf("Create(%s,%s): %v", typ, issue, err)
		}
	}
	mk("CLAIM", "APP-185", "worker-1")
	mk("BUILDING", "APP-185", "worker-1")
	mk("CLAIM", "APP-999", "worker-2")
	mk("RELEASE", "", "bart") // no issue

	// No filter -> all 4, newest first.
	all, err := repo.Query(ctx, models.QueryRoomEventsParams{RoomID: room.ID})
	if err != nil {
		t.Fatalf("Query all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("Query all len = %d; want 4", len(all))
	}
	if all[0].EventType != "RELEASE" {
		t.Fatalf("newest event = %q; want RELEASE (newest first)", all[0].EventType)
	}

	// Filter by type.
	claims, err := repo.Query(ctx, models.QueryRoomEventsParams{RoomID: room.ID, EventType: "CLAIM"})
	if err != nil {
		t.Fatalf("Query type: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("CLAIM events = %d; want 2", len(claims))
	}

	// Filter by issue.
	byIssue, err := repo.Query(ctx, models.QueryRoomEventsParams{RoomID: room.ID, Issue: "APP-185"})
	if err != nil {
		t.Fatalf("Query issue: %v", err)
	}
	if len(byIssue) != 2 {
		t.Fatalf("APP-185 events = %d; want 2", len(byIssue))
	}

	// Filter by type AND issue.
	both, err := repo.Query(ctx, models.QueryRoomEventsParams{RoomID: room.ID, EventType: "CLAIM", Issue: "APP-185"})
	if err != nil {
		t.Fatalf("Query type+issue: %v", err)
	}
	if len(both) != 1 || both[0].Actor != "worker-1" {
		t.Fatalf("CLAIM+APP-185 = %+v; want single worker-1 event", both)
	}

	// LatestByIssue returns the newest for that issue (BUILDING, since it came after CLAIM).
	latest, err := repo.LatestByIssue(ctx, room.ID, "APP-185")
	if err != nil {
		t.Fatalf("LatestByIssue: %v", err)
	}
	if latest == nil || latest.EventType != "BUILDING" {
		t.Fatalf("LatestByIssue(APP-185) = %+v; want BUILDING", latest)
	}
}
