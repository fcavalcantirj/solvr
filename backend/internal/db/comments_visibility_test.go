package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// createCommentVisibilityTargets inserts a problem, a question and an idea by authorID
// ("family" posts are owned by ownerID) with an approach, an answer and a response, and
// returns the comment target id of each kind. deleted soft-deletes the posts afterwards.
func createCommentVisibilityTargets(t *testing.T, pool *Pool, authorID, visibility, ownerID string, deleted bool) map[models.CommentTargetType]string {
	t.Helper()
	ctx := context.Background()
	var owner any
	if ownerID != "" {
		owner = ownerID
	}
	post := func(typ string) string {
		var id string
		if err := pool.QueryRow(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, owner_human_id)
			 VALUES ($1, $2, $3, 'human', $4, 'open', $5, $6::uuid) RETURNING id::text`,
			typ, "comment target "+uuid.NewString(), "comment target fixture "+uuid.NewString(), authorID, visibility, owner,
		).Scan(&id); err != nil {
			t.Fatalf("insert %s: %v", typ, err)
		}
		return id
	}
	problem, question, idea := post("problem"), post("question"), post("idea")
	child := func(query, parent string) string {
		var id string
		if err := pool.QueryRow(ctx, query, parent, authorID).Scan(&id); err != nil {
			t.Fatalf("insert child: %v", err)
		}
		return id
	}
	approach := child(`INSERT INTO approaches (problem_id, author_type, author_id, angle) VALUES ($1::uuid, 'human', $2, 'angle') RETURNING id::text`, problem)
	answer := child(`INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1::uuid, 'human', $2, 'answer') RETURNING id::text`, question)
	response := child(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type) VALUES ($1::uuid, 'human', $2, 'response', 'support') RETURNING id::text`, idea)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, "DELETE FROM comments WHERE target_id = ANY($1::uuid[])", []string{question, approach, answer, response}) //nolint:errcheck
		pool.Exec(c, "DELETE FROM approaches WHERE id = $1::uuid", approach)                                                   //nolint:errcheck
		pool.Exec(c, "DELETE FROM answers WHERE id = $1::uuid", answer)                                                        //nolint:errcheck
		pool.Exec(c, "DELETE FROM responses WHERE id = $1::uuid", response)                                                    //nolint:errcheck
		pool.Exec(c, "DELETE FROM posts WHERE id = ANY($1::uuid[])", []string{problem, question, idea})                        //nolint:errcheck
	})
	if deleted {
		if _, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = ANY($1::uuid[])", []string{problem, question, idea}); err != nil {
			t.Fatalf("soft-delete posts: %v", err)
		}
	}
	return map[models.CommentTargetType]string{
		models.CommentTargetPost:     question,
		models.CommentTargetApproach: approach,
		models.CommentTargetAnswer:   answer,
		models.CommentTargetResponse: response,
	}
}

// A comment target is visible exactly when the post it belongs to is (the rule GET
// /v1/posts/{id} applies): the target exists and is not deleted, and its post exists, is
// not deleted, and is public or owned by the caller's family.
func TestCommentsRepository_TargetVisibleTo_FollowsTheOwningPost(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	repo := NewCommentsRepository(pool)

	owner := createVisibilityTestUser(t, pool)
	stranger := createVisibilityTestUser(t, pool)
	public := createCommentVisibilityTargets(t, pool, owner, models.VisibilityPublic, "", false)
	family := createCommentVisibilityTargets(t, pool, owner, models.VisibilityFamily, owner, false)
	deleted := createCommentVisibilityTargets(t, pool, owner, models.VisibilityPublic, "", true)
	deletedFamily := createCommentVisibilityTargets(t, pool, owner, models.VisibilityFamily, owner, true)
	withDeletedChildren := createCommentVisibilityTargets(t, pool, owner, models.VisibilityPublic, "", false)
	if _, err := pool.Exec(ctx, "UPDATE approaches SET deleted_at = NOW() WHERE id = $1::uuid", withDeletedChildren[models.CommentTargetApproach]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE answers SET deleted_at = NOW() WHERE id = $1::uuid", withDeletedChildren[models.CommentTargetAnswer]); err != nil {
		t.Fatal(err)
	}

	kinds := []models.CommentTargetType{models.CommentTargetPost, models.CommentTargetApproach, models.CommentTargetAnswer, models.CommentTargetResponse}
	for _, kind := range kinds {
		cases := []struct {
			name, id, caller string
			want             bool
		}{
			{"public, anonymous", public[kind], "", true},
			{"public, another human", public[kind], stranger, true},
			{"family, its owner", family[kind], owner, true},
			{"family, anonymous", family[kind], "", false},
			{"family, another human", family[kind], stranger, false},
			{"deleted post, anonymous", deleted[kind], "", false},
			{"deleted family post, its owner", deletedFamily[kind], owner, false},
			{"absent", uuid.NewString(), owner, false},
			{"malformed", "not-a-valid-id", "", false},
		}
		if kind == models.CommentTargetApproach || kind == models.CommentTargetAnswer {
			cases = append(cases, struct {
				name, id, caller string
				want             bool
			}{"deleted contribution on a public post", withDeletedChildren[kind], "", false})
		}
		for _, tc := range cases {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				got, err := repo.TargetVisibleTo(ctx, kind, tc.id, tc.caller)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("visible = %v, want %v", got, tc.want)
				}
			})
		}
	}
	if _, err := repo.TargetVisibleTo(ctx, models.CommentTargetType("flag"), public[models.CommentTargetPost], ""); err == nil {
		t.Fatal("an unknown target type must be an error")
	}
}

// The list shows a family post's comments (on the post and on its contributions) to the
// family and to nobody else, and its total always counts exactly the rows the caller may see.
func TestCommentsRepository_List_FollowsTheCallersFamily(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	repo := NewCommentsRepository(pool)

	owner := createVisibilityTestUser(t, pool)
	stranger := createVisibilityTestUser(t, pool)
	family := createCommentVisibilityTargets(t, pool, owner, models.VisibilityFamily, owner, false)

	for kind, id := range family {
		if _, err := repo.Create(ctx, &models.Comment{
			TargetType: kind, TargetID: id, AuthorType: models.AuthorTypeHuman, AuthorID: owner, Content: "family comment",
		}); err != nil {
			t.Fatalf("create comment: %v", err)
		}
		for caller, want := range map[string]int{owner: 1, stranger: 0, "": 0} {
			t.Run(string(kind)+"/caller "+caller, func(t *testing.T) {
				got, total, err := repo.List(ctx, models.CommentListOptions{
					TargetType: kind, TargetID: id, Page: 1, PerPage: 10, CallerHuman: caller,
				})
				if err != nil {
					t.Fatalf("List: %v", err)
				}
				if len(got) != want || total != want {
					t.Fatalf("rows = %d, total = %d, want %d and %d", len(got), total, want, want)
				}
			})
		}
	}
}
