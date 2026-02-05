// Package services provides business logic for Solvr.
package services

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// OAuthUserServiceAdapter adapts OAuthUserService to the handler's interface.
// It converts between the service's internal types and the handler's expected types.
type OAuthUserServiceAdapter struct {
	service *OAuthUserService
}

// NewOAuthUserServiceAdapter creates a new adapter for OAuthUserService.
func NewOAuthUserServiceAdapter(service *OAuthUserService) *OAuthUserServiceAdapter {
	return &OAuthUserServiceAdapter{service: service}
}

// FindOrCreateUser adapts the service method to the handler interface.
// Converts handlers.OAuthUserInfoData to OAuthUserInfo and models.User to handlers.OAuthUserResult.
func (a *OAuthUserServiceAdapter) FindOrCreateUser(ctx context.Context, info *handlers.OAuthUserInfoData) (*handlers.OAuthUserResult, bool, error) {
	// Convert handler type to service type
	serviceInfo := &OAuthUserInfo{
		Provider:    info.Provider,
		ProviderID:  info.ProviderID,
		Email:       info.Email,
		DisplayName: info.DisplayName,
		AvatarURL:   info.AvatarURL,
	}

	// Call the service
	user, isNew, err := a.service.FindOrCreateUser(ctx, serviceInfo)
	if err != nil {
		return nil, false, err
	}

	// Convert result to handler type
	result := &handlers.OAuthUserResult{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
	}

	return result, isNew, nil
}
