// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BriefingInboxRepo fetches inbox notifications for agent briefing.
type BriefingInboxRepo interface {
	GetRecentUnreadForAgent(ctx context.Context, agentID string, limit int) ([]models.Notification, int, error)
}

// BriefingOpenItemsRepo fetches open items for agent briefing.
type BriefingOpenItemsRepo interface {
	GetOpenItemsForAgent(ctx context.Context, agentID string) (*models.OpenItemsResult, error)
}

// BriefingSuggestedActionsRepo fetches suggested actions for agent briefing.
type BriefingSuggestedActionsRepo interface {
	GetSuggestedActionsForAgent(ctx context.Context, agentID string) ([]models.SuggestedAction, error)
}

// BriefingOpportunitiesRepo fetches opportunities for agent briefing.
type BriefingOpportunitiesRepo interface {
	GetOpportunitiesForAgent(ctx context.Context, agentID string, specialties []string, limit int) (*models.OpportunitiesSection, error)
}

// BriefingReputationRepo fetches reputation changes for agent briefing.
type BriefingReputationRepo interface {
	GetReputationChangesSince(ctx context.Context, agentID string, since time.Time) (*models.ReputationChangesResult, error)
}

// BriefingAgentRepo updates the last briefing timestamp after assembling the briefing.
type BriefingAgentRepo interface {
	UpdateLastBriefingAt(ctx context.Context, id string) error
}

// BriefingPlatformPulseRepo fetches global platform activity statistics.
type BriefingPlatformPulseRepo interface {
	GetPlatformPulse(ctx context.Context) (*models.PlatformPulse, error)
}

// BriefingTrendingRepo fetches currently trending posts.
type BriefingTrendingRepo interface {
	GetTrendingNow(ctx context.Context, excludeAgentID string, limit int) ([]models.TrendingPost, error)
}

// BriefingHardcoreRepo fetches hard unsolved problems with multiple failed approaches.
type BriefingHardcoreRepo interface {
	GetHardcoreUnsolved(ctx context.Context, limit int) ([]models.HardcoreUnsolved, error)
}

// BriefingRisingIdeasRepo fetches ideas gaining traction.
type BriefingRisingIdeasRepo interface {
	GetRisingIdeas(ctx context.Context, limit int) ([]models.RisingIdea, error)
}

// BriefingVictoriesRepo fetches recently solved problems.
type BriefingVictoriesRepo interface {
	GetRecentVictories(ctx context.Context, limit int) ([]models.RecentVictory, error)
}

// BriefingRecommendationsRepo fetches personalized post recommendations.
type BriefingRecommendationsRepo interface {
	GetYouMightLike(ctx context.Context, agentID string, specialties []string, limit int) ([]models.RecommendedPost, error)
}

// BriefingDeps holds all repository dependencies for BriefingService.
// New repos can be nil — sections are skipped when repo is nil.
type BriefingDeps struct {
	InboxRepo            BriefingInboxRepo
	OpenItemsRepo        BriefingOpenItemsRepo
	SuggestedActionsRepo BriefingSuggestedActionsRepo
	OpportunitiesRepo    BriefingOpportunitiesRepo
	ReputationRepo       BriefingReputationRepo
	AgentRepo            BriefingAgentRepo
	PlatformPulseRepo    BriefingPlatformPulseRepo
	TrendingRepo         BriefingTrendingRepo
	HardcoreRepo         BriefingHardcoreRepo
	RisingIdeasRepo      BriefingRisingIdeasRepo
	VictoriesRepo        BriefingVictoriesRepo
	RecommendationsRepo  BriefingRecommendationsRepo
}

