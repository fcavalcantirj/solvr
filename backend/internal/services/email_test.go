// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockSMTPClient implements SMTPClient for testing.
type MockSMTPClient struct {
	mu           sync.Mutex
	SentEmails   []EmailMessage
	ShouldFail   bool
	FailureError error
	Delay        time.Duration
}

func (m *MockSMTPClient) Send(msg *EmailMessage) error {
	if m.Delay > 0 {
		time.Sleep(m.Delay)
	}

	if m.ShouldFail {
		if m.FailureError != nil {
			return m.FailureError
		}
		return errors.New("mock SMTP error")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentEmails = append(m.SentEmails, *msg)
	return nil
}

func (m *MockSMTPClient) GetSentEmails() []EmailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]EmailMessage{}, m.SentEmails...)
}

func TestEmailConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    EmailConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: EmailConfig{
				SMTPHost:  "smtp.example.com",
				SMTPPort:  587,
				SMTPUser:  "user@example.com",
				SMTPPass:  "password",
				FromEmail: "noreply@solvr.dev",
			},
			wantError: false,
		},
		{
			name: "missing host",
			config: EmailConfig{
				SMTPPort:  587,
				SMTPUser:  "user@example.com",
				SMTPPass:  "password",
				FromEmail: "noreply@solvr.dev",
			},
			wantError: true,
		},
		{
			name: "invalid port",
			config: EmailConfig{
				SMTPHost:  "smtp.example.com",
				SMTPPort:  0,
				SMTPUser:  "user@example.com",
				SMTPPass:  "password",
				FromEmail: "noreply@solvr.dev",
			},
			wantError: true,
		},
		{
			name: "missing from email",
			config: EmailConfig{
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				SMTPUser: "user@example.com",
				SMTPPass: "password",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestEmailService_SendEmail(t *testing.T) {
	mockClient := &MockSMTPClient{}
	service := NewEmailService(mockClient, "noreply@solvr.dev")

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
		Text:    "Test body",
	}

	err := service.SendEmail(context.Background(), msg)
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}

	sent := mockClient.GetSentEmails()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 sent email, got %d", len(sent))
	}

	if sent[0].To != "user@example.com" {
		t.Errorf("Expected to = user@example.com, got %s", sent[0].To)
	}
	if sent[0].Subject != "Test Subject" {
		t.Errorf("Expected subject = Test Subject, got %s", sent[0].Subject)
	}
	if sent[0].From != "noreply@solvr.dev" {
		t.Errorf("Expected from = noreply@solvr.dev, got %s", sent[0].From)
	}
}

func TestEmailService_SendEmail_WithError(t *testing.T) {
	mockClient := &MockSMTPClient{
		ShouldFail:   true,
		FailureError: errors.New("SMTP connection failed"),
	}
	service := NewEmailService(mockClient, "noreply@solvr.dev")

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	err := service.SendEmail(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "SMTP connection failed") {
		t.Errorf("Expected SMTP error, got: %v", err)
	}
}

func TestEmailService_SendEmail_MissingTo(t *testing.T) {
	mockClient := &MockSMTPClient{}
	service := NewEmailService(mockClient, "noreply@solvr.dev")

	msg := &EmailMessage{
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	err := service.SendEmail(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error for missing To, got nil")
	}

	if !errors.Is(err, ErrInvalidEmailRecipient) {
		t.Errorf("Expected ErrInvalidEmailRecipient, got: %v", err)
	}
}

func TestEmailService_SendEmail_MissingSubject(t *testing.T) {
	mockClient := &MockSMTPClient{}
	service := NewEmailService(mockClient, "noreply@solvr.dev")

	msg := &EmailMessage{
		To:   "user@example.com",
		HTML: "<p>Test body</p>",
	}

	err := service.SendEmail(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error for missing Subject, got nil")
	}

	if !errors.Is(err, ErrInvalidEmailSubject) {
		t.Errorf("Expected ErrInvalidEmailSubject, got: %v", err)
	}
}

func TestEmailService_SendEmailAsync(t *testing.T) {
	mockClient := &MockSMTPClient{Delay: 10 * time.Millisecond}
	service := NewEmailService(mockClient, "noreply@solvr.dev")

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	// SendEmailAsync should return immediately
	start := time.Now()
	service.SendEmailAsync(context.Background(), msg)
	elapsed := time.Since(start)

	if elapsed > 5*time.Millisecond {
		t.Errorf("SendEmailAsync took too long: %v", elapsed)
	}

	// Wait for async send to complete
	time.Sleep(50 * time.Millisecond)

	sent := mockClient.GetSentEmails()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 sent email, got %d", len(sent))
	}
}

