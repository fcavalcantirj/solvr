package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupPinTestConfig creates a temp config directory with an API key
func setupPinTestConfig(t *testing.T) (cleanup func()) {
	t.Helper()
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)

	configDir := filepath.Join(tempDir, ".solvr")
	os.MkdirAll(configDir, 0700)
	configPath := filepath.Join(configDir, "config")
	os.WriteFile(configPath, []byte("api-key=solvr_test_key_123\n"), 0600)

	return func() {
		os.Setenv("HOME", origHome)
	}
}

// --- pin command registration ---

func TestPinCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	pinCmd, _, err := rootCmd.Find([]string{"pin"})
	if err != nil {
		t.Fatalf("pin command not found: %v", err)
	}
	if pinCmd == nil {
		t.Fatal("pin command is nil")
	}
}

func TestPinCommand_HasSubcommands(t *testing.T) {
	rootCmd := NewRootCmd()
	subcommands := []string{"add", "ls", "status", "rm", "add-file"}

	for _, sub := range subcommands {
		t.Run(sub, func(t *testing.T) {
			_, _, err := rootCmd.Find([]string{"pin", sub})
			if err != nil {
				t.Fatalf("pin %s subcommand not found: %v", sub, err)
			}
		})
	}
}

// --- pin add ---

func TestPinAdd_CallsAPI(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/pins" {
			t.Errorf("expected /pins, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer solvr_test_key_123" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"requestid": "req-123",
			"status":    "queued",
			"created":   "2026-02-18T10:00:00Z",
			"pin": map[string]interface{}{
				"cid":  "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
				"name": "test-pin",
			},
			"delegates": []string{},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add", "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA", "--name", "test-pin", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin add failed: %v", err)
	}

	// Verify request body
	if receivedBody["cid"] != "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA" {
		t.Errorf("expected CID in body, got: %v", receivedBody["cid"])
	}
	if receivedBody["name"] != "test-pin" {
		t.Errorf("expected name in body, got: %v", receivedBody["name"])
	}

	output := buf.String()
	if !strings.Contains(output, "req-123") {
		t.Errorf("output should contain request ID, got: %s", output)
	}
	if !strings.Contains(output, "queued") {
		t.Errorf("output should contain status, got: %s", output)
	}
}

func TestPinAdd_RequiresAuth(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add", "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no API key configured")
	}
}

func TestPinAdd_RequiresCID(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no CID provided")
	}
}

func TestPinAdd_JSONOutput(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"requestid": "req-456",
			"status":    "queued",
			"created":   "2026-02-18T10:00:00Z",
			"pin": map[string]interface{}{
				"cid": "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
			},
			"delegates": []string{},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add", "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA", "--json", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin add --json failed: %v", err)
	}

	// Verify valid JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if result["requestid"] != "req-456" {
		t.Errorf("expected requestid in JSON output, got: %v", result)
	}
}

func TestPinAdd_HandlesAPIError(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "VALIDATION_ERROR",
				"message": "Invalid CID format",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add", "invalid-cid", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid CID")
	}
}

// --- pin ls ---

func TestPinLs_CallsAPI(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/pins" {
			t.Errorf("expected /pins, got %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 2,
			"results": []map[string]interface{}{
				{
					"requestid": "req-1",
					"status":    "pinned",
					"created":   "2026-02-18T10:00:00Z",
					"pin": map[string]interface{}{
						"cid":  "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
						"name": "file-1",
					},
				},
				{
					"requestid": "req-2",
					"status":    "queued",
					"created":   "2026-02-18T11:00:00Z",
					"pin": map[string]interface{}{
						"cid":  "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
						"name": "file-2",
					},
				},
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "ls", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin ls failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "req-1") {
		t.Errorf("output should contain req-1, got: %s", output)
	}
	if !strings.Contains(output, "pinned") {
		t.Errorf("output should contain status 'pinned', got: %s", output)
	}
	if !strings.Contains(output, "2 pin(s)") {
		t.Errorf("output should contain count, got: %s", output)
	}
}

func TestPinLs_StatusFilter(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count":   0,
			"results": []interface{}{},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "ls", "--status", "pinned", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin ls --status failed: %v", err)
	}

	if !strings.Contains(receivedQuery, "status=pinned") {
		t.Errorf("expected status=pinned in query, got: %s", receivedQuery)
	}
}

