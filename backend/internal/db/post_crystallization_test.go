package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:crystallization): crystallization reads the canonical
// posts and replies tables. A candidate is a live, public, published and approved post
// of any type, not yet crystallized, with at least one live human or agent reply, whose
// post and non-system replies have been unchanged for the stability period.

type crystalPostSeed struct {
	postType, status, publication, moderation, visibility string
	age                                                   time.Duration
	deleted                                               bool
	cid                                                   string
}

func insertCrystalPost(t *testing.T, pool *Pool, ctx context.Context, author, title string, s crystalPostSeed) string {
	t.Helper()
	if s.postType == "" {
		s.postType, s.status = "post", "open"
	}
	updated := time.Now().UTC().Add(-s.age)
	var deletedAt, cid, crystallizedAt any
	if s.deleted {
		deletedAt = updated
	}
	if s.cid != "" {
		cid, crystallizedAt = s.cid, updated
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility, upvotes, downvotes,
			created_at, updated_at, deleted_at, crystallization_cid, crystallized_at)
		VALUES ($1, $2, $3, ARRAY['crystal','ipfs'], $4, 'agent', $5,
			$6, $7, $8, 3, 1, $9, $9, $10, $11, $12)
		RETURNING id::text`,
		s.postType, title, "Crystal body for "+title, s.status, author, s.publication, s.moderation, s.visibility,
		updated, deletedAt, cid, crystallizedAt).Scan(&id))
	return id
}

func TestPostCrystallizationRepository_ListCandidates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	author := "agent_cry_" + time.Now().Format("150405.000000")
	insertRemapAgent(t, pool, ctx, author)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()

	const day = 24 * time.Hour
	eligible := crystalPostSeed{publication: "published", moderation: "approved", visibility: "public", age: 10 * day}
	with := func(mut func(s *crystalPostSeed)) crystalPostSeed {
		s := eligible
		mut(&s)
		return s
	}
	post := func(name string, s crystalPostSeed) string {
		return insertCrystalPost(t, pool, ctx, author, "crystal "+name+" "+uuid.NewString(), s)
	}
	reply := func(postID, authorType string, age time.Duration, deleted bool) {
		insertDuplicateReply(t, pool, ctx, postID, authorType, author, "crystal reply "+uuid.NewString(), age, deleted)
	}

	oldest := post("oldest", eligible)
	reply(oldest, "agent", 10*day, false)
	idea := post("idea", with(func(s *crystalPostSeed) { s.age = 8 * day })) // any post qualifies; the legacy idea type is retired (idx 68)
	reply(idea, "agent", 8*day, false)
	systemActivity := post("recent-system-reply", with(func(s *crystalPostSeed) { s.age = 9 * day }))
	reply(systemActivity, "agent", 9*day, false)
	reply(systemActivity, "system", day, false)

	excluded := map[string]string{}
	noReplies := post("no-replies", eligible)
	excluded[noReplies] = "a post without replies"
	systemOnly := post("system-only", eligible)
	reply(systemOnly, "system", 10*day, false)
	excluded[systemOnly] = "a post whose only reply is a system verdict"
	deletedReply := post("deleted-reply", eligible)
	reply(deletedReply, "agent", 10*day, true)
	excluded[deletedReply] = "a post whose only reply is deleted"
	for name, s := range map[string]crystalPostSeed{
		"family":       with(func(s *crystalPostSeed) { s.visibility = "family" }),
		"draft":        with(func(s *crystalPostSeed) { s.publication = "draft" }),
		"archived":     with(func(s *crystalPostSeed) { s.publication = "archived" }),
		"pending":      with(func(s *crystalPostSeed) { s.moderation = "pending" }),
		"rejected":     with(func(s *crystalPostSeed) { s.moderation = "rejected" }),
		"crystallized": with(func(s *crystalPostSeed) { s.cid = "bafyalready" }),
		"deleted":      with(func(s *crystalPostSeed) { s.deleted = true }),
		"recent-post":  with(func(s *crystalPostSeed) { s.age = day }),
	} {
		id := post(name, s)
		reply(id, "agent", 10*day, false)
		excluded[id] = "a " + name + " post"
	}
	recentReply := post("recent-reply", eligible)
	reply(recentReply, "agent", 10*day, false)
	reply(recentReply, "agent", day, false)
	excluded[recentReply] = "a post whose discussion changed inside the stability period"

	repo := NewPostCrystallizationRepository(pool)
	ids, err := repo.ListCrystallizationCandidates(ctx, 7*day, 100000)
	require.NoError(t, err)
	at := map[string]int{}
	for i, id := range ids {
		at[id] = i
	}
	for _, id := range []string{oldest, idea, systemActivity} {
		require.Contains(t, at, id, "an eligible post of any type must be a candidate")
	}
	for id, why := range excluded {
		require.NotContains(t, at, id, "%s must not be a crystallization candidate", why)
	}
	require.Less(t, at[oldest], at[systemActivity], "the longest-stable post comes first")
	require.Less(t, at[systemActivity], at[idea], "the longest-stable post comes first")

	one, err := repo.ListCrystallizationCandidates(ctx, 7*day, 1)
	require.NoError(t, err)
	require.Len(t, one, 1, "the limit bounds a run")

	longer, err := repo.ListCrystallizationCandidates(ctx, 9*day+12*time.Hour, 100000)
	require.NoError(t, err)
	require.Contains(t, longer, oldest)
	require.NotContains(t, longer, idea, "the stability period is a parameter")
}

func TestPostCrystallizationRepository_FindSnapshotPost(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	author := "agent_cry_" + time.Now().Format("150405.000000")
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, 'Crystal Agent', $2, 'active')`, author, "hash_"+author)
	require.NoError(t, err)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()

	const day = 24 * time.Hour
	title := "crystal snapshot " + uuid.NewString()
	id := insertCrystalPost(t, pool, ctx, author, title, crystalPostSeed{
		postType: "post", status: "open", publication: "published", moderation: "approved",
		visibility: "public", age: 10 * day, cid: "bafyseen",
	})

	repo := NewPostCrystallizationRepository(pool)
	got, err := repo.FindSnapshotPost(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
	require.Equal(t, title, got.Title)
	require.Equal(t, "Crystal body for "+title, got.Description)
	require.Equal(t, []string{"crystal", "ipfs"}, got.Tags)
	require.Equal(t, 3, got.Upvotes)
	require.Equal(t, 1, got.Downvotes)
	require.Equal(t, models.PublicationPublished, got.PublicationState)
	require.Equal(t, models.ModerationApproved, got.ModerationState)
	require.Equal(t, models.VisibilityPublic, got.Visibility)
	require.NotNil(t, got.CrystallizationCID)
	require.Equal(t, "bafyseen", *got.CrystallizationCID)
	require.WithinDuration(t, time.Now().Add(-10*day), got.UpdatedAt, time.Minute)
	require.WithinDuration(t, time.Now().Add(-10*day), got.CreatedAt, time.Minute)
	require.Equal(t, models.PostAuthor{Type: models.AuthorTypeAgent, ID: author, DisplayName: "Crystal Agent"}, got.Author)
	require.True(t, got.PublicEligible())

	family := insertCrystalPost(t, pool, ctx, author, "crystal family "+uuid.NewString(), crystalPostSeed{
		publication: "published", moderation: "approved", visibility: "family", age: 10 * day,
	})
	got, err = repo.FindSnapshotPost(ctx, family)
	require.NoError(t, err)
	require.Equal(t, models.VisibilityFamily, got.Visibility, "the stored visibility is returned, never defaulted")
	require.False(t, got.PublicEligible())
	require.Nil(t, got.CrystallizationCID)

	deleted := insertCrystalPost(t, pool, ctx, author, "crystal deleted "+uuid.NewString(), crystalPostSeed{
		publication: "published", moderation: "approved", visibility: "public", age: 10 * day, deleted: true,
	})
	for _, missing := range []string{deleted, uuid.NewString(), "not-a-uuid"} {
		_, err := repo.FindSnapshotPost(ctx, missing)
		require.True(t, errors.Is(err, ErrPostNotFound), "FindSnapshotPost(%q) error = %v, want ErrPostNotFound", missing, err)
	}
}
