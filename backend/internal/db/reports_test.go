package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestReportsRepository_Create(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	reportsRepo := NewReportsRepository(pool)
	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test user
	user := &models.User{
		Username:       "reportuser" + randomSuffix(),
		DisplayName:    "Report Test User",
		Email:          "reportuser" + randomSuffix() + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_report_" + randomSuffix(),
		Role:           "user",
	}
	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for reports",
		Description:  "This is a test question",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   createdUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create report
	report := &models.Report{
		TargetType:   models.ReportTargetPost,
		TargetID:     createdPost.ID,
		ReporterType: models.AuthorTypeHuman,
		ReporterID:   createdUser.ID,
		Reason:       models.ReportReasonSpam,
		Details:      "This looks like spam",
	}

	created, err := reportsRepo.Create(ctx, report)
	if err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	if created.ID == "" {
		t.Error("expected report ID to be set")
	}
	if created.Status != models.ReportStatusPending {
		t.Errorf("expected status 'pending', got '%s'", created.Status)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestReportsRepository_Create_Duplicate(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	reportsRepo := NewReportsRepository(pool)
	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test user
	user := &models.User{
		Username:       "reportuser2" + randomSuffix(),
		DisplayName:    "Report Test User 2",
		Email:          "reportuser2" + randomSuffix() + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_report2_" + randomSuffix(),
		Role:           "user",
	}
	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for duplicate reports",
		Description:  "This is a test question",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   createdUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Create first report
	report := &models.Report{
		TargetType:   models.ReportTargetPost,
		TargetID:     createdPost.ID,
		ReporterType: models.AuthorTypeHuman,
		ReporterID:   createdUser.ID,
		Reason:       models.ReportReasonSpam,
	}

	_, err = reportsRepo.Create(ctx, report)
	if err != nil {
		t.Fatalf("failed to create first report: %v", err)
	}

	// Try to create duplicate report
	duplicateReport := &models.Report{
		TargetType:   models.ReportTargetPost,
		TargetID:     createdPost.ID,
		ReporterType: models.AuthorTypeHuman,
		ReporterID:   createdUser.ID,
		Reason:       models.ReportReasonOffensive,
	}

	_, err = reportsRepo.Create(ctx, duplicateReport)
	if err != ErrReportExists {
		t.Errorf("expected ErrReportExists, got %v", err)
	}
}

func TestReportsRepository_HasReported(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	reportsRepo := NewReportsRepository(pool)
	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test user
	user := &models.User{
		Username:       "reportuser3" + randomSuffix(),
		DisplayName:    "Report Test User 3",
		Email:          "reportuser3" + randomSuffix() + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_report3_" + randomSuffix(),
		Role:           "user",
	}
	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for has reported",
		Description:  "This is a test question",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   createdUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Check before reporting
	hasReported, err := reportsRepo.HasReported(ctx, models.ReportTargetPost, createdPost.ID, "human", createdUser.ID)
	if err != nil {
		t.Fatalf("failed to check has reported: %v", err)
	}
	if hasReported {
		t.Error("expected hasReported to be false before reporting")
	}

	// Create report
	report := &models.Report{
		TargetType:   models.ReportTargetPost,
		TargetID:     createdPost.ID,
		ReporterType: models.AuthorTypeHuman,
		ReporterID:   createdUser.ID,
		Reason:       models.ReportReasonSpam,
	}
	_, err = reportsRepo.Create(ctx, report)
	if err != nil {
		t.Fatalf("failed to create report: %v", err)
	}

	// Check after reporting
	hasReported, err = reportsRepo.HasReported(ctx, models.ReportTargetPost, createdPost.ID, "human", createdUser.ID)
	if err != nil {
		t.Fatalf("failed to check has reported after: %v", err)
	}
	if !hasReported {
		t.Error("expected hasReported to be true after reporting")
	}
}

func randomSuffix() string {
	return string(models.AuthorTypeHuman) + "_test"
}
