package db

import (
	"context"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// createVisibilityTestUser creates a user through the repository (so every column a real
// signup writes is set) with a UUID-derived username, and removes it when the test ends.
func createVisibilityTestUser(t *testing.T, pool *Pool) string {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	user, err := NewUserRepository(pool).Create(context.Background(), &models.User{
		Username:       "pvt_" + suffix,
		DisplayName:    "Post Visibility Test",
		Email:          "pvt_" + suffix + "@test.solvr.dev",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "pvt_" + suffix,
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID) }) //nolint:errcheck
	return user.ID
}

// createVisibilityTestPost inserts a post by authorID ("family" posts are owned by
// ownerID), optionally soft-deleted, and removes it when the test ends.
func createVisibilityTestPost(t *testing.T, pool *Pool, authorID, visibility, ownerID string, deleted bool) string {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
		 VALUES ('question', $1, $2, 'human', $3, 'open', $4, $5::uuid) RETURNING id::text`,
		"post visibility "+uuid.NewString(), "post visibility fixture "+uuid.NewString(), authorID, visibility, owner,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1::uuid", id) }) //nolint:errcheck
	if deleted {
		if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1::uuid", id); err != nil {
			t.Fatalf("soft-delete post: %v", err)
		}
	}
	return id
}

// Both repositories answer "may this caller read the post?" with the rule GET
// /v1/posts/{id} applies: the post exists, is not deleted, and is public or owned by the
// caller's family. A malformed id is not an error, it is simply no post.
func TestPostVisibleTo_FollowsTheSinglePostReadRule(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	owner := createVisibilityTestUser(t, pool)
	stranger := createVisibilityTestUser(t, pool)
	public := createVisibilityTestPost(t, pool, owner, models.VisibilityPublic, "", false)
	family := createVisibilityTestPost(t, pool, owner, models.VisibilityFamily, owner, false)
	deleted := createVisibilityTestPost(t, pool, owner, models.VisibilityPublic, "", true)
	deletedFamily := createVisibilityTestPost(t, pool, owner, models.VisibilityFamily, owner, true)

	cases := []struct {
		name, postID, caller string
		want                 bool
	}{
		{"public post, anonymous", public, "", true},
		{"public post, another human", public, stranger, true},
		{"family post, its owner", family, owner, true},
		{"family post, anonymous", family, "", false},
		{"family post, another human", family, stranger, false},
		{"deleted post, anonymous", deleted, "", false},
		{"deleted family post, its owner", deletedFamily, owner, false},
		{"absent post", uuid.NewString(), owner, false},
		{"malformed id", "not-a-valid-id", "", false},
		{"empty id", "", owner, false},
	}
	checkers := map[string]func(ctx context.Context, postID, callerHuman string) (bool, error){
		"PostRepository.VisibleTo":      NewPostRepository(pool).VisibleTo,
		"ReplyRepository.PostVisibleTo": NewReplyRepository(pool).PostVisibleTo,
	}
	for name, visible := range checkers {
		for _, tc := range cases {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				got, err := visible(ctx, tc.postID, tc.caller)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("visible = %v, want %v", got, tc.want)
				}
			})
		}
	}
}
