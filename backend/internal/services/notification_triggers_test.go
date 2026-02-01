// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestNotifyOnNewAnswer tests notification trigger when answer created.
func TestNotifyOnNewAnswer(t *testing.T) {
	questionID := uuid.New().String()
	questionAuthorID := uuid.New().String()
	answerID := uuid.New().String()
	answererID := uuid.New().String()

	var createdNotif *NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotif = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				Type:      n.Type,
				Title:     n.Title,
				Body:      n.Body,
				Link:      n.Link,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	postLookup := &MockPostLookup{
		findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
			if id == questionID {
				return &PostInfo{
					ID:         questionID,
					Type:       "question",
					Title:      "How to do X?",
					AuthorType: "human",
					AuthorID:   questionAuthorID,
				}, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := NewNotificationService(repo, nil, nil, postLookup, nil)

	err := svc.NotifyOnNewAnswer(context.Background(), &NewAnswerEvent{
		AnswerID:     answerID,
		QuestionID:   questionID,
		AnswererID:   answererID,
		AnswererType: "human",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if createdNotif == nil {
		t.Fatal("expected notification to be created")
	}

	if createdNotif.UserID == nil || *createdNotif.UserID != questionAuthorID {
		t.Errorf("expected notification for user %s, got %v", questionAuthorID, createdNotif.UserID)
	}

	if createdNotif.Type != NotificationTypeAnswerCreated {
		t.Errorf("expected type %s, got %s", NotificationTypeAnswerCreated, createdNotif.Type)
	}
}

// TestNotifyOnNewAnswer_AuthorIsAgent tests notify when question author is an agent.
func TestNotifyOnNewAnswer_AuthorIsAgent(t *testing.T) {
	questionID := uuid.New().String()
	agentAuthorID := "test_agent"

	var createdNotif *NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotif = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				AgentID:   n.AgentID,
				Type:      n.Type,
				Title:     n.Title,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	postLookup := &MockPostLookup{
		findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
			return &PostInfo{
				ID:         questionID,
				Type:       "question",
				Title:      "How to do X?",
				AuthorType: "agent",
				AuthorID:   agentAuthorID,
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, postLookup, nil)

	err := svc.NotifyOnNewAnswer(context.Background(), &NewAnswerEvent{
		AnswerID:     uuid.New().String(),
		QuestionID:   questionID,
		AnswererID:   uuid.New().String(),
		AnswererType: "human",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if createdNotif.AgentID == nil || *createdNotif.AgentID != agentAuthorID {
		t.Errorf("expected notification for agent %s, got %v", agentAuthorID, createdNotif.AgentID)
	}
}

// TestNotifyOnNewAnswer_SelfAnswer tests no notification when answering own question.
func TestNotifyOnNewAnswer_SelfAnswer(t *testing.T) {
	questionID := uuid.New().String()
	authorID := uuid.New().String()

	var notifCreated bool

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			notifCreated = true
			return &NotificationRecord{ID: uuid.New().String()}, nil
		},
	}

	postLookup := &MockPostLookup{
		findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
			return &PostInfo{
				ID:         questionID,
				AuthorType: "human",
				AuthorID:   authorID, // Same as answerer
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, postLookup, nil)

	err := svc.NotifyOnNewAnswer(context.Background(), &NewAnswerEvent{
		AnswerID:     uuid.New().String(),
		QuestionID:   questionID,
		AnswererID:   authorID, // Same as question author
		AnswererType: "human",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if notifCreated {
		t.Error("should not create notification for self-answer")
	}
}

// TestNotifyOnApproachUpdate tests notification on approach status change.
func TestNotifyOnApproachUpdate(t *testing.T) {
	problemID := uuid.New().String()
	problemAuthorID := uuid.New().String()
	approachID := uuid.New().String()
	approachAuthorID := uuid.New().String()

	var createdNotif *NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotif = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				Type:      n.Type,
				Title:     n.Title,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	postLookup := &MockPostLookup{
		findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
			return &PostInfo{
				ID:         problemID,
				Type:       "problem",
				Title:      "Fix the bug",
				AuthorType: "human",
				AuthorID:   problemAuthorID,
			}, nil
		},
	}

	approachLookup := &MockApproachLookup{
		findByIDFunc: func(ctx context.Context, id string) (*ApproachInfo, error) {
			return &ApproachInfo{
				ID:         approachID,
				ProblemID:  problemID,
				AuthorType: "human",
				AuthorID:   approachAuthorID,
				Status:     "succeeded",
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, nil, postLookup, approachLookup)

	err := svc.NotifyOnApproachUpdate(context.Background(), &ApproachUpdateEvent{
		ApproachID: approachID,
		NewStatus:  "succeeded",
		OldStatus:  "working",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if createdNotif == nil {
		t.Fatal("expected notification to be created")
	}

	if createdNotif.UserID == nil || *createdNotif.UserID != problemAuthorID {
		t.Errorf("expected notification for problem author %s", problemAuthorID)
	}

	if createdNotif.Type != NotificationTypeApproachUpdated {
		t.Errorf("expected type %s, got %s", NotificationTypeApproachUpdated, createdNotif.Type)
	}
}

// TestNotifyOnComment tests notification when comment created.
func TestNotifyOnComment(t *testing.T) {
	answerID := uuid.New().String()
	answerAuthorID := uuid.New().String()
	commentAuthorID := uuid.New().String()

	var createdNotif *NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotif = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				Type:      n.Type,
				Title:     n.Title,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	answerLookup := &MockAnswerLookup{
		findByIDFunc: func(ctx context.Context, id string) (*AnswerInfo, error) {
			return &AnswerInfo{
				ID:         answerID,
				AuthorType: "human",
				AuthorID:   answerAuthorID,
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, answerLookup, nil, nil)

	err := svc.NotifyOnComment(context.Background(), &CommentEvent{
		CommentID:         uuid.New().String(),
		TargetType:        "answer",
		TargetID:          answerID,
		CommentAuthorID:   commentAuthorID,
		CommentAuthorType: "human",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if createdNotif == nil {
		t.Fatal("expected notification to be created")
	}

	if createdNotif.Type != NotificationTypeCommentCreated {
		t.Errorf("expected type %s, got %s", NotificationTypeCommentCreated, createdNotif.Type)
	}
}

// TestNotifyOnAcceptedAnswer tests notification when answer accepted.
func TestNotifyOnAcceptedAnswer(t *testing.T) {
	answerID := uuid.New().String()
	answerAuthorID := uuid.New().String()
	questionID := uuid.New().String()

	var createdNotif *NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotif = n
			return &NotificationRecord{
				ID:        uuid.New().String(),
				UserID:    n.UserID,
				Type:      n.Type,
				Title:     n.Title,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	answerLookup := &MockAnswerLookup{
		findByIDFunc: func(ctx context.Context, id string) (*AnswerInfo, error) {
			return &AnswerInfo{
				ID:         answerID,
				QuestionID: questionID,
				AuthorType: "human",
				AuthorID:   answerAuthorID,
			}, nil
		},
	}

	postLookup := &MockPostLookup{
		findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
			return &PostInfo{
				ID:    questionID,
				Title: "How to do X?",
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, answerLookup, postLookup, nil)

	err := svc.NotifyOnAcceptedAnswer(context.Background(), &AcceptedAnswerEvent{
		AnswerID:   answerID,
		QuestionID: questionID,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if createdNotif == nil {
		t.Fatal("expected notification to be created")
	}

	if createdNotif.UserID == nil || *createdNotif.UserID != answerAuthorID {
		t.Errorf("expected notification for answer author %s", answerAuthorID)
	}

	if createdNotif.Type != NotificationTypeAnswerAccepted {
		t.Errorf("expected type %s, got %s", NotificationTypeAnswerAccepted, createdNotif.Type)
	}
}

// TestNotifyOnUpvoteMilestone tests notification at upvote milestones (10, 50, 100).
func TestNotifyOnUpvoteMilestone(t *testing.T) {
	tests := []struct {
		name         string
		upvoteCount  int
		expectNotify bool
	}{
		{"at 10 upvotes", 10, true},
		{"at 50 upvotes", 50, true},
		{"at 100 upvotes", 100, true},
		{"at 9 upvotes - no milestone", 9, false},
		{"at 11 upvotes - no milestone", 11, false},
		{"at 49 upvotes - no milestone", 49, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postID := uuid.New().String()
			authorID := uuid.New().String()

			var notifCreated bool

			repo := &MockNotificationRepository{
				createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
					notifCreated = true
					return &NotificationRecord{
						ID:     uuid.New().String(),
						UserID: n.UserID,
						Type:   n.Type,
					}, nil
				},
				getUserUpvoteCountFunc: func(ctx context.Context, targetType, targetID string) (int, error) {
					return tt.upvoteCount, nil
				},
			}

			postLookup := &MockPostLookup{
				findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
					return &PostInfo{
						ID:         postID,
						Title:      "Test Post",
						AuthorType: "human",
						AuthorID:   authorID,
					}, nil
				},
			}

			svc := NewNotificationService(repo, nil, nil, postLookup, nil)

			err := svc.NotifyOnUpvoteMilestone(context.Background(), &UpvoteEvent{
				TargetType: "post",
				TargetID:   postID,
			})

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if notifCreated != tt.expectNotify {
				t.Errorf("expected notify=%v, got notify=%v", tt.expectNotify, notifCreated)
			}
		})
	}
}

// TestNotifyOnMention tests notification when @username mentioned in content.
func TestNotifyOnMention(t *testing.T) {
	mentionedUserID := uuid.New().String()
	mentionerID := uuid.New().String()
	postID := uuid.New().String()

	var createdNotifs []*NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotifs = append(createdNotifs, n)
			return &NotificationRecord{
				ID:     uuid.New().String(),
				UserID: n.UserID,
				Type:   n.Type,
			}, nil
		},
	}

	userLookup := &MockUserLookup{
		findByIDFunc: func(ctx context.Context, id string) (*UserInfo, error) {
			if id == "testuser" {
				return &UserInfo{
					ID:       mentionedUserID,
					Username: "testuser",
				}, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := NewNotificationService(repo, userLookup, nil, nil, nil)

	err := svc.NotifyOnMention(context.Background(), &MentionEvent{
		MentionedUsernames: []string{"testuser"},
		MentionerID:        mentionerID,
		MentionerType:      "human",
		ContentType:        "post",
		ContentID:          postID,
		Link:               "/posts/" + postID,
	}, userLookup)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(createdNotifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(createdNotifs))
	}

	if createdNotifs[0].Type != NotificationTypeMention {
		t.Errorf("expected type %s, got %s", NotificationTypeMention, createdNotifs[0].Type)
	}
}

// TestNotifyOnMention_SelfMention tests no notification for self-mention.
func TestNotifyOnMention_SelfMention(t *testing.T) {
	userID := uuid.New().String()
	username := "testuser"

	var notifCreated bool

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			notifCreated = true
			return &NotificationRecord{ID: uuid.New().String()}, nil
		},
	}

	userLookup := &MockUserLookup{
		findByIDFunc: func(ctx context.Context, id string) (*UserInfo, error) {
			return &UserInfo{
				ID:       userID,
				Username: username,
			}, nil
		},
	}

	svc := NewNotificationService(repo, userLookup, nil, nil, nil)

	err := svc.NotifyOnMention(context.Background(), &MentionEvent{
		MentionedUsernames: []string{username},
		MentionerID:        userID, // Same user
		MentionerType:      "human",
		ContentType:        "post",
		ContentID:          uuid.New().String(),
	}, userLookup)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if notifCreated {
		t.Error("should not create notification for self-mention")
	}
}

// TestNotifyOnComment_SelfComment tests no notification for self-comment.
func TestNotifyOnComment_SelfComment(t *testing.T) {
	answerID := uuid.New().String()
	authorID := uuid.New().String()

	var notifCreated bool

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			notifCreated = true
			return &NotificationRecord{ID: uuid.New().String()}, nil
		},
	}

	answerLookup := &MockAnswerLookup{
		findByIDFunc: func(ctx context.Context, id string) (*AnswerInfo, error) {
			return &AnswerInfo{
				ID:         answerID,
				AuthorType: "human",
				AuthorID:   authorID, // Same as commenter
			}, nil
		},
	}

	svc := NewNotificationService(repo, nil, answerLookup, nil, nil)

	err := svc.NotifyOnComment(context.Background(), &CommentEvent{
		CommentID:         uuid.New().String(),
		TargetType:        "answer",
		TargetID:          answerID,
		CommentAuthorID:   authorID, // Same as answer author
		CommentAuthorType: "human",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if notifCreated {
		t.Error("should not create notification for self-comment")
	}
}

// TestNotifyOnApproachUpdate_StatusMessages tests correct message for each status.
func TestNotifyOnApproachUpdate_StatusMessages(t *testing.T) {
	tests := []struct {
		status        string
		expectedTitle string
	}{
		{"succeeded", "Approach succeeded!"},
		{"stuck", "Approach is stuck"},
		{"failed", "Approach failed"},
		{"working", "Approach status updated"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			problemID := uuid.New().String()
			approachID := uuid.New().String()

			var createdNotif *NotificationInput

			repo := &MockNotificationRepository{
				createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
					createdNotif = n
					return &NotificationRecord{ID: uuid.New().String()}, nil
				},
			}

			postLookup := &MockPostLookup{
				findByIDFunc: func(ctx context.Context, id string) (*PostInfo, error) {
					return &PostInfo{
						ID:         problemID,
						Title:      "Test Problem",
						AuthorType: "human",
						AuthorID:   uuid.New().String(),
					}, nil
				},
			}

			approachLookup := &MockApproachLookup{
				findByIDFunc: func(ctx context.Context, id string) (*ApproachInfo, error) {
					return &ApproachInfo{
						ID:         approachID,
						ProblemID:  problemID,
						AuthorType: "human",
						AuthorID:   uuid.New().String(), // Different from problem author
						Status:     tt.status,
					}, nil
				},
			}

			svc := NewNotificationService(repo, nil, nil, postLookup, approachLookup)

			err := svc.NotifyOnApproachUpdate(context.Background(), &ApproachUpdateEvent{
				ApproachID: approachID,
				NewStatus:  tt.status,
				OldStatus:  "starting",
			})

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if createdNotif == nil {
				t.Fatal("expected notification to be created")
			}

			if createdNotif.Title != tt.expectedTitle {
				t.Errorf("expected title %q, got %q", tt.expectedTitle, createdNotif.Title)
			}
		})
	}
}

// TestNotifyOnMention_MultipleUsers tests notification to multiple mentioned users.
func TestNotifyOnMention_MultipleUsers(t *testing.T) {
	user1ID := uuid.New().String()
	user2ID := uuid.New().String()
	mentionerID := uuid.New().String()

	var createdNotifs []*NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotifs = append(createdNotifs, n)
			return &NotificationRecord{ID: uuid.New().String()}, nil
		},
	}

	userLookup := &MockUserLookup{
		findByIDFunc: func(ctx context.Context, id string) (*UserInfo, error) {
			switch id {
			case "user1":
				return &UserInfo{ID: user1ID, Username: "user1"}, nil
			case "user2":
				return &UserInfo{ID: user2ID, Username: "user2"}, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := NewNotificationService(repo, userLookup, nil, nil, nil)

	err := svc.NotifyOnMention(context.Background(), &MentionEvent{
		MentionedUsernames: []string{"user1", "user2"},
		MentionerID:        mentionerID,
		MentionerType:      "human",
		ContentType:        "post",
		ContentID:          uuid.New().String(),
	}, userLookup)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(createdNotifs) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(createdNotifs))
	}
}

// TestNotifyOnMention_SkipsNotFoundUsers tests skipping users that don't exist.
func TestNotifyOnMention_SkipsNotFoundUsers(t *testing.T) {
	existingUserID := uuid.New().String()

	var createdNotifs []*NotificationInput

	repo := &MockNotificationRepository{
		createFunc: func(ctx context.Context, n *NotificationInput) (*NotificationRecord, error) {
			createdNotifs = append(createdNotifs, n)
			return &NotificationRecord{ID: uuid.New().String()}, nil
		},
	}

	userLookup := &MockUserLookup{
		findByIDFunc: func(ctx context.Context, id string) (*UserInfo, error) {
			if id == "existing" {
				return &UserInfo{ID: existingUserID, Username: "existing"}, nil
			}
			return nil, errors.New("not found")
		},
	}

	svc := NewNotificationService(repo, userLookup, nil, nil, nil)

	err := svc.NotifyOnMention(context.Background(), &MentionEvent{
		MentionedUsernames: []string{"existing", "notfound", "alsonotfound"},
		MentionerID:        uuid.New().String(),
		MentionerType:      "human",
		ContentType:        "post",
		ContentID:          uuid.New().String(),
	}, userLookup)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should only create 1 notification for the existing user
	if len(createdNotifs) != 1 {
		t.Errorf("expected 1 notification, got %d", len(createdNotifs))
	}
}
