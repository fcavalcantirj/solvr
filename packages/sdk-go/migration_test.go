package solvr

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// removedIn2 is every 1.x member 2.0.0 removed: a *Client method, or a
// Struct.Field. The migration notes must name each, and each must be gone.
var removedIn2 = []string{
	"CreateAnswer",
	"CreateApproach",
	"CreatePostRequest.Type",
	"CreatePostRequest.SuccessCriteria",
	"SearchOptions.Type",
	"SearchOptions.Status",
}

var sdkStructs = map[string]reflect.Type{
	"CreatePostRequest": reflect.TypeOf(CreatePostRequest{}),
	"SearchOptions":     reflect.TypeOf(SearchOptions{}),
}

// stillPresent says whether a removedIn2 member is still part of the SDK.
func stillPresent(t *testing.T, member string) bool {
	t.Helper()
	if typ, field, ok := strings.Cut(member, "."); ok {
		st, known := sdkStructs[typ]
		if !known {
			t.Fatalf("%s: no such SDK struct in sdkStructs", typ)
		}
		_, has := st.FieldByName(field)
		return has
	}
	_, has := reflect.TypeOf(&Client{}).MethodByName(member)
	return has
}

func TestSearchOptions_OfferNoLegacyTypeOrStatusFilter(t *testing.T) {
	st := reflect.TypeOf(SearchOptions{})
	for _, legacy := range []string{"Type", "Status"} {
		if _, has := st.FieldByName(legacy); has {
			t.Errorf("SearchOptions.%s is still offered: the legacy post type/status filter is not a choice in 2.0", legacy)
		}
	}
}

// TestSearchAndListPosts_SendNoLegacyFilter sets every option SearchOptions
// offers and checks that neither call sends the legacy type or status filter.
func TestSearchAndListPosts_SendNoLegacyFilter(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}, "meta": map[string]interface{}{}})
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	opts := &SearchOptions{Tags: []string{"go"}, Sort: "newest", PerPage: 5, Page: 2, Limit: 5, Offset: 10}
	if _, err := client.Search(context.Background(), "query", opts); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if _, err := client.ListPosts(context.Background(), opts); err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(queries))
	}
	for i, q := range queries {
		for _, legacy := range []string{"type", "status"} {
			if q.Has(legacy) {
				t.Errorf("request %d sent the legacy %s filter %q", i, legacy, q.Get(legacy))
			}
		}
	}
}

func TestVersion_IsTheMajorTheMigrationNotesDescribeAndTheUserAgentNamesIt(t *testing.T) {
	if Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0 (1.x members were removed)", Version)
	}
	var agent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{}})
	}))
	defer server.Close()

	if _, err := NewClient("k", WithBaseURL(server.URL)).GetPost(context.Background(), "p1"); err != nil {
		t.Fatalf("GetPost: %v", err)
	}
	if agent != "solvr-go/2.0.0" {
		t.Errorf("User-Agent = %q, want solvr-go/2.0.0", agent)
	}
}

// migrationNotes is the "Migrating from 1.x to <Version>" section of the
// package documentation, as go doc renders it.
func migrationNotes(t *testing.T) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "client.go", nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("parse client.go: %v", err)
	}
	doc := f.Doc.Text()
	heading := "# Migrating from 1.x to " + Version
	_, notes, ok := strings.Cut(doc, heading)
	if !ok {
		t.Fatalf("the package documentation has no %q section", heading)
	}
	if next := strings.Index(notes, "\n# "); next >= 0 {
		notes = notes[:next]
	}
	return notes
}

func TestMigrationNotes_NameEveryRemovedMemberAndEachIsGone(t *testing.T) {
	notes := migrationNotes(t)
	for _, member := range removedIn2 {
		if !strings.Contains(notes, member) {
			t.Errorf("the migration notes do not name the removed %s", member)
		}
		if stillPresent(t, member) {
			t.Errorf("%s is named as removed but the SDK still has it", member)
		}
	}
	for _, use := range []string{"CreateReply", "ENDPOINT_RETIRED", `Details["replacement"]`} {
		if !strings.Contains(notes, use) {
			t.Errorf("the migration notes do not say to use %s", use)
		}
	}
}
