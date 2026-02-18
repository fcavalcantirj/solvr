// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock implementations for CrystallizationService dependencies ---

// mockPostFinder implements PostFinder for testing.
type mockPostFinder struct {
	post *models.PostWithAuthor
	err  error
}

func (m *mockPostFinder) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.post, nil
}

// mockCrystallizationCIDSetter implements CrystallizationCIDSetter for testing.
type mockCrystallizationCIDSetter struct {
	calledWith struct {
		postID string
		cid    string
	}
	err error
}

func (m *mockCrystallizationCIDSetter) SetCrystallizationCID(ctx context.Context, postID, cid string) error {
	m.calledWith.postID = postID
	m.calledWith.cid = cid
	return m.err
}

// mockApproachLister implements ApproachLister for testing.
type mockApproachLister struct {
	approaches []models.ApproachWithAuthor
	total      int
	err        error
}

func (m *mockApproachLister) ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.approaches, m.total, nil
}

// mockIPFSAdder implements IPFSContentAdder for testing.
type mockIPFSAdder struct {
	cid     string
	err     error
	content []byte // captures what was written
}

func (m *mockIPFSAdder) Add(ctx context.Context, reader io.Reader) (string, error) {
	if reader != nil {
		data, _ := io.ReadAll(reader)
		m.content = data
	}
	if m.err != nil {
		return "", m.err
	}
	return m.cid, nil
}

// mockIPFSPinner implements IPFSContentPinner for testing.
type mockIPFSPinner struct {
	pinnedCIDs []string
	err        error
}

func (m *mockIPFSPinner) Pin(ctx context.Context, cid string) error {
	if m.err != nil {
		return m.err
	}
	m.pinnedCIDs = append(m.pinnedCIDs, cid)
	return nil
}

// --- Helper to build test data ---

func solvedProblemPost(createdAt time.Time) *models.PostWithAuthor {
	return &models.PostWithAuthor{
		Post: models.Post{
			ID:              "problem-uuid-123",
			Type:            models.PostTypeProblem,
			Title:           "Race condition in async PostgreSQL queries",
			Description:     "Encountering a race condition when multiple async queries hit the same row.",
			Tags:            []string{"postgresql", "async", "concurrency"},
			PostedByType:    models.AuthorTypeHuman,
			PostedByID:      "user-uuid-456",
			Status:          models.PostStatusSolved,
			Upvotes:         42,
			Downvotes:       2,
			SuccessCriteria: []string{"No race condition under concurrent load"},
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-uuid-456",
			DisplayName: "Alice",
		},
	}
}

func succeededApproach() models.ApproachWithAuthor {
	return models.ApproachWithAuthor{
		Approach: models.Approach{
			ID:          "approach-uuid-789",
			ProblemID:   "problem-uuid-123",
			AuthorType:  models.AuthorTypeAgent,
			AuthorID:    "claude_assistant",
			Angle:       "Use database transactions with row-level locking",
			Method:      "SELECT ... FOR UPDATE to prevent concurrent modification",
			Assumptions: []string{"PostgreSQL supports row-level locking"},
			Status:      models.ApproachStatusSucceeded,
			Outcome:     "Row-level locking eliminates the race condition entirely.",
			Solution:    "Wrap concurrent queries in a transaction with SELECT ... FOR UPDATE.",
			CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
			UpdatedAt:   time.Now().Add(-10 * 24 * time.Hour),
		},
		Author: models.ApproachAuthor{
			Type:        models.AuthorTypeAgent,
			ID:          "claude_assistant",
			DisplayName: "Claude",
		},
	}
}

func failedApproach() models.ApproachWithAuthor {
	return models.ApproachWithAuthor{
		Approach: models.Approach{
			ID:         "approach-uuid-fail",
			ProblemID:  "problem-uuid-123",
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   "user-uuid-789",
			Angle:      "Retry with exponential backoff",
			Method:     "Catch deadlock errors and retry",
			Status:     models.ApproachStatusFailed,
			Outcome:    "Retrying masks the root cause and doesn't fix the race condition.",
			CreatedAt:  time.Now().Add(-15 * 24 * time.Hour),
			UpdatedAt:  time.Now().Add(-15 * 24 * time.Hour),
		},
		Author: models.ApproachAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-uuid-789",
			DisplayName: "Bob",
		},
	}
}

// --- Tests ---

