// Package auth provides authentication utilities for the Solvr API.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for authenticated users.
type Claims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"exp"`
	IssuedAt  time.Time `json:"iat"`
}

// jwtClaims is the internal JWT claims structure that includes standard claims.
type jwtClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthError represents an authentication error with a code.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewAuthError creates a new AuthError.
func NewAuthError(code, message string) *AuthError {
	return &AuthError{Code: code, Message: message}
}

// Common error codes per SPEC.md Part 5.4
const (
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeInvalidToken  = "INVALID_TOKEN"
	ErrCodeTokenExpired  = "TOKEN_EXPIRED"
	ErrCodeInvalidAPIKey = "INVALID_API_KEY"
)

// GenerateJWT creates a new JWT token for a user.
// Returns the signed token string or an error.
func GenerateJWT(secret, userID, email, role string, expiry time.Duration) (string, error) {
	if userID == "" {
		return "", NewAuthError(ErrCodeUnauthorized, "userID is required")
	}
	if email == "" {
		return "", NewAuthError(ErrCodeUnauthorized, "email is required")
	}

	now := time.Now()
	claims := jwtClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "solvr",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// ValidateJWT validates a JWT token and returns the claims.
// Returns an AuthError with appropriate code if validation fails.
func ValidateJWT(secret, tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, NewAuthError(ErrCodeInvalidToken, "token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		// Check for specific error types
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, NewAuthError(ErrCodeTokenExpired, "token has expired")
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, NewAuthError(ErrCodeInvalidToken, "invalid token signature")
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, NewAuthError(ErrCodeInvalidToken, "malformed token")
		}
		return nil, NewAuthError(ErrCodeInvalidToken, "invalid token")
	}

	if !token.Valid {
		return nil, NewAuthError(ErrCodeInvalidToken, "invalid token")
	}

	jwtClaims, ok := token.Claims.(*jwtClaims)
	if !ok {
		return nil, NewAuthError(ErrCodeInvalidToken, "invalid token claims")
	}

	// Convert to our Claims type
	claims := &Claims{
		UserID:   jwtClaims.UserID,
		Email:    jwtClaims.Email,
		Role:     jwtClaims.Role,
		IssuedAt: jwtClaims.IssuedAt.Time,
	}

	if jwtClaims.ExpiresAt != nil {
		claims.ExpiresAt = jwtClaims.ExpiresAt.Time
	}

	return claims, nil
}
