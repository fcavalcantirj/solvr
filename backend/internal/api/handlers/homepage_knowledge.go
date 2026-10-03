package handlers

import (
	"context"
	"log/slog"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// KnowledgeTotalsReader reads the one knowledge aggregate GET /v1/overview publishes.
type KnowledgeTotalsReader interface {
	GetKnowledgeTotals(ctx context.Context) ([]db.KnowledgeTypeTotals, error)
}

// OverviewStatsReader reads the product statistics and the knowledge aggregate the overview
// shows (db.CanonicalStatsRepository in production).
type OverviewStatsReader interface {
	GetAllStats(ctx context.Context) (*db.AllStatsResult, error)
	KnowledgeTotalsReader
}

// OverviewKnowledgeType is the knowledge aggregate for one post type ("post" only since the
// legacy post types were retired, idx 68).
type OverviewKnowledgeType struct {
	Type        string         `json:"type"`
	Label       string         `json:"label"`
	Total       int            `json:"total"`
	ByStatus    map[string]int `json:"by_status"`
	WithReplies int            `json:"with_replies"`
	Replies     int            `json:"replies"`
}

// OverviewKnowledge is the overview's knowledge section: the one place post and reply counts
// are defined (task idx 72). It replaced the type-specific statistics routes.
type OverviewKnowledge struct {
	Heading    string                  `json:"heading"`
	Definition string                  `json:"definition"`
	Types      []OverviewKnowledgeType `json:"types"`
}

var overviewKnowledgeLabels = map[string]string{"post": "Posts"}

// buildOverviewKnowledge renders the knowledge aggregate. A nil read renders no types.
func buildOverviewKnowledge(totals []db.KnowledgeTypeTotals) OverviewKnowledge {
	k := OverviewKnowledge{
		Heading: "Knowledge",
		Definition: "Counts the posts GET /v1/posts lists without signing in (public, not deleted, " +
			"not draft, pending review or rejected), split by status, with their canonical replies.",
		Types: make([]OverviewKnowledgeType, 0, len(totals)),
	}
	for _, t := range totals {
		k.Types = append(k.Types, OverviewKnowledgeType{
			Type:        t.Type,
			Label:       overviewKnowledgeLabels[t.Type],
			Total:       t.Total,
			ByStatus:    t.ByStatus,
			WithReplies: t.WithReplies,
			Replies:     t.Replies,
		})
	}
	return k
}

// readOverviewKnowledge reads the knowledge section; a failed read degrades to no types
// and a partial error naming the "knowledge" source.
func (h *HomepageOverviewHandler) readOverviewKnowledge(ctx context.Context) (OverviewKnowledge, string) {
	totals, err := h.statsRepo.GetKnowledgeTotals(ctx)
	if err != nil {
		slog.Error("homepage overview: knowledge totals failed", "error", err)
		return buildOverviewKnowledge(nil), "knowledge statistics unavailable: " + err.Error()
	}
	return buildOverviewKnowledge(totals), ""
}