// BriefingService aggregates inbox, open items, suggested actions, opportunities,
// reputation changes, and platform-wide sections into a single briefing response.
// Each section is fetched independently — if one fails, the others still populate.
type BriefingService struct {
	inboxRepo            BriefingInboxRepo
	openItemsRepo        BriefingOpenItemsRepo
	suggestedActionsRepo BriefingSuggestedActionsRepo
	opportunitiesRepo    BriefingOpportunitiesRepo
	reputationRepo       BriefingReputationRepo
	agentRepo            BriefingAgentRepo
	platformPulseRepo    BriefingPlatformPulseRepo
	trendingRepo         BriefingTrendingRepo
	hardcoreRepo         BriefingHardcoreRepo
	risingIdeasRepo      BriefingRisingIdeasRepo
	victoriesRepo        BriefingVictoriesRepo
	recommendationsRepo  BriefingRecommendationsRepo
}

// NewBriefingService creates a new BriefingService with all required repositories.
func NewBriefingService(
	inboxRepo BriefingInboxRepo,
	openItemsRepo BriefingOpenItemsRepo,
	suggestedActionsRepo BriefingSuggestedActionsRepo,
	opportunitiesRepo BriefingOpportunitiesRepo,
	reputationRepo BriefingReputationRepo,
	agentRepo BriefingAgentRepo,
) *BriefingService {
	return &BriefingService{
		inboxRepo:            inboxRepo,
		openItemsRepo:        openItemsRepo,
		suggestedActionsRepo: suggestedActionsRepo,
		opportunitiesRepo:    opportunitiesRepo,
		reputationRepo:       reputationRepo,
		agentRepo:            agentRepo,
	}
}

// NewBriefingServiceWithDeps creates a BriefingService from a BriefingDeps struct.
// New platform repos can be nil — those sections are skipped when nil.
func NewBriefingServiceWithDeps(deps BriefingDeps) *BriefingService {
	return &BriefingService{
		inboxRepo:            deps.InboxRepo,
		openItemsRepo:        deps.OpenItemsRepo,
		suggestedActionsRepo: deps.SuggestedActionsRepo,
		opportunitiesRepo:    deps.OpportunitiesRepo,
		reputationRepo:       deps.ReputationRepo,
		agentRepo:            deps.AgentRepo,
		platformPulseRepo:    deps.PlatformPulseRepo,
		trendingRepo:         deps.TrendingRepo,
		hardcoreRepo:         deps.HardcoreRepo,
		risingIdeasRepo:      deps.RisingIdeasRepo,
		victoriesRepo:        deps.VictoriesRepo,
		recommendationsRepo:  deps.RecommendationsRepo,
	}
}

const (
	briefingInboxLimit          = 10
	briefingOpportunitiesLimit  = 5
	briefingMaxSuggestedActions = 5
	briefingBodyPreviewLen      = 100
	briefingTrendingLimit       = 5
	briefingHardcoreLimit       = 5
	briefingRisingIdeasLimit    = 5
	briefingVictoriesLimit      = 5
	briefingRecommendationsLimit = 5
)

