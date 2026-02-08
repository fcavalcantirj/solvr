package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createNotificationTestUser creates a test user for notification tests.
func createNotificationTestUser(t *testing.T, pool *Pool) *models.User {
	t.Helper()
	ctx := context.Background()
	userRepo := NewUserRepository(pool)

	user := &models.User{
		Username:       "notifuser" + time.Now().Format("20060102150405.000000000"),
		DisplayName:    "Notification Test User",
		Email:          "notif" + time.Now().Format("20060102150405.000000000") + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_notif_" + time.Now().Format("20060102150405.000000000"),
		Role:           "user",
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

// insertTestNotification inserts a notification directly for testing.
func insertTestNotification(t *testing.T, pool *Pool, userID *string, agentID *string, nType, title string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO notifications (user_id, agent_id, type, title) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, agentID, nType, title,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test notification: %v", err)
	}
	return id
}

func TestNotificationsRepository_GetNotificationsForUser(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)
	user := createNotificationTestUser(t, pool)

	// Insert test notifications
	insertTestNotification(t, pool, &user.ID, nil, "answer.created", "Someone answered your question")
	insertTestNotification(t, pool, &user.ID, nil, "comment.created", "New comment on your post")

	notifications, total, err := repo.GetNotificationsForUser(context.Background(), user.ID, 1, 20)
	if err != nil {
		t.Fatalf("GetNotificationsForUser failed: %v", err)
	}

	if total < 2 {
		t.Errorf("expected total >= 2, got %d", total)
	}
	if len(notifications) < 2 {
		t.Errorf("expected at least 2 notifications, got %d", len(notifications))
	}

	// Should be ordered by created_at DESC
	if len(notifications) >= 2 {
		if notifications[0].CreatedAt.Before(notifications[1].CreatedAt) {
			t.Error("expected notifications ordered by created_at DESC")
		}
	}
}

func TestNotificationsRepository_GetNotificationsForAgent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)

	// Create an agent for the test
	agentRepo := NewAgentRepository(pool)
	agentID := "notif_test_agent_" + time.Now().Format("20060102150405")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Notification Test Agent",
		Status:      "active",
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}

	// Insert test notifications for agent
	insertTestNotification(t, pool, nil, &agentID, "post.mentioned", "You were mentioned")

	notifications, total, err := repo.GetNotificationsForAgent(context.Background(), agentID, 1, 20)
	if err != nil {
		t.Fatalf("GetNotificationsForAgent failed: %v", err)
	}

	if total < 1 {
		t.Errorf("expected total >= 1, got %d", total)
	}
	if len(notifications) < 1 {
		t.Errorf("expected at least 1 notification, got %d", len(notifications))
	}
}

func TestNotificationsRepository_MarkRead(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)
	user := createNotificationTestUser(t, pool)

	// Insert an unread notification
	notifID := insertTestNotification(t, pool, &user.ID, nil, "answer.created", "Test mark read")

	// Mark it as read
	updated, err := repo.MarkRead(context.Background(), notifID)
	if err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}

	if updated.ReadAt == nil {
		t.Error("expected ReadAt to be set after MarkRead")
	}
	if updated.ID != notifID {
		t.Errorf("expected ID %q, got %q", notifID, updated.ID)
	}
}

func TestNotificationsRepository_MarkRead_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)

	_, err := repo.MarkRead(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent notification")
	}
}

func TestNotificationsRepository_MarkAllReadForUser(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)
	user := createNotificationTestUser(t, pool)

	// Insert multiple unread notifications
	insertTestNotification(t, pool, &user.ID, nil, "answer.created", "Unread 1")
	insertTestNotification(t, pool, &user.ID, nil, "comment.created", "Unread 2")

	count, err := repo.MarkAllReadForUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("MarkAllReadForUser failed: %v", err)
	}

	if count < 2 {
		t.Errorf("expected count >= 2, got %d", count)
	}

	// Second call should return 0 (all already read)
	count2, err := repo.MarkAllReadForUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("MarkAllReadForUser second call failed: %v", err)
	}
	if count2 != 0 {
		t.Errorf("expected count 0 on second call, got %d", count2)
	}
}

func TestNotificationsRepository_MarkAllReadForAgent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)

	// Create an agent
	agentRepo := NewAgentRepository(pool)
	agentID := "notif_markall_agent_" + time.Now().Format("20060102150405")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "MarkAll Test Agent",
		Status:      "active",
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}

	// Insert unread notifications for agent
	insertTestNotification(t, pool, nil, &agentID, "post.mentioned", "Agent unread 1")
	insertTestNotification(t, pool, nil, &agentID, "answer.created", "Agent unread 2")

	count, err := repo.MarkAllReadForAgent(context.Background(), agentID)
	if err != nil {
		t.Fatalf("MarkAllReadForAgent failed: %v", err)
	}

	if count < 2 {
		t.Errorf("expected count >= 2, got %d", count)
	}
}

func TestNotificationsRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)
	user := createNotificationTestUser(t, pool)

	notifID := insertTestNotification(t, pool, &user.ID, nil, "answer.created", "Find me")

	found, err := repo.FindByID(context.Background(), notifID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if found.ID != notifID {
		t.Errorf("expected ID %q, got %q", notifID, found.ID)
	}
	if found.Title != "Find me" {
		t.Errorf("expected title 'Find me', got %q", found.Title)
	}
	if found.UserID == nil || *found.UserID != user.ID {
		t.Error("expected user_id to match test user")
	}
}

func TestNotificationsRepository_FindByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)

	_, err := repo.FindByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent notification")
	}
}

func TestNotificationsRepository_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)
	user := createNotificationTestUser(t, pool)

	// Insert 5 notifications
	for i := 0; i < 5; i++ {
		insertTestNotification(t, pool, &user.ID, nil, "test.type", "Pagination test")
	}

	// Get page 1 with perPage=2
	notifications, total, err := repo.GetNotificationsForUser(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("GetNotificationsForUser page 1 failed: %v", err)
	}

	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications on page 1, got %d", len(notifications))
	}
	if total < 5 {
		t.Errorf("expected total >= 5, got %d", total)
	}
}
