// Package config provides configuration loading and validation for the Solvr API.
package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// MinJWTSecretLength is the minimum required length for JWT secrets.
	// Per security audit: HS256 requires at least 256 bits (32 bytes) for adequate security.
	// This prevents brute-force attacks on the signing key.
	MinJWTSecretLength = 32
)

// Config holds all configuration values for the application.
type Config struct {
	// App
	AppEnv string
	AppURL string
	APIURL string
	Port   string

	// Database
	DatabaseURL string

	// JWT
	JWTSecret           string
	JWTExpiry           string
	RefreshTokenExpiry  string

	// OAuth - GitHub
	GitHubClientID     string
	GitHubClientSecret string

	// OAuth - Google
	GoogleClientID     string
	GoogleClientSecret string

	// SMTP
	SMTPHost  string
	SMTPPort  string
	SMTPUser  string
	SMTPPass  string
	FromEmail string

	// LLM
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string

	// Rate Limiting
	RateLimitAgentGeneral int
	RateLimitAgentSearch  int
	RateLimitHumanGeneral int

	// Monitoring
	SentryDSN string
	LogLevel  string

	// IPFS
	IPFSAPIURL        string
	MaxUploadSizeBytes int64

	// Embeddings
	EmbeddingProvider string // "voyage" or "ollama"
	VoyageAPIKey      string
	OllamaBaseURL     string
}

// Load reads configuration from environment variables.
// Returns an error if required variables are missing.
func Load() (*Config, error) {
	cfg := &Config{}

	// Required variables
	var missing []string

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	// Validate JWT secret strength
	// Per security audit: JWT secrets must be at least 32 bytes (256 bits) for HS256.
	if len(cfg.JWTSecret) < MinJWTSecretLength {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters (got %d) for adequate security", MinJWTSecretLength, len(cfg.JWTSecret))
	}

	// App config with defaults
	cfg.AppEnv = getEnvOrDefault("APP_ENV", "development")
	cfg.AppURL = getEnvOrDefault("APP_URL", "http://localhost:3000")
	cfg.APIURL = getEnvOrDefault("API_URL", "http://localhost:8080")
	cfg.Port = getEnvOrDefault("PORT", "8080")

	// JWT with defaults
	cfg.JWTExpiry = getEnvOrDefault("JWT_EXPIRY", "15m")
	cfg.RefreshTokenExpiry = getEnvOrDefault("REFRESH_TOKEN_EXPIRY", "7d")

	// OAuth (optional)
	cfg.GitHubClientID = os.Getenv("GITHUB_CLIENT_ID")
	cfg.GitHubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")

	// SMTP (optional)
	cfg.SMTPHost = os.Getenv("SMTP_HOST")
	cfg.SMTPPort = getEnvOrDefault("SMTP_PORT", "587")
	cfg.SMTPUser = os.Getenv("SMTP_USER")
	cfg.SMTPPass = os.Getenv("SMTP_PASS")
	cfg.FromEmail = getEnvOrDefault("FROM_EMAIL", "noreply@solvr.dev")

	// LLM (optional)
	cfg.LLMProvider = os.Getenv("LLM_PROVIDER")
	cfg.LLMAPIKey = os.Getenv("LLM_API_KEY")
	cfg.LLMModel = os.Getenv("LLM_MODEL")

	// Rate limiting with defaults
	cfg.RateLimitAgentGeneral = getEnvOrDefaultInt("RATE_LIMIT_AGENT_GENERAL", 120)
	cfg.RateLimitAgentSearch = getEnvOrDefaultInt("RATE_LIMIT_AGENT_SEARCH", 60)
	cfg.RateLimitHumanGeneral = getEnvOrDefaultInt("RATE_LIMIT_HUMAN_GENERAL", 60)

	// Monitoring
	cfg.SentryDSN = os.Getenv("SENTRY_DSN")
	cfg.LogLevel = getEnvOrDefault("LOG_LEVEL", "info")

	// IPFS
	cfg.IPFSAPIURL = getEnvOrDefault("IPFS_API_URL", "http://localhost:5001")
	cfg.MaxUploadSizeBytes = getEnvOrDefaultInt64("MAX_UPLOAD_SIZE_BYTES", 100*1024*1024) // 100MB

	// Embeddings
	cfg.EmbeddingProvider = getEnvOrDefault("EMBEDDING_PROVIDER", "voyage")
	cfg.VoyageAPIKey = os.Getenv("VOYAGE_API_KEY")
	cfg.OllamaBaseURL = getEnvOrDefault("OLLAMA_BASE_URL", "http://localhost:11434/v1")

	return cfg, nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvOrDefaultInt returns the environment variable as int or a default.
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvOrDefaultInt64 returns the environment variable as int64 or a default.
func getEnvOrDefaultInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil && intVal > 0 {
			return intVal
		}
	}
	return defaultValue
}
