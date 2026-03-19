// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/emailutil"
)

// EmailConfig holds SMTP configuration settings.
type EmailConfig struct {
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	FromEmail string
}

// Validate checks if the email config is valid.
func (c *EmailConfig) Validate() error {
	if c.SMTPHost == "" {
		return errors.New("SMTP host is required")
	}
	if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
		return errors.New("SMTP port must be between 1 and 65535")
	}
	if c.FromEmail == "" {
		return errors.New("From email is required")
	}
	return nil
}

// Email errors.
var (
	ErrInvalidEmailRecipient = errors.New("email recipient (To) is required")
	ErrInvalidEmailSubject   = errors.New("email subject is required")
	ErrInvalidEmailBody      = errors.New("email body (HTML or Text) is required")
)

// EmailMessage represents an email to be sent.
type EmailMessage struct {
	From    string
	To      string
	Subject string
	HTML    string
	Text    string
}

// Validate checks if the email message is valid.
func (m *EmailMessage) Validate() error {
	if m.To == "" {
		return ErrInvalidEmailRecipient
	}
	if m.Subject == "" {
		return ErrInvalidEmailSubject
	}
	if m.HTML == "" && m.Text == "" {
		return ErrInvalidEmailBody
	}
	return nil
}

// SMTPClient defines the interface for sending emails via SMTP.
type SMTPClient interface {
	Send(msg *EmailMessage) error
}

// EmailService handles email sending with optional retry logic.
type EmailService struct {
	client       SMTPClient
	fromEmail    string
	maxRetries   int
	retryBackoff time.Duration
	sendFunc     func(msg *EmailMessage) error // For testing override
}

// NewEmailService creates a new email service.
func NewEmailService(client SMTPClient, fromEmail string) *EmailService {
	svc := &EmailService{
		client:       client,
		fromEmail:    fromEmail,
		maxRetries:   1,
		retryBackoff: 0,
	}
	svc.sendFunc = func(msg *EmailMessage) error {
		return client.Send(msg)
	}
	return svc
}

// NewEmailServiceWithRetry creates a new email service with retry configuration.
func NewEmailServiceWithRetry(client SMTPClient, fromEmail string, maxRetries int, backoff time.Duration) *EmailService {
	svc := &EmailService{
		client:       client,
		fromEmail:    fromEmail,
		maxRetries:   maxRetries,
		retryBackoff: backoff,
	}
	svc.sendFunc = func(msg *EmailMessage) error {
		return client.Send(msg)
	}
	return svc
}

// SendEmail sends an email synchronously.
func (s *EmailService) SendEmail(ctx context.Context, msg *EmailMessage) error {
	if msg.To == "" {
		return ErrInvalidEmailRecipient
	}
	if msg.Subject == "" {
		return ErrInvalidEmailSubject
	}

	// Set From address if not already set
	if msg.From == "" {
		msg.From = s.fromEmail
	}

	return s.sendFunc(msg)
}

// SendEmailAsync sends an email asynchronously (non-blocking).
// The email is queued and sent in a background goroutine.
func (s *EmailService) SendEmailAsync(ctx context.Context, msg *EmailMessage) {
	go func() {
		// Use background context since the parent context may be canceled
		_ = s.SendEmail(context.Background(), msg)
	}()
}

// SendEmailWithRetry sends an email with retry logic.
// Retries up to maxRetries times with exponential backoff.
func (s *EmailService) SendEmailWithRetry(ctx context.Context, msg *EmailMessage) error {
	if msg.To == "" {
		return ErrInvalidEmailRecipient
	}
	if msg.Subject == "" {
		return ErrInvalidEmailSubject
	}

	// Set From address if not already set
	if msg.From == "" {
		msg.From = s.fromEmail
	}

	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := s.sendFunc(msg)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt < s.maxRetries {
			backoff := s.retryBackoff * time.Duration(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Continue to next retry
			}
		}
	}

	return fmt.Errorf("failed to send email after max retries (%d): %w", s.maxRetries, lastErr)
}

// Email templates per PRD requirements.

// EmailTemplate represents a rendered email template.
type EmailTemplate struct {
	Subject string
	HTML    string
	Text    string
}

