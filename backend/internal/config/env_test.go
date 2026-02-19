// Package config provides configuration loading and validation for the Solvr API.
package config

import (
	"os"
	"testing"
)

func TestLoad_RequiredVariables(t *testing.T) {
	// Set all required environment variables
	envVars := map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
		"JWT_SECRET":   "test-secret-key-at-least-32-chars-long",
		"APP_ENV":      "development",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DatabaseURL != envVars["DATABASE_URL"] {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, envVars["DATABASE_URL"])
	}
	if cfg.JWTSecret != envVars["JWT_SECRET"] {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, envVars["JWT_SECRET"])
	}
	if cfg.AppEnv != envVars["APP_ENV"] {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, envVars["APP_ENV"])
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	// Ensure DATABASE_URL is not set
	os.Unsetenv("DATABASE_URL")
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("APP_ENV")

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when DATABASE_URL is missing")
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Unsetenv("JWT_SECRET")
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("APP_ENV")

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when JWT_SECRET is missing")
	}
}

func TestLoad_DefaultAppEnv(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("APP_ENV")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want %q (default)", cfg.AppEnv, "development")
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("PORT")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q (default)", cfg.Port, "8080")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("PORT", "9000")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9000")
	}
}

func TestLoad_OptionalOAuthVars(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("GITHUB_CLIENT_ID", "github-id")
	os.Setenv("GITHUB_CLIENT_SECRET", "github-secret")
	os.Setenv("GOOGLE_CLIENT_ID", "google-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("GITHUB_CLIENT_ID")
		os.Unsetenv("GITHUB_CLIENT_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.GitHubClientID != "github-id" {
		t.Errorf("GitHubClientID = %q, want %q", cfg.GitHubClientID, "github-id")
	}
	if cfg.GitHubClientSecret != "github-secret" {
		t.Errorf("GitHubClientSecret = %q, want %q", cfg.GitHubClientSecret, "github-secret")
	}
	if cfg.GoogleClientID != "google-id" {
		t.Errorf("GoogleClientID = %q, want %q", cfg.GoogleClientID, "google-id")
	}
	if cfg.GoogleClientSecret != "google-secret" {
		t.Errorf("GoogleClientSecret = %q, want %q", cfg.GoogleClientSecret, "google-secret")
	}
}

func TestLoad_JWTExpiry(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("JWT_EXPIRY", "30m")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("JWT_EXPIRY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.JWTExpiry != "30m" {
		t.Errorf("JWTExpiry = %q, want %q", cfg.JWTExpiry, "30m")
	}
}

func TestLoad_DefaultJWTExpiry(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("JWT_EXPIRY")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.JWTExpiry != "15m" {
		t.Errorf("JWTExpiry = %q, want %q (default)", cfg.JWTExpiry, "15m")
	}
}

func TestLoad_RateLimitDefaults(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.RateLimitAgentGeneral != 120 {
		t.Errorf("RateLimitAgentGeneral = %d, want %d", cfg.RateLimitAgentGeneral, 120)
	}
	if cfg.RateLimitAgentSearch != 60 {
		t.Errorf("RateLimitAgentSearch = %d, want %d", cfg.RateLimitAgentSearch, 60)
	}
	if cfg.RateLimitHumanGeneral != 60 {
		t.Errorf("RateLimitHumanGeneral = %d, want %d", cfg.RateLimitHumanGeneral, 60)
	}
}

func TestLoad_LogLevelDefault(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
}

func TestLoad_IsDevelopment(t *testing.T) {
	tests := []struct {
		appEnv      string
		wantIsDev   bool
		wantIsProd  bool
	}{
		{"development", true, false},
		{"production", false, true},
		{"staging", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.appEnv, func(t *testing.T) {
			os.Setenv("DATABASE_URL", "postgres://localhost/db")
			os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
			os.Setenv("APP_ENV", tt.appEnv)
			defer os.Unsetenv("DATABASE_URL")
			defer os.Unsetenv("JWT_SECRET")
			defer os.Unsetenv("APP_ENV")

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned error: %v", err)
			}

			if cfg.IsDevelopment() != tt.wantIsDev {
				t.Errorf("IsDevelopment() = %v, want %v", cfg.IsDevelopment(), tt.wantIsDev)
			}
			if cfg.IsProduction() != tt.wantIsProd {
				t.Errorf("IsProduction() = %v, want %v", cfg.IsProduction(), tt.wantIsProd)
			}
		})
	}
}

// TestLoad_JWTSecretStrength verifies JWT secret meets minimum security requirements.
// Per security audit: JWT secrets should be at least 32 bytes (256 bits) for HS256.
// This provides adequate security against brute-force attacks.
func TestLoad_JWTSecretStrength(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		wantError bool
	}{
		{
			name:      "valid 32-char secret",
			secret:    "01234567890123456789012345678901",
			wantError: false,
		},
		{
			name:      "valid long secret",
			secret:    "this-is-a-very-long-secret-key-that-is-definitely-more-than-32-characters",
			wantError: false,
		},
		{
			name:      "exactly 32 bytes",
			secret:    "exactly-32-bytes-secret-keys!!!!",
			wantError: false,
		},
		{
			name:      "too short - 31 chars",
			secret:    "0123456789012345678901234567890",
			wantError: true,
		},
		{
			name:      "too short - 10 chars",
			secret:    "short-key!",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DATABASE_URL", "postgres://localhost/db")
			os.Setenv("JWT_SECRET", tt.secret)
			defer os.Unsetenv("DATABASE_URL")
			defer os.Unsetenv("JWT_SECRET")

			_, err := Load()

			if tt.wantError {
				if err == nil {
					t.Errorf("Load() should return error for short JWT_SECRET (%d chars)", len(tt.secret))
				}
			} else {
				if err != nil {
					t.Errorf("Load() returned unexpected error: %v", err)
				}
			}
		})
	}
}

