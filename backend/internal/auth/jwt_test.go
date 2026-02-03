// Package auth provides authentication utilities for the Solvr API.
package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateJWT(t *testing.T) {
	// Set up test secret
	secret := "test-secret-key-for-testing-purposes-only"

	tests := []struct {
		name    string
		userID  string
		email   string
		role    string
		wantErr bool
	}{
		{
			name:    "valid user claims",
			userID:  "550e8400-e29b-41d4-a716-446655440000",
			email:   "test@example.com",
			role:    "user",
			wantErr: false,
		},
		{
			name:    "valid admin claims",
			userID:  "550e8400-e29b-41d4-a716-446655440001",
			email:   "admin@example.com",
			role:    "admin",
			wantErr: false,
		},
		{
			name:    "empty userID should fail",
			userID:  "",
			email:   "test@example.com",
			role:    "user",
			wantErr: true,
		},
		{
			name:    "empty email should fail",
			userID:  "550e8400-e29b-41d4-a716-446655440000",
			email:   "",
			role:    "user",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(secret, tt.userID, tt.email, tt.role, 15*time.Minute)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if token == "" {
					t.Error("GenerateJWT() returned empty token")
				}
				// JWT should have 3 parts separated by dots
				parts := strings.Split(token, ".")
				if len(parts) != 3 {
					t.Errorf("GenerateJWT() token has %d parts, want 3", len(parts))
				}
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Generate a valid token first
	validToken, err := GenerateJWT(secret, "user-123", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Generate an expired token
	expiredToken, err := GenerateJWT(secret, "user-123", "test@example.com", "user", -1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate expired test token: %v", err)
	}

	tests := []struct {
		name      string
		token     string
		secret    string
		wantErr   bool
		errType   string
		wantEmail string
		wantRole  string
	}{
		{
			name:      "valid token",
			token:     validToken,
			secret:    secret,
			wantErr:   false,
			wantEmail: "test@example.com",
			wantRole:  "user",
		},
		{
			name:    "expired token",
			token:   expiredToken,
			secret:  secret,
			wantErr: true,
			errType: "TOKEN_EXPIRED",
		},
		{
			name:    "invalid signature (wrong secret)",
			token:   validToken,
			secret:  "wrong-secret",
			wantErr: true,
			errType: "INVALID_TOKEN",
		},
		{
			name:    "malformed token",
			token:   "not.a.valid.token",
			secret:  secret,
			wantErr: true,
			errType: "INVALID_TOKEN",
		},
		{
			name:    "empty token",
			token:   "",
			secret:  secret,
			wantErr: true,
			errType: "INVALID_TOKEN",
		},
		{
			name:    "tampered token",
			token:   validToken[:len(validToken)-5] + "XXXXX",
			secret:  secret,
			wantErr: true,
			errType: "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ValidateJWT(tt.secret, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err != nil && tt.errType != "" {
					authErr, ok := err.(*AuthError)
					if !ok {
						t.Errorf("ValidateJWT() expected AuthError, got %T", err)
						return
					}
					if authErr.Code != tt.errType {
						t.Errorf("ValidateJWT() error code = %v, want %v", authErr.Code, tt.errType)
					}
				}
				return
			}
			if claims == nil {
				t.Error("ValidateJWT() returned nil claims for valid token")
				return
			}
			if claims.Email != tt.wantEmail {
				t.Errorf("ValidateJWT() email = %v, want %v", claims.Email, tt.wantEmail)
			}
			if claims.Role != tt.wantRole {
				t.Errorf("ValidateJWT() role = %v, want %v", claims.Role, tt.wantRole)
			}
			if claims.UserID != "user-123" {
				t.Errorf("ValidateJWT() userID = %v, want %v", claims.UserID, "user-123")
			}
		})
	}
}

func TestGenerateJWTExpiry(t *testing.T) {
	secret := "test-secret-key-for-testing-purposes-only"

	// Generate with 15 minute expiry
	token, err := GenerateJWT(secret, "user-123", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	claims, err := ValidateJWT(secret, token)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}

	// Check expiry is roughly 15 minutes from now (allow 5 second margin)
	expectedExpiry := time.Now().Add(15 * time.Minute)
	if claims.ExpiresAt.Before(expectedExpiry.Add(-5*time.Second)) ||
		claims.ExpiresAt.After(expectedExpiry.Add(5*time.Second)) {
		t.Errorf("Token expiry %v not within expected range around %v",
			claims.ExpiresAt, expectedExpiry)
	}
}
