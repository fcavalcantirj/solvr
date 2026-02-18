package models

import (
	"testing"
	"time"
)

func TestIsValidPinStatus(t *testing.T) {
	tests := []struct {
		status PinStatus
		valid  bool
	}{
		{PinStatusQueued, true},
		{PinStatusPinning, true},
		{PinStatusPinned, true},
		{PinStatusFailed, true},
		{PinStatus("invalid"), false},
		{PinStatus(""), false},
		{PinStatus("pending"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := IsValidPinStatus(tt.status)
			if got != tt.valid {
				t.Errorf("IsValidPinStatus(%q) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestPin_ToPinResponse(t *testing.T) {
	now := time.Now().UTC()
	sizeBytes := int64(12345)

	t.Run("full pin with all fields", func(t *testing.T) {
		pin := &Pin{
			ID:        "req-123",
			CID:       "QmTest123",
			Status:    PinStatusPinned,
			Name:      "my-file",
			Origins:   []string{"/ip4/127.0.0.1/tcp/4001"},
			Meta:      map[string]string{"key": "value"},
			Delegates: []string{"/ip4/10.0.0.1/tcp/4001"},
			OwnerID:   "user-1",
			OwnerType: "user",
			SizeBytes: &sizeBytes,
			CreatedAt: now,
		}

		resp := pin.ToPinResponse()

		if resp.RequestID != "req-123" {
			t.Errorf("expected requestid req-123, got %s", resp.RequestID)
		}
		if resp.Status != PinStatusPinned {
			t.Errorf("expected status pinned, got %s", resp.Status)
		}
		if resp.Created != now {
			t.Errorf("expected created %v, got %v", now, resp.Created)
		}
		if resp.Pin.CID != "QmTest123" {
			t.Errorf("expected cid QmTest123, got %s", resp.Pin.CID)
		}
		if resp.Pin.Name != "my-file" {
			t.Errorf("expected name my-file, got %s", resp.Pin.Name)
		}
		if len(resp.Pin.Origins) != 1 {
			t.Errorf("expected 1 origin, got %d", len(resp.Pin.Origins))
		}
		if resp.Pin.Meta["key"] != "value" {
			t.Errorf("expected meta key=value, got %v", resp.Pin.Meta)
		}
		if len(resp.Delegates) != 1 {
			t.Errorf("expected 1 delegate, got %d", len(resp.Delegates))
		}
		if resp.Info == nil {
			t.Fatal("expected info to be set")
		}
		if *resp.Info.SizeBytes != 12345 {
			t.Errorf("expected size_bytes 12345, got %d", *resp.Info.SizeBytes)
		}
	})

	t.Run("minimal pin without optional fields", func(t *testing.T) {
		pin := &Pin{
			ID:        "req-456",
			CID:       "bafyTest456",
			Status:    PinStatusQueued,
			OwnerID:   "agent-1",
			OwnerType: "agent",
			CreatedAt: now,
		}

		resp := pin.ToPinResponse()

		if resp.RequestID != "req-456" {
			t.Errorf("expected requestid req-456, got %s", resp.RequestID)
		}
		if resp.Status != PinStatusQueued {
			t.Errorf("expected status queued, got %s", resp.Status)
		}
		if resp.Pin.CID != "bafyTest456" {
			t.Errorf("expected cid bafyTest456, got %s", resp.Pin.CID)
		}
		// Delegates should default to empty array, not nil
		if resp.Delegates == nil {
			t.Error("expected delegates to be non-nil empty array")
		}
		if len(resp.Delegates) != 0 {
			t.Errorf("expected 0 delegates, got %d", len(resp.Delegates))
		}
		// Info should be nil when SizeBytes is nil
		if resp.Info != nil {
			t.Error("expected info to be nil when size_bytes not set")
		}
	})
}

func TestPinStatusConstants(t *testing.T) {
	if PinStatusQueued != "queued" {
		t.Errorf("PinStatusQueued = %q, want %q", PinStatusQueued, "queued")
	}
	if PinStatusPinning != "pinning" {
		t.Errorf("PinStatusPinning = %q, want %q", PinStatusPinning, "pinning")
	}
	if PinStatusPinned != "pinned" {
		t.Errorf("PinStatusPinned = %q, want %q", PinStatusPinned, "pinned")
	}
	if PinStatusFailed != "failed" {
		t.Errorf("PinStatusFailed = %q, want %q", PinStatusFailed, "failed")
	}
}
