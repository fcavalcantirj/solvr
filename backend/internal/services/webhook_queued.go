package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"syscall"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ErrWebhookAddressNotPublic is returned when a webhook URL resolves to an address that is
// not on the public internet (loopback, private, link-local, shared, multicast or
// unspecified): the API never posts into its own network.
var ErrWebhookAddressNotPublic = errors.New("webhook address is not a public internet address")

// webhookDeliveryTimeout is the success window: a 2xx within 10 seconds (SPEC.md Part 12.3).
const webhookDeliveryTimeout = 10 * time.Second

// sharedAddressSpace is 100.64.0.0/10 (RFC 6598), which netip does not classify.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// isPublicWebhookIP reports whether a delivery may connect to ip.
func isPublicWebhookIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !sharedAddressSpace.Contains(ip)
}

// NewWebhookHTTPClient is the client deliveries are sent with: it connects only to public
// addresses — checked on the address it dials, after DNS, so a name cannot be pointed at the
// API's own network — answers within the success window, and follows no redirect (a 3xx is
// a failed delivery).
func NewWebhookHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: webhookDeliveryTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || !isPublicWebhookIP(ap.Addr()) {
				return fmt.Errorf("%w: %s", ErrWebhookAddressNotPublic, address)
			}
			return nil
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{
		Timeout:       webhookDeliveryTimeout,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// BuildDeliveryPayload is the body of every attempt of d: its delivery ID, event, schema
// version, the time the event occurred and the data fixed when it was queued. The same
// delivery always gives the same bytes, and so the same signature.
func (s *WebhookDeliveryService) BuildDeliveryPayload(d *models.WebhookDelivery) ([]byte, error) {
	data := d.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	return json.Marshal(models.WebhookDeliveryPayload{
		ID:            d.ID.String(),
		Event:         d.Event,
		SchemaVersion: d.SchemaVersion,
		Timestamp:     d.OccurredAt.UTC().Format(time.RFC3339),
		Data:          data,
	})
}

// SendQueued sends one attempt of a queued delivery to its webhook, signed with the
// webhook's secret, and records the webhook's health. It returns the receiver's status (0
// when there was no answer) and ErrWebhookDeliveryFailed unless the answer was a 2xx.
func (s *WebhookDeliveryService) SendQueued(ctx context.Context, d *models.WebhookDelivery) (int, error) {
	payload, err := s.BuildDeliveryPayload(d)
	if err != nil {
		return 0, fmt.Errorf("failed to build payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Webhook.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, s.recordFailure(ctx, &d.Webhook, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Solvr-Signature", s.GenerateSignature(payload, d.Webhook.Secret))
	req.Header.Set("X-Solvr-Event", d.Event)
	req.Header.Set("X-Solvr-Delivery-ID", d.ID.String())
	req.Header.Set("X-Solvr-Webhook-ID", d.WebhookID.String())
	req.Header.Set("X-Solvr-Delivery-Attempt", strconv.Itoa(d.Attempts+1))

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, s.recordFailure(ctx, &d.Webhook, err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, s.recordFailure(ctx, &d.Webhook, fmt.Errorf("webhook returned %d", resp.StatusCode))
	}
	// The receiver has the event: a failure to record the webhook's health must not make the
	// delivery look failed, or the retry would be a second delivery of a delivered event.
	if err := s.recordSuccess(ctx, &d.Webhook); err != nil {
		slog.Warn("webhook delivered but its health was not recorded", "webhook_id", d.WebhookID, "error", err)
	}
	return resp.StatusCode, nil
}
