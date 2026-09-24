package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestParseRoomListParams pins the GET /v1/rooms room-discovery pagination,
// ordering and filter contract ONCE at the handler layer (task idx 72 step 2:
// "define filters, ordering, pagination, and visibility once for each purpose").
// The db.RoomRepository.ListFiltered visibility/ordering behaviour is guarded by
// rooms_listfiltered_test.go; the request-parsing clamps that decide which
// RoomListParams reach that query had no test, so a dropped bound (e.g. the
// limit <= 100 clamp or the lenient fallbacks) would silently change the public
// discovery contract with nothing failing. This locks the parsing contract.
func TestParseRoomListParams(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  db.RoomListParams
	}{
		{
			name:  "defaults when no params",
			query: "",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent", Query: "", IncludeArchived: false},
		},
		{
			name:  "valid limit is honoured",
			query: "limit=5",
			want:  db.RoomListParams{Limit: 5, Offset: 0, Sort: "recent"},
		},
		{
			name:  "limit accepts the upper boundary 100",
			query: "limit=100",
			want:  db.RoomListParams{Limit: 100, Offset: 0, Sort: "recent"},
		},
		{
			name:  "limit above 100 falls back to default 20",
			query: "limit=101",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "limit zero falls back to default 20",
			query: "limit=0",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "negative limit falls back to default 20",
			query: "limit=-5",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "non-integer limit falls back to default 20",
			query: "limit=abc",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "valid offset is honoured",
			query: "offset=10",
			want:  db.RoomListParams{Limit: 20, Offset: 10, Sort: "recent"},
		},
		{
			name:  "negative offset falls back to 0",
			query: "offset=-1",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "non-integer offset falls back to 0",
			query: "offset=xyz",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "sort=active is honoured",
			query: "sort=active",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "active"},
		},
		{
			name:  "sort=recent is honoured",
			query: "sort=recent",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "unknown sort falls back to recent",
			query: "sort=bogus",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent"},
		},
		{
			name:  "query is trimmed of surrounding whitespace",
			query: "q=%20%20hello%20%20",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent", Query: "hello"},
		},
		{
			name:  "include_archived=true sets the flag",
			query: "include_archived=true",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent", IncludeArchived: true},
		},
		{
			name:  "include_archived other value leaves the flag false",
			query: "include_archived=yes",
			want:  db.RoomListParams{Limit: 20, Offset: 0, Sort: "recent", IncludeArchived: false},
		},
		{
			name:  "all params together",
			query: "limit=7&offset=3&sort=active&q=%20widget%20&include_archived=true",
			want:  db.RoomListParams{Limit: 7, Offset: 3, Sort: "active", Query: "widget", IncludeArchived: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/rooms?"+tc.query, nil)
			got := parseRoomListParams(req)
			if got != tc.want {
				t.Errorf("parseRoomListParams(?%s) = %+v, want %+v", tc.query, got, tc.want)
			}
		})
	}
}