// WelcomeEmailTemplate generates a welcome email for new users.
// Per PRD: "Email template: welcome" - Send on user signup.
func WelcomeEmailTemplate(displayName, username string) *EmailTemplate {
	subject := "Welcome to Solvr!"

	content := fmt.Sprintf(`
                            <h1 style="color: #1a1a1a; font-size: 24px; font-weight: 600; margin: 0 0 16px 0;">Welcome to Solvr, %s!</h1>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 12px 0;">You've joined the knowledge base for developers and AI agents.</p>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 20px 0;">Your username: <strong>%s</strong></p>
                            <h2 style="color: #3f3f46; font-size: 16px; font-weight: 600; margin: 0 0 12px 0;">What you can do:</h2>
                            <ul style="color: #3f3f46; font-size: 14px; line-height: 1.8; margin: 0 0 24px 0; padding-left: 20px;">
                                <li><strong>Ask Questions</strong> — Get help from humans and AI agents</li>
                                <li><strong>Share Problems</strong> — Collaborate on challenges</li>
                                <li><strong>Post Ideas</strong> — Start discussions and explorations</li>
                                <li><strong>Register AI Agents</strong> — Connect your AI agents to the platform</li>
                            </ul>
                            <p style="margin: 24px 0 0 0;">
                                <a href="https://solvr.dev/dashboard" style="display: inline-block; background-color: #0a0a0a; color: #ffffff; padding: 12px 24px; text-decoration: none; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 14px; font-weight: 600;">Go to Dashboard</a>
                            </p>`, displayName, username)

	html := emailutil.WrapInBrandedTemplate(content, "https://solvr.dev/settings/notifications", "You signed up for Solvr")

	text := fmt.Sprintf(`Welcome to Solvr, %s!

You've joined the knowledge base for developers and AI agents.

Your username: %s

What you can do:
- Ask Questions: Get help from humans and AI agents
- Share Problems: Collaborate on challenges
- Post Ideas: Start discussions and explorations
- Register AI Agents: Connect your AI agents to the platform

Get started: https://solvr.dev/dashboard

---
You're receiving this because you signed up for Solvr.

Manage notifications: https://solvr.dev/settings/notifications
`, displayName, username)

	return &EmailTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}

// NewAnswerEmailTemplate generates an email for new answer notifications.
// Per PRD: "Email template: new answer" - Include link to question.
func NewAnswerEmailTemplate(recipientName, questionTitle, questionURL string) *EmailTemplate {
	subject := "New answer to your question"

	content := fmt.Sprintf(`
                            <h1 style="color: #1a1a1a; font-size: 24px; font-weight: 600; margin: 0 0 16px 0;">Hi %s,</h1>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 16px 0;">Someone answered your question:</p>
                            <div style="background: #f4f4f5; padding: 16px; margin: 0 0 20px 0; border-left: 3px solid #0a0a0a;">
                                <strong style="color: #1a1a1a; font-size: 14px;">%s</strong>
                            </div>
                            <p style="margin: 24px 0 0 0;">
                                <a href="%s" style="display: inline-block; background-color: #0a0a0a; color: #ffffff; padding: 12px 24px; text-decoration: none; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 14px; font-weight: 600;">View Answer</a>
                            </p>`, recipientName, questionTitle, questionURL)

	html := emailutil.WrapInBrandedTemplate(content, "https://solvr.dev/settings/notifications", "You asked a question on Solvr")

	text := fmt.Sprintf(`Hi %s,

Someone answered your question:

"%s"

View the answer: %s

---
You're receiving this because you asked a question on Solvr.

Manage notifications: https://solvr.dev/settings/notifications
`, recipientName, questionTitle, questionURL)

	return &EmailTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}

// ApproachUpdateEmailTemplate generates an email for approach status changes.
// Per PRD: "Email template: approach update" - Create approach status change template.
func ApproachUpdateEmailTemplate(recipientName, problemTitle, newStatus, problemURL string) *EmailTemplate {
	var statusMessage string
	switch newStatus {
	case "succeeded":
		statusMessage = "has <strong style='color: #28a745;'>succeeded</strong>! 🎉"
	case "stuck":
		statusMessage = "is <strong style='color: #ffc107;'>stuck</strong> and needs help"
	case "failed":
		statusMessage = "has <strong style='color: #dc3545;'>failed</strong>"
	default:
		statusMessage = fmt.Sprintf("has a status update: <strong>%s</strong>", newStatus)
	}

	subject := fmt.Sprintf("Approach %s on your problem", newStatus)

	content := fmt.Sprintf(`
                            <h1 style="color: #1a1a1a; font-size: 24px; font-weight: 600; margin: 0 0 16px 0;">Hi %s,</h1>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 16px 0;">An approach to your problem %s</p>
                            <div style="background: #f4f4f5; padding: 16px; margin: 0 0 20px 0; border-left: 3px solid #0a0a0a;">
                                <strong style="color: #1a1a1a; font-size: 14px;">%s</strong>
                            </div>
                            <p style="margin: 24px 0 0 0;">
                                <a href="%s" style="display: inline-block; background-color: #0a0a0a; color: #ffffff; padding: 12px 24px; text-decoration: none; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 14px; font-weight: 600;">View Problem</a>
                            </p>`, recipientName, statusMessage, problemTitle, problemURL)

	html := emailutil.WrapInBrandedTemplate(content, "https://solvr.dev/settings/notifications", "You posted a problem on Solvr")

	var statusText string
	switch newStatus {
	case "succeeded":
		statusText = "has succeeded!"
	case "stuck":
		statusText = "is stuck and needs help"
	case "failed":
		statusText = "has failed"
	default:
		statusText = fmt.Sprintf("has a status update: %s", newStatus)
	}

	text := fmt.Sprintf(`Hi %s,

An approach to your problem %s

"%s"

View the problem: %s

---
You're receiving this because you posted a problem on Solvr.

Manage notifications: https://solvr.dev/settings/notifications
`, recipientName, statusText, problemTitle, problemURL)

	return &EmailTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}

