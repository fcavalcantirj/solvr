package services

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fastTestConfig returns an IPFS config with no retries for fast test execution.
func fastTestConfig() IPFSConfig {
	return IPFSConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
		RetryDelay: 0,
	}
}

// newTestService creates a KuboIPFSService with no retries for fast test execution.
func newTestService(baseURL string) *KuboIPFSService {
	return NewKuboIPFSServiceWithConfig(baseURL, fastTestConfig())
}

// TestNewKuboIPFSService tests constructor with various configurations.
func TestNewKuboIPFSService(t *testing.T) {
	t.Run("creates service with default timeout", func(t *testing.T) {
		svc := NewKuboIPFSService("http://localhost:5001")

		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if svc.baseURL != "http://localhost:5001" {
			t.Errorf("expected baseURL http://localhost:5001, got %s", svc.baseURL)
		}
		if svc.httpClient.Timeout != DefaultIPFSTimeout {
			t.Errorf("expected timeout %v, got %v", DefaultIPFSTimeout, svc.httpClient.Timeout)
		}
	})

	t.Run("creates service with custom timeout", func(t *testing.T) {
		timeout := 10 * time.Second
		svc := NewKuboIPFSServiceWithTimeout("http://localhost:5001", timeout)

		if svc.httpClient.Timeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, svc.httpClient.Timeout)
		}
	})

	t.Run("strips trailing slash from baseURL", func(t *testing.T) {
		svc := NewKuboIPFSService("http://localhost:5001/")

		if svc.baseURL != "http://localhost:5001" {
			t.Errorf("expected baseURL without trailing slash, got %s", svc.baseURL)
		}
	})
}

// TestKuboIPFSService_Pin tests the Pin method.
func TestKuboIPFSService_Pin(t *testing.T) {
	t.Run("pins CID successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/api/v0/pin/add") {
				t.Errorf("expected /api/v0/pin/add, got %s", r.URL.Path)
			}

			arg := r.URL.Query().Get("arg")
			if arg != "QmTest123" {
				t.Errorf("expected arg=QmTest123, got %s", arg)
			}

			progress := r.URL.Query().Get("progress")
			if progress != "false" {
				t.Errorf("expected progress=false, got %s", progress)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Pins": []string{"QmTest123"},
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		err := svc.Pin(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"Message": "internal error", "Code": 0}`))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		err := svc.Pin(context.Background(), "QmTest123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error on connection failure", func(t *testing.T) {
		svc := NewKuboIPFSServiceWithConfig("http://127.0.0.1:1", IPFSConfig{
			Timeout:    500 * time.Millisecond,
			MaxRetries: 0,
			RetryDelay: 0,
		})
		err := svc.Pin(context.Background(), "QmTest123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error for empty CID", func(t *testing.T) {
		svc := newTestService("http://localhost:5001")
		err := svc.Pin(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty CID")
		}
		if err != ErrEmptyCID {
			t.Errorf("expected ErrEmptyCID, got %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := svc.Pin(ctx, "QmTest123")
		if err == nil {
			t.Fatal("expected error from context cancellation")
		}
	})
}

// TestKuboIPFSService_Unpin tests the Unpin method.
func TestKuboIPFSService_Unpin(t *testing.T) {
	t.Run("unpins CID successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/api/v0/pin/rm") {
				t.Errorf("expected /api/v0/pin/rm, got %s", r.URL.Path)
			}

			arg := r.URL.Query().Get("arg")
			if arg != "QmTest123" {
				t.Errorf("expected arg=QmTest123, got %s", arg)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Pins": []string{"QmTest123"},
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		err := svc.Unpin(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when CID not pinned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"Message": "not pinned or pinned indirectly", "Code": 0}`))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		err := svc.Unpin(context.Background(), "QmNotPinned")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error for empty CID", func(t *testing.T) {
		svc := newTestService("http://localhost:5001")
		err := svc.Unpin(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty CID")
		}
		if err != ErrEmptyCID {
			t.Errorf("expected ErrEmptyCID, got %v", err)
		}
	})
}

