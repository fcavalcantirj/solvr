// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// CrystallizationVersion is the format version for crystallization snapshots.
const CrystallizationVersion = "1.0"

// Default stability period before crystallization is allowed.
const DefaultStabilityPeriod = 7 * 24 * time.Hour

// Crystallization-specific errors.
var (
	ErrNotAProblem        = errors.New("only problems can be crystallized")
	ErrNotSolved          = errors.New("problem must be solved before crystallization")
	ErrAlreadyCrystallized = errors.New("problem is already crystallized")
	ErrNotStableYet       = errors.New("problem has not been stable long enough for crystallization")
	ErrNoVerifiedApproach = errors.New("problem has no succeeded approach")
)

// PostFinder retrieves a post by ID.
type PostFinder interface {
	FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error)
}

// CrystallizationCIDSetter persists the crystallization CID for a post.
type CrystallizationCIDSetter interface {
	SetCrystallizationCID(ctx context.Context, postID, cid string) error
}

// ApproachLister lists approaches for a problem.
type ApproachLister interface {
	ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error)
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

// CrystallizationService handles snapshotting solved problems to IPFS.
type CrystallizationService struct {
	postFinder     PostFinder
	cidSetter      CrystallizationCIDSetter
	approachLister ApproachLister
	ipfsAdder      IPFSContentAdder
	ipfsPinner     IPFSContentPinner
	config         CrystallizationConfig
}

// NewCrystallizationService creates a new CrystallizationService with default config.
func NewCrystallizationService(
	postFinder PostFinder,
	cidSetter CrystallizationCIDSetter,
	approachLister ApproachLister,
	ipfsAdder IPFSContentAdder,
	ipfsPinner IPFSContentPinner,
) *CrystallizationService {
	return NewCrystallizationServiceWithConfig(
		postFinder, cidSetter, approachLister, ipfsAdder, ipfsPinner,
		DefaultCrystallizationConfig(),
	)
}

// NewCrystallizationServiceWithConfig creates a new CrystallizationService with custom config.
func NewCrystallizationServiceWithConfig(
	postFinder PostFinder,
	cidSetter CrystallizationCIDSetter,
	approachLister ApproachLister,
	ipfsAdder IPFSContentAdder,
	ipfsPinner IPFSContentPinner,
	config CrystallizationConfig,
) *CrystallizationService {
	return &CrystallizationService{
		postFinder:     postFinder,
		cidSetter:      cidSetter,
		approachLister: approachLister,
		ipfsAdder:      ipfsAdder,
		ipfsPinner:     ipfsPinner,
		config:         config,
	}
}

// CrystallizeProblem snapshots a solved problem and its approaches to IPFS.
// Returns the IPFS CID of the crystallized snapshot.
func (s *CrystallizationService) CrystallizeProblem(ctx context.Context, problemID string) (string, error) {
	// 1. Fetch the problem
	post, err := s.postFinder.FindByID(ctx, problemID)
	if err != nil {
		return "", fmt.Errorf("crystallize: find problem: %w", err)
	}

	// 2. Validate eligibility
	if err := s.validateEligibility(post); err != nil {
		return "", err
	}

	// 3. Fetch all approaches (succeeded + failed — all are valuable)
	approaches, _, err := s.approachLister.ListApproaches(ctx, problemID, models.ApproachListOptions{
		Page:    1,
		PerPage: 50, // Max per page, should cover most problems
	})
	if err != nil {
		return "", fmt.Errorf("crystallize: list approaches: %w", err)
	}

	// 4. Verify at least one succeeded approach exists
	hasSucceeded := false
	for _, a := range approaches {
		if a.Status == models.ApproachStatusSucceeded {
			hasSucceeded = true
			break
		}
	}
	if !hasSucceeded {
		return "", ErrNoVerifiedApproach
	}

	// 5. Build the snapshot
	snapshot := s.BuildSnapshot(post, approaches)

	// 6. Serialize to reader
	reader, err := s.SnapshotToReader(snapshot)
	if err != nil {
		return "", fmt.Errorf("crystallize: serialize snapshot: %w", err)
	}

	// 7. Upload to IPFS
	cid, err := s.ipfsAdder.Add(ctx, reader)
	if err != nil {
		return "", fmt.Errorf("crystallize: IPFS add: %w", err)
	}

	// 8. Pin the CID (non-fatal if this fails — data is already on IPFS)
	if pinErr := s.ipfsPinner.Pin(ctx, cid); pinErr != nil {
		slog.Warn("crystallize: pin failed (non-fatal)", "cid", cid, "error", pinErr)
	}

	// 9. Save CID to database
	if err := s.cidSetter.SetCrystallizationCID(ctx, problemID, cid); err != nil {
		return "", fmt.Errorf("crystallize: save CID: %w", err)
	}

	slog.Info("problem crystallized", "problem_id", problemID, "cid", cid)
	return cid, nil
}