func TestEmailService_SendEmailWithRetry_Success(t *testing.T) {
	callCount := 0
	mockClient := &MockSMTPClient{}

	// Override Send to track calls
	originalSend := mockClient.Send
	mockClient.Send = func(msg *EmailMessage) error {
		callCount++
		return originalSend(mockClient, msg)
	}

	service := NewEmailServiceWithRetry(mockClient, "noreply@solvr.dev", 3, 10*time.Millisecond)

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	err := service.SendEmailWithRetry(context.Background(), msg)
	if err != nil {
		t.Fatalf("SendEmailWithRetry() error = %v", err)
	}

	// Should succeed on first try
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestEmailService_SendEmailWithRetry_EventualSuccess(t *testing.T) {
	failCount := 0
	mockClient := &MockSMTPClient{}

	service := &EmailService{
		client:       mockClient,
		fromEmail:    "noreply@solvr.dev",
		maxRetries:   3,
		retryBackoff: 1 * time.Millisecond,
		sendFunc: func(msg *EmailMessage) error {
			failCount++
			if failCount < 3 {
				return errors.New("temporary failure")
			}
			mockClient.SentEmails = append(mockClient.SentEmails, *msg)
			return nil
		},
	}

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	err := service.SendEmailWithRetry(context.Background(), msg)
	if err != nil {
		t.Fatalf("SendEmailWithRetry() error = %v", err)
	}

	// Should succeed on third try (after 2 failures)
	if failCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", failCount)
	}
}

func TestEmailService_SendEmailWithRetry_MaxRetriesExceeded(t *testing.T) {
	mockClient := &MockSMTPClient{
		ShouldFail:   true,
		FailureError: errors.New("permanent failure"),
	}
	service := NewEmailServiceWithRetry(mockClient, "noreply@solvr.dev", 3, 1*time.Millisecond)

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	err := service.SendEmailWithRetry(context.Background(), msg)
	if err == nil {
		t.Fatal("Expected error after max retries, got nil")
	}

	if !strings.Contains(err.Error(), "max retries") {
		t.Errorf("Expected max retries error, got: %v", err)
	}
}