// TestKuboIPFSService_PinStatus tests the PinStatus method.
func TestKuboIPFSService_PinStatus(t *testing.T) {
	t.Run("returns pin type for pinned CID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/api/v0/pin/ls") {
				t.Errorf("expected /api/v0/pin/ls, got %s", r.URL.Path)
			}

			arg := r.URL.Query().Get("arg")
			if arg != "QmTest123" {
				t.Errorf("expected arg=QmTest123, got %s", arg)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Keys": map[string]interface{}{
					"QmTest123": map[string]interface{}{
						"Type": "recursive",
					},
				},
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		status, err := svc.PinStatus(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != "recursive" {
			t.Errorf("expected status 'recursive', got '%s'", status)
		}
	})

	t.Run("returns direct pin type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Keys": map[string]interface{}{
					"QmTest123": map[string]interface{}{
						"Type": "direct",
					},
				},
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		status, err := svc.PinStatus(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status != "direct" {
			t.Errorf("expected status 'direct', got '%s'", status)
		}
	})

	t.Run("returns error when CID not pinned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"Message": "path is not pinned", "Code": 0}`))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		_, err := svc.PinStatus(context.Background(), "QmNotPinned")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error for empty CID", func(t *testing.T) {
		svc := newTestService("http://localhost:5001")
		_, err := svc.PinStatus(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty CID")
		}
		if err != ErrEmptyCID {
			t.Errorf("expected ErrEmptyCID, got %v", err)
		}
	})
}

