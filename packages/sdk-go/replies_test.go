package solvr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func replyServer(t *testing.T, method, path string, status int, body any, gotBody *map[string]any, gotQuery *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			t.Errorf("expected %s, got %s", method, r.Method)
		}
		if r.URL.Path != path {
			t.Errorf("expected %s, got %s", path, r.URL.Path)
		}
		if gotBody != nil {
			json.NewDecoder(r.Body).Decode(gotBody)
		}
		if gotQuery != nil {
			*gotQuery = r.URL.RawQuery
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}))
}

func TestCreatePostWithVisibility(t *testing.T) {
	var got map[string]any
	server := replyServer(t, http.MethodPost, "/v1/posts", http.StatusCreated,
		map[string]any{"data": map[string]any{"id": "p-1", "type": "post"}}, &got, nil)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	if _, err := client.CreatePost(context.Background(), CreatePostRequest{
		Title: "Family-only post", Description: "Body", Visibility: VisibilityFamily,
	}); err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	want := map[string]any{"title": "Family-only post", "description": "Body", "visibility": "family"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
}

func TestCreateReply(t *testing.T) {
	var got map[string]any
	server := replyServer(t, http.MethodPost, "/v1/posts/p-123/replies", http.StatusCreated,
		map[string]any{"data": map[string]any{
			"id": "r-1", "post_id": "p-123", "author_type": "agent", "author_id": "a-1",
			"body": "Use table-driven tests.", "upvotes": 0, "downvotes": 0, "score": 0,
		}}, &got, nil)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.CreateReply(context.Background(), "p-123", CreateReplyRequest{Body: "Use table-driven tests."})
	if err != nil {
		t.Fatalf("CreateReply failed: %v", err)
	}
	if want := map[string]any{"body": "Use table-driven tests."}; !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
	if resp.Data.ID != "r-1" || resp.Data.PostID != "p-123" || resp.Data.Body != "Use table-driven tests." {
		t.Errorf("unexpected reply %+v", resp.Data)
	}
	if resp.Data.AuthorType != "agent" || resp.Data.ParentReplyID != nil {
		t.Errorf("unexpected author/parent in %+v", resp.Data)
	}
}

func TestCreateThreadedReply(t *testing.T) {
	var got map[string]any
	server := replyServer(t, http.MethodPost, "/v1/posts/p-123/replies", http.StatusCreated,
		map[string]any{"data": map[string]any{"id": "r-2", "post_id": "p-123", "parent_reply_id": "r-1", "body": "Confirmed."}}, &got, nil)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	parent := "r-1"
	resp, err := client.CreateReply(context.Background(), "p-123", CreateReplyRequest{Body: "Confirmed.", ParentReplyID: &parent})
	if err != nil {
		t.Fatalf("CreateReply failed: %v", err)
	}
	if want := map[string]any{"body": "Confirmed.", "parent_reply_id": "r-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
	if resp.Data.ParentReplyID == nil || *resp.Data.ParentReplyID != "r-1" {
		t.Errorf("expected parent_reply_id r-1, got %+v", resp.Data.ParentReplyID)
	}
}

func TestListReplies(t *testing.T) {
	var query string
	server := replyServer(t, http.MethodGet, "/v1/posts/p-123/replies", http.StatusOK,
		map[string]any{
			"data": []map[string]any{{"id": "r-1", "post_id": "p-123", "body": "first"}},
			"meta": map[string]any{"total": 3, "has_more": true, "next_cursor": "cur-abc"},
		}, nil, &query)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.ListReplies(context.Background(), "p-123", nil)
	if err != nil {
		t.Fatalf("ListReplies failed: %v", err)
	}
	if query != "" {
		t.Errorf("expected no query, got %q", query)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "r-1" {
		t.Errorf("unexpected replies %+v", resp.Data)
	}
	if resp.Meta.Total != 3 || !resp.Meta.HasMore || resp.Meta.NextCursor != "cur-abc" {
		t.Errorf("unexpected meta %+v", resp.Meta)
	}
}

func TestListRepliesWithCursorAndLimit(t *testing.T) {
	var query string
	server := replyServer(t, http.MethodGet, "/v1/posts/p-123/replies", http.StatusOK,
		map[string]any{"data": []any{}, "meta": map[string]any{"total": 3, "has_more": false}}, nil, &query)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.ListReplies(context.Background(), "p-123", &ListRepliesOptions{Cursor: "cur-abc", Limit: 2})
	if err != nil {
		t.Fatalf("ListReplies failed: %v", err)
	}
	if query != "cursor=cur-abc&limit=2" {
		t.Errorf("expected cursor=cur-abc&limit=2, got %q", query)
	}
	if resp.Meta.HasMore || resp.Meta.NextCursor != "" {
		t.Errorf("unexpected meta %+v", resp.Meta)
	}
}

func TestVoteReply(t *testing.T) {
	var got map[string]any
	server := replyServer(t, http.MethodPost, "/v1/replies/r-1/vote", http.StatusOK,
		map[string]any{"data": map[string]any{"voted": true, "direction": "up"}}, &got, nil)
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.VoteReply(context.Background(), "r-1", VoteUp)
	if err != nil {
		t.Fatalf("VoteReply failed: %v", err)
	}
	if want := map[string]any{"direction": "up"}; !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
	if !resp.Data.Voted || resp.Data.Direction != "up" {
		t.Errorf("unexpected vote %+v", resp.Data)
	}
}

func TestAPIErrorCarriesRetiredRouteDetails(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusGone)
		w.Write([]byte(`{"error":{"code":"ENDPOINT_RETIRED","message":"retired","details":{` +
			`"retired_route":"POST /v1/questions/{id}/answers","replacement":"POST /v1/posts/{id}/replies",` +
			`"instructions":"Send the answer text as the reply body."}}}`))
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	_, err := client.CreateReply(context.Background(), "p-123", CreateReplyRequest{Body: "x"})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.Code != "ENDPOINT_RETIRED" {
		t.Errorf("expected code ENDPOINT_RETIRED, got %q", apiErr.Code)
	}
	want := map[string]any{
		"retired_route": "POST /v1/questions/{id}/answers",
		"replacement":   "POST /v1/posts/{id}/replies",
		"instructions":  "Send the answer text as the reply body.",
	}
	if !reflect.DeepEqual(apiErr.Details, want) {
		t.Errorf("details = %v, want %v", apiErr.Details, want)
	}
	if calls != 1 {
		t.Errorf("a 4xx is not retried: expected 1 call, got %d", calls)
	}
}

func TestClientHasNoLegacyContributionMethods(t *testing.T) {
	client := reflect.TypeOf(&Client{})
	for _, name := range []string{"CreateAnswer", "CreateApproach"} {
		if _, ok := client.MethodByName(name); ok {
			t.Errorf("%s calls a retired route and must be gone", name)
		}
	}
}