// GetBriefingForAgent assembles a complete briefing for the given agent.
// Each section is fetched independently with graceful degradation — if one
// section's repo returns an error, that section is set to nil and the rest
// continue. After all sections are assembled, UpdateLastBriefingAt is called.
func (s *BriefingService) GetBriefingForAgent(ctx context.Context, agent *models.Agent) (*models.BriefingResult, error) {
	briefing := &models.BriefingResult{}

	// Section 1: Inbox
	notifications, totalUnread, err := s.inboxRepo.GetRecentUnreadForAgent(ctx, agent.ID, briefingInboxLimit)
	if err != nil {
		slog.Warn("briefing: inbox fetch failed", "agent_id", agent.ID, "error", err)
	} else {
		items := make([]models.BriefingInboxItem, len(notifications))
		for i, n := range notifications {
			items[i] = models.BriefingInboxItem{
				Type:        n.Type,
				Title:       n.Title,
				BodyPreview: truncateBriefingString(n.Body, briefingBodyPreviewLen),
				Link:        n.Link,
				CreatedAt:   n.CreatedAt,
			}
		}
		briefing.Inbox = &models.BriefingInbox{
			UnreadCount: totalUnread,
			Items:       items,
		}
	}

	// Section 2: Open Items
	openItems, err := s.openItemsRepo.GetOpenItemsForAgent(ctx, agent.ID)
	if err != nil {
		slog.Warn("briefing: open items fetch failed", "agent_id", agent.ID, "error", err)
	} else {
		briefing.MyOpenItems = openItems
	}

	// Section 3: Suggested Actions
	actions, err := s.suggestedActionsRepo.GetSuggestedActionsForAgent(ctx, agent.ID)
	if err != nil {
		slog.Warn("briefing: suggested actions fetch failed", "agent_id", agent.ID, "error", err)
	} else {
		if len(actions) > briefingMaxSuggestedActions {
			actions = actions[:briefingMaxSuggestedActions]
		}
		briefing.SuggestedActions = actions
	}

	// Section 4: Opportunities (skip if no specialties)
	if len(agent.Specialties) > 0 {
		opps, err := s.opportunitiesRepo.GetOpportunitiesForAgent(ctx, agent.ID, agent.Specialties, briefingOpportunitiesLimit)
		if err != nil {
			slog.Warn("briefing: opportunities fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.Opportunities = opps
		}
	}

	// Section 5: Reputation Changes
	since := agent.CreatedAt
	if agent.LastBriefingAt != nil {
		since = *agent.LastBriefingAt
	}
	repChanges, err := s.reputationRepo.GetReputationChangesSince(ctx, agent.ID, since)
	if err != nil {
		slog.Warn("briefing: reputation changes fetch failed", "agent_id", agent.ID, "error", err)
	} else {
		briefing.ReputationChanges = repChanges
	}

	// Section 6: Platform Pulse
	if s.platformPulseRepo != nil {
		pulse, err := s.platformPulseRepo.GetPlatformPulse(ctx)
		if err != nil {
			slog.Warn("briefing: platform pulse fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.PlatformPulse = pulse
		}
	}

	// Section 7: Trending Now
	if s.trendingRepo != nil {
		trending, err := s.trendingRepo.GetTrendingNow(ctx, agent.ID, briefingTrendingLimit)
		if err != nil {
			slog.Warn("briefing: trending fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.TrendingNow = trending
		}
	}

	// Section 8: Hardcore Unsolved
	if s.hardcoreRepo != nil {
		hardcore, err := s.hardcoreRepo.GetHardcoreUnsolved(ctx, briefingHardcoreLimit)
		if err != nil {
			slog.Warn("briefing: hardcore unsolved fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.HardcoreUnsolved = hardcore
		}
	}

	// Section 9: Rising Ideas
	if s.risingIdeasRepo != nil {
		ideas, err := s.risingIdeasRepo.GetRisingIdeas(ctx, briefingRisingIdeasLimit)
		if err != nil {
			slog.Warn("briefing: rising ideas fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.RisingIdeas = ideas
		}
	}

	// Section 10: Recent Victories
	if s.victoriesRepo != nil {
		victories, err := s.victoriesRepo.GetRecentVictories(ctx, briefingVictoriesLimit)
		if err != nil {
			slog.Warn("briefing: recent victories fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.RecentVictories = victories
		}
	}

	// Section 11: You Might Like
	if s.recommendationsRepo != nil {
		recs, err := s.recommendationsRepo.GetYouMightLike(ctx, agent.ID, agent.Specialties, briefingRecommendationsLimit)
		if err != nil {
			slog.Warn("briefing: recommendations fetch failed", "agent_id", agent.ID, "error", err)
		} else {
			briefing.YouMightLike = recs
		}
	}

	// Mark briefing as read
	if err := s.agentRepo.UpdateLastBriefingAt(ctx, agent.ID); err != nil {
		slog.Warn("briefing: UpdateLastBriefingAt failed", "agent_id", agent.ID, "error", err)
	}

	return briefing, nil
}

// truncateBriefingString truncates a string to maxLen characters.
func truncateBriefingString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
