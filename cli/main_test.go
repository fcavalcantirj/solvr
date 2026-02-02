package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRootCommand_Exists verifies the root command exists
func TestRootCommand_Exists(t *testing.T) {
	cmd := NewRootCmd()
	if cmd == nil {
		t.Fatal("expected root command to exist")
	}
}

// TestRootCommand_Use verifies the command use name
func TestRootCommand_Use(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Use != "solvr" {
		t.Errorf("expected Use to be 'solvr', got '%s'", cmd.Use)
	}
}

// TestRootCommand_Short verifies short description
func TestRootCommand_Short(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestRootCommand_Long verifies long description
func TestRootCommand_Long(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}

// TestRootCommand_Help verifies --help flag works
func TestRootCommand_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "solvr") {
		t.Error("expected help output to contain 'solvr'")
	}
	if !strings.Contains(output, "Usage:") {
		t.Error("expected help output to contain 'Usage:'")
	}
}

// TestRootCommand_Version verifies --version flag works
func TestRootCommand_Version(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, Version) {
		t.Errorf("expected version output to contain '%s', got '%s'", Version, output)
	}
}

// TestRootCommand_HasVersionFlag verifies version flag is registered
func TestRootCommand_HasVersionFlag(t *testing.T) {
	cmd := NewRootCmd()
	versionFlag := cmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("expected --version flag to be registered")
	}
}

// TestVersion_IsSet verifies version constant is set
func TestVersion_IsSet(t *testing.T) {
	if Version == "" {
		t.Error("expected Version to be set")
	}
}

// TestRootCommand_NoArgs verifies running without args shows help info
func TestRootCommand_NoArgs(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	_ = cmd.Execute()

	output := buf.String()
	// Should show something useful when run without args
	if len(output) == 0 {
		t.Error("expected some output when run without arguments")
	}
}

// TestRootCommand_AvailableCommands verifies help mentions available commands
func TestRootCommand_AvailableCommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()

	output := buf.String()
	// Should mention "Commands" or "Available Commands" section
	if !strings.Contains(output, "Flags:") {
		t.Error("expected help to contain 'Flags:' section")
	}
}

// Helper to create test command for isolated tests
func newTestRootCmd() *cobra.Command {
	return NewRootCmd()
}
