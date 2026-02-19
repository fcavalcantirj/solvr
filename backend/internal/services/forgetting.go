// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Default archival thresholds.
const (
	DefaultFailedDays     = 90
	DefaultSupersededDays = 180
)

// StaleApproachLister finds approaches eligible for archival.
type StaleApproachLister interface {
	ListStaleApproaches(ctx context.Context, failedDays, supersededDays int) ([]models.Approach, error)
}

// ApproachArchiver marks an approach as archived with an IPFS CID.
type ApproachArchiver interface {
	ArchiveApproach(ctx context.Context, approachID, cid string) error
}

// ForgettingConfig holds configuration for the forgetting service.
type ForgettingConfig struct {
	FailedDays     int
	SupersededDays int
}

// DefaultForgettingConfig returns the default configuration.
func DefaultForgettingConfig() ForgettingConfig {
	return ForgettingConfig{
		FailedDays:     DefaultFailedDays,
		SupersededDays: DefaultSupersededDays,
	}
}

// ForgettingResult contains the results of a forgetting run.
type ForgettingResult struct {
	Processed int `json:"processed"`
	Archived  int `json:"archived"`
	Failed    int `json:"failed"`
}

// ForgettingSnapshot is the document stored on IPFS when archiving an approach.
type ForgettingSnapshot struct {
	Version    string    `json:"version"`
	Origin     string    `json:"origin"`
	ApproachID string    `json:"approach_id"`
	ProblemID  string    `json:"problem_id"`
	AuthorType string    `json:"author_type"`
	AuthorID   string    `json:"author_id"`
	Angle      string    `json:"angle"`
	Method     string    `json:"method,omitempty"`
	Status     string    `json:"status"`
	Outcome    string    `json:"outcome,omitempty"`
	Solution   string    `json:"solution,omitempty"`
	IsLatest   bool      `json:"is_latest"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ArchivedAt time.Time `json:"archived_at"`
}

// ForgettingService handles auto-archival of stale approaches to IPFS.
type ForgettingService struct {
	lister   StaleApproachLister
	archiver ApproachArchiver
	ipfs     IPFSContentAdder
	config   ForgettingConfig
}

// NewForgettingService creates a new ForgettingService with default config.
func NewForgettingService(
	lister StaleApproachLister,
	archiver ApproachArchiver,
	ipfs IPFSContentAdder,
) *ForgettingService {
	return NewForgettingServiceWithConfig(lister, archiver, ipfs, DefaultForgettingConfig())
}

// NewForgettingServiceWithConfig creates a new ForgettingService with custom config.
func NewForgettingServiceWithConfig(
	lister StaleApproachLister,
	archiver ApproachArchiver,
	ipfs IPFSContentAdder,
	config ForgettingConfig,
) *ForgettingService {
	return &ForgettingService{
		lister:   lister,
		archiver: archiver,
		ipfs:     ipfs,
		config:   config,
	}
}

// ProcessForgetting finds stale approaches, archives them to IPFS, and marks them archived.
// Individual failures are non-fatal — the service continues processing remaining approaches.
func (s *ForgettingService) ProcessForgetting(ctx context.Context) (*ForgettingResult, error) {
	stale, err := s.lister.ListStaleApproaches(ctx, s.config.FailedDays, s.config.SupersededDays)
	if err != nil {
		return nil, fmt.Errorf("forgetting: list stale approaches: %w", err)
	}

	result := &ForgettingResult{Processed: len(stale)}

	for _, approach := range stale {
		if err := s.archiveOne(ctx, &approach); err != nil {
			slog.Warn("forgetting: failed to archive approach",
				"approach_id", approach.ID, "error", err)
			result.Failed++
			continue
		}
		result.Archived++
	}

	if result.Archived > 0 {
		slog.Info("forgetting: run complete",
			"processed", result.Processed,
			"archived", result.Archived,
			"failed", result.Failed)
	}

	return result, nil
}

// archiveOne snapshots a single approach to IPFS and marks it archived.
func (s *ForgettingService) archiveOne(ctx context.Context, approach *models.Approach) error {
	snapshot := s.buildSnapshot(approach)

	reader, err := snapshotToReader(snapshot)
	if err != nil {
		return fmt.Errorf("serialize snapshot: %w", err)
	}

	cid, err := s.ipfs.Add(ctx, reader)
	if err != nil {
		return fmt.Errorf("IPFS add: %w", err)
	}

	if err := s.archiver.ArchiveApproach(ctx, approach.ID, cid); err != nil {
		return fmt.Errorf("mark archived: %w", err)
	}

	return nil
}

// buildSnapshot creates a ForgettingSnapshot from an approach.
func (s *ForgettingService) buildSnapshot(approach *models.Approach) ForgettingSnapshot {
	return ForgettingSnapshot{
		Version:    "1.0",
		Origin:     "solvr.dev",
		ApproachID: approach.ID,
		ProblemID:  approach.ProblemID,
		AuthorType: string(approach.AuthorType),
		AuthorID:   approach.AuthorID,
		Angle:      approach.Angle,
		Method:     approach.Method,
		Status:     string(approach.Status),
		Outcome:    approach.Outcome,
		Solution:   approach.Solution,
		IsLatest:   approach.IsLatest,
		CreatedAt:  approach.CreatedAt,
		UpdatedAt:  approach.UpdatedAt,
		ArchivedAt: time.Now(),
	}
}

// snapshotToReader serializes a ForgettingSnapshot to an io.Reader.
func snapshotToReader(snapshot ForgettingSnapshot) (io.Reader, error) {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	return bytes.NewReader(data), nil
}
