package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage's featured pool (SPEC Part 26, "Featured rooms"): the operator
// curates it, the API shows its public rooms and quotes what each room set out
// to do (ask) and what came out of it (outcome).

func featuredTestPool(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func featuredTestRoom(t *testing.T, ctx context.Context, pool *db.Pool, prefix string) (string, uuid.UUID) {
	t.Helper()
	slug := prefix + "-" + uuid.NewString()[:8]
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})
	return slug, roomID
}

func featuredSay(t *testing.T, ctx context.Context, repo *db.MessageRepository, roomID uuid.UUID, authorType, name, content string) *models.Message {
	t.Helper()
	m, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: authorType, AgentName: name, Content: content, ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create message %q: %v", content, err)
	}
	return m
}

func featuredSlugs(rooms []db.FeaturedRoom) map[string]db.FeaturedRoom {
	out := make(map[string]db.FeaturedRoom, len(rooms))
	for _, r := range rooms {
		out[r.Slug] = r
	}
	return out
}

func TestFeaturedRooms_PoolHoldsOnlyPublicLiveRooms(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	slug, _ := featuredTestRoom(t, ctx, pool, "featpool")

	first, err := repo.Feature(ctx, slug, nil, nil)
	if err != nil {
		t.Fatalf("Feature: %v", err)
	}
	if first.Slug != slug || first.AskSeq != nil || first.OutcomeSeq != nil {
		t.Fatalf("unexpected featured row %+v", first)
	}

	list, err := repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if _, ok := featuredSlugs(list)[slug]; !ok {
		t.Fatalf("featured public room %s missing from the pool", slug)
	}

	// Featuring again names the quoted messages and keeps the original place in the pool.
	ask, outcome := 2, 5
	again, err := repo.Feature(ctx, slug, &ask, &outcome)
	if err != nil {
		t.Fatalf("Feature again: %v", err)
	}
	if again.AskSeq == nil || *again.AskSeq != 2 || again.OutcomeSeq == nil || *again.OutcomeSeq != 5 {
		t.Fatalf("overrides not stored: %+v", again)
	}
	if !again.FeaturedAt.Equal(first.FeaturedAt) {
		t.Fatalf("featured_at moved on update: %v -> %v", first.FeaturedAt, again.FeaturedAt)
	}

	// A room that turns private leaves the pool on the next read.
	if _, err := pool.Exec(ctx, `UPDATE rooms SET is_private = true WHERE slug = $1`, slug); err != nil {
		t.Fatalf("make private: %v", err)
	}
	list, err = repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic after private: %v", err)
	}
	if _, ok := featuredSlugs(list)[slug]; ok {
		t.Fatalf("private room %s still listed", slug)
	}

	// So does a deleted one.
	if _, err := pool.Exec(ctx, `UPDATE rooms SET is_private = false, deleted_at = now() WHERE slug = $1`, slug); err != nil {
		t.Fatalf("delete room: %v", err)
	}
	list, err = repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic after delete: %v", err)
	}
	if _, ok := featuredSlugs(list)[slug]; ok {
		t.Fatalf("deleted room %s still listed", slug)
	}
}

func TestFeaturedRooms_FeatureRefusesRoomsItMayNotShow(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)

	if _, err := repo.Feature(ctx, "no-such-room-"+uuid.NewString()[:8], nil, nil); !errors.Is(err, db.ErrRoomNotFound) {
		t.Fatalf("missing room: want ErrRoomNotFound, got %v", err)
	}

	slug, _ := featuredTestRoom(t, ctx, pool, "featpriv")
	if _, err := pool.Exec(ctx, `UPDATE rooms SET is_private = true WHERE slug = $1`, slug); err != nil {
		t.Fatalf("make private: %v", err)
	}
	if _, err := repo.Feature(ctx, slug, nil, nil); !errors.Is(err, db.ErrRoomNotFound) {
		t.Fatalf("private room: want ErrRoomNotFound, got %v", err)
	}
}

func TestFeaturedRooms_UnfeatureRemovesTheRoom(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	slug, _ := featuredTestRoom(t, ctx, pool, "featun")

	if _, err := repo.Feature(ctx, slug, nil, nil); err != nil {
		t.Fatalf("Feature: %v", err)
	}
	removed, err := repo.Unfeature(ctx, slug)
	if err != nil || !removed {
		t.Fatalf("Unfeature: removed=%v err=%v", removed, err)
	}
	removed, err = repo.Unfeature(ctx, slug)
	if err != nil || removed {
		t.Fatalf("second Unfeature: removed=%v err=%v (want false, nil)", removed, err)
	}
	list, err := repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if _, ok := featuredSlugs(list)[slug]; ok {
		t.Fatalf("unfeatured room %s still listed", slug)
	}
}

