package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: a queued delivery is sent with its delivery ID, event and schema version, and
// every attempt sends the same body and signature — a retry is the same event, not a new one.

type receivedDelivery struct {
	header http.Header
	body   []byte
}

func queuedDelivery(url, secret string, attempts int) *models.WebhookDelivery {
	return &models.WebhookDelivery{
		ID: uuid.New(), WebhookID: uuid.New(), NotificationID: uuid.New(),
		Event: models.NotificationReplyRemoved, SchemaVersion: 1,
		Data:       json.RawMessage(`{"agent_id":"agent_x","subject":{"post_id":"p1","reply_id":"r1"},"title":"Your reply was removed"}`),
		OccurredAt: time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC),
		Attempts:   attempts,
		Status:     models.WebhookDeliveryPending,
		Webhook:    models.Webhook{URL: url, Secret: secret, Status: models.WebhookStatusActive},
	}
}

func TestWebhookQueuedDelivery_EveryAttemptSendsTheSameIDBodyAndSignature(t *testing.T) {
	var mu sync.Mutex
	var got []receivedDelivery
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, receivedDelivery{header: r.Header.Clone(), body: body})
		mu.Unlock()
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	service := NewWebhookDeliveryService(repo, server.Client())
	const secret = "whsec-queued-test"
	d := queuedDelivery(server.URL, secret, 0)
	d.Webhook.ID = d.WebhookID

	status, err := service.SendQueued(context.Background(), d)
	require.ErrorIs(t, err, ErrWebhookDeliveryFailed)
	require.Equal(t, http.StatusServiceUnavailable, status)
	d.Attempts = 1
	status, err = service.SendQueued(context.Background(), d)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)

	require.Len(t, got, 2)
	require.Equal(t, string(got[0].body), string(got[1].body), "a retry sends the same bytes")
	for i, r := range got {
		require.Equal(t, d.ID.String(), r.header.Get("X-Solvr-Delivery-ID"), "attempt %d", i+1)
		require.Equal(t, d.WebhookID.String(), r.header.Get("X-Solvr-Webhook-ID"))
		require.Equal(t, models.NotificationReplyRemoved, r.header.Get("X-Solvr-Event"))
		require.Equal(t, []string{"1", "2"}[i], r.header.Get("X-Solvr-Delivery-Attempt"))
		require.Equal(t, "application/json", r.header.Get("Content-Type"))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(r.body)
		require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), r.header.Get("X-Solvr-Signature"))
	}
	require.Equal(t, got[0].header.Get("X-Solvr-Signature"), got[1].header.Get("X-Solvr-Signature"))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(got[0].body, &payload))
	require.Equal(t, map[string]any{
		"id": d.ID.String(), "event": "reply.removed", "schema_version": float64(1), "timestamp": "2026-10-01T22:00:00Z",
		"data": map[string]any{"agent_id": "agent_x", "subject": map[string]any{"post_id": "p1", "reply_id": "r1"},
			"title": "Your reply was removed"},
	}, payload)

	require.Len(t, repo.UpdateCalls, 2, "the webhook's health follows each attempt")
	require.Equal(t, 1, *repo.UpdateCalls[0].ConsecutiveFailures)
	require.Equal(t, 0, *repo.UpdateCalls[1].ConsecutiveFailures)
	require.Equal(t, models.WebhookStatusActive, *repo.UpdateCalls[1].Status)
}

func TestWebhookHTTPClient_RefusesPrivateAddressesAndRedirects(t *testing.T) {
	var reached atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewWebhookHTTPClient()
	require.Equal(t, 10*time.Second, client.Timeout, "success is a 2xx within 10 seconds (SPEC.md Part 12.3)")
	for _, url := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1),
		"https://10.0.0.1:1/", "https://169.254.169.254/latest/meta-data/", "https://[::1]:1/"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
		require.NoError(t, err)
		resp, err := client.Do(req)
		cancel()
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err, url)
		require.ErrorIs(t, err, ErrWebhookAddressNotPublic, url)
	}
	require.Zero(t, reached.Load(), "no request reached a private address")

	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		require.True(t, isPublicWebhookIP(mustParseIP(t, ip)), ip)
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0",
		"100.64.0.1", "::1", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "224.0.0.1"} {
		require.False(t, isPublicWebhookIP(mustParseIP(t, ip)), ip)
	}

	redirect := &http.Request{}
	require.ErrorIs(t, client.CheckRedirect(redirect, nil), http.ErrUseLastResponse, "a redirect is not followed: a 3xx is a failed delivery")
}

func mustParseIP(t *testing.T, s string) netip.Addr {
	t.Helper()
	ip, err := netip.ParseAddr(s)
	require.NoError(t, err)
	return ip
}
