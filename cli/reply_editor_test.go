package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ==============================================================
// Editor Mode Tests (split from reply_test.go)
// ==============================================================

// TestReplyCommand_EditorFlagExists verifies --editor flag exists
func TestReplyCommand_EditorFlagExists(t *testing.T) {
	rootCmd := NewRootCmd()
	replyCmd, _, _ := rootCmd.Find([]string{"reply"})

	flag := replyCmd.Flags().Lookup("editor")
	if flag == nil {
		t.Fatal("expected --editor flag to exist")
	}
	if flag.Shorthand != "e" {
		t.Errorf("expected -e shorthand for --editor, got '%s'", flag.Shorthand)
	}
}

// TestReplyCommand_EditorModeOpensEditor verifies editor is opened when --editor flag is used
func TestReplyCommand_EditorModeOpensEditor(t *testing.T) {
	// Create a mock editor script that writes predefined content to the temp file
	editorContent := "This is the editor content.\n\nIt has multiple lines."

	// Create mock server
	var receivedContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if c, ok := body["body"].(string); ok {
			receivedContent = c
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":      "reply_123",
				"post_id": "post_123",
				"body":    receivedContent,
			},
		})
	}))
	defer server.Close()

	// Create a mock editor function for testing
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		// Write content to the temp file as if an editor did
		return os.WriteFile(path, []byte(editorContent), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--editor",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the content from editor was sent
	if receivedContent != editorContent {
		t.Errorf("expected editor content '%s', got '%s'", editorContent, receivedContent)
	}
}

// TestReplyCommand_EditorModeWithNoEDITOREnv verifies error when EDITOR not set
func TestReplyCommand_EditorModeWithNoEDITOREnv(t *testing.T) {
	// Save and unset EDITOR and VISUAL
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")
	os.Unsetenv("EDITOR")
	os.Unsetenv("VISUAL")
	defer func() {
		if origEditor != "" {
			os.Setenv("EDITOR", origEditor)
		}
		if origVisual != "" {
			os.Setenv("VISUAL", origVisual)
		}
	}()

	// Mock openEditor to simulate no editor available
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return fmt.Errorf("no editor configured: set EDITOR or VISUAL environment variable")
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--editor",
		"--api-url", "http://localhost:8080",
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no editor is configured")
	}
	errStr := strings.ToLower(err.Error())
	if !strings.Contains(errStr, "editor") {
		t.Errorf("expected error to mention 'editor', got: %s", err.Error())
	}
}

// TestReplyCommand_EditorModeAbortOnEmptyBody verifies abort when editor content is empty
func TestReplyCommand_EditorModeAbortOnEmptyBody(t *testing.T) {
	// Mock openEditor to write empty content
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return os.WriteFile(path, []byte(""), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--editor",
		"--api-url", "http://localhost:8080",
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when editor content is empty")
	}
	errStr := strings.ToLower(err.Error())
	if !strings.Contains(errStr, "empty") && !strings.Contains(errStr, "abort") && !strings.Contains(errStr, "body") {
		t.Errorf("expected error about empty body or abort, got: %s", err.Error())
	}
}

// TestReplyCommand_EditorModeShortFlag verifies -e short flag works
func TestReplyCommand_EditorModeShortFlag(t *testing.T) {
	editorContent := "Content via short flag"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "reply_123"},
		})
	}))
	defer server.Close()

	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return os.WriteFile(path, []byte(editorContent), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"-e",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with -e flag: %v", err)
	}
}

// TestReplyCommand_EditorModeWithBodyFlagIgnoresEditor verifies --body takes precedence
func TestReplyCommand_EditorModeWithBodyFlagIgnoresEditor(t *testing.T) {
	var receivedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if c, ok := body["body"].(string); ok {
			receivedContent = c
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "reply_123"},
		})
	}))
	defer server.Close()

	// Editor should not be called if --body is provided
	editorCalled := false
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		editorCalled = true
		return os.WriteFile(path, []byte("Editor content"), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "Content from flag",
		"--editor", // This should be ignored since --body is provided
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if editorCalled {
		t.Error("editor should not be called when --body is provided")
	}
	if receivedContent != "Content from flag" {
		t.Errorf("expected 'Content from flag', got '%s'", receivedContent)
	}
}

// TestReplyCommand_EditorModeUsesVISUALEnv verifies VISUAL env var is used
func TestReplyCommand_EditorModeUsesVISUALEnv(t *testing.T) {
	// This test verifies the getEditorCommand function
	// Set VISUAL and unset EDITOR
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")

	os.Setenv("VISUAL", "vim")
	os.Unsetenv("EDITOR")

	defer func() {
		if origEditor != "" {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
		if origVisual != "" {
			os.Setenv("VISUAL", origVisual)
		} else {
			os.Unsetenv("VISUAL")
		}
	}()

	editor := getEditorCommand()
	if editor != "vim" {
		t.Errorf("expected 'vim' from VISUAL, got '%s'", editor)
	}
}

// TestReplyCommand_EditorModeFallsBackToEDITOR verifies EDITOR is used if VISUAL not set
func TestReplyCommand_EditorModeFallsBackToEDITOR(t *testing.T) {
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")

	os.Unsetenv("VISUAL")
	os.Setenv("EDITOR", "nano")

	defer func() {
		if origEditor != "" {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
		if origVisual != "" {
			os.Setenv("VISUAL", origVisual)
		} else {
			os.Unsetenv("VISUAL")
		}
	}()

	editor := getEditorCommand()
	if editor != "nano" {
		t.Errorf("expected 'nano' from EDITOR, got '%s'", editor)
	}
}

// TestReplyCommand_EditorModePreferVISUALOverEDITOR verifies VISUAL takes precedence over EDITOR
func TestReplyCommand_EditorModePreferVISUALOverEDITOR(t *testing.T) {
	origEditor := os.Getenv("EDITOR")
	origVisual := os.Getenv("VISUAL")

	os.Setenv("VISUAL", "code")
	os.Setenv("EDITOR", "vim")

	defer func() {
		if origEditor != "" {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
		if origVisual != "" {
			os.Setenv("VISUAL", origVisual)
		} else {
			os.Unsetenv("VISUAL")
		}
	}()

	editor := getEditorCommand()
	if editor != "code" {
		t.Errorf("expected 'code' from VISUAL (should take precedence), got '%s'", editor)
	}
}

// TestReplyCommand_EditorModeWithWhitespaceOnlyBody verifies whitespace-only is rejected
func TestReplyCommand_EditorModeWithWhitespaceOnlyBody(t *testing.T) {
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return os.WriteFile(path, []byte("   \n\t\n   "), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--editor",
		"--api-url", "http://localhost:8080",
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when editor content is whitespace only")
	}
}

// TestReplyCommand_NoBodyNoEditorRequiresFlag verifies appropriate error message
func TestReplyCommand_NoBodyNoEditorRequiresFlag(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither --body nor --editor is provided")
	}
	errStr := err.Error()
	// Error should mention that body or editor is required
	if !strings.Contains(errStr, "body") && !strings.Contains(errStr, "editor") {
		t.Errorf("expected error to mention body or editor, got: %s", errStr)
	}
}

// TestReplyCommand_HelpMentionsEditor verifies help text mentions editor mode
func TestReplyCommand_HelpMentionsEditor(t *testing.T) {
	rootCmd := NewRootCmd()
	replyCmd, _, _ := rootCmd.Find([]string{"reply"})

	helpText := replyCmd.Long

	if !strings.Contains(strings.ToLower(helpText), "editor") {
		t.Error("help should mention 'editor' mode")
	}
}
