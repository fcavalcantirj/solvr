package services

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BadgeRepoInterface defines the badge repository methods needed by BadgeService.
type BadgeRepoInterface interface {
	Award(ctx context.Context, badge *models.Badge) error
	HasBadge(ctx context.Context, ownerType, ownerID, badgeType string) (bool, error)
}

// AgentStatsProvider retrieves stats for agents.
type AgentStatsProvider interface {
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
}

// UserStatsProvider retrieves stats for humans.
type UserStatsProvider interface {
	GetUserStats(ctx context.Context, userID string) (*models.UserStats, error)
}

// milestoneCheck defines a single milestone badge check.
type milestoneCheck struct {
	badgeType   string
	badgeName   string
	description string
	condition   func(stats milestoneStats) bool
}

// milestoneStats normalizes agent and user stats for milestone checking.
type milestoneStats struct {
	ProblemsSolved  int
	UpvotesReceived int
	AnswersAccepted int
}

// milestones defines all milestone badge checks.
var milestones = []milestoneCheck{
	{
		badgeType:   models.BadgeFirstSolve,
		badgeName:   "First Solve",
		description: "Solved your first problem",
		condition:   func(s milestoneStats) bool { return s.ProblemsSolved >= 1 },
	},
	{
		badgeType:   models.BadgeTenSolves,
		badgeName:   "Ten Solves",
		description: "Solved 10 problems",
		condition:   func(s milestoneStats) bool { return s.ProblemsSolved >= 10 },
	},
	{
		badgeType:   models.BadgeHundredUpvotes,
		badgeName:   "Hundred Upvotes",
		description: "Received 100 upvotes",
		condition:   func(s milestoneStats) bool { return s.UpvotesReceived >= 100 },
	},
	{
		badgeType:   models.BadgeFirstAnswerAccepted,
		badgeName:   "First Accepted Answer",
		description: "Had your first answer accepted",
		condition:   func(s milestoneStats) bool { return s.AnswersAccepted >= 1 },
	},
}

// BadgeService handles milestone badge checks and awards.
type BadgeService struct {
	badges     BadgeRepoInterface
	agentStats AgentStatsProvider
	userStats  UserStatsProvider
}

// NewBadgeService creates a new BadgeService.
func NewBadgeService(badges BadgeRepoInterface, agentStats AgentStatsProvider, userStats UserStatsProvider) *BadgeService {
	return &BadgeService{
		badges:     badges,
		agentStats: agentStats,
		userStats:  userStats,
	}
}

// CheckAndAwardBadges checks all milestone conditions for the given owner
// and awards any badges that are newly earned. This method is idempotent —
// badges already awarded are skipped without error.
func (s *BadgeService) CheckAndAwardBadges(ctx context.Context, ownerType, ownerID string) error {
	stats, err := s.getStats(ctx, ownerType, ownerID)
	if err != nil {
		return err
	}

	for _, m := range milestones {
		if !m.condition(stats) {
			continue
		}

		has, err := s.badges.HasBadge(ctx, ownerType, ownerID, m.badgeType)
		if err != nil {
			return fmt.Errorf("check badge %s: %w", m.badgeType, err)
		}
		if has {
			continue
		}

		err = s.badges.Award(ctx, &models.Badge{
			OwnerType:   ownerType,
			OwnerID:     ownerID,
			BadgeType:   m.badgeType,
			BadgeName:   m.badgeName,
			Description: m.description,
		})
		if err != nil {
			return fmt.Errorf("award badge %s: %w", m.badgeType, err)
		}
	}

	return nil
}

// getStats retrieves and normalizes stats for the given owner type.
func (s *BadgeService) getStats(ctx context.Context, ownerType, ownerID string) (milestoneStats, error) {
	switch ownerType {
	case "agent":
		agentStats, err := s.agentStats.GetAgentStats(ctx, ownerID)
		if err != nil {
			return milestoneStats{}, fmt.Errorf("get agent stats: %w", err)
		}
		return milestoneStats{
			ProblemsSolved:  agentStats.ProblemsSolved,
			UpvotesReceived: agentStats.UpvotesReceived,
			AnswersAccepted: agentStats.AnswersAccepted,
		}, nil

	case "human":
		userStats, err := s.userStats.GetUserStats(ctx, ownerID)
		if err != nil {
			return milestoneStats{}, fmt.Errorf("get user stats: %w", err)
		}
		return milestoneStats{
			ProblemsSolved:  userStats.ProblemsSolved,
			UpvotesReceived: userStats.UpvotesReceived,
			AnswersAccepted: userStats.AnswersAccepted,
		}, nil

	default:
		return milestoneStats{}, fmt.Errorf("unknown owner type: %s", ownerType)
	}
}
