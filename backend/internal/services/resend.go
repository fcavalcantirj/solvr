package services

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	resend "github.com/resend/resend-go/v3"
)

// ResendClient sends emails via the Resend API using the resend-go/v3 SDK.
// It satisfies an EmailSender interface via Go's implicit interface satisfaction.
type ResendClient struct {
	client    *resend.Client
	fromEmail string
}

// NewResendClient creates a new ResendClient.
// apiKey is the Resend API key (RESEND_API_KEY env var).
// fromEmail is the sender address (e.g., "noreply@solvr.dev").
func NewResendClient(apiKey, fromEmail string) *ResendClient {
	return &ResendClient{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
	}
}

// SetBaseURL overrides the Resend API base URL (for testing with httptest).
// The resend-go SDK uses a *url.URL for BaseURL, so we parse the string.
// A trailing slash is appended if missing to ensure correct URL resolution.
func (c *ResendClient) SetBaseURL(rawURL string) {
	if !strings.HasSuffix(rawURL, "/") {
		rawURL += "/"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	c.client.BaseURL = u
}

// Send sends a single email via the Resend API.
// Parameters: to (recipient), subject, htmlBody, textBody (can be empty).
// The from address is set at construction time as "Solvr <fromEmail>".
func (c *ResendClient) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("Solvr <%s>", c.fromEmail),
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
	}

	_, err := c.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("resend send failed: %w", err)
	}

	return nil
}