func TestPinLs_JSONOutput(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 1,
			"results": []map[string]interface{}{
				{
					"requestid": "req-1",
					"status":    "pinned",
					"created":   "2026-02-18T10:00:00Z",
					"pin":       map[string]interface{}{"cid": "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA"},
				},
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "ls", "--json", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin ls --json failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestPinLs_RequiresAuth(t *testing.T) {
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "ls"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no API key configured")
	}
}

// --- pin status ---

func TestPinStatus_CallsAPI(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/pins/req-123" {
			t.Errorf("expected /pins/req-123, got %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"requestid": "req-123",
			"status":    "pinned",
			"created":   "2026-02-18T10:00:00Z",
			"pin": map[string]interface{}{
				"cid":  "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
				"name": "my-file",
			},
			"delegates": []string{},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "status", "req-123", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin status failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "req-123") {
		t.Errorf("output should contain request ID, got: %s", output)
	}
	if !strings.Contains(output, "pinned") {
		t.Errorf("output should contain status, got: %s", output)
	}
	if !strings.Contains(output, "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA") {
		t.Errorf("output should contain CID, got: %s", output)
	}
}

func TestPinStatus_RequiresRequestID(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "status"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no requestid provided")
	}
}

func TestPinStatus_NotFound(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "Pin not found",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "status", "nonexistent", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for not found pin")
	}
}

// --- pin rm ---

func TestPinRm_CallsAPI(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	var receivedMethod, receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, "")
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "rm", "req-123", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin rm failed: %v", err)
	}

	if receivedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", receivedMethod)
	}
	if receivedPath != "/pins/req-123" {
		t.Errorf("expected /pins/req-123, got %s", receivedPath)
	}

	output := buf.String()
	if !strings.Contains(output, "req-123") {
		t.Errorf("output should mention the request ID, got: %s", output)
	}
}

func TestPinRm_RequiresRequestID(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "rm"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no requestid provided")
	}
}

func TestPinRm_NotFound(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "Pin not found",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "rm", "nonexistent", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for not found pin")
	}
}

// --- pin add-file ---

func TestPinAddFile_CallsUploadAndPin(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	// Create a temp file to upload
	tempFile := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(tempFile, []byte("hello world"), 0644)

	callOrder := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/add":
			callOrder = append(callOrder, "add")
			// Verify multipart
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Errorf("expected multipart/form-data, got: %s", r.Header.Get("Content-Type"))
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"cid":  "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
				"size": 11,
			})
		case r.Method == "POST" && r.URL.Path == "/pins":
			callOrder = append(callOrder, "pin")
			// Verify the CID from upload is used
			body, _ := io.ReadAll(r.Body)
			var reqBody map[string]interface{}
			json.Unmarshal(body, &reqBody)
			if reqBody["cid"] != "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA" {
				t.Errorf("expected CID from upload, got: %v", reqBody["cid"])
			}

			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"requestid": "req-789",
				"status":    "queued",
				"created":   "2026-02-18T10:00:00Z",
				"pin": map[string]interface{}{
					"cid": "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
				},
				"delegates": []string{},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add-file", tempFile, "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin add-file failed: %v", err)
	}

	// Verify: upload first, then pin
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 API calls, got %d", len(callOrder))
	}
	if callOrder[0] != "add" || callOrder[1] != "pin" {
		t.Errorf("expected [add, pin] order, got: %v", callOrder)
	}

	output := buf.String()
	if !strings.Contains(output, "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA") {
		t.Errorf("output should contain CID, got: %s", output)
	}
}

func TestPinAddFile_RequiresFilePath(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add-file"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no file path provided")
	}
}

func TestPinAddFile_FileNotFound(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add-file", "/nonexistent/file.txt", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPinAddFile_WithName(t *testing.T) {
	cleanup := setupPinTestConfig(t)
	defer cleanup()

	tempFile := filepath.Join(t.TempDir(), "data.json")
	os.WriteFile(tempFile, []byte(`{"key":"value"}`), 0644)

	var pinName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/add":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"cid":  "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA",
				"size": 15,
			})
		case r.Method == "POST" && r.URL.Path == "/pins":
			body, _ := io.ReadAll(r.Body)
			var reqBody map[string]interface{}
			json.Unmarshal(body, &reqBody)
			pinName, _ = reqBody["name"].(string)

			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"requestid": "req-999",
				"status":    "queued",
				"created":   "2026-02-18T10:00:00Z",
				"pin":       map[string]interface{}{"cid": "QmTzQ1JRkWErjk39mryYw2WVaphAZNAREyMchXzYQ7c9oA", "name": "custom-name"},
				"delegates": []string{},
			})
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"pin", "add-file", tempFile, "--name", "custom-name", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("pin add-file --name failed: %v", err)
	}

	if pinName != "custom-name" {
		t.Errorf("expected name 'custom-name', got: %s", pinName)
	}
}
