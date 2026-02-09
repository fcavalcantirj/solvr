package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestGetAuthInfo(t *testing.T) {
	tests := []struct {
		name           string
		setupCtx       func(ctx context.Context) context.Context
		expectNil      bool
		expectType     models.AuthorType
		expectID       string
		expectRole     string
	}{
		{
			name: "agent context returns agent type",
			setupCtx: func(ctx context.Context) context.Context {
				agent := &models.Agent{ID: "agent_TestBot", DisplayName: "Test Bot"}
				return context.WithValue(ctx, auth.AgentContextKey, agent)
			},
			expectType: models.AuthorTypeAgent,
			expectID:   "agent_TestBot",
			expectRole: "",
		},
		{
			name: "claims context returns human type",
			setupCtx: func(ctx context.Context) context.Context {
				claims := &auth.Claims{UserID: "user-123", Email: "test@example.com", Role: "admin"}
				return context.WithValue(ctx, auth.ClaimsContextKey, claims)
			},
			expectType: models.AuthorTypeHuman,
			expectID:   "user-123",
			expectRole: "admin",
		},
		{
			name: "both contexts returns agent (agent-first priority)",
			setupCtx: func(ctx context.Context) context.Context {
				agent := &models.Agent{ID: "agent_Priority", DisplayName: "Priority Agent"}
				claims := &auth.Claims{UserID: "user-456", Email: "test@example.com", Role: "user"}
				ctx = context.WithValue(ctx, auth.AgentContextKey, agent)
				ctx = context.WithValue(ctx, auth.ClaimsContextKey, claims)
				return ctx
			},
			expectType: models.AuthorTypeAgent,
			expectID:   "agent_Priority",
			expectRole: "",
		},
		{
			name: "no context returns nil",
			setupCtx: func(ctx context.Context) context.Context {
				return ctx
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req = req.WithContext(tt.setupCtx(req.Context()))

			info := GetAuthInfo(req)

			if tt.expectNil {
				if info != nil {
					t.Errorf("expected nil, got %+v", info)
				}
				return
			}

			if info == nil {
				t.Fatal("expected non-nil AuthInfo, got nil")
			}

			if info.AuthorType != tt.expectType {
				t.Errorf("AuthorType = %q, want %q", info.AuthorType, tt.expectType)
			}
			if info.AuthorID != tt.expectID {
				t.Errorf("AuthorID = %q, want %q", info.AuthorID, tt.expectID)
			}
			if info.Role != tt.expectRole {
				t.Errorf("Role = %q, want %q", info.Role, tt.expectRole)
			}
		})
	}
}
