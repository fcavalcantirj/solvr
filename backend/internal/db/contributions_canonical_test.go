package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3 and 5: GET /v1/users/{id}/contributions and GET /v1/me/contributions
// list an author's answers, approaches and responses. The router serves them from the replies
// migrated from those rows; the legacy ListByAuthor methods of the answers, approaches and
// responses repositories stay only on the legacy routes until the tables go.
func TestLegacyContributions_ServedCanonically(t *testing.T) {
	for _, ctor := range []string{
		"db.NewCanonicalAnswerContributionsRepository(",
		"db.NewCanonicalApproachContributionsRepository(",
		"db.NewCanonicalResponseContributionsRepository(",
	} {
		assert.ElementsMatch(t, []string{"internal/api/router.go"}, productionSourcesContaining(t, ctor),
			"the contribution listings are served canonically: %s", ctor)
	}
	for _, ctor := range []string{"db.NewAnswersRepository(", "db.NewApproachesRepository(", "db.NewResponsesRepository("} {
		assert.Empty(t, productionSourcesContaining(t, ctor), "no route builds %s any more", ctor)
	}

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/contributions_canonical.go", dep.Key, "the canonical contributions name no legacy table or type")
	}
	for _, key := range []string{"code:internal/db/answers.go", "code:internal/db/approaches.go", "code:internal/db/responses.go"} {
		d, ok := LegacyDependencyDispositions[key]
		require.True(t, ok, key)
		assert.Equal(t, LegacyActionRetire, d.Action, key)
		assert.False(t, d.Done, "%s still serves the legacy routes", key)
		assert.Contains(t, d.Note, "contributions_canonical.go", "%s: the note names the served contribution listings", key)
	}
}

// contributionRow is the part of one listed contribution the contributions handler reads.
type contributionRow struct {
	ID, ParentID, AuthorType, AuthorID, Preview, Status, Title string
	CreatedAt                                                  time.Time
}

type contributionPage struct {
	Rows  []contributionRow
	Total int
}

type answerContributionLister interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.AnswerWithContext, int, error)
}

type approachContributionLister interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ApproachWithContext, int, error)
}

type responseContributionLister interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ResponseWithContext, int, error)
}

func listAnswerContributions(t *testing.T, repo answerContributionLister, authorType, authorID string, page, perPage int) contributionPage {
	t.Helper()
	items, total, err := repo.ListByAuthor(context.Background(), authorType, authorID, page, perPage)
	require.NoError(t, err)
	out := contributionPage{Rows: []contributionRow{}, Total: total}
	for _, a := range items {
		out.Rows = append(out.Rows, contributionRow{ID: a.ID, ParentID: a.QuestionID, AuthorType: string(a.AuthorType),
			AuthorID: a.AuthorID, Preview: a.Content, Title: a.QuestionTitle, CreatedAt: a.CreatedAt.UTC()})
	}
	return out
}

func listApproachContributions(t *testing.T, repo approachContributionLister, authorType, authorID string, page, perPage int) contributionPage {
	t.Helper()
	items, total, err := repo.ListByAuthor(context.Background(), authorType, authorID, page, perPage)
	require.NoError(t, err)
	out := contributionPage{Rows: []contributionRow{}, Total: total}
	for _, a := range items {
		out.Rows = append(out.Rows, contributionRow{ID: a.ID, ParentID: a.ProblemID, AuthorType: string(a.AuthorType),
			AuthorID: a.AuthorID, Preview: a.Angle, Status: string(a.Status), Title: a.ProblemTitle, CreatedAt: a.CreatedAt.UTC()})
	}
	return out
}

func listResponseContributions(t *testing.T, repo responseContributionLister, authorType, authorID string, page, perPage int) contributionPage {
	t.Helper()
	items, total, err := repo.ListByAuthor(context.Background(), authorType, authorID, page, perPage)
	require.NoError(t, err)
	out := contributionPage{Rows: []contributionRow{}, Total: total}
	for _, r := range items {
		out.Rows = append(out.Rows, contributionRow{ID: r.ID, ParentID: r.IdeaID, AuthorType: string(r.AuthorType),
			AuthorID: r.AuthorID, Preview: r.Content, Title: r.IdeaTitle, CreatedAt: r.CreatedAt.UTC()})
	}
	return out
}

