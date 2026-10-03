// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"io"
	"time"
)

// Default stability period before crystallization is allowed.
const DefaultStabilityPeriod = 7 * 24 * time.Hour

// Crystallization-specific errors.
var (
	ErrAlreadyCrystallized = errors.New("problem is already crystallized")
	ErrNotStableYet        = errors.New("problem has not been stable long enough for crystallization")
)

// CrystallizationCIDSetter persists the crystallization CID for a post.
type CrystallizationCIDSetter interface {
	SetCrystallizationCID(ctx context.Context, postID, cid string) error
}

// IPFSContentAdder uploads content to IPFS and returns a CID.
type IPFSContentAdder interface {
	Add(ctx context.Context, reader io.Reader) (string, error)
}

// IPFSContentPinner pins a CID to the IPFS node.
type IPFSContentPinner interface {
	Pin(ctx context.Context, cid string) error
}

// CrystallizationConfig holds configuration for the crystallization service.
type CrystallizationConfig struct {
	// StabilityPeriod is how long a solved problem must be unchanged before crystallization.
	StabilityPeriod time.Duration
}

// DefaultCrystallizationConfig returns the default configuration.
func DefaultCrystallizationConfig() CrystallizationConfig {
	return CrystallizationConfig{
		StabilityPeriod: DefaultStabilityPeriod,
	}
}

// SnapshotAuthor is author information within a crystallization snapshot.
type SnapshotAuthor struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}