// TestKuboIPFSService_Add tests the Add method.
func TestKuboIPFSService_Add(t *testing.T) {
	t.Run("adds content and returns CID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/api/v0/add") {
				t.Errorf("expected /api/v0/add, got %s", r.URL.Path)
			}

			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/form-data") {
				t.Errorf("expected multipart/form-data content type, got %s", ct)
			}

			err := r.ParseMultipartForm(32 << 20)
			if err != nil {
				t.Fatalf("failed to parse multipart form: %v", err)
			}

			file, _, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("failed to get file from form: %v", err)
			}
			defer file.Close()

			content, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}
			if string(content) != "hello world" {
				t.Errorf("expected content 'hello world', got '%s'", string(content))
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Hash": "QmAddedContent123",
				"Size": "11",
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		cid, err := svc.Add(context.Background(), strings.NewReader("hello world"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cid != "QmAddedContent123" {
			t.Errorf("expected CID QmAddedContent123, got %s", cid)
		}
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"Message": "add failed", "Code": 0}`))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		_, err := svc.Add(context.Background(), strings.NewReader("test"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error for nil reader", func(t *testing.T) {
		svc := newTestService("http://localhost:5001")
		_, err := svc.Add(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error for nil reader")
		}
		if err != ErrNilReader {
			t.Errorf("expected ErrNilReader, got %v", err)
		}
	})

	t.Run("handles empty content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Hash": "QmEmptyFile",
				"Size": "0",
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		cid, err := svc.Add(context.Background(), bytes.NewReader([]byte{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cid != "QmEmptyFile" {
			t.Errorf("expected CID QmEmptyFile, got %s", cid)
		}
	})
}

// TestKuboIPFSService_ObjectStat tests the ObjectStat method.
func TestKuboIPFSService_ObjectStat(t *testing.T) {
	t.Run("returns size for existing object", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/api/v0/object/stat") {
				t.Errorf("expected /api/v0/object/stat, got %s", r.URL.Path)
			}

			arg := r.URL.Query().Get("arg")
			if arg != "QmTest123" {
				t.Errorf("expected arg=QmTest123, got %s", arg)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Hash":           "QmTest123",
				"NumLinks":       0,
				"BlockSize":      256,
				"LinksSize":      0,
				"DataSize":       256,
				"CumulativeSize": 256,
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		size, err := svc.ObjectStat(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != 256 {
			t.Errorf("expected size 256, got %d", size)
		}
	})

	t.Run("returns error for empty CID", func(t *testing.T) {
		svc := newTestService("http://localhost:5001")
		_, err := svc.ObjectStat(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty CID")
		}
		if err != ErrEmptyCID {
			t.Errorf("expected ErrEmptyCID, got %v", err)
		}
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"Message": "object not found", "Code": 0}`))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		_, err := svc.ObjectStat(context.Background(), "QmTest123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestIPFSServiceInterface verifies KuboIPFSService implements IPFSService.
func TestIPFSServiceInterface(t *testing.T) {
	var _ IPFSService = (*KuboIPFSService)(nil)
}

// TestKuboIPFSService_RetryOnTransientFailure tests retry logic.
func TestKuboIPFSService_RetryOnTransientFailure(t *testing.T) {
	t.Run("retries on 502 and succeeds", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte(`{"Message": "bad gateway", "Code": 0}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Pins": []string{"QmTest123"},
			})
		}))
		defer server.Close()

		svc := NewKuboIPFSServiceWithConfig(server.URL, IPFSConfig{
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			RetryDelay: 10 * time.Millisecond,
		})

		err := svc.Pin(context.Background(), "QmTest123")
		if err != nil {
			t.Fatalf("unexpected error after retry: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"Message": "bad gateway", "Code": 0}`))
		}))
		defer server.Close()

		svc := NewKuboIPFSServiceWithConfig(server.URL, IPFSConfig{
			Timeout:    5 * time.Second,
			MaxRetries: 2,
			RetryDelay: 10 * time.Millisecond,
		})

		err := svc.Pin(context.Background(), "QmTest123")
		if err == nil {
			t.Fatal("expected error after max retries")
		}
		// 1 initial + 2 retries = 3 total attempts
		if attempts != 3 {
			t.Errorf("expected 3 attempts (1 + 2 retries), got %d", attempts)
		}
	})

	t.Run("does not retry on 400 client error", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"Message": "invalid CID", "Code": 0}`))
		}))
		defer server.Close()

		svc := NewKuboIPFSServiceWithConfig(server.URL, IPFSConfig{
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			RetryDelay: 10 * time.Millisecond,
		})

		err := svc.Pin(context.Background(), "InvalidCID")
		if err == nil {
			t.Fatal("expected error for bad request")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry for 400), got %d", attempts)
		}
	})
}

// TestIPFSConfig_Defaults tests default config values.
func TestIPFSConfig_Defaults(t *testing.T) {
	cfg := DefaultIPFSConfig()

	if cfg.Timeout != DefaultIPFSTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultIPFSTimeout, cfg.Timeout)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("expected default max retries %d, got %d", DefaultMaxRetries, cfg.MaxRetries)
	}
	if cfg.RetryDelay != DefaultRetryDelay {
		t.Errorf("expected default retry delay %v, got %v", DefaultRetryDelay, cfg.RetryDelay)
	}
}

// TestNodeInfo tests the NodeInfo method.
func TestNodeInfo(t *testing.T) {
	t.Run("returns node info on success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v0/id" {
				t.Errorf("expected path /api/v0/id, got %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ID":              "12D3KooWTest123",
				"AgentVersion":    "kubo/0.39.0/",
				"ProtocolVersion": "ipfs/0.1.0",
			})
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		info, err := svc.NodeInfo(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if info.PeerID != "12D3KooWTest123" {
			t.Errorf("expected peer ID '12D3KooWTest123', got '%s'", info.PeerID)
		}
		if info.AgentVersion != "kubo/0.39.0/" {
			t.Errorf("expected agent version 'kubo/0.39.0/', got '%s'", info.AgentVersion)
		}
		if info.ProtocolVersion != "ipfs/0.1.0" {
			t.Errorf("expected protocol version 'ipfs/0.1.0', got '%s'", info.ProtocolVersion)
		}
	})

	t.Run("returns error on server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		info, err := svc.NodeInfo(context.Background())

		if err == nil {
			t.Fatal("expected error")
		}
		if info != nil {
			t.Errorf("expected nil info, got %+v", info)
		}
	})

	t.Run("returns error on connection refused", func(t *testing.T) {
		svc := newTestService("http://127.0.0.1:1")
		info, err := svc.NodeInfo(context.Background())

		if err == nil {
			t.Fatal("expected error")
		}
		if info != nil {
			t.Errorf("expected nil info, got %+v", info)
		}
	})

	t.Run("returns error on invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not json"))
		}))
		defer server.Close()

		svc := newTestService(server.URL)
		info, err := svc.NodeInfo(context.Background())

		if err == nil {
			t.Fatal("expected error")
		}
		if info != nil {
			t.Errorf("expected nil info, got %+v", info)
		}
	})
}
