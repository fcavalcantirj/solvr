package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func createViewsTestUser(t *testing.T, repo *UserRepository) *models.User {
	t.Helper()
	ctx := context.Background()

	now := time.Now()
	ts := now.Format("150405.000000")
	username := "vw" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4]
	user := &models.User{
		Username:       username,
		DisplayName:    "Views Test User",
		Email:          "views" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_views_" + ts,
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func TestViewsRepository_RecordView(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	viewsRepo := NewViewsRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createViewsTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for views tracking",
		Description:  "This is a test question to track views by users",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Record a view
	viewCount, err := viewsRepo.RecordView(ctx, createdPost.ID, "human", testUser.ID)
	if err != nil {
		t.Fatalf("failed to record view: %v", err)
	}

	if viewCount != 1 {
		t.Errorf("expected view count 1, got %d", viewCount)
	}
}

func TestViewsRepository_RecordView_Duplicate(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	viewsRepo := NewViewsRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createViewsTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for duplicate views",
		Description:  "This is a test question to test duplicate views",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Record first view
	count1, err := viewsRepo.RecordView(ctx, createdPost.ID, "human", testUser.ID)
	if err != nil {
		t.Fatalf("failed to record first view: %v", err)
	}

	// Record duplicate view - should not increase count
	count2, err := viewsRepo.RecordView(ctx, createdPost.ID, "human", testUser.ID)
	if err != nil {
		t.Fatalf("failed to record duplicate view: %v", err)
	}

	if count1 != count2 {
		t.Errorf("duplicate view should not increase count, got %d then %d", count1, count2)
	}
}

func TestViewsRepository_RecordView_MultipleUsers(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	viewsRepo := NewViewsRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test users
	user1 := createViewsTestUser(t, userRepo)
	user2 := createViewsTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for multiple viewers",
		Description:  "This is a test question to test multiple viewers",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user1.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Record views from different users
	_, err = viewsRepo.RecordView(ctx, createdPost.ID, "human", user1.ID)
	if err != nil {
		t.Fatalf("failed to record first user view: %v", err)
	}

	count, err := viewsRepo.RecordView(ctx, createdPost.ID, "human", user2.ID)
	if err != nil {
		t.Fatalf("failed to record second user view: %v", err)
	}

	if count != 2 {
		t.Errorf("expected view count 2 after two different users, got %d", count)
	}
}

func TestViewsRepository_GetViewCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	viewsRepo := NewViewsRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createViewsTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for getting view count",
		Description:  "This is a test question to get view count",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Check initial count
	count, err := viewsRepo.GetViewCount(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to get view count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected initial view count 0, got %d", count)
	}

	// Record a view
	_, err = viewsRepo.RecordView(ctx, createdPost.ID, "human", testUser.ID)
	if err != nil {
		t.Fatalf("failed to record view: %v", err)
	}

	// Check updated count
	count, err = viewsRepo.GetViewCount(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to get updated view count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected view count 1, got %d", count)
	}
}
