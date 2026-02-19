// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock implementations for ForgettingService dependencies ---

// mockStaleApproachLister implements StaleApproachLister for testing.
type mockStaleApproachLister struct {
	approaches []models.Approach
	err        error
}

func (m *mockStaleApproachLister) ListStaleApproaches(ctx context.Context, failedDays, supersededDays int) ([]models.Approach, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.approaches, nil
}

// mockApproachArchiver implements ApproachArchiver for testing.
type mockApproachArchiver struct {
	archivedIDs []string
	archivedCIDs map[string]string // id -> cid
	err         error
}

func (m *mockApproachArchiver) ArchiveApproach(ctx context.Context, approachID, cid string) error {
	if m.err != nil {
		return m.err
	}
	m.archivedIDs = append(m.archivedIDs, approachID)
	if m.archivedCIDs == nil {
		m.archivedCIDs = make(map[string]string)
	}
	m.archivedCIDs[approachID] = cid
	return nil
}

// mockForgettingIPFSAdder implements IPFSContentAdder for forgetting tests.
type mockForgettingIPFSAdder struct {
	cid     string
	err     error
	content []byte
}

func (m *mockForgettingIPFSAdder) Add(ctx context.Context, reader io.Reader) (string, error) {
	if reader != nil {
		data, _ := io.ReadAll(reader)
		m.content = data
	}
	if m.err != nil {
		return "", m.err
	}
	return m.cid, nil
}

// --- Helper to build test approach data ---

func staleFailedApproach() models.Approach {
	return models.Approach{
		ID:         "approach-failed-old",
		ProblemID:  "problem-1",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "agent-1",
		Angle:      "Retry with backoff",
		Method:     "Exponential retry",
		Status:     models.ApproachStatusFailed,
		IsLatest:   true,
		CreatedAt:  time.Now().Add(-100 * 24 * time.Hour),
		UpdatedAt:  time.Now().Add(-100 * 24 * time.Hour),
	}
}

func staleSupersededApproach() models.Approach {
	return models.Approach{
		ID:         "approach-superseded-old",
		ProblemID:  "problem-2",
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   "user-1",
		Angle:      "Database locking",
		Method:     "SELECT FOR UPDATE",
		Status:     models.ApproachStatusSucceeded,
		IsLatest:   false,
		CreatedAt:  time.Now().Add(-200 * 24 * time.Hour),
		UpdatedAt:  time.Now().Add(-200 * 24 * time.Hour),
	}
}

// --- Tests ---

