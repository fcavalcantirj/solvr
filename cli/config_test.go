package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigCommand_Exists verifies the config command exists
func TestConfigCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	configCmd, _, err := rootCmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("config command not found: %v", err)
	}
	if configCmd == nil {
		t.Fatal("expected config command to exist")
	}
}

// TestConfigCommand_Use verifies the command use name
func TestConfigCommand_Use(t *testing.T) {
	rootCmd := NewRootCmd()
	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd.Use != "config" {
		t.Errorf("expected Use to be 'config', got '%s'", configCmd.Use)
	}
}

// TestConfigCommand_Short verifies short description
func TestConfigCommand_Short(t *testing.T) {
	rootCmd := NewRootCmd()
	configCmd, _, _ := rootCmd.Find([]string{"config"})
	if configCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

// TestConfigCommand_Help verifies --help flag works
func TestConfigCommand_Help(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "config") {
		t.Error("expected help output to contain 'config'")
	}
	if !strings.Contains(output, "set") {
		t.Error("expected help output to mention 'set' subcommand")
	}
	if !strings.Contains(output, "get") {
		t.Error("expected help output to mention 'get' subcommand")
	}
}

// TestConfigSetCommand_Exists verifies the config set subcommand exists
func TestConfigSetCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	setCmd, _, err := rootCmd.Find([]string{"config", "set"})
	if err != nil {
		t.Fatalf("config set command not found: %v", err)
	}
	if setCmd == nil {
		t.Fatal("expected config set command to exist")
	}
}

// TestConfigSetCommand_Use verifies the command use name
func TestConfigSetCommand_Use(t *testing.T) {
	rootCmd := NewRootCmd()
	setCmd, _, _ := rootCmd.Find([]string{"config", "set"})
	if setCmd.Use != "set <key> <value>" {
		t.Errorf("expected Use to be 'set <key> <value>', got '%s'", setCmd.Use)
	}
}

// TestConfigGetCommand_Exists verifies the config get subcommand exists
func TestConfigGetCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	getCmd, _, err := rootCmd.Find([]string{"config", "get"})
	if err != nil {
		t.Fatalf("config get command not found: %v", err)
	}
	if getCmd == nil {
		t.Fatal("expected config get command to exist")
	}
}

// TestConfigGetCommand_Use verifies the command use name
func TestConfigGetCommand_Use(t *testing.T) {
	rootCmd := NewRootCmd()
	getCmd, _, _ := rootCmd.Find([]string{"config", "get"})
	if !strings.HasPrefix(getCmd.Use, "get") {
		t.Errorf("expected Use to start with 'get', got '%s'", getCmd.Use)
	}
}

// TestConfigSet_RequiresKeyValue verifies set needs key and value args
func TestConfigSet_RequiresKeyValue(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set"})

	err := rootCmd.Execute()
	// Should error without key and value
	if err == nil {
		t.Error("expected error when calling set without arguments")
	}
}

// TestConfigSet_SavesAPIKey verifies api-key is saved to config file
func TestConfigSet_SavesAPIKey(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "api-key", "solvr_test123"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, ".solvr", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to be created at ~/.solvr/config")
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if !strings.Contains(string(content), "api-key") {
		t.Error("expected config file to contain 'api-key'")
	}
	if !strings.Contains(string(content), "solvr_test123") {
		t.Error("expected config file to contain the API key value")
	}
}

// TestConfigGet_ShowsConfiguration verifies get shows current config
func TestConfigGet_ShowsConfiguration(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".solvr")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config")
	configContent := "api-key=solvr_mykey123\napi-url=https://api.solvr.dev\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "get"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "api-key") {
		t.Error("expected output to show api-key")
	}
	// API key should be masked for security
	if strings.Contains(output, "solvr_mykey123") {
		t.Error("expected API key to be masked, not shown in full")
	}
	if !strings.Contains(output, "api-url") {
		t.Error("expected output to show api-url")
	}
}

