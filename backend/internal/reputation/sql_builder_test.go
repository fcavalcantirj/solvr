package reputation

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildReputationSQL_ContainsAllComponents(t *testing.T) {
	sql := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "",
		IncludeBonus:   false,
	})

	// Verify all components are present
	requiredParts := []string{
		// Point multipliers
		fmt.Sprintf("* %d", PointsProblemSolved),
		fmt.Sprintf("* %d", PointsProblemContributed),
		fmt.Sprintf("* %d", PointsAnswerAccepted),
		fmt.Sprintf("* %d", PointsAnswerGiven),
		fmt.Sprintf("* %d", PointsIdeaPosted),
		fmt.Sprintf("* %d", PointsResponseGiven),
		fmt.Sprintf("* %d", PointsUpvoteReceived),
		fmt.Sprintf("* %d", PointsDownvoteReceived),
		// Tables
		"votes v",
		"answers ans",
		"responses r",
		"posts p",
		// Filters
		"deleted_at IS NULL",
		"confirmed = true",
		"is_accepted = true",
	}

	for _, part := range requiredParts {
		if !strings.Contains(sql, part) {
			t.Errorf("SQL missing required part: %s", part)
		}
	}
}

func TestBuildReputationSQL_BonusOnlyForAgents(t *testing.T) {
	// Agent with bonus
	agentSQL := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "agent",
		EntityIDColumn: "a.id",
		AuthorType:     "agent",
		IncludeBonus:   true,
	})
	if !strings.Contains(agentSQL, "agents.reputation") {
		t.Error("Agent SQL should include bonus")
	}

	// User without bonus
	userSQL := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		IncludeBonus:   false,
	})
	if strings.Contains(userSQL, "agents.reputation") {
		t.Error("User SQL should NOT include bonus")
	}
}

func TestBuildReputationSQL_TimeFilter(t *testing.T) {
	// With time filter
	withFilter := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "AND created_at >= $3",
		IncludeBonus:   false,
	})

	// Should contain the time filter multiple times (once per subquery)
	count := strings.Count(withFilter, "AND created_at >= $3")
	if count < 5 { // At least 5 subqueries should have time filter
		t.Errorf("Time filter should appear multiple times, got %d occurrences", count)
	}

	// Without time filter
	withoutFilter := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "",
		IncludeBonus:   false,
	})

	if strings.Contains(withoutFilter, "created_at >=") {
		t.Error("SQL without time filter should not contain time condition")
	}
}

func TestBuildReputationSQL_EntityTypeMatching(t *testing.T) {
	tests := []struct {
		name           string
		entityIDColumn string
		authorType     string
	}{
		{
			name:           "user entity",
			entityIDColumn: "u.id::text",
			authorType:     "human",
		},
		{
			name:           "agent entity",
			entityIDColumn: "a.id",
			authorType:     "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := BuildReputationSQL(SQLBuilderOptions{
				EntityType:     tt.name,
				EntityIDColumn: tt.entityIDColumn,
				AuthorType:     tt.authorType,
				TimeFilter:     "",
				IncludeBonus:   false,
			})

			// Verify entity ID column appears in WHERE clauses
			if !strings.Contains(sql, tt.entityIDColumn) {
				t.Errorf("SQL should contain entity ID column: %s", tt.entityIDColumn)
			}

			// Verify author type appears in WHERE clauses
			authorTypeCount := strings.Count(sql, fmt.Sprintf("'%s'", tt.authorType))
			if authorTypeCount < 5 {
				t.Errorf("Author type '%s' should appear multiple times, got %d", tt.authorType, authorTypeCount)
			}
		})
	}
}

func TestBuildReputationSQL_AllPostTypes(t *testing.T) {
	sql := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "",
		IncludeBonus:   false,
	})

	// Verify all post types are covered
	postTypes := []string{
		"type = 'problem'",
		"type = 'idea'",
		"status = 'solved'",
	}

	for _, postType := range postTypes {
		if !strings.Contains(sql, postType) {
			t.Errorf("SQL should handle post type: %s", postType)
		}
	}
}

func TestBuildReputationSQL_VoteDirections(t *testing.T) {
	sql := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "",
		IncludeBonus:   false,
	})

	// Verify both vote directions are handled
	if !strings.Contains(sql, "direction = 'up'") {
		t.Error("SQL should handle upvotes")
	}
	if !strings.Contains(sql, "direction = 'down'") {
		t.Error("SQL should handle downvotes")
	}

	// Verify confirmed votes only
	confirmedCount := strings.Count(sql, "confirmed = true")
	if confirmedCount < 2 {
		t.Errorf("Should check confirmed votes at least twice (up and down), got %d", confirmedCount)
	}
}

func TestBuildReputationSQL_AllTargetTypes(t *testing.T) {
	sql := BuildReputationSQL(SQLBuilderOptions{
		EntityType:     "user",
		EntityIDColumn: "u.id::text",
		AuthorType:     "human",
		TimeFilter:     "",
		IncludeBonus:   false,
	})

	// Verify all vote target types are covered
	targetTypes := []string{
		"target_type = 'post'",
		"target_type = 'answer'",
		"target_type = 'response'",
	}

	for _, targetType := range targetTypes {
		// Each target type should appear twice (once for upvotes, once for downvotes)
		count := strings.Count(sql, targetType)
		if count < 2 {
			t.Errorf("Target type %s should appear at least twice, got %d", targetType, count)
		}
	}
}
