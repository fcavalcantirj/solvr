package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestUserRepository_HardDelete tests permanently deleting a user from the database.
func TestUserRepository_HardDelete(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user
	user := &models.User{
		Username:       "harddeleteuser",
		DisplayName:    "Hard Delete User",
		Email:          "harddelete@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "harddelete_123",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Hard delete the user
	err = repo.HardDelete(ctx, created.ID)
	if err != nil {
		t.Fatalf("HardDelete() error = %v", err)
	}

	// Verify user is gone (even FindByID with deleted_at check should fail)
	_, err = repo.FindByID(ctx, created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID() after hard delete error = %v, want ErrNotFound", err)
	}
}

// TestUserRepository_HardDelete_NotFound tests hard deleting a non-existent user.
func TestUserRepository_HardDelete_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Try to hard delete non-existent user
	err := repo.HardDelete(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("HardDelete() error = %v, want ErrNotFound", err)
	}
}

// TestUserRepository_ListDeleted tests listing soft-deleted users.
func TestUserRepository_ListDeleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create 2 users and soft-delete them
	user1 := &models.User{
		Username:       "deleted1",
		DisplayName:    "Deleted User 1",
		Email:          "deleted1@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "deleted1_123",
		Role:           models.UserRoleUser,
	}
	created1, err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create(user1) error = %v", err)
	}

	user2 := &models.User{
		Username:       "deleted2",
		DisplayName:    "Deleted User 2",
		Email:          "deleted2@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "deleted2_123",
		Role:           models.UserRoleUser,
	}
	created2, err := repo.Create(ctx, user2)
	if err != nil {
		t.Fatalf("Create(user2) error = %v", err)
	}

	// Soft-delete both users
	err = repo.Delete(ctx, created1.ID)
	if err != nil {
		t.Fatalf("Delete(user1) error = %v", err)
	}
	time.Sleep(10 * time.Millisecond) // Ensure different deleted_at timestamps
	err = repo.Delete(ctx, created2.ID)
	if err != nil {
		t.Fatalf("Delete(user2) error = %v", err)
	}

	// List deleted users
	users, total, err := repo.ListDeleted(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListDeleted() error = %v", err)
	}

	// Should find at least our 2 deleted users
	if total < 2 {
		t.Errorf("ListDeleted() total = %d, want >= 2", total)
	}

	// Verify our deleted users are in the list
	found1, found2 := false, false
	for _, u := range users {
		if u.ID == created1.ID {
			found1 = true
			if u.DeletedAt == nil {
				t.Error("ListDeleted() user1 should have deleted_at set")
			}
		}
		if u.ID == created2.ID {
			found2 = true
			if u.DeletedAt == nil {
				t.Error("ListDeleted() user2 should have deleted_at set")
			}
		}
	}

	if !found1 {
		t.Error("ListDeleted() did not return user1")
	}
	if !found2 {
		t.Error("ListDeleted() did not return user2")
	}
}

