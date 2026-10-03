package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task idx 73 step 3: GET /v1/replies?author_type=&author_id= lists one author's replies across
// posts, the canonical replacement of the contribution listings. ListPageByAuthor orders newest
// first on the keyset (created_at, id), leaves out deleted replies and replies on deleted posts,
// and lists a reply only when its post passes the GET /v1/posts/{id} read rule for the viewer.
func TestReplyRepository_ListPageByAuthor(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the fixture cleanup below

	ctx := context.Background()
	repo := NewReplyRepository(pool)
	author := createCommentTestUser(t, pool)
	owner := createCommentTestUser(t, pool)
	public := createCommentTestPost(t, pool, owner.ID)
	family := createCommentTestPost(t, pool, owner.ID)
	gone := createCommentTestPost(t, pool, owner.ID)
	family.Title = "Family post the author replied to " + family.ID
	if _, err := pool.Exec(ctx, `UPDATE posts SET visibility = 'family', owner_human_id = $2, title = $3 WHERE id = $1`,
		family.ID, owner.ID, family.Title); err != nil {
		t.Fatalf("make the family post: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range []string{public.ID, family.ID, gone.ID} {
			_, _ = pool.Exec(bg, "DELETE FROM replies WHERE post_id = $1", id)
			_, _ = pool.Exec(bg, "DELETE FROM posts WHERE id = $1", id)
		}
	})

	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	insert := func(postID, authorID, body string, at time.Time, deleted bool) string {
		t.Helper()
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at, deleted_at)
			VALUES ($1, 'human', $2, $3, $4, $4, CASE WHEN $5 THEN NOW() END)
			RETURNING id::text`, postID, authorID, body, at, deleted).Scan(&id)
		if err != nil {
			t.Fatalf("insert reply %q: %v", body, err)
		}
		return id
	}
	oldest := insert(public.ID, author.ID, "oldest public reply", t0, false)
	insert(public.ID, author.ID, "deleted reply", t0.Add(time.Minute), true)
	onFamily := insert(family.ID, author.ID, "reply on a family post", t0.Add(2*time.Minute), false)
	onGone := insert(gone.ID, author.ID, "reply on a deleted post", t0.Add(3*time.Minute), false)
	tieA := insert(public.ID, author.ID, "newest reply A", t0.Add(4*time.Minute), false)
	tieB := insert(public.ID, author.ID, "newest reply B", t0.Add(4*time.Minute), false)
	insert(public.ID, owner.ID, "another author's reply", t0.Add(5*time.Minute), false)
	if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1", gone.ID); err != nil {
		t.Fatalf("delete the post: %v", err)
	}
	_ = onGone

	// Same created_at: the larger id comes first, so the keyset stays total.
	newest, second := tieA, tieB
	if tieB > tieA {
		newest, second = tieB, tieA
	}

	list := func(viewer string, limit int, before *models.ReplyWithPost) ([]models.ReplyWithPost, int) {
		t.Helper()
		params := models.ReplyAuthorPageParams{
			AuthorType: models.AuthorTypeHuman, AuthorID: author.ID, ViewerHuman: viewer, Limit: limit,
		}
		if before != nil {
			at := before.CreatedAt
			params.BeforeCreatedAt, params.BeforeID = &at, before.ID
		}
		got, total, err := repo.ListPageByAuthor(ctx, params)
		if err != nil {
			t.Fatalf("ListPageByAuthor(viewer %q): %v", viewer, err)
		}
		return got, total
	}
	ids := func(rs []models.ReplyWithPost) []string {
		out := make([]string, 0, len(rs))
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}
	assertIDs := func(name string, got []models.ReplyWithPost, want ...string) {
		t.Helper()
		g := ids(got)
		if len(g) != len(want) {
			t.Fatalf("%s: ids = %v, want %v", name, g, want)
		}
		for i := range want {
			if g[i] != want[i] {
				t.Fatalf("%s: ids = %v, want %v", name, g, want)
			}
		}
	}

	t.Run("anonymous reads the replies on posts it may read, newest first", func(t *testing.T) {
		got, total := list("", 10, nil)
		assertIDs("anonymous", got, newest, second, oldest)
		if total != 3 {
			t.Errorf("total = %d, want 3 (deleted reply, family post and deleted post left out)", total)
		}
		first := got[0]
		if first.PostID != public.ID || first.Post.ID != public.ID {
			t.Errorf("post_id/post.id = %s/%s, want %s", first.PostID, first.Post.ID, public.ID)
		}
		if first.Post.Title != public.Title || first.Post.Type != string(models.PostTypePost) {
			t.Errorf("post = %+v, want title %q type question", first.Post, public.Title)
		}
		if first.Author.ID != author.ID || first.Author.Type != models.AuthorTypeHuman || first.Author.DisplayName == "" {
			t.Errorf("author = %+v, want the reply's author resolved", first.Author)
		}
		if got[2].Body != "oldest public reply" || !got[2].CreatedAt.Equal(t0) {
			t.Errorf("oldest = %q at %v, want the oldest public reply at %v", got[2].Body, got[2].CreatedAt, t0)
		}
	})

	t.Run("the post's family also reads the reply on its family post", func(t *testing.T) {
		got, total := list(owner.ID, 10, nil)
		assertIDs("family", got, newest, second, onFamily, oldest)
		if total != 4 {
			t.Errorf("total = %d, want 4", total)
		}
		if got[2].Post.Title != family.Title {
			t.Errorf("family post title = %q, want %q", got[2].Post.Title, family.Title)
		}
	})

	t.Run("another family does not", func(t *testing.T) {
		got, total := list(author.ID, 10, nil)
		assertIDs("other family", got, newest, second, oldest)
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
	})

	t.Run("the keyset pages without skipping or repeating a reply", func(t *testing.T) {
		var seen []string
		var before *models.ReplyWithPost
		for page := range 6 {
			got, total := list(owner.ID, 1, before)
			if total != 4 {
				t.Fatalf("page %d: total = %d, want 4 on every page", page, total)
			}
			if len(got) == 0 {
				break
			}
			seen = append(seen, got[0].ID)
			before = &got[0]
		}
		want := []string{newest, second, onFamily, oldest}
		if len(seen) != len(want) {
			t.Fatalf("paged ids = %v, want %v", seen, want)
		}
		for i := range want {
			if seen[i] != want[i] {
				t.Fatalf("paged ids = %v, want %v", seen, want)
			}
		}
	})

	t.Run("the author is matched by type and id", func(t *testing.T) {
		got, total, err := repo.ListPageByAuthor(ctx, models.ReplyAuthorPageParams{
			AuthorType: models.AuthorTypeAgent, AuthorID: author.ID, Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListPageByAuthor: %v", err)
		}
		if len(got) != 0 || total != 0 {
			t.Errorf("an agent with the user's id lists %v (total %d), want none", ids(got), total)
		}
	})
}
