package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/reputation"
)

// TestUserReputation_AfterCommenting verifies that users who comment earn reputation.
// Bug 6: users who only commented showed 0 reputation — comments were not included in
// the reputation calculation SQL (BuildReputationSQL had no comments CTE subquery).
//
// TDD RED: This test MUST FAIL before the fix (no PointsCommentGiven in constants,
// no comments subquery in sql_builder.go).
// TDD GREEN: Passes after adding PointsCommentGiven and comments SQL to BuildReputationSQL.
func TestUserReputation_AfterCommenting(t *testing.T) {
	databaseURL := testDatabaseURL()
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create test user
	userRepo := NewUserRepository(pool)
	suffix := time.Now().Format("150405")
	user := &models.User{
		Username:       "reptest" + suffix,
		DisplayName:    "Reputation Tester",
		Email:          "reptest" + time.Now().Format("20060102150405") + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_rep_" + time.Now().Format("20060102150405"),
		Role:           "user",
	}
	user, err = userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a post so we have something to comment on
	postRepo := NewPostRepository(pool)
	post := &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Test post for comment rep " + time.Now().Format("20060102150405.000000000"),
		Description:  "Reputation test post",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusOpen,
	}
	post, err = postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create a comment by the user
	commentsRepo := NewCommentsRepository(pool)
	comment := &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Content:    "This is a test comment for reputation testing",
	}
	_, err = commentsRepo.Create(ctx, comment)
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	// Fetch user list — this runs BuildReputationSQL
	users, _, err := userRepo.List(ctx, models.PublicUserListOptions{
		Limit:  100,
		Offset: 0,
		Sort:   models.PublicUserSortNewest,
	})
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}

	// Find our test user
	var testUser *models.UserListItem
	for i := range users {
		if users[i].ID == user.ID {
			testUser = &users[i]
			break
		}
	}
	if testUser == nil {
		t.Fatalf("test user %s not found in List() results", user.ID)
	}

	// The user posted 1 idea (15 pts) + 1 comment (PointsCommentGiven pts)
	// Without Bug 6 fix: reputation = 15 (no comment points)
	// With Bug 6 fix:   reputation = 15 + reputation.PointsCommentGiven
	expectedReputation := reputation.PointsIdeaPosted + reputation.PointsCommentGiven
	if testUser.Reputation != expectedReputation {
		t.Errorf("expected reputation %d (idea %d + comment %d), got %d — comments may not be counted in reputation",
			expectedReputation,
			reputation.PointsIdeaPosted,
			reputation.PointsCommentGiven,
			testUser.Reputation,
		)
	}
}
