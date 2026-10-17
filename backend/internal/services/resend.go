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
	replyTo   string
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

// FromAddress is the address this client sends from.
func (c *ResendClient) FromAddress() string {
	return c.fromEmail
}

// SetReplyTo sets the Reply-To of every email this client sends; empty sends none (SPEC.md 16.6).
func (c *ResendClient) SetReplyTo(addr string) {
	c.replyTo = addr
}

// ReplyTo is the Reply-To this client sends, or "" for none.
func (c *ResendClient) ReplyTo() string {
	return c.replyTo
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

// Send sends a single email via the Resend API and returns Resend's id for it, which its
// webhook events name (SPEC.md 16.6).
// Parameters: to (recipient), subject, htmlBody, textBody (can be empty).
// The from address is set at construction time as "Solvr <fromEmail>", and the Reply-To, if any,
// by SetReplyTo.
// Optional variadic headers map allows passing custom headers (e.g., List-Unsubscribe).
func (c *ResendClient) Send(ctx context.Context, to, subject, htmlBody, textBody string, headers ...map[string]string) (string, error) {
	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("Solvr <%s>", c.fromEmail),
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
		ReplyTo: c.replyTo,
	}
	if len(headers) > 0 && headers[0] != nil {
		params.Headers = headers[0]
	}

	sent, err := c.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("resend send failed: %w", err)
	}
	if sent == nil {
		return "", nil
	}
	return sent.Id, nil
}
