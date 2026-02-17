package reputation

import "testing"

func TestActivityCounts_Calculate(t *testing.T) {
	tests := []struct {
		name     string
		counts   ActivityCounts
		expected int
	}{
		{
			name: "SPEC example - full activity",
			counts: ActivityCounts{
				ProblemsSolved:      1, // 100
				ProblemsContributed: 2, // 50
				AnswersAccepted:     1, // 50
				AnswersGiven:        2, // 20
				IdeasPosted:         2, // 30
				ResponsesGiven:      1, // 5
				UpvotesReceived:     1, // 2
				DownvotesReceived:   1, // -1
				Bonus:               0,
			},
			expected: 256, // Total
		},
		{
			name: "agent with bonus",
			counts: ActivityCounts{
				ProblemsSolved: 1,
				Bonus:          50, // Claimed agent
			},
			expected: 150,
		},
		{
			name:     "no activity",
			counts:   ActivityCounts{},
			expected: 0,
		},
		{
			name: "negative reputation from downvotes",
			counts: ActivityCounts{
				DownvotesReceived: 10,
			},
			expected: -10,
		},
		{
			name: "only answers",
			counts: ActivityCounts{
				AnswersAccepted: 2, // 100
				AnswersGiven:    5, // 50
			},
			expected: 150,
		},
		{
			name: "mixed positive and negative",
			counts: ActivityCounts{
				ProblemsSolved:    1,  // 100
				UpvotesReceived:   10, // 20
				DownvotesReceived: 5,  // -5
			},
			expected: 115,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.counts.Calculate()
			if got != tt.expected {
				t.Errorf("Calculate() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestConstants_Values(t *testing.T) {
	// Verify constants match SPEC.md Part 10.3
	tests := []struct {
		name     string
		constant int
		expected int
	}{
		{"ProblemsSolved", PointsProblemSolved, 100},
		{"ProblemsContributed", PointsProblemContributed, 25},
		{"AnswersAccepted", PointsAnswerAccepted, 50},
		{"AnswersGiven", PointsAnswerGiven, 10},
		{"IdeasPosted", PointsIdeaPosted, 15},
		{"ResponsesGiven", PointsResponseGiven, 5},
		{"UpvotesReceived", PointsUpvoteReceived, 2},
		{"DownvotesReceived", PointsDownvoteReceived, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %d, want %d (per SPEC.md)", tt.name, tt.constant, tt.expected)
			}
		})
	}
}
