package handlers

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func boolPtr(b bool) *bool { return &b }

// TestParsePostListOptions pins the canonical GET /v1/posts filter, ordering,
// pagination and visibility contract ONCE at the handler layer (task idx 72
// step 2: "define filters, ordering, pagination, and visibility once for each
// purpose"). GET /v1/posts is the canonical knowledge-discovery resource and the
// legacy problem/idea/question feeds are adapters that reach parsePostListOptions
// through the SAME parser (legacy_feed.go, posts.go). The request-parsing clamps
// and lenient fallbacks that decide which models.PostListOptions reach the repo
// query had no dedicated test, so a dropped pagination bound, a widened sort/
// timeframe whitelist, or a broken self-view IncludeHidden gate would silently
// change the public discovery contract with nothing failing. This locks it.
//
// Mirrors the rooms-discovery parse contract pinned by
// TestParseRoomListParams (rooms_list_options_test.go).
func TestParsePostListOptions(t *testing.T) {
	// base is the parse result for GET /v1/posts with no query params and no auth.
	base := func() models.PostListOptions {
		return models.PostListOptions{Page: 1, PerPage: 20}
	}

	cases := []struct {
		name    string
		query   string
		claims  *auth.Claims
		want    models.PostListOptions
		wantErr bool
	}{
		// ---- pagination ----
		{
			name:  "defaults when no params",
			query: "",
			want:  base(),
		},
		{
			name:  "valid page and per_page are honoured",
			query: "page=3&per_page=10",
			want:  models.PostListOptions{Page: 3, PerPage: 10},
		},
		{
			name:  "per_page accepts the upper boundary 50",
			query: "per_page=50",
			want:  models.PostListOptions{Page: 1, PerPage: 50},
		},
		{
			name:    "per_page above 50 is rejected",
			query:   "per_page=51",
			wantErr: true,
		},
		{
			name:    "per_page zero is rejected",
			query:   "per_page=0",
			wantErr: true,
		},
		{
			name:    "per_page negative is rejected",
			query:   "per_page=-1",
			wantErr: true,
		},
		{
			name:    "per_page non-integer is rejected",
			query:   "per_page=abc",
			wantErr: true,
		},
		{
			name:    "page zero is rejected",
			query:   "page=0",
			wantErr: true,
		},
		{
			name:    "page negative is rejected",
			query:   "page=-2",
			wantErr: true,
		},
		{
			name:    "page non-integer is rejected",
			query:   "page=x",
			wantErr: true,
		},

		// ---- filters ----
		{
			name:  "type=post selects every post (no type filter)",
			query: "type=post",
			want:  models.PostListOptions{Page: 1, PerPage: 20},
		},
		{
			name:    "a retired legacy type is an error",
			query:   "type=problem",
			wantErr: true,
		},
		{
			name:    "an unknown type is an error",
			query:   "type=bogus",
			wantErr: true,
		},
		{
			name:  "status filter passes through",
			query: "status=open",
			want:  models.PostListOptions{Page: 1, PerPage: 20, Status: models.PostStatus("open")},
		},
		{
			name:  "tags are split on comma and trimmed",
			query: "tags=go,+api+,db",
			want:  models.PostListOptions{Page: 1, PerPage: 20, Tags: []string{"go", "api", "db"}},
		},
		{
			name:  "has_answer true sets pointer to true",
			query: "has_answer=true",
			want:  models.PostListOptions{Page: 1, PerPage: 20, HasAnswer: boolPtr(true)},
		},
		{
			name:  "has_answer false sets pointer to false",
			query: "has_answer=false",
			want:  models.PostListOptions{Page: 1, PerPage: 20, HasAnswer: boolPtr(false)},
		},
		{
			name:  "has_answer other value is ignored (nil)",
			query: "has_answer=maybe",
			want:  base(),
		},
		{
			name:  "needs_help true sets the flag",
			query: "needs_help=true",
			want:  models.PostListOptions{Page: 1, PerPage: 20, NeedsHelp: true},
		},
		{
			name:  "needs_help non-true is false",
			query: "needs_help=1",
			want:  base(),
		},

		// ---- ordering ----
		{
			name:  "sort votes is honoured",
			query: "sort=votes",
			want:  models.PostListOptions{Page: 1, PerPage: 20, Sort: "votes"},
		},
		{
			name:  "sort hot is honoured",
			query: "sort=hot",
			want:  models.PostListOptions{Page: 1, PerPage: 20, Sort: "hot"},
		},
		{
			name:  "unknown sort is ignored (empty)",
			query: "sort=bogus",
			want:  base(),
		},
		{
			name:  "timeframe week is honoured",
			query: "timeframe=week",
			want:  models.PostListOptions{Page: 1, PerPage: 20, Timeframe: "week"},
		},
		{
			name:  "unknown timeframe is ignored (empty)",
			query: "timeframe=year",
			want:  base(),
		},
		{
			name:  "author filter passes through",
			query: "author_type=agent&author_id=abc",
			want:  models.PostListOptions{Page: 1, PerPage: 20, AuthorType: models.AuthorType("agent"), AuthorID: "abc"},
		},

		// ---- visibility ----
		{
			name:  "anonymous caller gets no viewer scoping and no hidden posts",
			query: "",
			want:  base(),
		},
		{
			name:   "authenticated human sets viewer identity and family scope",
			query:  "",
			claims: &auth.Claims{UserID: "user-1", Role: "user"},
			want: models.PostListOptions{
				Page: 1, PerPage: 20,
				ViewerType: models.AuthorTypeHuman, ViewerID: "user-1", ViewerHuman: "user-1",
			},
		},
		{
			name:   "author self-view enables IncludeHidden",
			query:  "author_type=human&author_id=user-1",
			claims: &auth.Claims{UserID: "user-1", Role: "user"},
			want: models.PostListOptions{
				Page: 1, PerPage: 20,
				AuthorType: models.AuthorTypeHuman, AuthorID: "user-1", IncludeHidden: true,
				ViewerType: models.AuthorTypeHuman, ViewerID: "user-1", ViewerHuman: "user-1",
			},
		},
		{
			name:   "viewing another author does NOT enable IncludeHidden",
			query:  "author_type=human&author_id=user-2",
			claims: &auth.Claims{UserID: "user-1", Role: "user"},
			want: models.PostListOptions{
				Page: 1, PerPage: 20,
				AuthorType: models.AuthorTypeHuman, AuthorID: "user-2", IncludeHidden: false,
				ViewerType: models.AuthorTypeHuman, ViewerID: "user-1", ViewerHuman: "user-1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/v1/posts?"+tc.query, nil)
			if tc.claims != nil {
				r = r.WithContext(auth.ContextWithClaims(r.Context(), tc.claims))
			}

			got, err := parsePostListOptions(r)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePostListOptions(%q) = %+v, nil error; want an error", tc.query, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePostListOptions(%q) unexpected error: %v", tc.query, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parsePostListOptions(%q)\n got  %+v\n want %+v", tc.query, got, tc.want)
			}
		})
	}
}