func TestProcessForgetting_ArchivesStaleApproaches(t *testing.T) {
	stale := []models.Approach{staleFailedApproach(), staleSupersededApproach()}

	ipfsAdder := &mockForgettingIPFSAdder{cid: "bafyforgotten123"}
	archiver := &mockApproachArchiver{}

	svc := NewForgettingService(
		&mockStaleApproachLister{approaches: stale},
		archiver,
		ipfsAdder,
	)

	result, err := svc.ProcessForgetting(context.Background())
	if err != nil {
		t.Fatalf("ProcessForgetting() error = %v, want nil", err)
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Archived != 2 {
		t.Errorf("Archived = %d, want 2", result.Archived)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}

	// Verify both approaches were archived
	if len(archiver.archivedIDs) != 2 {
		t.Fatalf("Expected 2 archived approaches, got %d", len(archiver.archivedIDs))
	}
	if archiver.archivedCIDs["approach-failed-old"] != "bafyforgotten123" {
		t.Errorf("Expected failed approach to be archived with CID bafyforgotten123")
	}
	if archiver.archivedCIDs["approach-superseded-old"] != "bafyforgotten123" {
		t.Errorf("Expected superseded approach to be archived with CID bafyforgotten123")
	}
}

func TestProcessForgetting_NoStaleApproaches(t *testing.T) {
	ipfsAdder := &mockForgettingIPFSAdder{cid: "bafytest"}
	archiver := &mockApproachArchiver{}

	svc := NewForgettingService(
		&mockStaleApproachLister{approaches: nil},
		archiver,
		ipfsAdder,
	)

	result, err := svc.ProcessForgetting(context.Background())
	if err != nil {
		t.Fatalf("ProcessForgetting() error = %v, want nil", err)
	}

	if result.Processed != 0 {
		t.Errorf("Processed = %d, want 0", result.Processed)
	}
	if result.Archived != 0 {
		t.Errorf("Archived = %d, want 0", result.Archived)
	}
	if len(archiver.archivedIDs) != 0 {
		t.Errorf("No approaches should be archived, got %d", len(archiver.archivedIDs))
	}
}

func TestProcessForgetting_IPFSFailureContinuesOthers(t *testing.T) {
	stale := []models.Approach{staleFailedApproach(), staleSupersededApproach()}

	// IPFS fails — should continue processing but count failures
	ipfsAdder := &mockForgettingIPFSAdder{err: errors.New("IPFS unreachable")}
	archiver := &mockApproachArchiver{}

	svc := NewForgettingService(
		&mockStaleApproachLister{approaches: stale},
		archiver,
		ipfsAdder,
	)

	result, err := svc.ProcessForgetting(context.Background())
	if err != nil {
		t.Fatalf("ProcessForgetting() error = %v, want nil (individual failures are non-fatal)", err)
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Failed != 2 {
		t.Errorf("Failed = %d, want 2", result.Failed)
	}
	if result.Archived != 0 {
		t.Errorf("Archived = %d, want 0", result.Archived)
	}
}

func TestProcessForgetting_ArchiveFailureContinuesOthers(t *testing.T) {
	stale := []models.Approach{staleFailedApproach(), staleSupersededApproach()}

	ipfsAdder := &mockForgettingIPFSAdder{cid: "bafytest"}
	archiver := &mockApproachArchiver{err: errors.New("database error")}

	svc := NewForgettingService(
		&mockStaleApproachLister{approaches: stale},
		archiver,
		ipfsAdder,
	)

	result, err := svc.ProcessForgetting(context.Background())
	if err != nil {
		t.Fatalf("ProcessForgetting() error = %v, want nil", err)
	}

	if result.Failed != 2 {
		t.Errorf("Failed = %d, want 2", result.Failed)
	}
	if result.Archived != 0 {
		t.Errorf("Archived = %d, want 0", result.Archived)
	}
}

func TestProcessForgetting_ListStaleFailure(t *testing.T) {
	ipfsAdder := &mockForgettingIPFSAdder{cid: "bafytest"}
	archiver := &mockApproachArchiver{}

	svc := NewForgettingService(
		&mockStaleApproachLister{err: errors.New("database error")},
		archiver,
		ipfsAdder,
	)

	_, err := svc.ProcessForgetting(context.Background())
	if err == nil {
		t.Fatal("ProcessForgetting() expected error when ListStaleApproaches fails")
	}
}

func TestProcessForgetting_SnapshotContainsApproachData(t *testing.T) {
	stale := []models.Approach{staleFailedApproach()}

	ipfsAdder := &mockForgettingIPFSAdder{cid: "bafytest"}
	archiver := &mockApproachArchiver{}

	svc := NewForgettingService(
		&mockStaleApproachLister{approaches: stale},
		archiver,
		ipfsAdder,
	)

	_, err := svc.ProcessForgetting(context.Background())
	if err != nil {
		t.Fatalf("ProcessForgetting() error = %v", err)
	}

	// Verify the IPFS content is a valid JSON snapshot of the approach
	if len(ipfsAdder.content) == 0 {
		t.Fatal("Expected IPFS content to be non-empty")
	}

	var snapshot ForgettingSnapshot
	if err := json.Unmarshal(ipfsAdder.content, &snapshot); err != nil {
		t.Fatalf("IPFS content is not valid JSON: %v", err)
	}

	if snapshot.ApproachID != "approach-failed-old" {
		t.Errorf("Snapshot ApproachID = %q, want %q", snapshot.ApproachID, "approach-failed-old")
	}
	if snapshot.Angle != "Retry with backoff" {
		t.Errorf("Snapshot Angle = %q, want %q", snapshot.Angle, "Retry with backoff")
	}
	if snapshot.Status != "failed" {
		t.Errorf("Snapshot Status = %q, want %q", snapshot.Status, "failed")
	}
	if snapshot.Origin != "solvr.dev" {
		t.Errorf("Snapshot Origin = %q, want %q", snapshot.Origin, "solvr.dev")
	}
}

func TestProcessForgetting_DefaultConfig(t *testing.T) {
	config := DefaultForgettingConfig()
	if config.FailedDays != 90 {
		t.Errorf("FailedDays = %d, want 90", config.FailedDays)
	}
	if config.SupersededDays != 180 {
		t.Errorf("SupersededDays = %d, want 180", config.SupersededDays)
	}
}
