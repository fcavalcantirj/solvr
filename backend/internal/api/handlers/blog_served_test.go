package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 27.1, blog posts: every answer that carries a blog post serves a clean
// meta_description and excerpt, composed at read time; the stored row is not rewritten.

const servedBody = "# Two agents\n\nThey **plan** and [build](https://x.test) together, then review the result."

func servedFixturePost() models.BlogPostWithAuthor {
	p := createTestBlogPost("served", "Served Blog Post Title")
	p.Body = servedBody
	p.Excerpt = models.GenerateExcerpt(servedBody, 500) // what create stored when no excerpt was given
	p.MetaDescription = ""
	return p
}

const servedDescription = "Two agents They plan and build together, then review the result."

func blogData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Data
}

func TestGetBlogPost_ServesAComposedDescriptionAndAPlainExcerpt(t *testing.T) {
	repo := NewMockBlogPostRepository()
	post := servedFixturePost()
	repo.SetPost(&post)
	r := chi.NewRouter()
	r.Get("/blog/{slug}", NewBlogHandler(repo).GetBySlug)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blog/served", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := blogData(t, w)
	assert.Equal(t, servedDescription, data["meta_description"])
	assert.Equal(t, servedDescription, data["excerpt"])
	assert.Equal(t, servedBody, data["body"], "the body is served as written")
	assert.Equal(t, "", repo.post.MetaDescription, "the repository's post is not changed")
}

func TestGetBlogPost_KeepsTheAuthorsOwnDescription(t *testing.T) {
	repo := NewMockBlogPostRepository()
	post := servedFixturePost()
	post.MetaDescription = "What the author wrote"
	post.Excerpt = "An **own** excerpt"
	repo.SetPost(&post)
	r := chi.NewRouter()
	r.Get("/blog/{slug}", NewBlogHandler(repo).GetBySlug)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blog/served", nil))
	data := blogData(t, w)
	assert.Equal(t, "What the author wrote", data["meta_description"])
	assert.Equal(t, "An own excerpt", data["excerpt"])
}

func TestListAndFeatured_ServeCleanText(t *testing.T) {
	repo := NewMockBlogPostRepository()
	post := servedFixturePost()
	repo.SetPosts([]models.BlogPostWithAuthor{post}, 1)
	repo.SetFeaturedPost(&post)
	h := NewBlogHandler(repo)

	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest(http.MethodGet, "/v1/blog", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var list struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Data, 1)
	assert.Equal(t, servedDescription, list.Data[0]["meta_description"])
	assert.Equal(t, servedDescription, list.Data[0]["excerpt"])

	w = httptest.NewRecorder()
	h.GetFeatured(w, httptest.NewRequest(http.MethodGet, "/v1/blog/featured", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, servedDescription, blogData(t, w)["meta_description"])
}

// An update writes what was stored, never the composed text: a description composed from
// the old body would otherwise outlive the next edit of the body.
func TestUpdateBlogPost_StoresNoComposedText(t *testing.T) {
	repo := NewMockBlogPostRepository()
	post := servedFixturePost()
	repo.SetPost(&post)
	r := chi.NewRouter()
	r.Patch("/blog/{slug}", NewBlogHandler(repo).Update)
	req := httptest.NewRequest(http.MethodPatch, "/blog/served", strings.NewReader(`{"title":"Served Blog Post Title, edited"}`))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, repo.updatedPost)
	assert.Equal(t, "", repo.updatedPost.MetaDescription)
	assert.Equal(t, models.GenerateExcerpt(servedBody, 500), repo.updatedPost.Excerpt)
	assert.Equal(t, servedDescription, blogData(t, w)["meta_description"], "the answer is served like a read")
}

// votingBlogRepo moves the post's counts the way the vote triggers do.
type votingBlogRepo struct {
	*MockBlogPostRepository
	rereadErr error
	voted     bool
}

func (v *votingBlogRepo) Vote(ctx context.Context, id, voterType, voterID, direction string) error {
	if err := v.MockBlogPostRepository.Vote(ctx, id, voterType, voterID, direction); err != nil {
		return err
	}
	if direction == "up" {
		v.post.Upvotes++
	} else {
		v.post.Downvotes++
	}
	v.post.VoteScore = v.post.Upvotes - v.post.Downvotes
	v.post.UserVote = &direction
	v.voted = true
	return nil
}

func (v *votingBlogRepo) FindBySlugForViewer(ctx context.Context, slug string, viewerType models.AuthorType, viewerID string) (*models.BlogPostWithAuthor, error) {
	if v.voted && v.rereadErr != nil {
		return nil, v.rereadErr
	}
	return v.MockBlogPostRepository.FindBySlugForViewer(ctx, slug, viewerType, viewerID)
}

func voteOnServed(t *testing.T, repo *votingBlogRepo, direction string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/blog/{slug}/vote", NewBlogHandler(repo).Vote)
	req := httptest.NewRequest(http.MethodPost, "/blog/served/vote", strings.NewReader(`{"direction":"`+direction+`"}`))
	req = addBlogAuthContext(req, "other-user-456", "user")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// The page reads vote_score from the answer: it went blank after a vote, because the answer
// carried only status and direction (measured in batch C2: "1" became "").
func TestVoteBlogPost_AnswersTheNewCountsUnderTheReadsNames(t *testing.T) {
	post := servedFixturePost() // 5 up, 1 down
	repo := &votingBlogRepo{MockBlogPostRepository: NewMockBlogPostRepository()}
	repo.SetPost(&post)
	w := voteOnServed(t, repo, "up")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := blogData(t, w)
	assert.Equal(t, "ok", data["status"])
	assert.Equal(t, "up", data["direction"])
	assert.EqualValues(t, 5, data["vote_score"])
	assert.EqualValues(t, 6, data["upvotes"])
	assert.EqualValues(t, 1, data["downvotes"])
	assert.Equal(t, "up", data["user_vote"])
}

// When the post cannot be read back the vote still stands; the counts are left out, so the
// page keeps the score it shows instead of a wrong one.
func TestVoteBlogPost_LeavesTheCountsOutWhenThePostCannotBeReadBack(t *testing.T) {
	post := servedFixturePost()
	repo := &votingBlogRepo{MockBlogPostRepository: NewMockBlogPostRepository(), rereadErr: errors.New("db hiccup")}
	repo.SetPost(&post)
	w := voteOnServed(t, repo, "down")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := blogData(t, w)
	assert.Equal(t, "down", data["direction"])
	assert.NotContains(t, data, "vote_score")
	assert.Equal(t, "down", repo.votedDirection)
}
