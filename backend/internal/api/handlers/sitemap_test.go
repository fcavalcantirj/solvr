package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockSitemapRepository implements SitemapRepositoryInterface for testing
type MockSitemapRepository struct {
	Posts  []models.SitemapPost
	Agents []models.SitemapAgent
	Users  []models.SitemapUser
	Err    error

	// For GetSitemapCounts
	Counts    *models.SitemapCounts
	CountsErr error

	// For GetPaginatedSitemapURLs
	PaginatedResult *models.SitemapURLs
	PaginatedErr    error
	PaginatedOpts   *models.SitemapURLsOptions // captures last call opts
}

func (m *MockSitemapRepository) GetSitemapURLs(ctx context.Context) (*models.SitemapURLs, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &models.SitemapURLs{
		Posts:  m.Posts,
		Agents: m.Agents,
		Users:  m.Users,
	}, nil
}

func (m *MockSitemapRepository) GetSitemapCounts(ctx context.Context) (*models.SitemapCounts, error) {
	if m.CountsErr != nil {
		return nil, m.CountsErr
	}
	return m.Counts, nil
}

func (m *MockSitemapRepository) GetPaginatedSitemapURLs(ctx context.Context, opts models.SitemapURLsOptions) (*models.SitemapURLs, error) {
	m.PaginatedOpts = &opts
	if m.PaginatedErr != nil {
		return nil, m.PaginatedErr
	}
	return m.PaginatedResult, nil
}

func TestSitemapHandler_GetSitemapURLs(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name           string
		mockRepo       *MockSitemapRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "returns all posts, agents, and users",
			mockRepo: &MockSitemapRepository{
				Posts: []models.SitemapPost{
					{ID: "post-1", Type: "problem", UpdatedAt: now},
					{ID: "post-2", Type: "question", UpdatedAt: now},
					{ID: "post-3", Type: "idea", UpdatedAt: now},
				},
				Agents: []models.SitemapAgent{
					{ID: "agent-1", UpdatedAt: now},
					{ID: "agent-2", UpdatedAt: now},
				},
				Users: []models.SitemapUser{
					{ID: "user-1", UpdatedAt: now},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})
				agents := data["agents"].([]interface{})
				users := data["users"].([]interface{})

				if len(posts) != 3 {
					t.Errorf("expected 3 posts, got %d", len(posts))
				}
				if len(agents) != 2 {
					t.Errorf("expected 2 agents, got %d", len(agents))
				}
				if len(users) != 1 {
					t.Errorf("expected 1 user, got %d", len(users))
				}

				// Check post fields
				post := posts[0].(map[string]interface{})
				if post["id"] != "post-1" {
					t.Errorf("expected post id 'post-1', got '%v'", post["id"])
				}
				if post["type"] != "problem" {
					t.Errorf("expected post type 'problem', got '%v'", post["type"])
				}
				if _, ok := post["updated_at"]; !ok {
					t.Error("expected updated_at field on post")
				}

				// Check agent fields
				agent := agents[0].(map[string]interface{})
				if agent["id"] != "agent-1" {
					t.Errorf("expected agent id 'agent-1', got '%v'", agent["id"])
				}

				// Check user fields
				user := users[0].(map[string]interface{})
				if user["id"] != "user-1" {
					t.Errorf("expected user id 'user-1', got '%v'", user["id"])
				}
			},
		},
		{
			name: "returns empty arrays for empty database",
			mockRepo: &MockSitemapRepository{
				Posts:  []models.SitemapPost{},
				Agents: []models.SitemapAgent{},
				Users:  []models.SitemapUser{},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})
				agents := data["agents"].([]interface{})
				users := data["users"].([]interface{})

				if len(posts) != 0 {
					t.Errorf("expected 0 posts, got %d", len(posts))
				}
				if len(agents) != 0 {
					t.Errorf("expected 0 agents, got %d", len(agents))
				}
				if len(users) != 0 {
					t.Errorf("expected 0 users, got %d", len(users))
				}
			},
		},
		{
			name: "returns 500 on repository error",
			mockRepo: &MockSitemapRepository{
				Err: context.DeadlineExceeded,
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INTERNAL_ERROR" {
					t.Errorf("expected error code 'INTERNAL_ERROR', got '%v'", errObj["code"])
				}
			},
		},
		{
			name: "excludes drafts and deleted posts (repo handles filtering)",
			mockRepo: &MockSitemapRepository{
				Posts: []models.SitemapPost{
					{ID: "open-post", Type: "problem", UpdatedAt: now},
				},
				Agents: []models.SitemapAgent{},
				Users:  []models.SitemapUser{},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})
				if len(posts) != 1 {
					t.Errorf("expected 1 post (non-draft, non-deleted), got %d", len(posts))
				}
				post := posts[0].(map[string]interface{})
				if post["id"] != "open-post" {
					t.Errorf("expected post id 'open-post', got '%v'", post["id"])
				}
			},
		},
		{
			name: "returns correct content types in response",
			mockRepo: &MockSitemapRepository{
				Posts: []models.SitemapPost{
					{ID: "p1", Type: "problem", UpdatedAt: now},
					{ID: "q1", Type: "question", UpdatedAt: now},
					{ID: "i1", Type: "idea", UpdatedAt: now},
				},
				Agents: []models.SitemapAgent{{ID: "a1", UpdatedAt: now}},
				Users:  []models.SitemapUser{{ID: "u1", UpdatedAt: now}},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})

				types := map[string]bool{}
				for _, p := range posts {
					post := p.(map[string]interface{})
					types[post["type"].(string)] = true
				}
				for _, expected := range []string{"problem", "question", "idea"} {
					if !types[expected] {
						t.Errorf("expected post type '%s' in results", expected)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSitemapHandler(tt.mockRepo)
			req := httptest.NewRequest("GET", "/v1/sitemap/urls", nil)
			rec := httptest.NewRecorder()

			handler.GetSitemapURLs(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Check Content-Type
			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}
		})
	}
}