func TestFeaturedRooms_BookendsFallBackToFirstAndLastMessages(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	msgs := db.NewMessageRepository(pool)
	_, roomID := featuredTestRoom(t, ctx, pool, "featbook")

	featuredSay(t, ctx, msgs, roomID, "system", "", "planner joined")
	first := featuredSay(t, ctx, msgs, roomID, "agent", "planner", "Build the parser")
	featuredSay(t, ctx, msgs, roomID, "agent", "executor", "on it")
	last := featuredSay(t, ctx, msgs, roomID, "agent", "executor", "Parser shipped, tests green")
	featuredSay(t, ctx, msgs, roomID, "system", "", "executor left")

	ask, outcome, err := repo.FindRoomBookends(ctx, roomID, nil, nil)
	if err != nil {
		t.Fatalf("FindRoomBookends: %v", err)
	}
	if ask == nil || ask.ID != first.ID {
		t.Fatalf("ask: want the first non-system message %d, got %+v", first.ID, ask)
	}
	if outcome == nil || outcome.ID != last.ID {
		t.Fatalf("outcome: want the last non-system message %d, got %+v", last.ID, outcome)
	}
}

func TestFeaturedRooms_BookendsPreferPinnedEntries(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	msgs := db.NewMessageRepository(pool)
	_, roomID := featuredTestRoom(t, ctx, pool, "featpin")

	featuredSay(t, ctx, msgs, roomID, "agent", "planner", "hello, room open")
	directive := featuredSay(t, ctx, msgs, roomID, "agent", "planner", "DIRECTIVE: build the parser")
	result := featuredSay(t, ctx, msgs, roomID, "agent", "executor", "RESULT: parser shipped")
	featuredSay(t, ctx, msgs, roomID, "agent", "executor", "signing off")
	for _, id := range []int64{directive.ID, result.ID} {
		if _, err := msgs.Pin(ctx, roomID, id); err != nil {
			t.Fatalf("Pin %d: %v", id, err)
		}
	}

	ask, outcome, err := repo.FindRoomBookends(ctx, roomID, nil, nil)
	if err != nil {
		t.Fatalf("FindRoomBookends: %v", err)
	}
	if ask == nil || ask.ID != directive.ID {
		t.Fatalf("ask: want the pinned directive %d, got %+v", directive.ID, ask)
	}
	if outcome == nil || outcome.ID != result.ID {
		t.Fatalf("outcome: want the pinned result %d (not the sign-off), got %+v", result.ID, outcome)
	}
}

func TestFeaturedRooms_BookendsHonourTheOperatorsChoice(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	msgs := db.NewMessageRepository(pool)
	_, roomID := featuredTestRoom(t, ctx, pool, "featseq")

	featuredSay(t, ctx, msgs, roomID, "agent", "jack", "hey everyone, just joined")
	plan := featuredSay(t, ctx, msgs, roomID, "agent", "mack", "Plan: compare two short-selling strategies")
	finding := featuredSay(t, ctx, msgs, roomID, "agent", "jack", "Finding: strategy B halves the drawdown")
	featuredSay(t, ctx, msgs, roomID, "agent", "jack", "signing off")

	askSeq, outcomeSeq := *plan.SequenceNum, *finding.SequenceNum
	ask, outcome, err := repo.FindRoomBookends(ctx, roomID, &askSeq, &outcomeSeq)
	if err != nil {
		t.Fatalf("FindRoomBookends: %v", err)
	}
	if ask == nil || ask.ID != plan.ID || outcome == nil || outcome.ID != finding.ID {
		t.Fatalf("want the named messages %d/%d, got %+v / %+v", plan.ID, finding.ID, ask, outcome)
	}

	// Naming the same message twice never quotes it twice: the outcome falls back.
	ask, outcome, err = repo.FindRoomBookends(ctx, roomID, &askSeq, &askSeq)
	if err != nil {
		t.Fatalf("FindRoomBookends same seq: %v", err)
	}
	if ask == nil || outcome == nil || ask.ID == outcome.ID {
		t.Fatalf("ask and outcome must differ, got %+v / %+v", ask, outcome)
	}
}

func TestFeaturedRooms_SingleMessageRoomHasNoOutcome(t *testing.T) {
	pool, ctx := featuredTestPool(t)
	repo := db.NewFeaturedRoomRepository(pool)
	msgs := db.NewMessageRepository(pool)
	_, roomID := featuredTestRoom(t, ctx, pool, "featone")

	only := featuredSay(t, ctx, msgs, roomID, "agent", "planner", "Room is open")
	ask, outcome, err := repo.FindRoomBookends(ctx, roomID, nil, nil)
	if err != nil {
		t.Fatalf("FindRoomBookends: %v", err)
	}
	if ask == nil || ask.ID != only.ID || outcome != nil {
		t.Fatalf("want ask=%d and no outcome, got %+v / %+v", only.ID, ask, outcome)
	}
}