func TestCrystallizeProblem_Success(t *testing.T) {
	// Problem solved 10 days ago (stable for 7 days)
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach(), failedApproach()}

	ipfsAdder := &mockIPFSAdder{cid: "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"}
	ipfsPinner := &mockIPFSPinner{}
	cidSetter := &mockCrystallizationCIDSetter{}

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		cidSetter,
		&mockApproachLister{approaches: approaches, total: 2},
		ipfsAdder,
		ipfsPinner,
	)

	cid, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err != nil {
		t.Fatalf("CrystallizeProblem() error = %v, want nil", err)
	}

	if cid != "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi" {
		t.Errorf("CrystallizeProblem() cid = %q, want %q", cid, "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
	}

	// Verify CID was pinned
	if len(ipfsPinner.pinnedCIDs) != 1 || ipfsPinner.pinnedCIDs[0] != cid {
		t.Errorf("Expected CID to be pinned, got pinnedCIDs = %v", ipfsPinner.pinnedCIDs)
	}

	// Verify crystallization CID was saved to database
	if cidSetter.calledWith.postID != "problem-uuid-123" {
		t.Errorf("SetCrystallizationCID called with postID = %q, want %q", cidSetter.calledWith.postID, "problem-uuid-123")
	}
	if cidSetter.calledWith.cid != cid {
		t.Errorf("SetCrystallizationCID called with cid = %q, want %q", cidSetter.calledWith.cid, cid)
	}

	// Verify snapshot content was valid JSON with expected fields
	var snapshot CrystallizationSnapshot
	if err := json.Unmarshal(ipfsAdder.content, &snapshot); err != nil {
		t.Fatalf("Snapshot content is not valid JSON: %v", err)
	}
	if snapshot.Version != CrystallizationVersion {
		t.Errorf("Snapshot version = %q, want %q", snapshot.Version, CrystallizationVersion)
	}
	if snapshot.ProblemID != "problem-uuid-123" {
		t.Errorf("Snapshot ProblemID = %q, want %q", snapshot.ProblemID, "problem-uuid-123")
	}
	if snapshot.Problem.Title != "Race condition in async PostgreSQL queries" {
		t.Errorf("Snapshot problem title = %q, want correct title", snapshot.Problem.Title)
	}
	if len(snapshot.Approaches) != 2 {
		t.Errorf("Snapshot approaches count = %d, want 2", len(snapshot.Approaches))
	}
	if snapshot.CrystallizedAt.IsZero() {
		t.Error("Snapshot CrystallizedAt should not be zero")
	}
}

func TestCrystallizeProblem_NotStableYet(t *testing.T) {
	// Problem solved only 3 days ago (not stable for 7 days)
	createdAt := time.Now().Add(-3 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)
	// Set UpdatedAt to 3 days ago as well (recent activity)
	post.UpdatedAt = createdAt

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{approaches: []models.ApproachWithAuthor{succeededApproach()}, total: 1},
		&mockIPFSAdder{cid: "bafytest"},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error for non-stable problem, got nil")
	}
	if !errors.Is(err, ErrNotStableYet) {
		t.Errorf("CrystallizeProblem() error = %v, want ErrNotStableYet", err)
	}
}

func TestCrystallizeProblem_AlreadyCrystallized(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)
	existingCID := "bafyexistingcid"
	post.CrystallizationCID = &existingCID

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error for already crystallized, got nil")
	}
	if !errors.Is(err, ErrAlreadyCrystallized) {
		t.Errorf("CrystallizeProblem() error = %v, want ErrAlreadyCrystallized", err)
	}
}

func TestCrystallizeProblem_NotSolved(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)
	post.Status = models.PostStatusOpen // not solved

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error for non-solved problem, got nil")
	}
	if !errors.Is(err, ErrNotSolved) {
		t.Errorf("CrystallizeProblem() error = %v, want ErrNotSolved", err)
	}
}

func TestCrystallizeProblem_NotAProblem(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)
	post.Type = models.PostTypeQuestion // not a problem

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error for non-problem post, got nil")
	}
	if !errors.Is(err, ErrNotAProblem) {
		t.Errorf("CrystallizeProblem() error = %v, want ErrNotAProblem", err)
	}
}

func TestCrystallizeProblem_NoSucceededApproach(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	// Only failed approach, no succeeded one
	approaches := []models.ApproachWithAuthor{failedApproach()}

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{approaches: approaches, total: 1},
		&mockIPFSAdder{cid: "bafytest"},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error when no succeeded approach, got nil")
	}
	if !errors.Is(err, ErrNoVerifiedApproach) {
		t.Errorf("CrystallizeProblem() error = %v, want ErrNoVerifiedApproach", err)
	}
}

func TestCrystallizeProblem_IPFSAddFailure(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach()}
	ipfsErr := errors.New("IPFS node unreachable")

	cidSetter := &mockCrystallizationCIDSetter{}
	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		cidSetter,
		&mockApproachLister{approaches: approaches, total: 1},
		&mockIPFSAdder{err: ipfsErr},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error on IPFS failure, got nil")
	}

	// The post should NOT have been updated (crystallization failed)
	if cidSetter.calledWith.postID != "" {
		t.Error("SetCrystallizationCID should not be called when IPFS Add fails")
	}
}

