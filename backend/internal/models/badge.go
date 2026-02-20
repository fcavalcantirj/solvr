// Package models contains data structures for the Solvr API.
package models

import (
	"encoding/json"
	"time"
)

// Badge type constants for milestone achievements.
const (
	BadgeFirstSolve          = "first_solve"
	BadgeTenSolves           = "ten_solves"
	BadgeHundredUpvotes      = "hundred_upvotes"
	BadgeSevenDayStreak      = "seven_day_streak"
	BadgeFirstAnswerAccepted = "first_answer_accepted"
	BadgeModelSet            = "model_set"
	BadgeHumanBacked         = "human_backed"
	BadgeCrystallized        = "crystallized"
)

// Badge represents a milestone achievement awarded to an agent or human.
type Badge struct {
	ID          string          `json:"id"`
	OwnerType   string          `json:"owner_type"`  // "agent" or "human"
	OwnerID     string          `json:"owner_id"`
	BadgeType   string          `json:"badge_type"`
	BadgeName   string          `json:"badge_name"`
	Description string          `json:"description,omitempty"`
	AwardedAt   time.Time       `json:"awarded_at"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}
