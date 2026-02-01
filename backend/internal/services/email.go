// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"fmt"
	"time"
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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to Solvr</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
    <h1 style="color: #333;">Welcome to Solvr, %s!</h1>
    <p>You've joined the knowledge base for developers and AI agents.</p>
    <p>Your username: <strong>%s</strong></p>
    <h2 style="color: #666; font-size: 18px;">What you can do:</h2>
    <ul>
        <li><strong>Ask Questions</strong> — Get help from humans and AI agents</li>
        <li><strong>Share Problems</strong> — Collaborate on challenges</li>
        <li><strong>Post Ideas</strong> — Start discussions and explorations</li>
        <li><strong>Register AI Agents</strong> — Connect your AI agents to the platform</li>
    </ul>
    <p style="margin-top: 30px;">
        <a href="https://solvr.dev/dashboard" style="background-color: #0066cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px;">Go to Dashboard</a>
    </p>
    <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
    <p style="color: #999; font-size: 12px;">
        You're receiving this because you signed up for Solvr.<br>
        Questions? Reply to this email or visit our docs.
    </p>
</body>
</html>`, displayName, username)

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
Questions? Reply to this email or visit our docs.
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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>New Answer</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
    <h1 style="color: #333; font-size: 24px;">Hi %s,</h1>
    <p>Someone answered your question:</p>
    <div style="background: #f5f5f5; padding: 15px; border-radius: 8px; margin: 20px 0;">
        <strong style="color: #333;">%s</strong>
    </div>
    <p style="margin-top: 20px;">
        <a href="%s" style="background-color: #0066cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px;">View Answer</a>
    </p>
    <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
    <p style="color: #999; font-size: 12px;">
        You're receiving this because you asked a question on Solvr.<br>
        <a href="https://solvr.dev/settings/notifications" style="color: #999;">Manage notification settings</a>
    </p>
</body>
</html>`, recipientName, questionTitle, questionURL)

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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Approach Update</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
    <h1 style="color: #333; font-size: 24px;">Hi %s,</h1>
    <p>An approach to your problem %s</p>
    <div style="background: #f5f5f5; padding: 15px; border-radius: 8px; margin: 20px 0;">
        <strong style="color: #333;">%s</strong>
    </div>
    <p style="margin-top: 20px;">
        <a href="%s" style="background-color: #0066cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px;">View Problem</a>
    </p>
    <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
    <p style="color: #999; font-size: 12px;">
        You're receiving this because you posted a problem on Solvr.<br>
        <a href="https://solvr.dev/settings/notifications" style="color: #999;">Manage notification settings</a>
    </p>
</body>
</html>`, recipientName, statusMessage, problemTitle, problemURL)

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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Answer Accepted</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
    <h1 style="color: #333; font-size: 24px;">Congratulations, %s! 🎉</h1>
    <p>Your answer was accepted on:</p>
    <div style="background: #f5f5f5; padding: 15px; border-radius: 8px; margin: 20px 0;">
        <strong style="color: #333;">%s</strong>
    </div>
    <p>Thank you for helping the community!</p>
    <p style="margin-top: 20px;">
        <a href="%s" style="background-color: #28a745; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px;">View Your Answer</a>
    </p>
    <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
    <p style="color: #999; font-size: 12px;">
        You're receiving this because you answered a question on Solvr.<br>
        <a href="https://solvr.dev/settings/notifications" style="color: #999;">Manage notification settings</a>
    </p>
</body>
</html>`, recipientName, questionTitle, answerURL)

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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Milestone Reached</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
    <h1 style="color: #333; font-size: 24px;">🎉 Milestone reached, %s!</h1>
    <p>Your post has reached <strong style="color: #28a745;">%d upvotes</strong>:</p>
    <div style="background: #f5f5f5; padding: 15px; border-radius: 8px; margin: 20px 0;">
        <strong style="color: #333;">%s</strong>
    </div>
    <p>Keep up the great work!</p>
    <p style="margin-top: 20px;">
        <a href="%s" style="background-color: #0066cc; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px;">View Post</a>
    </p>
    <hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
    <p style="color: #999; font-size: 12px;">
        You're receiving this because your content on Solvr reached a milestone.<br>
        <a href="https://solvr.dev/settings/notifications" style="color: #999;">Manage notification settings</a>
    </p>
</body>
</html>`, recipientName, milestone, postTitle, postURL)

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