func TestCrystallizeProblem_PinFailureStillSavesCID(t *testing.T) {
	// Pinning failure is non-fatal — CID is still saved because the data was uploaded
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach()}

	ipfsAdder := &mockIPFSAdder{cid: "bafytest"}
	ipfsPinner := &mockIPFSPinner{err: errors.New("pin failed")}
	cidSetter := &mockCrystallizationCIDSetter{}

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		cidSetter,
		&mockApproachLister{approaches: approaches, total: 1},
		ipfsAdder,
		ipfsPinner,
	)

	cid, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err != nil {
		t.Fatalf("CrystallizeProblem() error = %v, want nil (pin failure is non-fatal)", err)
	}
	if cid != "bafytest" {
		t.Errorf("CrystallizeProblem() cid = %q, want %q", cid, "bafytest")
	}

	// CID should still be saved
	if cidSetter.calledWith.cid != "bafytest" {
		t.Errorf("SetCrystallizationCID should still be called even when pin fails")
	}
}

func TestCrystallizeProblem_PostNotFound(t *testing.T) {
	svc := NewCrystallizationService(
		&mockPostFinder{err: errors.New("post not found")},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error for non-existent post, got nil")
	}
}

func TestCrystallizeProblem_SnapshotContainsAllApproaches(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach(), failedApproach()}

	ipfsAdder := &mockIPFSAdder{cid: "bafytest"}
	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{approaches: approaches, total: 2},
		ipfsAdder,
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err != nil {
		t.Fatalf("CrystallizeProblem() error = %v", err)
	}

	var snapshot CrystallizationSnapshot
	if err := json.Unmarshal(ipfsAdder.content, &snapshot); err != nil {
		t.Fatalf("Invalid snapshot JSON: %v", err)
	}

	// Both approaches (succeeded AND failed) should be included
	if len(snapshot.Approaches) != 2 {
		t.Fatalf("Expected 2 approaches in snapshot, got %d", len(snapshot.Approaches))
	}

	// Verify approach data is included
	hasSucceeded := false
	hasFailed := false
	for _, a := range snapshot.Approaches {
		if a.Status == string(models.ApproachStatusSucceeded) {
			hasSucceeded = true
			if a.Solution == "" {
				t.Error("Succeeded approach should include solution")
			}
		}
		if a.Status == string(models.ApproachStatusFailed) {
			hasFailed = true
			if a.Outcome == "" {
				t.Error("Failed approach should include outcome (learnings)")
			}
		}
	}
	if !hasSucceeded {
		t.Error("Snapshot should include succeeded approach")
	}
	if !hasFailed {
		t.Error("Snapshot should include failed approach (valuable learnings)")
	}
}

func TestCrystallizeProblem_CustomStabilityPeriod(t *testing.T) {
	// Problem solved 2 days ago, with 1-day stability period configured
	createdAt := time.Now().Add(-2 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)
	post.UpdatedAt = createdAt

	approaches := []models.ApproachWithAuthor{succeededApproach()}
	ipfsAdder := &mockIPFSAdder{cid: "bafytest"}

	svc := NewCrystallizationServiceWithConfig(
		&mockPostFinder{post: post},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{approaches: approaches, total: 1},
		ipfsAdder,
		&mockIPFSPinner{},
		CrystallizationConfig{StabilityPeriod: 24 * time.Hour},
	)

	cid, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err != nil {
		t.Fatalf("CrystallizeProblem() with 1-day stability error = %v, want nil", err)
	}
	if cid != "bafytest" {
		t.Errorf("CrystallizeProblem() cid = %q, want %q", cid, "bafytest")
	}
}

