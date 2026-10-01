package db

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Conditional edits (spec.json idx 74 step 5): an edit carries the version the
// client last read, and the write itself refuses a row that moved since, so two
// writers holding the same version cannot both win (the lost update a separate
// read-then-write check lets through under concurrency). A nil expected version
// is an unconditional write (If-Match: *). Integration tests: DATABASE_URL must
// point at a local test database.

// conditionalWriters runs n concurrent writes and counts how many succeeded and
// how many were refused as a version conflict; any other error fails the test.
func conditionalWriters(t *testing.T, n int, write func(i int) error) (won, conflicted int) {
	t.Helper()
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			err := write(i)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				won++
			case errors.Is(err, models.ErrVersionConflict):
				conflicted++
			default:
				t.Errorf("writer %d: unexpected error %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	return won, conflicted
}

// assertVersionConflict checks err is a version conflict that reports the row's
// current version, so the API can hand the client the ETag to refetch with.
func assertVersionConflict(t *testing.T, err error, current time.Time) {
	t.Helper()
	var vc *models.VersionConflictError
	if !errors.As(err, &vc) || !errors.Is(err, models.ErrVersionConflict) {
		t.Fatalf("err = %v, want a *models.VersionConflictError", err)
	}
	if !vc.Current.Equal(current) {
		t.Errorf("conflict reports version %v, want the stored %v", vc.Current, current)
	}
}

func TestConditionalEdits_PostWritesOnlyAtTheExpectedVersion(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewPostRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", post.ID) })
	read := post.UpdatedAt

	first := *post
	first.Title = "Conditional edit at the read version"
	written, err := repo.UpdateIfUnmodified(ctx, &first, &read)
	if err != nil {
		t.Fatalf("edit at the read version: %v", err)
	}
	if !written.UpdatedAt.After(read) {
		t.Fatalf("updated_at did not advance: %v -> %v", read, written.UpdatedAt)
	}

	stale := *post
	stale.Title = "Stale edit that must not land"
	_, err = repo.UpdateIfUnmodified(ctx, &stale, &read)
	assertVersionConflict(t, err, written.UpdatedAt)
	stored, err := repo.FindByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Title != first.Title || !stored.UpdatedAt.Equal(written.UpdatedAt) {
		t.Errorf("a refused edit changed the row: title %q updated_at %v", stored.Title, stored.UpdatedAt)
	}

	anyVersion := *post
	anyVersion.Title = "Unconditional edit with no expected version"
	if _, err := repo.UpdateIfUnmodified(ctx, &anyVersion, nil); err != nil {
		t.Fatalf("unconditional edit: %v", err)
	}

	missing := *post
	missing.ID = uuid.NewString()
	if _, err := repo.UpdateIfUnmodified(ctx, &missing, &read); !errors.Is(err, ErrPostNotFound) {
		t.Errorf("absent post err = %v, want ErrPostNotFound (not a version conflict)", err)
	}
}

func TestConditionalEdits_PostConcurrentWritersAtOneVersionHaveOneWinner(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	repo := NewPostRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", post.ID) })
	read := post.UpdatedAt

	won, conflicted := conditionalWriters(t, 16, func(i int) error {
		edit := *post
		edit.Title = "Concurrent writer edit number " + time.Now().Format("150405.000000")
		_, err := repo.UpdateIfUnmodified(context.Background(), &edit, &read)
		return err
	})
	if won != 1 || conflicted != 15 {
		t.Errorf("16 writers at one version: %d won, %d conflicted; want 1 and 15", won, conflicted)
	}
}

func TestConditionalEdits_ReplyWritesOnlyAtTheExpectedVersion(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewReplyRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1", post.ID) })
	reply, err := repo.Create(ctx, &models.Reply{PostID: post.ID, AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Body: "original reply body"})
	if err != nil {
		t.Fatalf("Create reply: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM replies WHERE id = $1", reply.ID) })
	read := reply.UpdatedAt

	written, err := repo.Update(ctx, reply.ID, models.AuthorTypeHuman, user.ID, "edited at the read version", nil, &read)
	if err != nil {
		t.Fatalf("edit at the read version: %v", err)
	}
	_, err = repo.Update(ctx, reply.ID, models.AuthorTypeHuman, user.ID, "stale edit", nil, &read)
	assertVersionConflict(t, err, written.UpdatedAt)
	stored, err := repo.GetByID(ctx, reply.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Body != "edited at the read version" {
		t.Errorf("a refused edit changed the body to %q", stored.Body)
	}

	// The author check stays first: a non-author learns nothing from a conflict.
	if _, err := repo.Update(ctx, reply.ID, models.AuthorTypeAgent, "intruder", "hacked", nil, &read); !errors.Is(err, ErrReplyForbidden) {
		t.Errorf("non-author with a stale version err = %v, want ErrReplyForbidden", err)
	}
	if _, err := repo.Update(ctx, reply.ID, models.AuthorTypeHuman, user.ID, "unconditional edit", nil, nil); err != nil {
		t.Fatalf("unconditional edit: %v", err)
	}

	latest, err := repo.GetByID(ctx, reply.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	at := latest.UpdatedAt
	won, conflicted := conditionalWriters(t, 16, func(i int) error {
		_, err := repo.Update(context.Background(), reply.ID, models.AuthorTypeHuman, user.ID, "concurrent edit", nil, &at)
		return err
	})
	if won != 1 || conflicted != 15 {
		t.Errorf("16 writers at one version: %d won, %d conflicted; want 1 and 15", won, conflicted)
	}
}

func TestConditionalEdits_RoomWritesOnlyAtTheExpectedVersion(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{DisplayName: "condedit room " + time.Now().Format("150405.000000"), OwnerID: uuid.Nil})
	if err != nil {
		t.Fatalf("Create room: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM rooms WHERE id = $1", room.ID) })
	read := room.UpdatedAt

	first := "edited at the read version"
	written, err := repo.Update(ctx, room.ID, models.UpdateRoomParams{Description: &first}, &read)
	if err != nil {
		t.Fatalf("edit at the read version: %v", err)
	}
	stale := "stale edit"
	_, err = repo.Update(ctx, room.ID, models.UpdateRoomParams{Description: &stale}, &read)
	assertVersionConflict(t, err, written.UpdatedAt)
	stored, err := repo.GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if stored.Description == nil || *stored.Description != first {
		t.Errorf("a refused edit changed the description to %v", stored.Description)
	}

	if _, err := repo.Update(ctx, uuid.New(), models.UpdateRoomParams{Description: &stale}, &read); !errors.Is(err, ErrRoomNotFound) {
		t.Errorf("absent room err = %v, want ErrRoomNotFound (not a version conflict)", err)
	}
	if _, err := repo.Update(ctx, room.ID, models.UpdateRoomParams{Description: &stale}, nil); err != nil {
		t.Fatalf("unconditional edit: %v", err)
	}

	latest, err := repo.GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	at := latest.UpdatedAt
	won, conflicted := conditionalWriters(t, 16, func(i int) error {
		d := "concurrent edit"
		_, err := repo.Update(context.Background(), room.ID, models.UpdateRoomParams{Description: &d}, &at)
		return err
	})
	if won != 1 || conflicted != 15 {
		t.Errorf("16 writers at one version: %d won, %d conflicted; want 1 and 15", won, conflicted)
	}
}
