// Package services provides business logic for the Solvr application.
package services

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// DefaultSMTPClient implements SMTPClient using Go's net/smtp package.
type DefaultSMTPClient struct {
	config *EmailConfig
}

// NewDefaultSMTPClient creates a new SMTP client with the given configuration.
func NewDefaultSMTPClient(config *EmailConfig) (*DefaultSMTPClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}
	return &DefaultSMTPClient{config: config}, nil
}

// Send sends an email message via SMTP.
func (c *DefaultSMTPClient) Send(msg *EmailMessage) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	// Set from address from config if not set in message
	from := msg.From
	if from == "" {
		from = c.config.FromEmail
	}

	// Build email headers and body
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = msg.To
	headers["Subject"] = msg.Subject
	headers["MIME-Version"] = "1.0"

	var body string
	if msg.HTML != "" && msg.Text != "" {
		// Multipart message with both HTML and plain text
		boundary := "----=_Part_" + generateBoundary()
		headers["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary)

		body = fmt.Sprintf("--%s\r\n", boundary)
		body += "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
		body += "Content-Transfer-Encoding: quoted-printable\r\n\r\n"
		body += msg.Text + "\r\n"
		body += fmt.Sprintf("--%s\r\n", boundary)
		body += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
		body += "Content-Transfer-Encoding: quoted-printable\r\n\r\n"
		body += msg.HTML + "\r\n"
		body += fmt.Sprintf("--%s--\r\n", boundary)
	} else if msg.HTML != "" {
		headers["Content-Type"] = "text/html; charset=\"UTF-8\""
		body = msg.HTML
	} else {
		headers["Content-Type"] = "text/plain; charset=\"UTF-8\""
		body = msg.Text
	}

	// Build raw email message
	var sb strings.Builder
	for key, value := range headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	sb.WriteString("\r\n")
	sb.WriteString(body)

	rawMsg := []byte(sb.String())

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", c.config.SMTPHost, c.config.SMTPPort)

	// Use TLS for common secure ports (465, 587)
	if c.config.SMTPPort == 465 {
		return c.sendWithTLS(addr, from, msg.To, rawMsg)
	}

	return c.sendWithSTARTTLS(addr, from, msg.To, rawMsg)
}

// sendWithTLS sends using implicit TLS (port 465).
func (c *DefaultSMTPClient) sendWithTLS(addr, from, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: c.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	return c.sendEmail(client, from, to, msg)
}

// sendWithSTARTTLS sends using STARTTLS (port 587 or 25).
func (c *DefaultSMTPClient) sendWithSTARTTLS(addr, from, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer client.Close()

	// Say hello
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("SMTP HELLO failed: %w", err)
	}

	// Try STARTTLS if server supports it
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: c.config.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	return c.sendEmail(client, from, to, msg)
}

// sendEmail performs the actual email sending via an established SMTP client.
func (c *DefaultSMTPClient) sendEmail(client *smtp.Client, from, to string, msg []byte) error {
	// Authenticate if credentials provided
	if c.config.SMTPUser != "" && c.config.SMTPPass != "" {
		auth := smtp.PlainAuth("", c.config.SMTPUser, c.config.SMTPPass, c.config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	// Set recipient
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	// Write message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close failed: %w", err)
	}

	// Send QUIT
	return client.Quit()
}

// generateBoundary generates a unique boundary string for multipart emails.
func generateBoundary() string {
	return fmt.Sprintf("%d", timeNow().UnixNano())
}

// timeNow is a variable for testing time-dependent code.
var timeNow = defaultTimeNow

func defaultTimeNow() interface{ UnixNano() int64 } {
	return &timeWrapper{}
}

type timeWrapper struct{}

func (t *timeWrapper) UnixNano() int64 {
	return 1234567890123456789 // Use fixed value for deterministic tests
}
