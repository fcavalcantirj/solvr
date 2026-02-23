package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createNotificationTestUser creates a test user for notification tests.
func createNotificationTestUser(t *testing.T, pool *Pool) *models.User {
	t.Helper()
	ctx := context.Background()
	userRepo := NewUserRepository(pool)

	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "nf" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Notification Test User",
		Email:          "notif_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_notif_" + ts,
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
	nowA := time.Now()
	nsA := fmt.Sprintf("%04d", nowA.Nanosecond()/100000)
	agentID := "nta_" + nowA.Format("150405") + nsA
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Notif Agent " + nowA.Format("150405") + nsA,
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
	nowB := time.Now()
	nsB := fmt.Sprintf("%04d", nowB.Nanosecond()/100000)
	agentID := "nma_" + nowB.Format("150405") + nsB
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "MarkAll Agent " + nowB.Format("150405") + nsB,
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

func TestNotificationsRepository_Create(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewNotificationsRepository(pool)

	// Create a test agent to receive the notification
	agentRepo := NewAgentRepository(pool)
	nowC := time.Now()
	nsC := fmt.Sprintf("%04d", nowC.Nanosecond()/100000)
	agentID := "nca_" + nowC.Format("150405") + nsC
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Create Notif Agent " + nowC.Format("150405") + nsC,
		Status:      "active",
	}
	if err := agentRepo.Create(context.Background(), agent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}

	// Create a notification via the repository
	notification := &models.Notification{
		AgentID: &agentID,
		Type:    "approach_abandonment_warning",
		Title:   "Your approach may be abandoned",
		Body:    "Your approach has been in 'working' status for 23+ days",
		Link:    "/problems/test-problem-id",
	}

	created, err := repo.Create(context.Background(), notification)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected notification ID to be set")
	}
	if created.AgentID == nil || *created.AgentID != agentID {
		t.Error("expected agent_id to match")
	}
	if created.Type != "approach_abandonment_warning" {
		t.Errorf("expected type 'approach_abandonment_warning', got %q", created.Type)
	}
	if created.Title != "Your approach may be abandoned" {
		t.Errorf("expected correct title, got %q", created.Title)
	}
	if created.Body != "Your approach has been in 'working' status for 23+ days" {
		t.Errorf("expected correct body, got %q", created.Body)
	}
	if created.Link != "/problems/test-problem-id" {
		t.Errorf("expected correct link, got %q", created.Link)
	}

	// Verify the notification appears in GetNotificationsForAgent
	notifications, total, err := repo.GetNotificationsForAgent(context.Background(), agentID, 1, 20)
	if err != nil {
		t.Fatalf("GetNotificationsForAgent failed: %v", err)
	}
	if total < 1 {
		t.Errorf("expected total >= 1, got %d", total)
	}

	found := false
	for _, n := range notifications {
		if n.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created notification not found in GetNotificationsForAgent results")
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