// AcceptedAnswerEmailTemplate generates an email when an answer is accepted.
func AcceptedAnswerEmailTemplate(recipientName, questionTitle, answerURL string) *EmailTemplate {
	subject := "Your answer was accepted!"

	content := fmt.Sprintf(`
                            <h1 style="color: #1a1a1a; font-size: 24px; font-weight: 600; margin: 0 0 16px 0;">Congratulations, %s!</h1>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 16px 0;">Your answer was accepted on:</p>
                            <div style="background: #f4f4f5; padding: 16px; margin: 0 0 16px 0; border-left: 3px solid #0a0a0a;">
                                <strong style="color: #1a1a1a; font-size: 14px;">%s</strong>
                            </div>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 24px 0;">Thank you for helping the community!</p>
                            <p style="margin: 0;">
                                <a href="%s" style="display: inline-block; background-color: #0a0a0a; color: #ffffff; padding: 12px 24px; text-decoration: none; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 14px; font-weight: 600;">View Your Answer</a>
                            </p>`, recipientName, questionTitle, answerURL)

	html := emailutil.WrapInBrandedTemplate(content, "https://solvr.dev/settings/notifications", "You answered a question on Solvr")

	text := fmt.Sprintf(`Congratulations, %s!

Your answer was accepted on:

"%s"

Thank you for helping the community!

View your answer: %s

---
You're receiving this because you answered a question on Solvr.

Manage notifications: https://solvr.dev/settings/notifications
`, recipientName, questionTitle, answerURL)

	return &EmailTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}

// UpvoteMilestoneEmailTemplate generates an email for upvote milestones.
func UpvoteMilestoneEmailTemplate(recipientName, postTitle string, milestone int, postURL string) *EmailTemplate {
	subject := fmt.Sprintf("Your post reached %d upvotes!", milestone)

	content := fmt.Sprintf(`
                            <h1 style="color: #1a1a1a; font-size: 24px; font-weight: 600; margin: 0 0 16px 0;">Milestone reached, %s!</h1>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 16px 0;">Your post has reached <strong style="color: #16a34a;">%d upvotes</strong>:</p>
                            <div style="background: #f4f4f5; padding: 16px; margin: 0 0 16px 0; border-left: 3px solid #0a0a0a;">
                                <strong style="color: #1a1a1a; font-size: 14px;">%s</strong>
                            </div>
                            <p style="color: #3f3f46; font-size: 14px; line-height: 1.6; margin: 0 0 24px 0;">Keep up the great work!</p>
                            <p style="margin: 0;">
                                <a href="%s" style="display: inline-block; background-color: #0a0a0a; color: #ffffff; padding: 12px 24px; text-decoration: none; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 14px; font-weight: 600;">View Post</a>
                            </p>`, recipientName, milestone, postTitle, postURL)

	html := emailutil.WrapInBrandedTemplate(content, "https://solvr.dev/settings/notifications", "Your content on Solvr reached a milestone")

	text := fmt.Sprintf(`Milestone reached, %s!

Your post has reached %d upvotes:

"%s"

Keep up the great work!

View your post: %s

---
You're receiving this because your content on Solvr reached a milestone.

Manage notifications: https://solvr.dev/settings/notifications
`, recipientName, milestone, postTitle, postURL)

	return &EmailTemplate{
		Subject: subject,
		HTML:    html,
		Text:    text,
	}
}
