package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 2.8: a person whose stored display name is an e-mail address (two production rows,
// recon 2026-10-04) is named by their username in every public answer that names them: post
// and reply authors, blog authors, the users list, the leaderboard and a room's owner. The
// stored row is left as it is.
func TestPublicAnswers_NameAPersonByUsernameWhenTheStoredNameIsAnEmail(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	n := time.Now().UnixNano() % 1000000000
	email := fmt.Sprintf("mail.me%d@example.com", n)
	username := fmt.Sprintf("pubname%d", n)

	user, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: username, DisplayName: email, Email: email,
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: fmt.Sprintf("gh_pubname%d", n), Role: models.UserRoleUser,
	})
	require.NoError(t, err)

	var postID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state)
		VALUES ('post', 'A post by a person whose name is an address', 'Body of the post.', 'human', $1, 'open', 'published', 'approved')
		RETURNING id::text`, user.ID).Scan(&postID))
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1::uuid, 'human', $2, 'A reply.')`, postID, user.ID)
	require.NoError(t, err)
	slug := fmt.Sprintf("pubname-blog-%d", n)
	_, err = pool.Exec(ctx, `INSERT INTO blog_posts (slug, title, body, posted_by_type, posted_by_id, status, published_at)
		VALUES ($1, 'A blog post by the same person', 'A blog body long enough to be a post.', 'human', $2, 'published', NOW())`, slug, user.ID)
	require.NoError(t, err)
	var roomID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name) VALUES ($1, 'Owned room') RETURNING id`,
		fmt.Sprintf("pubname-room-%d", n)).Scan(&roomID))
	_, err = pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2::uuid, 'owner', 'system')`, roomID, user.ID)
	require.NoError(t, err)

	answers := map[string]any{}
	named := map[string]string{}

	post, err := NewPostRepository(pool).FindByID(ctx, postID)
	require.NoError(t, err)
	answers["post read"], named["post read"] = post, post.Author.DisplayName

	posts, _, err := NewPostRepository(pool).List(ctx, models.PostListOptions{AuthorType: models.AuthorTypeHuman, AuthorID: user.ID, Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, posts, 1)
	answers["post list"], named["post list"] = posts, posts[0].Author.DisplayName

	replies, _, err := NewReplyRepository(pool).ListByPost(ctx, models.ReplyListOptions{PostID: postID, Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, replies, 1)
	answers["reply list"], named["reply list"] = replies, replies[0].Author.DisplayName

	blog, err := NewBlogPostRepository(pool).FindBySlug(ctx, slug)
	require.NoError(t, err)
	answers["blog read"], named["blog read"] = blog, blog.Author.DisplayName
	blogs, _, err := NewBlogPostRepository(pool).List(ctx, models.BlogPostListOptions{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, blogs, 1)
	answers["blog list"], named["blog list"] = blogs, blogs[0].Author.DisplayName

	users, _, err := NewCanonicalReputationUserRepository(pool).List(ctx, models.PublicUserListOptions{Limit: 100, Sort: models.PublicUserSortNewest})
	require.NoError(t, err)
	for _, u := range users {
		if u.ID == user.ID {
			answers["users list"], named["users list"] = users, u.DisplayName
		}
	}

	board, _, err := NewCanonicalLeaderboardRepository(pool).GetLeaderboard(ctx, models.LeaderboardOptions{Type: "users", Timeframe: "all_time", Limit: 100})
	require.NoError(t, err)
	for _, e := range board {
		if e.ID == user.ID {
			answers["leaderboard"], named["leaderboard"] = board, e.DisplayName
		}
	}

	rooms, err := NewRoomRepository(pool).ListFiltered(ctx, RoomListParams{Limit: 50})
	require.NoError(t, err)
	for _, r := range rooms {
		if r.ID == roomID && r.OwnerDisplayName != nil {
			answers["room owner"], named["room owner"] = rooms, *r.OwnerDisplayName
		}
	}

	for _, answer := range []string{"post read", "post list", "reply list", "blog read", "blog list", "users list", "leaderboard", "room owner"} {
		assert.Equal(t, username, named[answer], "%s names the person by their username", answer)
		raw, err := json.Marshal(answers[answer])
		require.NoError(t, err)
		assert.NotContains(t, string(raw), email, "%s carries no e-mail address", answer)
	}

	var stored string
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1::uuid`, user.ID).Scan(&stored))
	assert.Equal(t, email, stored, "the stored row is not rewritten")
}