func TestSitemapHandler_GetSitemapCounts(t *testing.T) {
	tests := []struct {
		name           string
		mockRepo       *MockSitemapRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "returns correct counts",
			mockRepo: &MockSitemapRepository{
				Counts: &models.SitemapCounts{
					Posts:  42,
					Agents: 15,
					Users:  8,
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				if int(data["posts"].(float64)) != 42 {
					t.Errorf("expected posts=42, got %v", data["posts"])
				}
				if int(data["agents"].(float64)) != 15 {
					t.Errorf("expected agents=15, got %v", data["agents"])
				}
				if int(data["users"].(float64)) != 8 {
					t.Errorf("expected users=8, got %v", data["users"])
				}
			},
		},
		{
			name: "returns zeros for empty database",
			mockRepo: &MockSitemapRepository{
				Counts: &models.SitemapCounts{
					Posts:  0,
					Agents: 0,
					Users:  0,
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				if int(data["posts"].(float64)) != 0 {
					t.Errorf("expected posts=0, got %v", data["posts"])
				}
				if int(data["agents"].(float64)) != 0 {
					t.Errorf("expected agents=0, got %v", data["agents"])
				}
				if int(data["users"].(float64)) != 0 {
					t.Errorf("expected users=0, got %v", data["users"])
				}
			},
		},
		{
			name: "returns 500 on repository error",
			mockRepo: &MockSitemapRepository{
				CountsErr: context.DeadlineExceeded,
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INTERNAL_ERROR" {
					t.Errorf("expected error code 'INTERNAL_ERROR', got '%v'", errObj["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSitemapHandler(tt.mockRepo)
			req := httptest.NewRequest("GET", "/v1/sitemap/counts", nil)
			rec := httptest.NewRecorder()

			handler.GetSitemapCounts(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}
		})
	}
}

func TestSitemapHandler_GetSitemapURLs_Pagination(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name           string
		queryString    string
		mockRepo       *MockSitemapRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
		checkOpts      func(t *testing.T, opts *models.SitemapURLsOptions)
	}{
		{
			name:        "type=posts returns only posts array populated",
			queryString: "?type=posts&page=1&per_page=100",
			mockRepo: &MockSitemapRepository{
				PaginatedResult: &models.SitemapURLs{
					Posts:  []models.SitemapPost{{ID: "p1", Type: "problem", UpdatedAt: now}},
					Agents: []models.SitemapAgent{},
					Users:  []models.SitemapUser{},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})
				agents := data["agents"].([]interface{})
				users := data["users"].([]interface{})
				if len(posts) != 1 {
					t.Errorf("expected 1 post, got %d", len(posts))
				}
				if len(agents) != 0 {
					t.Errorf("expected 0 agents, got %d", len(agents))
				}
				if len(users) != 0 {
					t.Errorf("expected 0 users, got %d", len(users))
				}
			},
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				if opts == nil {
					t.Fatal("expected paginated opts to be set")
				}
				if opts.Type != "posts" {
					t.Errorf("expected type 'posts', got '%s'", opts.Type)
				}
				if opts.Page != 1 {
					t.Errorf("expected page 1, got %d", opts.Page)
				}
				if opts.PerPage != 100 {
					t.Errorf("expected per_page 100, got %d", opts.PerPage)
				}
			},
		},
		{
			name:        "type=agents returns only agents",
			queryString: "?type=agents&page=2&per_page=50",
			mockRepo: &MockSitemapRepository{
				PaginatedResult: &models.SitemapURLs{
					Posts:  []models.SitemapPost{},
					Agents: []models.SitemapAgent{{ID: "a1", UpdatedAt: now}},
					Users:  []models.SitemapUser{},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				agents := data["agents"].([]interface{})
				if len(agents) != 1 {
					t.Errorf("expected 1 agent, got %d", len(agents))
				}
			},
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				if opts.Type != "agents" {
					t.Errorf("expected type 'agents', got '%s'", opts.Type)
				}
				if opts.Page != 2 {
					t.Errorf("expected page 2, got %d", opts.Page)
				}
				if opts.PerPage != 50 {
					t.Errorf("expected per_page 50, got %d", opts.PerPage)
				}
			},
		},
		{
			name:        "type=users returns only users",
			queryString: "?type=users",
			mockRepo: &MockSitemapRepository{
				PaginatedResult: &models.SitemapURLs{
					Posts:  []models.SitemapPost{},
					Agents: []models.SitemapAgent{},
					Users:  []models.SitemapUser{{ID: "u1", UpdatedAt: now}},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				users := data["users"].([]interface{})
				if len(users) != 1 {
					t.Errorf("expected 1 user, got %d", len(users))
				}
			},
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				if opts.Type != "users" {
					t.Errorf("expected type 'users', got '%s'", opts.Type)
				}
				// Should default to page=1, per_page=5000
				if opts.Page != 1 {
					t.Errorf("expected default page 1, got %d", opts.Page)
				}
				if opts.PerPage != 5000 {
					t.Errorf("expected default per_page 5000, got %d", opts.PerPage)
				}
			},
		},
		{
			name:        "invalid type returns 400",
			queryString: "?type=invalid",
			mockRepo:    &MockSitemapRepository{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INVALID_PARAM" {
					t.Errorf("expected error code 'INVALID_PARAM', got '%v'", errObj["code"])
				}
			},
		},
		{
			name:        "per_page exceeding max returns 400",
			queryString: "?type=posts&per_page=10000",
			mockRepo:    &MockSitemapRepository{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INVALID_PARAM" {
					t.Errorf("expected error code 'INVALID_PARAM', got '%v'", errObj["code"])
				}
			},
		},
		{
			name:        "page < 1 returns 400",
			queryString: "?type=posts&page=0",
			mockRepo:    &MockSitemapRepository{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INVALID_PARAM" {
					t.Errorf("expected error code 'INVALID_PARAM', got '%v'", errObj["code"])
				}
			},
		},
		{
			name:        "no type param falls back to existing behavior (backward compat)",
			queryString: "",
			mockRepo: &MockSitemapRepository{
				Posts:  []models.SitemapPost{{ID: "p1", Type: "problem", UpdatedAt: now}},
				Agents: []models.SitemapAgent{{ID: "a1", UpdatedAt: now}},
				Users:  []models.SitemapUser{{ID: "u1", UpdatedAt: now}},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				posts := data["posts"].([]interface{})
				agents := data["agents"].([]interface{})
				users := data["users"].([]interface{})
				if len(posts) != 1 {
					t.Errorf("expected 1 post, got %d", len(posts))
				}
				if len(agents) != 1 {
					t.Errorf("expected 1 agent, got %d", len(agents))
				}
				if len(users) != 1 {
					t.Errorf("expected 1 user, got %d", len(users))
				}
			},
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				// Should NOT have called paginated path
				if opts != nil {
					t.Error("expected paginated path NOT to be called for backward compat")
				}
			},
		},
		{
			name:        "paginated repo error returns 500",
			queryString: "?type=posts",
			mockRepo: &MockSitemapRepository{
				PaginatedErr: context.DeadlineExceeded,
			},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				errObj := body["error"].(map[string]interface{})
				if errObj["code"] != "INTERNAL_ERROR" {
					t.Errorf("expected error code 'INTERNAL_ERROR', got '%v'", errObj["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSitemapHandler(tt.mockRepo)
			req := httptest.NewRequest("GET", "/v1/sitemap/urls"+tt.queryString, nil)
			rec := httptest.NewRecorder()

			handler.GetSitemapURLs(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}

			if tt.checkOpts != nil {
				tt.checkOpts(t, tt.mockRepo.PaginatedOpts)
			}
		})
	}
}