// TestJWTSecretMinLength documents the minimum required length.
// If this constant changes, security documentation must be updated.
func TestJWTSecretMinLength(t *testing.T) {
	// Document the expected minimum - 32 bytes for HS256 (256 bits)
	const expectedMinLength = 32
	if MinJWTSecretLength != expectedMinLength {
		t.Errorf("MinJWTSecretLength = %d, expected %d. "+
			"Update SECURITY.md if this change is intentional", MinJWTSecretLength, expectedMinLength)
	}
}

// TestLoad_IPFSAPIURLDefault verifies IPFS_API_URL defaults to localhost:5001.
func TestLoad_IPFSAPIURLDefault(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("IPFS_API_URL")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.IPFSAPIURL != "http://localhost:5001" {
		t.Errorf("IPFSAPIURL = %q, want %q (default)", cfg.IPFSAPIURL, "http://localhost:5001")
	}
}

// TestLoad_IPFSAPIURLCustom verifies custom IPFS_API_URL is loaded from env.
func TestLoad_IPFSAPIURLCustom(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("IPFS_API_URL", "http://65.109.134.87:5001")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("IPFS_API_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.IPFSAPIURL != "http://65.109.134.87:5001" {
		t.Errorf("IPFSAPIURL = %q, want %q", cfg.IPFSAPIURL, "http://65.109.134.87:5001")
	}
}

// TestLoad_MaxUploadSizeDefault verifies MAX_UPLOAD_SIZE_BYTES defaults to 100MB.
func TestLoad_MaxUploadSizeDefault(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("MAX_UPLOAD_SIZE_BYTES")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Default: 100MB = 100 * 1024 * 1024
	want := int64(100 * 1024 * 1024)
	if cfg.MaxUploadSizeBytes != want {
		t.Errorf("MaxUploadSizeBytes = %d, want %d (default 100MB)", cfg.MaxUploadSizeBytes, want)
	}
}

// TestLoad_MaxUploadSizeCustom verifies MAX_UPLOAD_SIZE_BYTES from env.
func TestLoad_MaxUploadSizeCustom(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("MAX_UPLOAD_SIZE_BYTES", "52428800") // 50MB
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("MAX_UPLOAD_SIZE_BYTES")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := int64(52428800)
	if cfg.MaxUploadSizeBytes != want {
		t.Errorf("MaxUploadSizeBytes = %d, want %d", cfg.MaxUploadSizeBytes, want)
	}
}

// TestLoad_EmbeddingProviderDefault verifies EMBEDDING_PROVIDER defaults to "voyage".
func TestLoad_EmbeddingProviderDefault(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("EMBEDDING_PROVIDER")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.EmbeddingProvider != "voyage" {
		t.Errorf("EmbeddingProvider = %q, want %q (default)", cfg.EmbeddingProvider, "voyage")
	}
}

// TestLoad_EmbeddingProviderOllama verifies EMBEDDING_PROVIDER can be set to "ollama".
func TestLoad_EmbeddingProviderOllama(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("EMBEDDING_PROVIDER", "ollama")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("EMBEDDING_PROVIDER")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.EmbeddingProvider != "ollama" {
		t.Errorf("EmbeddingProvider = %q, want %q", cfg.EmbeddingProvider, "ollama")
	}
}

// TestLoad_VoyageAPIKey verifies VOYAGE_API_KEY is loaded from env.
func TestLoad_VoyageAPIKey(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("VOYAGE_API_KEY", "test-voyage-key-123")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("VOYAGE_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.VoyageAPIKey != "test-voyage-key-123" {
		t.Errorf("VoyageAPIKey = %q, want %q", cfg.VoyageAPIKey, "test-voyage-key-123")
	}
}

// TestLoad_OllamaBaseURLDefault verifies OLLAMA_BASE_URL defaults to localhost.
func TestLoad_OllamaBaseURLDefault(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Unsetenv("OLLAMA_BASE_URL")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.OllamaBaseURL != "http://localhost:11434/v1" {
		t.Errorf("OllamaBaseURL = %q, want %q (default)", cfg.OllamaBaseURL, "http://localhost:11434/v1")
	}
}

// TestLoad_OllamaBaseURLCustom verifies OLLAMA_BASE_URL can be customized.
func TestLoad_OllamaBaseURLCustom(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("OLLAMA_BASE_URL", "http://gpu-server:11434/v1")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("OLLAMA_BASE_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.OllamaBaseURL != "http://gpu-server:11434/v1" {
		t.Errorf("OllamaBaseURL = %q, want %q", cfg.OllamaBaseURL, "http://gpu-server:11434/v1")
	}
}

// TestLoad_MaxUploadSizeInvalid verifies invalid MAX_UPLOAD_SIZE_BYTES falls back to default.
func TestLoad_MaxUploadSizeInvalid(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars")
	os.Setenv("MAX_UPLOAD_SIZE_BYTES", "not-a-number")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("MAX_UPLOAD_SIZE_BYTES")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := int64(100 * 1024 * 1024)
	if cfg.MaxUploadSizeBytes != want {
		t.Errorf("MaxUploadSizeBytes = %d, want %d (default on invalid input)", cfg.MaxUploadSizeBytes, want)
	}
}
