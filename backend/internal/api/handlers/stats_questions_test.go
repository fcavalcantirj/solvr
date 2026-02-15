package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsHandler_GetQuestionsStats(t *testing.T) {
	t.Run("returns all stats fields", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         100,
				"answered_count":          75,
				"response_rate":           75.0,
				"avg_response_time_hours": 4.5,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		if int(data["total_questions"].(float64)) != 100 {
			t.Errorf("expected total_questions=100, got %v", data["total_questions"])
		}
		if int(data["answered_count"].(float64)) != 75 {
			t.Errorf("expected answered_count=75, got %v", data["answered_count"])
		}
		if data["response_rate"].(float64) != 75.0 {
			t.Errorf("expected response_rate=75.0, got %v", data["response_rate"])
		}
		if data["avg_response_time_hours"].(float64) != 4.5 {
			t.Errorf("expected avg_response_time_hours=4.5, got %v", data["avg_response_time_hours"])
		}
	})

	t.Run("returns zeros for empty database", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         0,
				"answered_count":          0,
				"response_rate":           0.0,
				"avg_response_time_hours": 0.0,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		if int(data["total_questions"].(float64)) != 0 {
			t.Errorf("expected total_questions=0, got %v", data["total_questions"])
		}
	})

	t.Run("returns 500 on repository error", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsErr: fmt.Errorf("database error"),
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		errObj := body["error"].(map[string]interface{})
		if errObj["code"] != "INTERNAL_ERROR" {
			t.Errorf("expected error code INTERNAL_ERROR, got %v", errObj["code"])
		}
	})

	t.Run("includes recently_answered in response", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         50,
				"answered_count":          30,
				"response_rate":           60.0,
				"avg_response_time_hours": 2.0,
			},
			RecentlyAnsweredResult: []map[string]any{
				{"id": "q1", "title": "How to fix auth?", "answerer_name": "helper-bot", "answerer_type": "agent", "time_to_answer_hours": 3.5},
				{"id": "q2", "title": "Best Go patterns", "answerer_name": "alice", "answerer_type": "human", "time_to_answer_hours": 1.2},
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		recentlyAnswered := data["recently_answered"].([]interface{})
		if len(recentlyAnswered) != 2 {
			t.Errorf("expected 2 recently answered, got %d", len(recentlyAnswered))
		}

		first := recentlyAnswered[0].(map[string]interface{})
		if first["title"] != "How to fix auth?" {
			t.Errorf("expected first title 'How to fix auth?', got %v", first["title"])
		}
	})

	t.Run("recently_answered is empty array when none exist", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         5,
				"answered_count":          0,
				"response_rate":           0.0,
				"avg_response_time_hours": 0.0,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		recentlyAnswered := data["recently_answered"].([]interface{})
		if len(recentlyAnswered) != 0 {
			t.Errorf("expected 0 recently answered, got %d", len(recentlyAnswered))
		}
	})

	t.Run("includes top_answerers in response", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         50,
				"answered_count":          30,
				"response_rate":           60.0,
				"avg_response_time_hours": 2.0,
			},
			TopAnswerersResult: []map[string]any{
				{"author_id": "a1", "display_name": "answer-bot", "author_type": "agent", "answer_count": 15, "accept_rate": 80.0},
				{"author_id": "u1", "display_name": "bob", "author_type": "human", "answer_count": 10, "accept_rate": 60.0},
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		topAnswerers := data["top_answerers"].([]interface{})
		if len(topAnswerers) != 2 {
			t.Errorf("expected 2 top answerers, got %d", len(topAnswerers))
		}

		first := topAnswerers[0].(map[string]interface{})
		if first["display_name"] != "answer-bot" {
			t.Errorf("expected first display_name 'answer-bot', got %v", first["display_name"])
		}
		if int(first["answer_count"].(float64)) != 15 {
			t.Errorf("expected first answer_count 15, got %v", first["answer_count"])
		}
	})

	t.Run("top_answerers is empty array when none exist", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			QuestionsStatsResult: map[string]any{
				"total_questions":         5,
				"answered_count":          0,
				"response_rate":           0.0,
				"avg_response_time_hours": 0.0,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/questions", nil)
		rec := httptest.NewRecorder()

		handler.GetQuestionsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		topAnswerers := data["top_answerers"].([]interface{})
		if len(topAnswerers) != 0 {
			t.Errorf("expected 0 top answerers, got %d", len(topAnswerers))
		}
	})
}
