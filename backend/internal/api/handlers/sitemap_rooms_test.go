package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestSitemapHandler_GetSitemapURLs_Rooms(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	mockRepo := &MockSitemapRepository{
		Posts:  []models.SitemapPost{},
		Agents: []models.SitemapAgent{},
		Users:  []models.SitemapUser{},
		Rooms: []models.SitemapRoom{
			{Slug: "room-one", LastActiveAt: now},
			{Slug: "room-two", LastActiveAt: now.Add(-1 * time.Hour)},
		},
	}

	handler := NewSitemapHandler(mockRepo)
	req := httptest.NewRequest("GET", "/v1/sitemap/urls", nil)
	rec := httptest.NewRecorder()

	handler.GetSitemapURLs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := body["data"].(map[string]interface{})

	roomsRaw, ok := data["rooms"]
	if !ok {
		t.Fatal("expected 'rooms' key in response data")
	}

	rooms := roomsRaw.([]interface{})
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}

	room := rooms[0].(map[string]interface{})
	if room["slug"] != "room-one" {
		t.Errorf("expected slug 'room-one', got '%v'", room["slug"])
	}
	if _, ok := room["last_active_at"]; !ok {
		t.Error("expected last_active_at field on room")
	}
}

func TestSitemapHandler_GetSitemapURLs_RoomsPaginated(t *testing.T) {
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
			name:        "type=rooms returns rooms array",
			queryString: "?type=rooms&page=1&per_page=100",
			mockRepo: &MockSitemapRepository{
				PaginatedResult: &models.SitemapURLs{
					Posts:     []models.SitemapPost{},
					Agents:    []models.SitemapAgent{},
					Users:     []models.SitemapUser{},
					BlogPosts: []models.SitemapBlogPost{},
					Rooms:     []models.SitemapRoom{{Slug: "test-room", LastActiveAt: now}},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				roomsRaw, ok := data["rooms"]
				if !ok {
					t.Fatal("expected 'rooms' key in response data")
				}
				rooms := roomsRaw.([]interface{})
				if len(rooms) != 1 {
					t.Errorf("expected 1 room, got %d", len(rooms))
				}
				room := rooms[0].(map[string]interface{})
				if room["slug"] != "test-room" {
					t.Errorf("expected slug 'test-room', got '%v'", room["slug"])
				}
				if _, ok := room["last_active_at"]; !ok {
					t.Error("expected last_active_at field on room")
				}
			},
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				if opts == nil {
					t.Fatal("expected paginated opts to be set")
				}
				if opts.Type != "rooms" {
					t.Errorf("expected type 'rooms', got '%s'", opts.Type)
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
			name:        "type=rooms with defaults",
			queryString: "?type=rooms",
			mockRepo: &MockSitemapRepository{
				PaginatedResult: &models.SitemapURLs{
					Posts:     []models.SitemapPost{},
					Agents:    []models.SitemapAgent{},
					Users:     []models.SitemapUser{},
					BlogPosts: []models.SitemapBlogPost{},
					Rooms:     []models.SitemapRoom{},
				},
			},
			expectedStatus: http.StatusOK,
			checkOpts: func(t *testing.T, opts *models.SitemapURLsOptions) {
				if opts.Page != 1 {
					t.Errorf("expected default page 1, got %d", opts.Page)
				}
				if opts.PerPage != 2500 {
					t.Errorf("expected default per_page 2500, got %d", opts.PerPage)
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

func TestGetSitemapCounts_IncludesRooms(t *testing.T) {
	mockRepo := &MockSitemapRepository{
		Counts: &models.SitemapCounts{
			Posts:     42,
			Agents:    15,
			Users:     8,
			BlogPosts: 7,
			Rooms:     3,
		},
	}

	handler := NewSitemapHandler(mockRepo)
	req := httptest.NewRequest("GET", "/v1/sitemap/counts", nil)
	rec := httptest.NewRecorder()

	handler.GetSitemapCounts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := body["data"].(map[string]interface{})

	roomsCount, ok := data["rooms"]
	if !ok {
		t.Fatal("expected 'rooms' key in counts response")
	}
	if int(roomsCount.(float64)) != 3 {
		t.Errorf("expected rooms=3, got %v", roomsCount)
	}
}
