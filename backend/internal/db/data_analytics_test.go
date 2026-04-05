package db

import (
	"testing"
)

// TestWindowToInterval_ValidWindows verifies that valid window strings map to correct SQL intervals.
func TestWindowToInterval_ValidWindows(t *testing.T) {
	tests := []struct {
		window   string
		expected string
	}{
		{"1h", "1 hour"},
		{"24h", "24 hours"},
		{"7d", "7 days"},
	}

	for _, tc := range tests {
		t.Run(tc.window, func(t *testing.T) {
			got, err := windowToInterval(tc.window)
			if err != nil {
				t.Fatalf("windowToInterval(%q) returned unexpected error: %v", tc.window, err)
			}
			if got != tc.expected {
				t.Errorf("windowToInterval(%q) = %q, want %q", tc.window, got, tc.expected)
			}
		})
	}
}

// TestWindowToInterval_InvalidWindow verifies that invalid window strings return an error.
func TestWindowToInterval_InvalidWindow(t *testing.T) {
	invalidWindows := []string{
		"",
		"1d",
		"48h",
		"30d",
		"invalid",
		"1 hour",
		"all",
		"0h",
		"; DROP TABLE search_queries; --",
	}

	for _, w := range invalidWindows {
		t.Run(w, func(t *testing.T) {
			_, err := windowToInterval(w)
			if err == nil {
				t.Errorf("windowToInterval(%q) expected error, got nil", w)
			}
		})
	}
}

// TestNewDataAnalyticsRepository_NotNil verifies that the constructor returns a non-nil repository.
func TestNewDataAnalyticsRepository_NotNil(t *testing.T) {
	// We can't create a real pool in unit tests, but we can verify the constructor accepts nil pool.
	// A nil pool will cause panic on actual queries, which is tested via integration tests.
	// Here we just verify the struct is created correctly.
	repo := NewDataAnalyticsRepository(nil)
	if repo == nil {
		t.Fatal("NewDataAnalyticsRepository returned nil")
	}
}

// TestKnownBotSearcherIDs verifies the bot exclusion list is populated and contains expected entries.
func TestKnownBotSearcherIDs_NotEmpty(t *testing.T) {
	if len(KnownBotSearcherIDs) == 0 {
		t.Fatal("KnownBotSearcherIDs must not be empty")
	}
}

func TestKnownBotSearcherIDs_ContainsExpectedBots(t *testing.T) {
	expectedBots := []string{"e48fb1b2", "agent_NaoParis"}
	botSet := make(map[string]bool, len(KnownBotSearcherIDs))
	for _, id := range KnownBotSearcherIDs {
		botSet[id] = true
	}
	for _, expected := range expectedBots {
		if !botSet[expected] {
			t.Errorf("KnownBotSearcherIDs missing expected bot %q", expected)
		}
	}
}

// TestBuildBotExclusionClause verifies the SQL bot exclusion fragment is non-empty.
func TestBuildBotExclusionClause(t *testing.T) {
	clause := buildBotExclusionClause()
	if clause == "" {
		t.Error("buildBotExclusionClause returned empty string")
	}
	// Verify it contains the expected structure
	if len(clause) < 10 {
		t.Errorf("buildBotExclusionClause returned suspiciously short clause: %q", clause)
	}
}
