package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostCommand_InteractiveFlagExists verifies the --interactive flag exists
func TestPostCommand_InteractiveFlagExists(t *testing.T) {
	postCmd := NewPostCmd()
	flag := postCmd.Flags().Lookup("interactive")
	if flag == nil {
		t.Error("expected --interactive flag to exist")
	}
}

// TestPostCommand_InteractiveShortFlag verifies -i short flag exists
func TestPostCommand_InteractiveShortFlag(t *testing.T) {
	postCmd := NewPostCmd()
	flag := postCmd.Flags().ShorthandLookup("i")
	if flag == nil {
		t.Error("expected -i short flag to exist")
	}
}

// TestPostCommand_InteractivePromptsForType tests that interactive mode prompts for type
func TestPostCommand_InteractivePromptsForType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate user input: type=1 (question), title, description
	input := "1\nTest Title\nTest description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	output := buf.String()
	// Should prompt for type selection
	if !strings.Contains(output, "Type") && !strings.Contains(output, "type") {
		t.Errorf("expected output to prompt for type, got: %s", output)
	}
}

// TestPostCommand_InteractivePromptsForTitle tests that interactive mode prompts for title
func TestPostCommand_InteractivePromptsForTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Prompted Title",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate user input: title and description
	input := "Prompted Title\nTest description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{"question"}) // type provided as arg

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	output := buf.String()
	// Should prompt for title
	if !strings.Contains(output, "Title") && !strings.Contains(output, "title") {
		t.Errorf("expected output to prompt for title, got: %s", output)
	}
}

// TestPostCommand_InteractivePromptsForDescription tests that interactive mode prompts for description
func TestPostCommand_InteractivePromptsForDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate user input: description
	input := "Test description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.Flags().Set("title", "Test Title") // title provided via flag
	postCmd.SetArgs([]string{"question"})      // type provided as arg

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	output := buf.String()
	// Should prompt for description
	if !strings.Contains(output, "Description") && !strings.Contains(output, "description") {
		t.Errorf("expected output to prompt for description, got: %s", output)
	}
}

// TestPostCommand_InteractivePromptsForTags tests that interactive mode prompts for tags
func TestPostCommand_InteractivePromptsForTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate user input: tags only (others provided)
	input := "go,async,postgres\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.Flags().Set("title", "Test Title")
	postCmd.Flags().Set("description", "Test description that is long enough for validation")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	output := buf.String()
	// Should prompt for tags
	if !strings.Contains(output, "Tags") && !strings.Contains(output, "tags") {
		t.Errorf("expected output to prompt for tags, got: %s", output)
	}
}

// TestPostCommand_InteractiveUsesProvidedFlags tests that interactive mode skips prompts for provided flags
func TestPostCommand_InteractiveUsesProvidedFlags(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "problem",
				"title": "Flag Title",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// No input needed - all provided via flags
	input := ""
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.Flags().Set("title", "Flag Title")
	postCmd.Flags().Set("description", "Flag description that is long enough for validation")
	postCmd.Flags().Set("tags", "test,flag")
	postCmd.SetArgs([]string{"problem"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	// Verify the flags were used, not prompts
	if receivedPayload["title"] != "Flag Title" {
		t.Errorf("expected title 'Flag Title', got '%v'", receivedPayload["title"])
	}
	if receivedPayload["type"] != "problem" {
		t.Errorf("expected type 'problem', got '%v'", receivedPayload["type"])
	}
}

// TestPostCommand_InteractiveSendsCorrectPayload tests that interactive input is sent correctly
func TestPostCommand_InteractiveSendsCorrectPayload(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "idea",
				"title": "Interactive Title",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate full interactive input: type (3=idea), title, description, tags
	input := "3\nInteractive Title\nInteractive description that is long enough for validation\ngo,interactive\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	// Verify payload
	if receivedPayload["type"] != "idea" {
		t.Errorf("expected type 'idea', got '%v'", receivedPayload["type"])
	}
	if receivedPayload["title"] != "Interactive Title" {
		t.Errorf("expected title 'Interactive Title', got '%v'", receivedPayload["title"])
	}
	if receivedPayload["description"] != "Interactive description that is long enough for validation" {
		t.Errorf("expected description in payload")
	}
}

// TestPostCommand_InteractiveTypeByName tests selecting type by name instead of number
func TestPostCommand_InteractiveTypeByName(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate input with type name instead of number
	input := "question\nTest Title\nTest description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode failed: %v", err)
	}

	if receivedPayload["type"] != "question" {
		t.Errorf("expected type 'question', got '%v'", receivedPayload["type"])
	}
}

// TestPostCommand_InteractiveEmptyTagsAllowed tests that empty tags input is allowed
func TestPostCommand_InteractiveEmptyTagsAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate input with empty tags (just press enter)
	input := "1\nTest Title\nTest description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode should allow empty tags: %v", err)
	}
}

// TestPostCommand_InteractiveInvalidTypeRetry tests that invalid type prompts again
func TestPostCommand_InteractiveInvalidTypeRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate input: invalid type first, then valid
	input := "invalid\n1\nTest Title\nTest description that is long enough for validation\n\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode should retry on invalid type: %v", err)
	}

	output := buf.String()
	// Should show error message for invalid type
	if !strings.Contains(strings.ToLower(output), "invalid") {
		t.Errorf("expected output to mention 'invalid' type, got: %s", output)
	}
}

// TestPostCommand_InteractiveHelpText tests that help mentions interactive mode
func TestPostCommand_InteractiveHelpText(t *testing.T) {
	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetArgs([]string{"--help"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "interactive") && !strings.Contains(output, "-i") {
		t.Error("help should mention interactive mode or -i flag")
	}
}

// TestPostCommand_InteractiveNoArgsTriggersPrompt tests that no args with -i triggers type prompt
func TestPostCommand_InteractiveNoArgsTriggersPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "problem",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Simulate input for all fields
	input := "2\nProblem Title\nProblem description that is long enough for validation\ngo,test\n"
	stdinReader := strings.NewReader(input)

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.SetIn(stdinReader)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("interactive", "true")
	postCmd.SetArgs([]string{}) // No type arg

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("interactive mode should work without args: %v", err)
	}
}

// TestPostCommand_NonInteractiveStillRequiresFields tests that non-interactive still requires fields
func TestPostCommand_NonInteractiveStillRequiresFields(t *testing.T) {
	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	// No interactive flag, no title
	postCmd.Flags().Set("description", "Some description")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("non-interactive mode should still require title")
	}
}