func TestEmailService_SendEmailWithRetry_ContextCanceled(t *testing.T) {
	mockClient := &MockSMTPClient{
		ShouldFail:   true,
		FailureError: errors.New("temporary failure"),
	}
	service := NewEmailServiceWithRetry(mockClient, "noreply@solvr.dev", 5, 100*time.Millisecond)

	msg := &EmailMessage{
		To:      "user@example.com",
		Subject: "Test Subject",
		HTML:    "<p>Test body</p>",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := service.SendEmailWithRetry(ctx, msg)
	if err == nil {
		t.Fatal("Expected context error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context error, got: %v", err)
	}
}

// Template tests

func TestEmailTemplate_Welcome(t *testing.T) {
	template := WelcomeEmailTemplate("John Doe", "john_doe")

	if template.Subject != "Welcome to Solvr!" {
		t.Errorf("Expected subject 'Welcome to Solvr!', got '%s'", template.Subject)
	}

	if !strings.Contains(template.HTML, "John Doe") {
		t.Error("HTML should contain user's display name")
	}

	if !strings.Contains(template.HTML, "john_doe") {
		t.Error("HTML should contain username")
	}

	if !strings.Contains(template.Text, "John Doe") {
		t.Error("Text should contain user's display name")
	}
}

func TestEmailTemplate_NewAnswer(t *testing.T) {
	template := NewAnswerEmailTemplate(
		"John Doe",
		"How do I use async/await?",
		"https://solvr.dev/questions/123",
	)

	if template.Subject != "New answer to your question" {
		t.Errorf("Expected subject 'New answer to your question', got '%s'", template.Subject)
	}

	if !strings.Contains(template.HTML, "How do I use async/await?") {
		t.Error("HTML should contain question title")
	}

	if !strings.Contains(template.HTML, "https://solvr.dev/questions/123") {
		t.Error("HTML should contain question link")
	}
}

func TestEmailTemplate_ApproachUpdate(t *testing.T) {
	tests := []struct {
		status         string
		expectedPhrase string
	}{
		{"succeeded", "succeeded"},
		{"stuck", "needs help"},
		{"failed", "failed"},
		{"working", "status update"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			template := ApproachUpdateEmailTemplate(
				"Jane Doe",
				"Fix the race condition bug",
				tt.status,
				"https://solvr.dev/problems/456",
			)

			if !strings.Contains(strings.ToLower(template.HTML), tt.expectedPhrase) {
				t.Errorf("HTML should contain '%s' for status '%s'", tt.expectedPhrase, tt.status)
			}

			if !strings.Contains(template.HTML, "Fix the race condition bug") {
				t.Error("HTML should contain problem title")
			}
		})
	}
}

func TestEmailTemplate_AcceptedAnswer(t *testing.T) {
	template := AcceptedAnswerEmailTemplate(
		"Alex Smith",
		"How to optimize database queries?",
		"https://solvr.dev/questions/789#answer-abc",
	)

	if !strings.Contains(template.Subject, "accepted") {
		t.Errorf("Subject should contain 'accepted', got '%s'", template.Subject)
	}

	if !strings.Contains(template.HTML, "Alex Smith") {
		t.Error("HTML should contain user's name")
	}

	if !strings.Contains(template.HTML, "How to optimize database queries?") {
		t.Error("HTML should contain question title")
	}
}

func TestEmailTemplate_UpvoteMilestone(t *testing.T) {
	tests := []int{10, 50, 100}

	for _, milestone := range tests {
		t.Run(string(rune(milestone)), func(t *testing.T) {
			template := UpvoteMilestoneEmailTemplate(
				"Developer Dave",
				"My awesome post",
				milestone,
				"https://solvr.dev/posts/xyz",
			)

			if !strings.Contains(template.Subject, "upvotes") {
				t.Errorf("Subject should contain 'upvotes', got '%s'", template.Subject)
			}

			if !strings.Contains(template.HTML, "My awesome post") {
				t.Error("HTML should contain post title")
			}
		})
	}
}

func TestEmailMessage_Validate(t *testing.T) {
	tests := []struct {
		name      string
		msg       EmailMessage
		wantError bool
	}{
		{
			name: "valid with HTML",
			msg: EmailMessage{
				To:      "user@example.com",
				Subject: "Test",
				HTML:    "<p>Hello</p>",
			},
			wantError: false,
		},
		{
			name: "valid with Text",
			msg: EmailMessage{
				To:      "user@example.com",
				Subject: "Test",
				Text:    "Hello",
			},
			wantError: false,
		},
		{
			name: "valid with both",
			msg: EmailMessage{
				To:      "user@example.com",
				Subject: "Test",
				HTML:    "<p>Hello</p>",
				Text:    "Hello",
			},
			wantError: false,
		},
		{
			name: "missing To",
			msg: EmailMessage{
				Subject: "Test",
				HTML:    "<p>Hello</p>",
			},
			wantError: true,
		},
		{
			name: "missing Subject",
			msg: EmailMessage{
				To:   "user@example.com",
				HTML: "<p>Hello</p>",
			},
			wantError: true,
		},
		{
			name: "missing body (both HTML and Text)",
			msg: EmailMessage{
				To:      "user@example.com",
				Subject: "Test",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
