package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Mock IPFS Adder ---

// MockIPFSAdder implements IPFSAdder for testing.
type MockIPFSAdder struct {
	addErr    error
	returnCID string
	addedData []byte // tracks what was uploaded
}

func NewMockIPFSAdder() *MockIPFSAdder {
	return &MockIPFSAdder{
		returnCID: "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
}

func (m *MockIPFSAdder) Add(ctx context.Context, reader io.Reader) (string, error) {
	if m.addErr != nil {
		return "", m.addErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	m.addedData = data
	return m.returnCID, nil
}

// Verify MockIPFSAdder implements IPFSAdder at compile time.
var _ IPFSAdder = (*MockIPFSAdder)(nil)

// --- Test Helpers ---

func createMultipartFileRequest(t *testing.T, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/add", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// --- POST /v1/add Tests ---

func TestUploadHandler_AddContent_Success(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte("hello ipfs world")
	req := createMultipartFileRequest(t, "file", "test.txt", content)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Response is raw format (no data envelope) for IPFS API compatibility
	if resp["cid"] != "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" {
		t.Errorf("expected CID in response, got %v", resp["cid"])
	}

	// size should be reported
	size, ok := resp["size"].(float64)
	if !ok {
		t.Fatal("expected size in response")
	}
	if int(size) != len(content) {
		t.Errorf("expected size %d, got %v", len(content), size)
	}

	// Verify adder received the content
	if !bytes.Equal(adder.addedData, content) {
		t.Errorf("expected adder to receive original content")
	}
}

func TestUploadHandler_AddContent_Success_Agent(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte("agent upload test")
	req := createMultipartFileRequest(t, "file", "agent-data.json", content)
	req = addPinsAgentContext(req, "agent-test-001")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_NoAuth(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte("no auth content")
	req := createMultipartFileRequest(t, "file", "test.txt", content)
	// No auth context

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_OversizedFile(t *testing.T) {
	adder := NewMockIPFSAdder()
	maxSize := int64(100) // 100 bytes for testing
	handler := NewUploadHandler(adder, maxSize)

	// Create content larger than maxSize
	content := bytes.Repeat([]byte("x"), 200)
	req := createMultipartFileRequest(t, "file", "big.bin", content)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_MissingFileField(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	// Create multipart with wrong field name
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("wrong_field", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/add", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_NotMultipart(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	req := httptest.NewRequest(http.MethodPost, "/v1/add", strings.NewReader("plain text"))
	req.Header.Set("Content-Type", "text/plain")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_EmptyFile(t *testing.T) {
	adder := NewMockIPFSAdder()
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte{} // empty
	req := createMultipartFileRequest(t, "file", "empty.txt", content)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty file, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_IPFSError(t *testing.T) {
	adder := NewMockIPFSAdder()
	adder.addErr = errors.New("ipfs: node unreachable")
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte("some content")
	req := createMultipartFileRequest(t, "file", "test.txt", content)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadHandler_AddContent_ResponseFormat(t *testing.T) {
	adder := NewMockIPFSAdder()
	adder.returnCID = "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"
	handler := NewUploadHandler(adder, DefaultMaxUploadSize)

	content := []byte("format check content")
	req := createMultipartFileRequest(t, "file", "test.txt", content)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.AddContent(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Response is raw format (no data envelope) for IPFS API compatibility
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	// Must have cid and size
	if resp["cid"] == nil {
		t.Error("missing 'cid' field in response")
	}
	if resp["size"] == nil {
		t.Error("missing 'size' field in response")
	}

	if resp["cid"] != "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi" {
		t.Errorf("expected CIDv1, got %v", resp["cid"])
	}
}

func TestUploadHandler_DefaultMaxUploadSize(t *testing.T) {
	// Verify the default constant is 100MB
	expected := int64(100 * 1024 * 1024)
	if DefaultMaxUploadSize != expected {
		t.Errorf("expected DefaultMaxUploadSize to be %d (100MB), got %d", expected, DefaultMaxUploadSize)
	}
}
