// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestFormatSearchResults_SimilarityDisplayed: the semantic cosine similarity is rendered
// as a percent (replacing the old misleading raw-score "Relevance"), and keyword-only
// results (nil similarity) do not fabricate one. BART-155.
func TestFormatSearchResults_SimilarityDisplayed(t *testing.T) {
	results := []models.SearchResult{
		{ID: "sem", Type: "problem", Title: "semantic hit", Score: 0.03, Similarity: ptrFloat64(0.84)},
		{ID: "kw", Type: "answer", Title: "keyword hit", Score: 0.5},
	}
	text := formatSearchResults(results, 2, true)

	if !strings.Contains(text, "Similarity: 84% (semantic)") {
		t.Errorf("expected cosine similarity line for semantic result, got:\n%s", text)
	}
	// The keyword-only result must not present a fake percent from raw ts_rank score.
	if strings.Contains(text, "Relevance: 50%") || strings.Contains(text, "Relevance: 3%") {
		t.Errorf("did not expect misleading raw-score relevance, got:\n%s", text)
	}
}

// TestFormatSearchResults_NoConfidentMatchGuidance: when confident_match is false the
// output leads with ASK guidance; when true it does not.
func TestFormatSearchResults_NoConfidentMatchGuidance(t *testing.T) {
	results := []models.SearchResult{{ID: "p1", Type: "problem", Title: "weak", Similarity: ptrFloat64(0.4)}}

	notConfident := formatSearchResults(results, 1, false)
	if !strings.Contains(notConfident, "No confident match") {
		t.Errorf("expected no-confident-match guidance, got:\n%s", notConfident)
	}

	confident := formatSearchResults(results, 1, true)
	if strings.Contains(confident, "No confident match") {
		t.Errorf("did not expect ASK guidance when confident, got:\n%s", confident)
	}
}

// TestMCPExecuteSearch_NoConfidentMatch: end-to-end through executeSearch, a below-threshold
// top similarity produces the ASK guidance in the tool text (uses the shared mock repo).
func TestMCPExecuteSearch_NoConfidentMatch(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "weak match", Similarity: ptrFloat64(0.4)}}, 1)
	repo.SetTopSimilarity(ptrFloat64(0.4))
	handler := NewMCPHandler(repo, nil) // default threshold 0.85 → 0.4 is not confident

	res, err := handler.executeSearch(context.Background(), map[string]interface{}{"query": "race condition"})
	if err != nil {
		t.Fatalf("executeSearch failed: %v", err)
	}
	text := mcpResultText(t, res)
	if !strings.Contains(text, "No confident match") {
		t.Errorf("expected ASK guidance for below-threshold match, got:\n%s", text)
	}
	if !strings.Contains(text, "Similarity: 40% (semantic)") {
		t.Errorf("expected cosine similarity line, got:\n%s", text)
	}
}

// TestMCPExecuteSearch_ConfidentMatch: an above-threshold top similarity omits the ASK
// guidance and honors a lowered threshold override.
func TestMCPExecuteSearch_ConfidentMatch(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "problem", Title: "strong match", Similarity: ptrFloat64(0.92)}}, 1)
	repo.SetTopSimilarity(ptrFloat64(0.92))
	handler := NewMCPHandler(repo, nil)

	res, err := handler.executeSearch(context.Background(), map[string]interface{}{"query": "race condition"})
	if err != nil {
		t.Fatalf("executeSearch failed: %v", err)
	}
	text := mcpResultText(t, res)
	if strings.Contains(text, "No confident match") {
		t.Errorf("did not expect ASK guidance for confident match, got:\n%s", text)
	}
}

// mcpResultText extracts the text content from an MCP tool result map.
func mcpResultText(t *testing.T, res interface{}) string {
	t.Helper()
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", res)
	}
	content, ok := m["content"].([]map[string]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content, got %v", m["content"])
	}
	text, _ := content[0]["text"].(string)
	return text
}
