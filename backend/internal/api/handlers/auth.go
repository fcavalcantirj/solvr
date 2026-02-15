package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandlers handles email/password authentication.
type AuthHandlers struct {
	config   *OAuthConfig
	userRepo UserRepositoryForAuth
}

// UserRepositoryForAuth defines required DB methods for auth operations.
type UserRepositoryForAuth interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	Create(ctx context.Context, user *models.User) (*models.User, error)
}

// RegisterRequest is the request body for registration.
type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// RegisterResponse is the success response for registration.
type RegisterResponse struct {
	AccessToken  string               `json:"access_token"`
	RefreshToken string               `json:"refresh_token"`
	User         RegisterUserResponse `json:"user"`
}

// RegisterUserResponse contains user info in registration response.
type RegisterUserResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

// Username validation regex: 3-30 characters, alphanumeric or underscores only.
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

// NewAuthHandlers creates a new AuthHandlers instance.
func NewAuthHandlers(config *OAuthConfig, userRepo UserRepositoryForAuth) *AuthHandlers {
	return &AuthHandlers{
		config:   config,
		userRepo: userRepo,
	}
}

// Register handles POST /v1/auth/register for email/password registration.
// Per PRD Task 48: Email/password registration with bcrypt.
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Parse request body
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Step 2: Validate input
	if err := validateEmail(req.Email); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_EMAIL", err.Error())
		return
	}

	if err := validatePassword(req.Password); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_PASSWORD", err.Error())
		return
	}

	if err := validateUsername(req.Username); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_USERNAME", err.Error())
		return
	}

	// Step 3: Check email uniqueness
	existingUser, err := h.userRepo.FindByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		log.Printf("FindByEmail failed: %v", err)
		writeInternalError(w, "Database error")
		return
	}
	if existingUser != nil {
		writeErrorResponse(w, http.StatusConflict, "DUPLICATE_EMAIL", "Email already registered")
		return
	}

	// Step 4: Check username uniqueness
	existingUser, err = h.userRepo.FindByUsername(ctx, req.Username)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		log.Printf("FindByUsername failed: %v", err)
		writeInternalError(w, "Database error")
		return
	}
	if existingUser != nil {
		writeErrorResponse(w, http.StatusConflict, "DUPLICATE_USERNAME", "Username already taken")
		return
	}

	// Step 5: Hash password with bcrypt
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bcrypt.GenerateFromPassword failed: %v", err)
		writeInternalError(w, "Failed to hash password")
		return
	}

	// Step 6: Create user
	user := &models.User{
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
		Role:         models.UserRoleUser,
	}

	createdUser, err := h.userRepo.Create(ctx, user)
	if err != nil {
		// Handle duplicate errors (race condition)
		if errors.Is(err, db.ErrDuplicateEmail) {
			writeErrorResponse(w, http.StatusConflict, "DUPLICATE_EMAIL", "Email already registered")
			return
		}
		if errors.Is(err, db.ErrDuplicateUsername) {
			writeErrorResponse(w, http.StatusConflict, "DUPLICATE_USERNAME", "Username already taken")
			return
		}
		log.Printf("Create user failed: %v", err)
		writeInternalError(w, "Database error")
		return
	}

	// Step 7: Generate JWT
	jwtExpiry, err := time.ParseDuration(h.config.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute // Default
	}

	accessToken, err := auth.GenerateJWT(h.config.JWTSecret, createdUser.ID, createdUser.Email, createdUser.Role, jwtExpiry)
	if err != nil {
		log.Printf("JWT generation failed: %v", err)
		writeInternalError(w, "Failed to generate access token")
		return
	}

	// Step 8: Generate refresh token
	refreshToken := auth.GenerateRefreshToken()

	// Step 9: Return success response
	resp := RegisterResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: RegisterUserResponse{
			ID:          createdUser.ID,
			Username:    createdUser.Username,
			DisplayName: createdUser.DisplayName,
			Email:       createdUser.Email,
			Role:        createdUser.Role,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// validateEmail validates email format using net/mail.ParseAddress.
func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// validatePassword validates password meets minimum requirements.
// Per PRD Task 48: Minimum 8 characters.
func validatePassword(password string) error {
	trimmed := strings.TrimSpace(password)
	if len(trimmed) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

// validateUsername validates username format.
// Per PRD Task 48: 3-30 characters, alphanumeric or underscores only.
func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username must be 3-30 characters, alphanumeric or underscores only")
	}
	return nil
}

// writeErrorResponse writes a JSON error response with given status code and error details.
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
