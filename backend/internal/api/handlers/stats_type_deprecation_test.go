package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// knowledgeStatsRepo wires a KnowledgeTotalsReader onto MockStatsRepository so the
// type-specific-statistics adapters (task idx 72, SPEC 26.5) can be exercised through
// their canonical knowledge source instead of their legacy count queries.
type knowledgeStatsRepo struct {
	*MockStatsRepository
	totals []db.KnowledgeTypeTotals
}

func (m *knowledgeStatsRepo) GetKnowledgeTotals(ctx context.Context) ([]db.KnowledgeTypeTotals, error) {
	return m.totals, nil
}

// decodeStatsData decodes the {"data": {...}} envelope the stats endpoints return.
func decodeStatsData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("response has no data object: %v", body)
	}
	return data
}

// TestTypeStats_AnnounceCanonicalSuccessor pins SPEC 26.5: every type-specific statistics
// response marks itself deprecated in favour of GET /v1/overview while keeping its cache
// header. The header is set regardless of whether the knowledge aggregate is wired.
func TestTypeStats_AnnounceCanonicalSuccessor(t *testing.T) {
	cases := []struct {
		name  string
		serve func(*StatsHandler, http.ResponseWriter, *http.Request)
		path  string
	}{
		{"problems", (*StatsHandler).GetProblemsStats, "/v1/stats/problems"},
		{"questions", (*StatsHandler).GetQuestionsStats, "/v1/stats/questions"},
		{"ideas", (*StatsHandler).GetIdeasStats, "/v1/stats/ideas"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewStatsHandler(&MockStatsRepository{})
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()
			tc.serve(handler, rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if got := rec.Header().Get("Deprecation"); got != "true" {
				t.Errorf("expected Deprecation 'true', got %q", got)
			}
			if got := rec.Header().Get("Link"); got != `</v1/overview>; rel="successor-version"` {
				t.Errorf("expected canonical successor Link, got %q", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
				t.Errorf("expected Cache-Control preserved, got %q", got)
			}
		})
	}
}

// TestTypeStats_CountsSourcedFromCanonicalKnowledgeAggregate pins SPEC 26.5: when the
// knowledge aggregate is wired, the count fields come from it, NOT from the legacy count
// queries. The legacy repo deliberately returns 999s so a build that stopped sourcing
// counts from the aggregate would return those and fail here.
func TestTypeStats_CountsSourcedFromCanonicalKnowledgeAggregate(t *testing.T) {
	repo := &knowledgeStatsRepo{
		MockStatsRepository: &MockStatsRepository{
			ProblemsStatsResult: map[string]any{
				"total_problems":      999,
				"solved_count":        999,
				"active_approaches":   4,
				"avg_solve_time_days": 6,
			},
			QuestionsStatsResult: map[string]any{
				"total_questions":         999,
				"answered_count":          999,
				"response_rate":           99.0,
				"avg_response_time_hours": 8.0,
			},
		},
		totals: []db.KnowledgeTypeTotals{
			{Type: "problem", Total: 7, ByStatus: map[string]int{"solved": 3}},
			{Type: "question", Total: 5, WithAcceptedReply: 2},
			{Type: "idea", Total: 11, ByStatus: map[string]int{"spark": 8, "developing": 3}},
			{Type: "post", Total: 2},
		},
	}
	handler := NewStatsHandler(repo)
	handler.SetKnowledgeTotals(repo)

	t.Run("problems", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.GetProblemsStats(rec, httptest.NewRequest("GET", "/v1/stats/problems", nil))
		data := decodeStatsData(t, rec)
		if got := int(data["total_problems"].(float64)); got != 7 {
			t.Errorf("total_problems: want 7 (problem.total), got %d", got)
		}
		if got := int(data["solved_count"].(float64)); got != 3 {
			t.Errorf("solved_count: want 3 (problem.by_status.solved), got %d", got)
		}
		// A legacy-only field with no canonical equivalent stays served (SPEC 26.5).
		if got := int(data["active_approaches"].(float64)); got != 4 {
			t.Errorf("active_approaches (legacy-only) should be preserved, got %d", got)
		}
	})

	t.Run("questions", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.GetQuestionsStats(rec, httptest.NewRequest("GET", "/v1/stats/questions", nil))
		data := decodeStatsData(t, rec)
		if got := int(data["total_questions"].(float64)); got != 5 {
			t.Errorf("total_questions: want 5 (question.total), got %d", got)
		}
		if got := int(data["answered_count"].(float64)); got != 2 {
			t.Errorf("answered_count: want 2 (question.with_accepted_reply), got %d", got)
		}
		if got := data["response_rate"].(float64); got != 40 {
			t.Errorf("response_rate: want 40 (2*100/5), got %v", got)
		}
	})

	t.Run("ideas", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.GetIdeasStats(rec, httptest.NewRequest("GET", "/v1/stats/ideas", nil))
		data := decodeStatsData(t, rec)
		counts, ok := data["counts_by_status"].(map[string]any)
		if !ok {
			t.Fatalf("counts_by_status missing or wrong type: %v", data)
		}
		want := map[string]int{"spark": 8, "developing": 3, "total": 11}
		for k, v := range want {
			got, present := counts[k]
			if !present {
				t.Errorf("counts_by_status[%s] missing", k)
				continue
			}
			if int(got.(float64)) != v {
				t.Errorf("counts_by_status[%s]: want %d, got %v", k, v, got)
			}
		}
	})
}
