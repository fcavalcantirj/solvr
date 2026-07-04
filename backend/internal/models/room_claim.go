package models

import (
	"time"

	"github.com/google/uuid"
)

// Claim acquisition outcomes returned by the claim endpoint.
const (
	// ClaimOutcomeWon means the caller now holds the lock.
	ClaimOutcomeWon = "won"
	// ClaimOutcomeHeld means another live holder owns the lock.
	ClaimOutcomeHeld = "held"
)

// RoomClaim is a compare-and-set lock scoped to (room_id, claim_key).
// Fields match migration 000077_create_room_claims.up.sql.
type RoomClaim struct {
	RoomID    uuid.UUID `json:"room_id"`
	Key       string    `json:"key"`
	Holder    string    `json:"holder"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AcquireClaimParams holds parameters for a claim acquisition attempt.
type AcquireClaimParams struct {
	RoomID     uuid.UUID
	Key        string
	Holder     string
	TTLSeconds int
}
