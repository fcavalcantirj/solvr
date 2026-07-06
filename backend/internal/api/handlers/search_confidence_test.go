// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ptrFloat64 returns a pointer to v (BART-155 test helper).
func ptrFloat64(v float64) *float64 { return &v }

// decodeSearchMeta runs the handler against the given URL and returns the parsed
// data + meta objects. Fails the test on any non-200 or malformed body.
func decodeSearchMeta(t *testing.T, handler *SearchHandler, url string) ([]interface{}, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	handler.Search(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data, _ := resp["data"].([]interface{})
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta object, got %T", resp["meta"])
	}
	return data, meta
}

// TestSearch_Meta_ConfidentMatch_AboveThreshold: a top similarity that clears the
// default confidence bar (0.85) yields meta.confident_match=true and echoes top_similarity.
func TestSearch_Meta_ConfidentMatch_AboveThreshold(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "match", Similarity: ptrFloat64(0.91)}}, 1)
	repo.SetMethod("hybrid")
	repo.SetTopSimilarity(ptrFloat64(0.91))
	handler := NewSearchHandler(repo)

	_, meta := decodeSearchMeta(t, handler, "/v1/search?q=race+condition")

	if meta["confident_match"] != true {
		t.Errorf("expected confident_match=true, got %v", meta["confident_match"])
	}
	ts, ok := meta["top_similarity"].(float64)
	if !ok || ts < 0.90 || ts > 0.92 {
		t.Errorf("expected top_similarity ~0.91, got %v", meta["top_similarity"])
	}
}

// TestSearch_Meta_NotConfident_BelowThreshold: a top similarity below the bar biases to ASK.
func TestSearch_Meta_NotConfident_BelowThreshold(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "weak", Similarity: ptrFloat64(0.42)}}, 1)
	repo.SetMethod("hybrid")
	repo.SetTopSimilarity(ptrFloat64(0.42))
	handler := NewSearchHandler(repo)

	_, meta := decodeSearchMeta(t, handler, "/v1/search?q=race+condition")

	if meta["confident_match"] != false {
		t.Errorf("expected confident_match=false, got %v", meta["confident_match"])
	}
	if ts, ok := meta["top_similarity"].(float64); !ok || ts < 0.41 || ts > 0.43 {
		t.Errorf("expected top_similarity ~0.42, got %v", meta["top_similarity"])
	}
}

// TestSearch_Meta_NotConfident_NilTopSimilarity: keyword-only (fulltext) path has no
// semantic measure — confident_match is false and top_similarity is omitted entirely.
func TestSearch_Meta_NotConfident_NilTopSimilarity(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "kw only"}}, 1)
	repo.SetMethod("fulltext")
	repo.SetTopSimilarity(nil)
	handler := NewSearchHandler(repo)

	_, meta := decodeSearchMeta(t, handler, "/v1/search?q=race+condition")

	if meta["confident_match"] != false {
		t.Errorf("expected confident_match=false for fulltext, got %v", meta["confident_match"])
	}
	if _, present := meta["top_similarity"]; present {
		t.Errorf("expected top_similarity to be omitted for nil, got %v", meta["top_similarity"])
	}
}

// TestSearch_Result_IncludesSimilarity: a per-result cosine similarity is surfaced in the
// response body; a keyword-only result omits it.
func TestSearch_Result_IncludesSimilarity(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{ID: "sem", Type: "problem", Title: "semantic", Similarity: ptrFloat64(0.88)},
		{ID: "kw", Type: "problem", Title: "keyword"},
	}, 2)
	repo.SetMethod("hybrid")
	handler := NewSearchHandler(repo)

	data, _ := decodeSearchMeta(t, handler, "/v1/search?q=race")

	if len(data) != 2 {
		t.Fatalf("expected 2 results, got %d", len(data))
	}
	first := data[0].(map[string]interface{})
	if sim, ok := first["similarity"].(float64); !ok || sim < 0.87 || sim > 0.89 {
		t.Errorf("expected result[0].similarity ~0.88, got %v", first["similarity"])
	}
	second := data[1].(map[string]interface{})
	if _, present := second["similarity"]; present {
		t.Errorf("expected keyword result to omit similarity, got %v", second["similarity"])
	}
}

// TestSearch_MinSimilarity_Parsed: a valid ?min_similarity is forwarded to the repo opts.
func TestSearch_MinSimilarity_Parsed(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)
	handler := NewSearchHandler(repo)

	decodeSearchMeta(t, handler, "/v1/search?q=race&min_similarity=0.7")

	if repo.searchOpts.MinSimilarity != 0.7 {
		t.Errorf("expected opts.MinSimilarity=0.7 forwarded, got %v", repo.searchOpts.MinSimilarity)
	}
}

// TestSearch_MinSimilarity_InvalidIgnored: out-of-range / non-numeric values are ignored
// (default 0 = no filter, full recall).
func TestSearch_MinSimilarity_InvalidIgnored(t *testing.T) {
	cases := []string{"2", "-0.5", "abc", ""}
	for _, v := range cases {
		repo := NewMockSearchRepository()
		repo.SetResults([]models.SearchResult{}, 0)
		handler := NewSearchHandler(repo)

		url := "/v1/search?q=race"
		if v != "" {
			url += "&min_similarity=" + v
		}
		decodeSearchMeta(t, handler, url)

		if repo.searchOpts.MinSimilarity != 0 {
			t.Errorf("min_similarity=%q: expected opts.MinSimilarity=0 (ignored), got %v", v, repo.searchOpts.MinSimilarity)
		}
	}
}

// TestSearch_ConfidenceThreshold_Override: SetConfidenceThreshold lowers the bar so a
// mid similarity now counts as confident (env-driven knob, per BART-155 decision #3).
func TestSearch_ConfidenceThreshold_Override(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "mid", Similarity: ptrFloat64(0.5)}}, 1)
	repo.SetMethod("hybrid")
	repo.SetTopSimilarity(ptrFloat64(0.5))
	handler := NewSearchHandler(repo)
	handler.SetConfidenceThreshold(0.4)

	_, meta := decodeSearchMeta(t, handler, "/v1/search?q=race")

	if meta["confident_match"] != true {
		t.Errorf("expected confident_match=true with threshold 0.4 and topSim 0.5, got %v", meta["confident_match"])
	}
}
