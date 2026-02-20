package models

import "testing"

func TestIsValidApproachStatus_Abandoned(t *testing.T) {
	// "abandoned" should be a valid approach status for stale content auto-cleanup.
	if !IsValidApproachStatus(ApproachStatusAbandoned) {
		t.Fatal("expected 'abandoned' to be a valid approach status")
	}

	// Verify the constant has the correct string value.
	if ApproachStatusAbandoned != "abandoned" {
		t.Fatalf("expected ApproachStatusAbandoned to be 'abandoned', got %q", ApproachStatusAbandoned)
	}
}
