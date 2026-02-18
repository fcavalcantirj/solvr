// Package config provides configuration loading and startup logging for the Solvr API.
package config

import (
	"log/slog"
)

// LogStartupConfig logs the server configuration at startup.
// This provides visibility into what config the server started with.
// Sensitive values (secrets, passwords, keys) are NEVER logged.
func LogStartupConfig(logger *slog.Logger, cfg *Config, dbConnected bool) {
	// Environment
	env := "unknown"
	if cfg != nil && cfg.AppEnv != "" {
		env = cfg.AppEnv
	}

	// Database status
	dbStatus := "not connected"
	if dbConnected {
		dbStatus = "connected"
	}

	// JWT secret status (presence only, never the value)
	jwtStatus := "not configured"
	if cfg != nil && cfg.JWTSecret != "" {
		jwtStatus = "configured"
	}

	// OAuth providers
	githubOAuth := "disabled"
	if cfg != nil && cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		githubOAuth = "enabled"
	}

	googleOAuth := "disabled"
	if cfg != nil && cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		googleOAuth = "enabled"
	}

	// Log main configuration summary
	logger.Info("Solvr API Configuration",
		"environment", env,
		"database", dbStatus,
		"jwt_secret", jwtStatus,
		"github_oauth", githubOAuth,
		"google_oauth", googleOAuth,
	)

	// Log middleware status (these are always enabled in production)
	logger.Info("Middleware enabled",
		"logging", "enabled",
		"cors", "enabled",
		"rate_limiting", "enabled",
	)

	// IPFS configuration
	if cfg != nil && cfg.IPFSAPIURL != "" {
		logger.Info("IPFS Configuration",
			"ipfs_api_url", cfg.IPFSAPIURL,
			"max_upload_size_mb", cfg.MaxUploadSizeBytes/(1024*1024),
		)
	}

	// Log rate limits if configured
	if cfg != nil {
		agentGeneral := cfg.RateLimitAgentGeneral
		if agentGeneral == 0 {
			agentGeneral = 120 // default
		}
		agentSearch := cfg.RateLimitAgentSearch
		if agentSearch == 0 {
			agentSearch = 60 // default
		}
		humanGeneral := cfg.RateLimitHumanGeneral
		if humanGeneral == 0 {
			humanGeneral = 60 // default
		}

		logger.Info("Rate limits (requests/minute)",
			"agent_general", agentGeneral,
			"agent_search", agentSearch,
			"human_general", humanGeneral,
		)
	}
}
