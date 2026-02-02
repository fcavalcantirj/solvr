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
// Editor Mode Tests (split from answer_test.go)
// ==============================================================

// TestAnswerCommand_EditorFlagExists verifies --editor flag exists
func TestAnswerCommand_EditorFlagExists(t *testing.T) {
	rootCmd := NewRootCmd()
	answerCmd, _, _ := rootCmd.Find([]string{"answer"})

	flag := answerCmd.Flags().Lookup("editor")
	if flag == nil {
		t.Fatal("expected --editor flag to exist")
	}
	if flag.Shorthand != "e" {
		t.Errorf("expected -e shorthand for --editor, got '%s'", flag.Shorthand)
	}
}

// TestAnswerCommand_EditorModeOpensEditor verifies editor is opened when --editor flag is used
func TestAnswerCommand_EditorModeOpensEditor(t *testing.T) {
	// Create a mock editor script that writes predefined content to the temp file
	editorContent := "This is the editor content.\n\nIt has multiple lines."

	// Create mock server
	var receivedContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if c, ok := body["content"].(string); ok {
			receivedContent = c
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "answer_123",
				"question_id": "post_123",
				"content":     receivedContent,
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
		"answer", "post_123",
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

// TestAnswerCommand_EditorModeWithNoEDITOREnv verifies error when EDITOR not set
func TestAnswerCommand_EditorModeWithNoEDITOREnv(t *testing.T) {
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
		"answer", "post_123",
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

// TestAnswerCommand_EditorModeAbortOnEmptyContent verifies abort when editor content is empty
func TestAnswerCommand_EditorModeAbortOnEmptyContent(t *testing.T) {
	// Mock openEditor to write empty content
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return os.WriteFile(path, []byte(""), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
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
	if !strings.Contains(errStr, "empty") && !strings.Contains(errStr, "abort") && !strings.Contains(errStr, "content") {
		t.Errorf("expected error about empty content or abort, got: %s", err.Error())
	}
}

// TestAnswerCommand_EditorModeShortFlag verifies -e short flag works
func TestAnswerCommand_EditorModeShortFlag(t *testing.T) {
	editorContent := "Content via short flag"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "answer_123"},
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
		"answer", "post_123",
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

// TestAnswerCommand_EditorModeWithContentFlagIgnoresEditor verifies --content takes precedence
func TestAnswerCommand_EditorModeWithContentFlagIgnoresEditor(t *testing.T) {
	var receivedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if c, ok := body["content"].(string); ok {
			receivedContent = c
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "answer_123"},
		})
	}))
	defer server.Close()

	// Editor should not be called if --content is provided
	editorCalled := false
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		editorCalled = true
		return os.WriteFile(path, []byte("Editor content"), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "Content from flag",
		"--editor", // This should be ignored since --content is provided
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
		t.Error("editor should not be called when --content is provided")
	}
	if receivedContent != "Content from flag" {
		t.Errorf("expected 'Content from flag', got '%s'", receivedContent)
	}
}

// TestAnswerCommand_EditorModeUsesVISUALEnv verifies VISUAL env var is used
func TestAnswerCommand_EditorModeUsesVISUALEnv(t *testing.T) {
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

// TestAnswerCommand_EditorModeFallsBackToEDITOR verifies EDITOR is used if VISUAL not set
func TestAnswerCommand_EditorModeFallsBackToEDITOR(t *testing.T) {
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

// TestAnswerCommand_EditorModePreferVISUALOverEDITOR verifies VISUAL takes precedence over EDITOR
func TestAnswerCommand_EditorModePreferVISUALOverEDITOR(t *testing.T) {
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

// TestAnswerCommand_EditorModeWithWhitespaceOnlyContent verifies whitespace-only is rejected
func TestAnswerCommand_EditorModeWithWhitespaceOnlyContent(t *testing.T) {
	originalOpenEditor := openEditor
	openEditor = func(path string) error {
		return os.WriteFile(path, []byte("   \n\t\n   "), 0644)
	}
	defer func() { openEditor = originalOpenEditor }()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
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

// TestAnswerCommand_NoContentNoEditorRequiresFlag verifies appropriate error message
func TestAnswerCommand_NoContentNoEditorRequiresFlag(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither --content nor --editor is provided")
	}
	errStr := err.Error()
	// Error should mention that content or editor is required
	if !strings.Contains(errStr, "content") && !strings.Contains(errStr, "editor") {
		t.Errorf("expected error to mention content or editor, got: %s", errStr)
	}
}

// TestAnswerCommand_HelpMentionsEditor verifies help text mentions editor mode
func TestAnswerCommand_HelpMentionsEditor(t *testing.T) {
	rootCmd := NewRootCmd()
	answerCmd, _, _ := rootCmd.Find([]string{"answer"})

	helpText := answerCmd.Long

	if !strings.Contains(strings.ToLower(helpText), "editor") {
		t.Error("help should mention 'editor' mode")
	}
}
