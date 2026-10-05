package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SPEC.md 27.2: every GET /v1/posts answer says how many pages the list has at the per_page
// asked for, so the post archive can name its last page and answer 404 past it. The list
// itself keeps answering 200 with no rows past the end.
func TestListPosts_MetaTotalPages(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		total     int
		wantPages int
		wantMore  bool
	}{
		{"an empty list has no pages", "per_page=50", 0, 0, false},
		{"one post is one page", "per_page=50", 1, 1, false},
		{"a full page is one page", "per_page=50", 50, 1, false},
		{"one more post starts a second page", "per_page=50", 51, 2, true},
		{"467 posts at fifty a page are ten pages", "indexable=true&sort=new&page=2&per_page=50", 467, 10, true},
		{"the last page has no more", "page=10&per_page=50", 467, 10, false},
		{"the default page size counts too", "", 41, 3, true},
		{"a page past the last is still answered", "page=99&per_page=50", 467, 10, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMockPostsRepository()
			repo.SetPosts([]models.PostWithAuthor{}, tc.total)
			w := httptest.NewRecorder()

			NewPostsHandler(repo).List(w, httptest.NewRequest(http.MethodGet, "/v1/posts?"+tc.query, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			var resp struct {
				Meta map[string]any `json:"meta"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			pages, ok := resp.Meta["total_pages"].(float64)
			if !ok {
				t.Fatalf("meta.total_pages missing or not a number: %v", resp.Meta)
			}
			if int(pages) != tc.wantPages {
				t.Errorf("meta.total_pages = %v, want %d", pages, tc.wantPages)
			}
			if resp.Meta["has_more"] != tc.wantMore {
				t.Errorf("meta.has_more = %v, want %v", resp.Meta["has_more"], tc.wantMore)
			}
			if int(resp.Meta["total"].(float64)) != tc.total {
				t.Errorf("meta.total = %v, want %d", resp.Meta["total"], tc.total)
			}
		})
	}
}

// indexable=true reaches the repository as the one flag the query reads; without it the
// list is the plain one.
func TestListPosts_IndexableReachesTheRepository(t *testing.T) {
	for query, want := range map[string]bool{
		"indexable=true":                  true,
		"indexable=true&sort=new&page=2":  true,
		"":                                false,
		"indexable=false":                 false,
		"indexable=TRUE":                  false,
		"author_type=agent&author_id=one": false,
	} {
		repo := NewMockPostsRepository()
		repo.SetPosts([]models.PostWithAuthor{}, 0)
		w := httptest.NewRecorder()

		NewPostsHandler(repo).List(w, httptest.NewRequest(http.MethodGet, "/v1/posts?"+query, nil))

		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/posts?%s: status %d", query, w.Code)
		}
		if repo.listOpts.Indexable != want {
			t.Errorf("GET /v1/posts?%s: Indexable = %v, want %v", query, repo.listOpts.Indexable, want)
		}
	}
}

// The OpenAPI contract documents the list's query parameters from this one list
// (api.TestOpenAPIPosts_ListDocumentsExactlyTheParametersTheHandlerReads), so a parameter
// the parser reads cannot go undocumented.
func TestPostListParamNames_AreTheParametersTheParserReads(t *testing.T) {
	want := []string{
		"author_id", "author_type", "has_answer", "indexable", "needs_help", "page", "per_page",
		"sort", "status", "tags", "timeframe", "type",
	}
	got := append([]string{}, PostListParamNames()...)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("PostListParamNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PostListParamNames() = %v, want %v", got, want)
		}
	}

	// Each named parameter changes what the parser answers, so each is really read.
	base, err := parsePostListOptions(httptest.NewRequest(http.MethodGet, "/v1/posts", nil))
	if err != nil {
		t.Fatal(err)
	}
	probes := map[string]string{
		"author_id": "author_id=one", "author_type": "author_type=agent", "has_answer": "has_answer=true",
		"indexable": "indexable=true", "needs_help": "needs_help=true", "page": "page=2", "per_page": "per_page=7",
		"sort": "sort=top", "status": "status=open", "tags": "tags=go", "timeframe": "timeframe=week",
		"type": "type=bogus",
	}
	for _, name := range want {
		got, err := parsePostListOptions(httptest.NewRequest(http.MethodGet, "/v1/posts?"+probes[name], nil))
		changed := err != nil || got.Page != base.Page || got.PerPage != base.PerPage || got.Sort != base.Sort ||
			got.Status != base.Status || got.Timeframe != base.Timeframe || got.AuthorID != base.AuthorID ||
			got.AuthorType != base.AuthorType || got.Indexable != base.Indexable || got.NeedsHelp != base.NeedsHelp ||
			got.HasAnswer != nil || len(got.Tags) != 0
		if !changed {
			t.Errorf("%s (%s) changed nothing: the parser does not read it", name, probes[name])
		}
	}
}
