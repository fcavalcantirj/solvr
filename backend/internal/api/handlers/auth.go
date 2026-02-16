package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	config         *OAuthConfig
	userRepo       UserRepositoryForAuth
	authMethodRepo AuthMethodRepository
}

// UserRepositoryForAuth defines required DB methods for auth operations.
type UserRepositoryForAuth interface {
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	Create(ctx context.Context, user *models.User) (*models.User, error)
	Delete(ctx context.Context, id string) error
}

// AuthMethodRepository defines required DB methods for auth method operations.
type AuthMethodRepository interface {
	Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error)
	FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*models.AuthMethod, error)
	GetEmailAuthMethod(ctx context.Context, userID string) (*models.AuthMethod, error)
	UpdateLastUsed(ctx context.Context, methodID string) error
	HasEmailAuth(ctx context.Context, userID string) (bool, error)
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

// LoginRequest is the request body for log/slogin.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the success response for log/slogin.
type LoginResponse struct {
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	User         LoginUserResponse `json:"user"`
}

// LoginUserResponse contains user info in log/slogin response.
type LoginUserResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

// Username validation regex: 3-30 characters, alphanumeric or underscores only.
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

// NewAuthHandlers creates a new AuthHandlers instance.
func NewAuthHandlers(config *OAuthConfig, userRepo UserRepositoryForAuth, authMethodRepo AuthMethodRepository) *AuthHandlers {
	return &AuthHandlers{
		config:         config,
		userRepo:       userRepo,
		authMethodRepo: authMethodRepo,
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
		slog.Error("FindByEmail failed", "error", err, "op", "Register")
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
		slog.Error("FindByUsername failed", "error", err, "op", "Register")
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
		slog.Error("bcrypt hash failed", "error", err, "op", "Register")
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
		slog.Error("user creation failed", "error", err, "op", "Register")
		writeInternalError(w, "Database error")
		return
	}

	// Step 6.5: Create auth_method entry for email/password (atomic with user creation)
	authMethod := &models.AuthMethod{
		UserID:       createdUser.ID,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
	}
	_, err = h.authMethodRepo.Create(ctx, authMethod)
	if err != nil {
		// CRITICAL: User created but auth_method failed - delete user to rollback
		slog.Error("auth_method creation failed, rolling back user creation",
			"error", err,
			"op", "Register",
			"user_id", createdUser.ID)

		// Attempt to delete the stranded user
		if deleteErr := h.userRepo.Delete(ctx, createdUser.ID); deleteErr != nil {
			slog.Error("failed to rollback user creation",
				"error", deleteErr,
				"user_id", createdUser.ID)
		}

		writeErrorResponse(w, http.StatusInternalServerError, "REGISTRATION_FAILED",
			"Failed to complete registration. Please try again.")
		return
	}

	// Step 7: Generate JWT
	jwtExpiry, err := time.ParseDuration(h.config.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute // Default
	}

	accessToken, err := auth.GenerateJWT(h.config.JWTSecret, createdUser.ID, createdUser.Email, createdUser.Role, jwtExpiry)
	if err != nil {
		slog.Error("JWT generation failed", "error", err, "op", "Register")
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

// Login handles POST /v1/auth/log/slogin for email/password authentication.
// Per PRD Task 49: Email/password log/slogin with bcrypt verification.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Parse request body
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Step 2: Validate input
	if req.Email == "" || req.Password == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}

	// Step 3: Look up user by email
	user, err := h.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		// User not found - return generic error (no email enumeration)
		if errors.Is(err, db.ErrNotFound) {
			writeErrorResponse(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
			return
		}
		// Database error
		slog.Error("FindByEmail failed", "error", err, "op", "Login")
		writeInternalError(w, "Database error")
		return
	}

	// Step 4: Query auth methods to get password hash
	authMethods, err := h.authMethodRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		slog.Error("FindByUserID failed", "error", err, "op", "Login")
		writeInternalError(w, "Failed to query auth methods")
		return
	}

	// Step 4.5: Find email auth method
	var emailMethod *models.AuthMethod
	for _, method := range authMethods {
		if method.AuthProvider == models.AuthProviderEmail {
			emailMethod = method
			break
		}
	}

	if emailMethod == nil || emailMethod.PasswordHash == "" {
		// User exists but doesn't have email/password auth
		// Find what OAuth providers they have
		oauthProviders := []string{}
		for _, method := range authMethods {
			if method.AuthProvider != models.AuthProviderEmail {
				oauthProviders = append(oauthProviders, method.AuthProvider)
			}
		}

		message := "This account uses OAuth"
		if len(oauthProviders) > 0 {
			message = fmt.Sprintf("This account uses %s. Please sign in with %s.",
				strings.Join(oauthProviders, " or "),
				oauthProviders[0])
		}

		writeErrorResponse(w, http.StatusUnauthorized, "OAUTH_ONLY_USER", message)
		return
	}

	// Step 5: Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(emailMethod.PasswordHash), []byte(req.Password)); err != nil {
		// Wrong password - return generic error (no password enumeration)
		writeErrorResponse(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}

	// Step 5.5: Update last_used_at for this auth method
	if err := h.authMethodRepo.UpdateLastUsed(ctx, emailMethod.ID); err != nil {
		// Log but don't fail login
		slog.Warn("last_used_at update failed", "error", err, "op", "Login", "method_id", emailMethod.ID)
	}

	// Step 6: Generate JWT
	jwtExpiry, err := time.ParseDuration(h.config.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute // Default
	}

	accessToken, err := auth.GenerateJWT(h.config.JWTSecret, user.ID, user.Email, user.Role, jwtExpiry)
	if err != nil {
		slog.Error("JWT generation failed", "error", err, "op", "Login")
		writeInternalError(w, "Failed to generate access token")
		return
	}

	// Step 7: Generate refresh token
	refreshToken := auth.GenerateRefreshToken()

	// Step 8: Return success response
	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: LoginUserResponse{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Email:       user.Email,
			Role:        user.Role,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
