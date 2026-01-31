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