// Task idx 76 steps 3 and 5: in a database holding only this fixture, the canonical
// contribution listings give every author the legacy lists once the contribution cutover has
// run, with each item's id now the id of the reply it became, and still do after the legacy
// tables are dropped. Before the cutover they are empty. Native replies are no answer,
// approach or response and are not listed; deleting a migrated reply removes it.
func TestCanonicalContributions_KeepTheLegacyListsAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	insertID := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	a, b := "agent_contrib_a", "agent_contrib_b"
	insertRemapAgent(t, pool, ctx, a)
	insertRemapAgent(t, pool, ctx, b)
	h := "5b0f4c1e-7a51-4c6e-9d59-2a8f0c6d1e01" // a human author id

	post := func(postType, title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, "contribution listing body", []string{"contrib"}, "open", "agent", b)
	}
	qPublic := post("question", "contributions: a public question")
	qFamily := post("question", "contributions: a family question")
	exec(`UPDATE posts SET visibility = 'family' WHERE id = $1`, qFamily)
	qDeleted := post("question", "contributions: a deleted question")
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, qDeleted)
	pPublic := post("problem", "contributions: a public problem")
	iPublic := post("idea", "contributions: a public idea")

	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	at := func(minute int) time.Time { return base.Add(time.Duration(minute) * time.Minute) }
	answer := func(questionID, authorType, authorID, content string, minute int, deleted bool) string {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = at(minute + 30)
		}
		return insertID(`INSERT INTO answers (question_id, author_type, author_id, content, created_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`, questionID, authorType, authorID, content, at(minute), deletedAt)
	}
	approach := func(problemID, authorID, angle, status string, minute int, deleted bool) string {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = at(minute + 30)
		}
		return insertID(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, assumptions,
				differs_from, status, created_at, updated_at, deleted_at)
			VALUES ($1, 'agent', $2, $3, 'Method text', ARRAY['assume one'], '{}'::uuid[], $4, $5, $5, $6)
			RETURNING id::text`, problemID, authorID, angle, status, at(minute), deletedAt)
	}
	response := func(ideaID, authorType, authorID, content string, minute int) string {
		t.Helper()
		return insertID(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type, created_at)
			VALUES ($1, $2, $3, $4, 'build', $5) RETURNING id::text`, ideaID, authorType, authorID, content, at(minute))
	}

	a1 := answer(qPublic, "agent", a, "Answer one on the public question", 1, false)
	answer(qFamily, "agent", a, "Answer two on the family question", 2, false)
	answer(qPublic, "agent", a, "Answer three, deleted", 3, true)
	answer(qDeleted, "agent", a, "Answer four on the deleted question", 4, false)
	answer(qPublic, "agent", b, "Another agent's answer", 5, false)
	answer(qPublic, "human", h, "The human's answer", 6, false)
	ap1 := approach(pPublic, a, "Angle one", "succeeded", 7, false)
	approach(pPublic, a, "Angle two", "working", 8, false)
	approach(pPublic, a, "Angle three, deleted", "failed", 9, true)
	approach(pPublic, b, "Another agent's angle", "stuck", 10, false)
	response(iPublic, "agent", a, "Response one on the idea", 11)
	response(iPublic, "human", h, "The human's response", 12)
	insertComment(t, pool, ctx, "post", qPublic, a, "a comment is no contribution here")
	insertComment(t, pool, ctx, "approach", ap1, a, "nor is a comment on an approach")

	authors := []struct{ typ, id string }{{"agent", a}, {"agent", b}, {"human", h}, {"human", a}}
	type snapshot map[string]contributionPage // "<kind> <author type> <author id>" -> page 1 of 20
	legacy := func() snapshot {
		t.Helper()
		out := snapshot{}
		for _, au := range authors {
			out["answer "+au.typ+" "+au.id] = listAnswerContributions(t, NewAnswersRepository(pool), au.typ, au.id, 1, 20)
			out["approach "+au.typ+" "+au.id] = listApproachContributions(t, NewApproachesRepository(pool), au.typ, au.id, 1, 20)
			out["response "+au.typ+" "+au.id] = listResponseContributions(t, NewResponsesRepository(pool), au.typ, au.id, 1, 20)
		}
		return out
	}
	answers := NewCanonicalAnswerContributionsRepository(pool)
	approaches := NewCanonicalApproachContributionsRepository(pool)
	responses := NewCanonicalResponseContributionsRepository(pool)
	canonical := func() snapshot {
		t.Helper()
		out := snapshot{}
		for _, au := range authors {
			out["answer "+au.typ+" "+au.id] = listAnswerContributions(t, answers, au.typ, au.id, 1, 20)
			out["approach "+au.typ+" "+au.id] = listApproachContributions(t, approaches, au.typ, au.id, 1, 20)
			out["response "+au.typ+" "+au.id] = listResponseContributions(t, responses, au.typ, au.id, 1, 20)
		}
		return out
	}

	before := legacy()
	// The fixture is what the legacy lists are: pin the ones the canonical lists must keep.
	agentAnswers := before["answer agent "+a]
	require.Equal(t, 3, agentAnswers.Total, "live answers only")
	require.Len(t, agentAnswers.Rows, 3)
	assert.Equal(t, []string{"Answer four on the deleted question", "Answer two on the family question", "Answer one on the public question"},
		[]string{agentAnswers.Rows[0].Preview, agentAnswers.Rows[1].Preview, agentAnswers.Rows[2].Preview}, "newest first")
	assert.Equal(t, []string{"contributions: a deleted question", "", "contributions: a public question"},
		[]string{agentAnswers.Rows[0].Title, agentAnswers.Rows[1].Title, agentAnswers.Rows[2].Title},
		"a family post's title is withheld; a deleted public post's is not")
	agentApproaches := before["approach agent "+a]
	require.Equal(t, 2, agentApproaches.Total)
	require.Len(t, agentApproaches.Rows, 2)
	assert.Equal(t, contributionRow{ID: ap1, ParentID: pPublic, AuthorType: "agent", AuthorID: a, Preview: "Angle one",
		Status: "succeeded", Title: "contributions: a public problem", CreatedAt: at(7)}, agentApproaches.Rows[1])
	assert.Equal(t, "working", agentApproaches.Rows[0].Status)
	require.Equal(t, 1, before["response agent "+a].Total)
	require.Equal(t, 1, before["answer human "+h].Total)
	require.Equal(t, 1, before["response human "+h].Total)
	require.Equal(t, 1, before["answer agent "+b].Total)
	require.Equal(t, 1, before["approach agent "+b].Total)
	require.Equal(t, 0, before["answer human "+a].Total, "the author type is part of the author")

	empty := snapshot{}
	for key := range before {
		empty[key] = contributionPage{Rows: []contributionRow{}}
	}
	assert.Equal(t, empty, canonical(), "before the cutover no contribution is a reply yet")

	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	// The legacy lists, each item carrying the id of the reply it became.
	replyID := func(legacyType, legacyID string) string {
		t.Helper()
		return insertID(`SELECT id::text FROM replies WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID)
	}
	want := snapshot{}
	for key, page := range before {
		kind, _, _ := strings.Cut(key, " ")
		rows := []contributionRow{}
		for _, row := range page.Rows {
			row.ID = replyID(kind, row.ID)
			rows = append(rows, row)
		}
		want[key] = contributionPage{Rows: rows, Total: page.Total}
	}
	assert.Equal(t, want, canonical(), "the legacy lists, read from the migrated replies")
	a1Reply := replyID("answer", a1)

	dropLegacy()
	assert.Equal(t, want, canonical(), "the canonical listings need no legacy table")

	// Pagination and the per-page bounds are the legacy repositories'.
	second := listApproachContributions(t, approaches, "agent", a, 2, 1)
	assert.Equal(t, contributionPage{Rows: want["approach agent "+a].Rows[1:2], Total: 2}, second, "page 2 of 1")
	assert.Equal(t, want["answer agent "+a], listAnswerContributions(t, answers, "agent", a, 0, 0), "page 0 and per_page 0 read page 1 of 20")

	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'a native reply')`, qPublic, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'a native reply on an idea')`, iPublic, a)
	exec(`UPDATE replies SET deleted_at = NOW() WHERE id = $1`, a1Reply)
	gotAnswers := listAnswerContributions(t, answers, "agent", a, 1, 20)
	assert.Equal(t, contributionPage{Rows: want["answer agent "+a].Rows[:2], Total: 2}, gotAnswers,
		"a deleted migrated reply leaves the list; a native reply is no answer")
	assert.Equal(t, want["response agent "+a], listResponseContributions(t, responses, "agent", a, 1, 20),
		"a native reply is no response")
}