// TestConfigGet_SpecificKey verifies get can show specific key
func TestConfigGet_SpecificKey(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".solvr")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config")
	configContent := "api-key=solvr_mykey123\napi-url=https://api.solvr.dev\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "get", "api-url"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "https://api.solvr.dev") {
		t.Errorf("expected output to show api-url value, got: %s", output)
	}
}

// TestConfigGet_NoConfig verifies get handles missing config gracefully
func TestConfigGet_NoConfig(t *testing.T) {
	// Create temp directory without config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "get"})

	err = rootCmd.Execute()
	// Should not error, just show empty/default config
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should indicate no config or show defaults
	if len(output) == 0 {
		t.Error("expected some output even with no config")
	}
}

// TestConfigSet_SuccessMessage verifies set shows success message
func TestConfigSet_SuccessMessage(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "api-key", "solvr_newkey"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "set") || !strings.Contains(output, "api-key") {
		t.Errorf("expected success message mentioning the key, got: %s", output)
	}
}

// TestConfigSet_CreatesDirectory verifies set creates ~/.solvr directory
func TestConfigSet_CreatesDirectory(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "api-key", "solvr_test"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify directory was created with correct permissions
	configDir := filepath.Join(tmpDir, ".solvr")
	info, err := os.Stat(configDir)
	if os.IsNotExist(err) {
		t.Error("expected ~/.solvr directory to be created")
	}
	if err != nil {
		t.Fatalf("failed to stat config dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected ~/.solvr to be a directory")
	}
}

// TestGetConfigDir returns the config directory path
func TestGetConfigDir(t *testing.T) {
	dir := getConfigDir()
	if dir == "" {
		t.Error("expected config dir to be non-empty")
	}
	if !strings.HasSuffix(dir, ".solvr") {
		t.Errorf("expected config dir to end with .solvr, got: %s", dir)
	}
}

// TestGetConfigPath returns the config file path
func TestGetConfigPath(t *testing.T) {
	path := getConfigPath()
	if path == "" {
		t.Error("expected config path to be non-empty")
	}
	if !strings.HasSuffix(path, "config") {
		t.Errorf("expected config path to end with 'config', got: %s", path)
	}
}

// TestMaskAPIKey verifies API key masking
func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "****"},                           // Short keys are fully masked
		{"solvr_abc123xyz", "solvr_****3xyz"},       // Normal keys show first 6 and last 4
		{"solvr_verylongapikey", "solvr_****ikey"}, // Longer keys
	}

	for _, tc := range tests {
		result := maskAPIKey(tc.input)
		if result != tc.expected {
			t.Errorf("maskAPIKey(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

// TestConfigSet_UpdatesExistingValue verifies updating existing config value
func TestConfigSet_UpdatesExistingValue(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create existing config
	configDir := filepath.Join(tmpDir, ".solvr")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config")
	if err := os.WriteFile(configPath, []byte("api-key=old_key\n"), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"config", "set", "api-key", "new_key"})

	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the config was updated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if strings.Contains(string(content), "old_key") {
		t.Error("expected old key to be replaced")
	}
	if !strings.Contains(string(content), "new_key") {
		t.Error("expected new key to be present")
	}
}

// TestLoadConfig_SkipsComments verifies comments are ignored
func TestLoadConfig_SkipsComments(t *testing.T) {
	// Create temp directory for config
	tmpDir, err := os.MkdirTemp("", "solvr-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config with comments
	configDir := filepath.Join(tmpDir, ".solvr")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config")
	configContent := "# This is a comment\napi-key=test_key\n\n# Another comment\napi-url=https://api.solvr.dev\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Override config dir
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(config) != 2 {
		t.Errorf("expected 2 config values, got %d", len(config))
	}
	if config["api-key"] != "test_key" {
		t.Errorf("expected api-key to be 'test_key', got '%s'", config["api-key"])
	}
}