// TestUserRepository_List_ReputationCalculation tests the reputation GET /v1/users lists
// (CanonicalReputationUserRepository, idx 76): reputation earned under the legacy rules is kept
// as history (reputation_history), confirmed votes on the user's posts and replies score live
// (+2 up, -1 down), and writing a post or reply scores nothing by itself. The legacy SPEC 10.3
// formula it used to pin (UserRepository.List over answers and responses) was deleted with the
// legacy tables (idx 68).
func TestUserRepository_List_ReputationCalculation(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)
	served := NewCanonicalReputationUserRepository(pool)

	suffix := time.Now().Format("150405.000")
	created, err := userRepo.Create(ctx, &models.User{
		Username:       "reptest_" + suffix,
		DisplayName:    "Reputation Test User",
		Email:          "reptest_" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_reptest_" + suffix,
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Create user error = %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM reputation_history WHERE owner_type = 'human' AND owner_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE voter_id LIKE $1", "reptest_voter_"+suffix+"%")
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE author_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	post := func(title string) string {
		p, err := postRepo.Create(ctx, &models.Post{
			Type: models.PostTypePost, Title: title, Description: "Test", Tags: []string{"test"},
			PostedByType: models.AuthorTypeHuman, PostedByID: created.ID, Status: models.PostStatusOpen,
		})
		if err != nil {
			t.Fatalf("create post %q: %v", title, err)
		}
		return p.ID
	}
	upvoted, downvoted := post("Upvoted post"), post("Downvoted post")
	reply := seedMigratedReply(t, pool, ctx, upvoted, "answer", "human", created.ID, "A reply that is upvoted")

	voters := 0
	vote := func(targetType, targetID, direction string) {
		voters++
		if _, err := pool.Exec(ctx, `
			INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', $3, $4, true)`,
			targetType, targetID, fmt.Sprintf("reptest_voter_%s_%d", suffix, voters), direction); err != nil {
			t.Fatalf("vote %s %s: %v", direction, targetType, err)
		}
	}
	vote("post", upvoted, "up")
	vote("post", downvoted, "down")
	vote("reply", reply, "up")
	if _, err := pool.Exec(ctx, `
		INSERT INTO reputation_history (source, source_id, owner_type, owner_id, points, earned_at)
		VALUES ('problem_solved', gen_random_uuid(), 'human', $1, 100, NOW())`, created.ID); err != nil {
		t.Fatalf("seed reputation history: %v", err)
	}

	// 100 (history) + 2 (post upvote) - 1 (post downvote) + 2 (reply upvote); the two posts and
	// the reply themselves score nothing.
	expectedReputation := 103

	stats, err := served.GetUserStats(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserStats() error = %v", err)
	}
	if stats.Reputation != expectedReputation {
		t.Errorf("GetUserStats() Reputation = %d, want %d", stats.Reputation, expectedReputation)
	}

	users, _, err := served.List(ctx, models.PublicUserListOptions{
		Limit: 100,
		Sort:  models.PublicUserSortNewest,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var foundUser *models.UserListItem
	for i, u := range users {
		if u.ID == created.ID {
			foundUser = &users[i]
			break
		}
	}
	if foundUser == nil {
		t.Fatal("List() did not return our test user")
	}
	if foundUser.Reputation != expectedReputation {
		t.Errorf("List() Reputation = %d, want %d", foundUser.Reputation, expectedReputation)
	}
}

// TestUserRepository_ListActiveEmails tests that ListActiveEmails returns only non-deleted users.
func TestUserRepository_ListActiveEmails(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	repo := NewUserRepository(pool)

	// Create a unique test suffix to avoid collisions
	suffix := time.Now().Format("150405")

	// Create an active user
	activeUser, err := repo.Create(ctx, &models.User{
		Username:    "active_email_" + suffix,
		DisplayName: "Active User",
		Email:       "active_email_" + suffix + "@test.com",
	})
	if err != nil {
		t.Fatalf("failed to create active user: %v", err)
	}

	// Create a user that will be soft-deleted
	deletedUser, err := repo.Create(ctx, &models.User{
		Username:    "deleted_email_" + suffix,
		DisplayName: "Deleted User",
		Email:       "deleted_email_" + suffix + "@test.com",
	})
	if err != nil {
		t.Fatalf("failed to create user to delete: %v", err)
	}

	// Soft-delete the second user
	err = repo.Delete(ctx, deletedUser.ID)
	if err != nil {
		t.Fatalf("failed to soft-delete user: %v", err)
	}

	// Call ListActiveEmails
	recipients, err := repo.ListActiveEmails(ctx)
	if err != nil {
		t.Fatalf("ListActiveEmails() error = %v", err)
	}

	// Assert: active user IS in the result
	foundActive := false
	foundDeleted := false
	for _, r := range recipients {
		if r.ID == activeUser.ID {
			foundActive = true
			if r.Email != activeUser.Email {
				t.Errorf("expected email %q, got %q", activeUser.Email, r.Email)
			}
			if r.DisplayName != activeUser.DisplayName {
				t.Errorf("expected display_name %q, got %q", activeUser.DisplayName, r.DisplayName)
			}
		}
		if r.ID == deletedUser.ID {
			foundDeleted = true
		}
	}

	if !foundActive {
		t.Error("active user was NOT found in ListActiveEmails result")
	}
	if foundDeleted {
		t.Error("soft-deleted user WAS found in ListActiveEmails result (should be excluded)")
	}

	// Cleanup: hard-delete test users
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE username LIKE 'active_email_%' OR username LIKE 'deleted_email_%'")
}
