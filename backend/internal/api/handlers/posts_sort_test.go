package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListPosts_SortByTop tests that sort=top is passed to the repository.
func TestListPosts_SortByTop(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=top", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "top" {
		t.Errorf("expected sort 'top', got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_SortByVotes tests that sort=votes is passed to the repository.
func TestListPosts_SortByVotes(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=votes", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "votes" {
		t.Errorf("expected sort 'votes', got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_SortByHot tests that sort=hot (trending) is passed to the repository.
func TestListPosts_SortByHot(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=hot", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "hot" {
		t.Errorf("expected sort 'hot', got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_SortByNewest tests that sort=newest is passed to the repository.
func TestListPosts_SortByNewest(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=newest", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "newest" {
		t.Errorf("expected sort 'newest', got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_SortByNew tests that sort=new (frontend alias) is passed to the repository.
func TestListPosts_SortByNew(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=new", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "new" {
		t.Errorf("expected sort 'new', got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_DefaultSortEmpty tests that no sort param defaults to empty string.
func TestListPosts_DefaultSortEmpty(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "" {
		t.Errorf("expected empty sort (default newest), got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_InvalidSortIgnored tests that invalid sort values are silently ignored.
func TestListPosts_InvalidSortIgnored(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=invalid", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "" {
		t.Errorf("expected empty sort for invalid value, got '%s'", repo.listOpts.Sort)
	}
}

// TestListPosts_TimeframeToday tests that timeframe=today is passed to the repository.
func TestListPosts_TimeframeToday(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?timeframe=today", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Timeframe != "today" {
		t.Errorf("expected timeframe 'today', got '%s'", repo.listOpts.Timeframe)
	}
}

// TestListPosts_TimeframeWeek tests that timeframe=week is passed to the repository.
func TestListPosts_TimeframeWeek(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?timeframe=week", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Timeframe != "week" {
		t.Errorf("expected timeframe 'week', got '%s'", repo.listOpts.Timeframe)
	}
}

// TestListPosts_TimeframeMonth tests that timeframe=month is passed to the repository.
func TestListPosts_TimeframeMonth(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?timeframe=month", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Timeframe != "month" {
		t.Errorf("expected timeframe 'month', got '%s'", repo.listOpts.Timeframe)
	}
}

// TestListPosts_InvalidTimeframeIgnored tests that invalid timeframe values are silently ignored.
func TestListPosts_InvalidTimeframeIgnored(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?timeframe=invalid", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Timeframe != "" {
		t.Errorf("expected empty timeframe for invalid value, got '%s'", repo.listOpts.Timeframe)
	}
}

// TestListPosts_SortAndTimeframeCombined tests that sort and timeframe can be used together.
func TestListPosts_SortAndTimeframeCombined(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts(nil, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=top&timeframe=week", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "top" {
		t.Errorf("expected sort 'top', got '%s'", repo.listOpts.Sort)
	}

	if repo.listOpts.Timeframe != "week" {
		t.Errorf("expected timeframe 'week', got '%s'", repo.listOpts.Timeframe)
	}
}
