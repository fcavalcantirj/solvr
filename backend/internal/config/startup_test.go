// Package config provides configuration loading and startup logging for the Solvr API.
package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogStartupConfig(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		dbConnected    bool
		expectedLogs   []string
		unexpectedLogs []string
	}{
		{
			name: "full config with DB connected",
			cfg: &Config{
				AppEnv:             "production",
				JWTSecret:          "super-secret-key-that-is-long-enough-32bytes",
				GitHubClientID:     "github-id",
				GitHubClientSecret: "github-secret",
				GoogleClientID:     "google-id",
				GoogleClientSecret: "google-secret",
			},
			dbConnected: true,
			expectedLogs: []string{
				"Solvr API Configuration",
				"environment=production",
				"database=connected",
				"jwt_secret=configured",
				"github_oauth=enabled",
				"google_oauth=enabled",
			},
			unexpectedLogs: []string{
				"super-secret-key", // JWT secret value should NOT appear
				"github-secret",    // OAuth secrets should NOT appear
				"google-secret",    // OAuth secrets should NOT appear
			},
		},
		{
			name: "development with no OAuth",
			cfg: &Config{
				AppEnv:    "development",
				JWTSecret: "dev-secret-key-that-is-long-enough-32bytes",
			},
			dbConnected: false,
			expectedLogs: []string{
				"environment=development",
				`database="not connected"`, // slog quotes values with spaces
				"jwt_secret=configured",
				"github_oauth=disabled",
				"google_oauth=disabled",
			},
			unexpectedLogs: []string{
				"dev-secret-key",
			},
		},
		{
			name: "staging with only GitHub OAuth",
			cfg: &Config{
				AppEnv:             "staging",
				JWTSecret:          "staging-secret-key-that-is-long-enough",
				GitHubClientID:     "github-id",
				GitHubClientSecret: "github-secret",
			},
			dbConnected: true,
			expectedLogs: []string{
				"environment=staging",
				"database=connected",
				"github_oauth=enabled",
				"google_oauth=disabled",
			},
		},
		{
			name: "nil config",
			cfg:  nil,
			expectedLogs: []string{
				"environment=unknown",
				`database="not connected"`,    // slog quotes values with spaces
				`jwt_secret="not configured"`, // slog quotes values with spaces
				"github_oauth=disabled",
				"google_oauth=disabled",
			},
		},
		{
			name: "empty JWT secret",
			cfg: &Config{
				AppEnv:    "development",
				JWTSecret: "",
			},
			dbConnected: false,
			expectedLogs: []string{
				`jwt_secret="not configured"`, // slog quotes values with spaces
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var buf bytes.Buffer
			handler := slog.NewTextHandler(&buf, nil)
			logger := slog.New(handler)

			// Call the function under test
			LogStartupConfig(logger, tt.cfg, tt.dbConnected)

			logOutput := buf.String()

			// Check expected logs are present
			for _, expected := range tt.expectedLogs {
				if !strings.Contains(logOutput, expected) {
					t.Errorf("expected log to contain %q, got:\n%s", expected, logOutput)
				}
			}

			// Check unexpected logs are NOT present
			for _, unexpected := range tt.unexpectedLogs {
				if strings.Contains(logOutput, unexpected) {
					t.Errorf("log should NOT contain %q (sensitive data), got:\n%s", unexpected, logOutput)
				}
			}
		})
	}
}

func TestLogStartupConfig_MiddlewareEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	logger := slog.New(handler)

	cfg := &Config{
		AppEnv:    "production",
		JWTSecret: "test-jwt-secret-that-is-long-enough-32",
	}

	LogStartupConfig(logger, cfg, true)

	logOutput := buf.String()

	// Should log that middleware is enabled
	expectedMiddleware := []string{
		"logging=enabled",
		"cors=enabled",
		"rate_limiting=enabled",
	}

	for _, expected := range expectedMiddleware {
		if !strings.Contains(logOutput, expected) {
			t.Errorf("expected log to contain middleware info %q, got:\n%s", expected, logOutput)
		}
	}
}

func TestLogStartupConfig_RateLimits(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	logger := slog.New(handler)

	cfg := &Config{
		AppEnv:                "production",
		JWTSecret:             "test-jwt-secret-that-is-long-enough-32",
		RateLimitAgentGeneral: 120,
		RateLimitAgentSearch:  60,
		RateLimitHumanGeneral: 60,
	}

	LogStartupConfig(logger, cfg, true)

	logOutput := buf.String()

	// Should log rate limit configuration
	if !strings.Contains(logOutput, "agent_general=120") {
		t.Errorf("expected log to contain agent rate limit, got:\n%s", logOutput)
	}
}