func TestFindCrystallizationCandidates_FiltersCorrectly(t *testing.T) {
	// Test the candidate scanning logic
	svc := NewCrystallizationService(
		&mockPostFinder{},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	// A post that is a valid candidate
	validCandidate := &models.PostWithAuthor{
		Post: models.Post{
			ID:        "valid-post",
			Type:      models.PostTypeProblem,
			Status:    models.PostStatusSolved,
			UpdatedAt: time.Now().Add(-10 * 24 * time.Hour),
		},
	}

	if !svc.IsCrystallizationCandidate(validCandidate) {
		t.Error("Expected solved problem with stable period to be a candidate")
	}

	// Already crystallized
	cidStr := "bafyexisting"
	alreadyCrystallized := &models.PostWithAuthor{
		Post: models.Post{
			ID:                 "crystallized-post",
			Type:               models.PostTypeProblem,
			Status:             models.PostStatusSolved,
			UpdatedAt:          time.Now().Add(-10 * 24 * time.Hour),
			CrystallizationCID: &cidStr,
		},
	}
	if svc.IsCrystallizationCandidate(alreadyCrystallized) {
		t.Error("Already crystallized post should NOT be a candidate")
	}

	// Not a problem
	notAProblem := &models.PostWithAuthor{
		Post: models.Post{
			ID:        "question-post",
			Type:      models.PostTypeQuestion,
			Status:    models.PostStatusAnswered,
			UpdatedAt: time.Now().Add(-10 * 24 * time.Hour),
		},
	}
	if svc.IsCrystallizationCandidate(notAProblem) {
		t.Error("Non-problem post should NOT be a candidate")
	}

	// Not solved
	notSolved := &models.PostWithAuthor{
		Post: models.Post{
			ID:        "open-post",
			Type:      models.PostTypeProblem,
			Status:    models.PostStatusOpen,
			UpdatedAt: time.Now().Add(-10 * 24 * time.Hour),
		},
	}
	if svc.IsCrystallizationCandidate(notSolved) {
		t.Error("Non-solved problem should NOT be a candidate")
	}

	// Not stable yet (updated 3 days ago)
	notStable := &models.PostWithAuthor{
		Post: models.Post{
			ID:        "recent-post",
			Type:      models.PostTypeProblem,
			Status:    models.PostStatusSolved,
			UpdatedAt: time.Now().Add(-3 * 24 * time.Hour),
		},
	}
	if svc.IsCrystallizationCandidate(notStable) {
		t.Error("Recently updated post should NOT be a candidate")
	}
}

func TestBuildSnapshot_StructureIsCorrect(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach(), failedApproach()}

	svc := NewCrystallizationService(
		&mockPostFinder{},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	snapshot := svc.BuildSnapshot(post, approaches)

	if snapshot.Version != CrystallizationVersion {
		t.Errorf("Version = %q, want %q", snapshot.Version, CrystallizationVersion)
	}
	if snapshot.ProblemID != "problem-uuid-123" {
		t.Errorf("ProblemID = %q, want %q", snapshot.ProblemID, "problem-uuid-123")
	}
	if snapshot.Problem.Title != post.Title {
		t.Errorf("Problem.Title = %q, want %q", snapshot.Problem.Title, post.Title)
	}
	if snapshot.Problem.Description != post.Description {
		t.Errorf("Problem.Description mismatch")
	}
	if len(snapshot.Problem.Tags) != 3 {
		t.Errorf("Problem.Tags = %v, want 3 tags", snapshot.Problem.Tags)
	}
	if len(snapshot.Approaches) != 2 {
		t.Errorf("Approaches count = %d, want 2", len(snapshot.Approaches))
	}
	if snapshot.Problem.Author.DisplayName != "Alice" {
		t.Errorf("Problem.Author.DisplayName = %q, want %q", snapshot.Problem.Author.DisplayName, "Alice")
	}

	// Verify the snapshot serializes to valid JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Snapshot should serialize to JSON: %v", err)
	}
	if !json.Valid(data) {
		t.Error("Serialized snapshot is not valid JSON")
	}

	// Verify it contains expected text (for search/discoverability)
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "Race condition in async PostgreSQL queries") {
		t.Error("Snapshot JSON should contain problem title")
	}
	if !strings.Contains(jsonStr, "solvr.dev") {
		t.Error("Snapshot JSON should contain origin (solvr.dev)")
	}
}

func TestBuildSnapshot_SerializesToReader(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach()}

	svc := NewCrystallizationService(
		&mockPostFinder{},
		&mockCrystallizationCIDSetter{},
		&mockApproachLister{},
		&mockIPFSAdder{},
		&mockIPFSPinner{},
	)

	snapshot := svc.BuildSnapshot(post, approaches)
	reader, err := svc.SnapshotToReader(snapshot)
	if err != nil {
		t.Fatalf("SnapshotToReader() error = %v", err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		t.Fatalf("Reading snapshot reader: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Snapshot reader should produce non-empty content")
	}

	var parsed CrystallizationSnapshot
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Snapshot reader content is not valid JSON: %v", err)
	}
	if parsed.ProblemID != "problem-uuid-123" {
		t.Errorf("Parsed ProblemID = %q, want %q", parsed.ProblemID, "problem-uuid-123")
	}
}

func TestCrystallizeProblem_SetCrystallizationCIDFailure(t *testing.T) {
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	post := solvedProblemPost(createdAt)

	approaches := []models.ApproachWithAuthor{succeededApproach()}

	ipfsAdder := &mockIPFSAdder{cid: "bafytest"}
	cidSetter := &mockCrystallizationCIDSetter{err: errors.New("database error")}

	svc := NewCrystallizationService(
		&mockPostFinder{post: post},
		cidSetter,
		&mockApproachLister{approaches: approaches, total: 1},
		ipfsAdder,
		&mockIPFSPinner{},
	)

	_, err := svc.CrystallizeProblem(context.Background(), "problem-uuid-123")
	if err == nil {
		t.Fatal("CrystallizeProblem() expected error when SetCrystallizationCID fails")
	}
}
