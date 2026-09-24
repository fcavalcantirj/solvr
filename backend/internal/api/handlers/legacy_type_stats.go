package handlers

import (
	"context"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// KnowledgeTotalsReader reads the one knowledge aggregate GET /v1/overview publishes.
type KnowledgeTotalsReader interface {
	GetKnowledgeTotals(ctx context.Context) ([]db.KnowledgeTypeTotals, error)
}

// SetKnowledgeTotals makes GET /v1/stats/problems|questions|ideas adapters over the
// overview knowledge aggregate (task idx 72, family "type-specific-statistics"): their
// counts come from it instead of their own count queries. The typed sidebar lists they
// still carry are legacy-only fields with no canonical equivalent.
func (h *StatsHandler) SetKnowledgeTotals(k KnowledgeTotalsReader) {
	h.knowledge = k
}

// knowledgeFor returns the knowledge aggregate for one type, or nil when the handler is
// not wired to one.
func (h *StatsHandler) knowledgeFor(ctx context.Context, postType string) (*db.KnowledgeTypeTotals, error) {
	if h.knowledge == nil {
		return nil, nil
	}
	all, err := h.knowledge.GetKnowledgeTotals(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Type == postType {
			return &all[i], nil
		}
	}
	return &db.KnowledgeTypeTotals{Type: postType, ByStatus: map[string]int{}}, nil
}

// applyKnowledgeCounts overwrites a legacy type's count fields with the knowledge aggregate.
func applyKnowledgeCounts(stats map[string]any, k *db.KnowledgeTypeTotals) {
	switch k.Type {
	case "problem":
		stats["total_problems"] = k.Total
		stats["solved_count"] = k.ByStatus["solved"]
	case "question":
		stats["total_questions"] = k.Total
		stats["answered_count"] = k.WithAcceptedReply
		rate := 0.0
		if k.Total > 0 {
			rate = float64(k.WithAcceptedReply) / float64(k.Total) * 100
		}
		stats["response_rate"] = rate
	}
}

// ideaCountsFromKnowledge renders the legacy counts_by_status object (statuses + total).
func ideaCountsFromKnowledge(k *db.KnowledgeTypeTotals) map[string]int {
	counts := make(map[string]int, len(k.ByStatus)+1)
	for status, n := range k.ByStatus {
		counts[status] = n
	}
	counts["total"] = k.Total
	return counts
}

// markTypeStatsDeprecated announces the canonical successor of a type-specific route.
func markTypeStatsDeprecated(w http.ResponseWriter) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</v1/overview>; rel="successor-version"`)
}