// validateEligibility checks if a post is eligible for crystallization.
func (s *CrystallizationService) validateEligibility(post *models.PostWithAuthor) error {
	if post.Type != models.PostTypeProblem {
		return ErrNotAProblem
	}
	if post.Status != models.PostStatusSolved {
		return ErrNotSolved
	}
	if post.CrystallizationCID != nil {
		return ErrAlreadyCrystallized
	}
	if time.Since(post.UpdatedAt) < s.config.StabilityPeriod {
		return ErrNotStableYet
	}
	return nil
}

// IsCrystallizationCandidate checks if a post is a candidate for crystallization
// without returning an error. Useful for filtering in batch processing.
func (s *CrystallizationService) IsCrystallizationCandidate(post *models.PostWithAuthor) bool {
	return s.validateEligibility(post) == nil
}

// CrystallizationSnapshot is the immutable document stored on IPFS.
type CrystallizationSnapshot struct {
	// Version is the snapshot format version.
	Version string `json:"version"`

	// Origin identifies this snapshot as coming from Solvr.
	Origin string `json:"origin"`

	// ProblemID is the original problem ID on Solvr.
	ProblemID string `json:"problem_id"`

	// CrystallizedAt is when this snapshot was created.
	CrystallizedAt time.Time `json:"crystallized_at"`

	// Problem contains the problem data at time of crystallization.
	Problem SnapshotProblem `json:"problem"`

	// Approaches contains all approaches (succeeded and failed).
	Approaches []SnapshotApproach `json:"approaches"`
}

// SnapshotProblem is the problem data within a crystallization snapshot.
type SnapshotProblem struct {
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Tags            []string          `json:"tags,omitempty"`
	SuccessCriteria []string          `json:"success_criteria,omitempty"`
	Status          string            `json:"status"`
	Upvotes         int               `json:"upvotes"`
	Downvotes       int               `json:"downvotes"`
	Author          SnapshotAuthor    `json:"author"`
	CreatedAt       time.Time         `json:"created_at"`
}

// SnapshotApproach is approach data within a crystallization snapshot.
type SnapshotApproach struct {
	Angle       string         `json:"angle"`
	Method      string         `json:"method,omitempty"`
	Assumptions []string       `json:"assumptions,omitempty"`
	Status      string         `json:"status"`
	Outcome     string         `json:"outcome,omitempty"`
	Solution    string         `json:"solution,omitempty"`
	Author      SnapshotAuthor `json:"author"`
	CreatedAt   time.Time      `json:"created_at"`
}

// SnapshotAuthor is author information within a crystallization snapshot.
type SnapshotAuthor struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// BuildSnapshot creates a CrystallizationSnapshot from a post and its approaches.
func (s *CrystallizationService) BuildSnapshot(
	post *models.PostWithAuthor,
	approaches []models.ApproachWithAuthor,
) CrystallizationSnapshot {
	snapshotApproaches := make([]SnapshotApproach, 0, len(approaches))
	for _, a := range approaches {
		snapshotApproaches = append(snapshotApproaches, SnapshotApproach{
			Angle:       a.Angle,
			Method:      a.Method,
			Assumptions: a.Assumptions,
			Status:      string(a.Status),
			Outcome:     a.Outcome,
			Solution:    a.Solution,
			Author: SnapshotAuthor{
				Type:        string(a.Author.Type),
				ID:          a.Author.ID,
				DisplayName: a.Author.DisplayName,
			},
			CreatedAt: a.CreatedAt,
		})
	}

	return CrystallizationSnapshot{
		Version:        CrystallizationVersion,
		Origin:         "solvr.dev",
		ProblemID:      post.ID,
		CrystallizedAt: time.Now(),
		Problem: SnapshotProblem{
			Title:           post.Title,
			Description:     post.Description,
			Tags:            post.Tags,
			SuccessCriteria: post.SuccessCriteria,
			Status:          string(post.Status),
			Upvotes:         post.Upvotes,
			Downvotes:       post.Downvotes,
			Author: SnapshotAuthor{
				Type:        string(post.Author.Type),
				ID:          post.Author.ID,
				DisplayName: post.Author.DisplayName,
			},
			CreatedAt: post.CreatedAt,
		},
		Approaches: snapshotApproaches,
	}
}

// SnapshotToReader serializes a CrystallizationSnapshot to an io.Reader.
func (s *CrystallizationService) SnapshotToReader(snapshot CrystallizationSnapshot) (io.Reader, error) {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("crystallize: marshal snapshot: %w", err)
	}
	return bytes.NewReader(data), nil
}
