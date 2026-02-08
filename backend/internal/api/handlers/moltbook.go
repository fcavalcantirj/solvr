// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// MoltbookConfig contains Moltbook API configuration.
// Per SPEC.md Part 5.2: Moltbook Integration.
type MoltbookConfig struct {
	// MoltbookAPIURL is the base URL for the Moltbook API.
	MoltbookAPIURL string
}

// MoltbookAgentServiceInterface defines the interface for agent operations needed by Moltbook auth.
type MoltbookAgentServiceInterface interface {
	FindByMoltbookID(ctx context.Context, moltbookID string) (*MoltbookAgentRecord, error)
	CreateAgentFromMoltbook(ctx context.Context, data *MoltbookVerifyAgentData) (*MoltbookAgentRecord, string, error)
	GenerateNewAPIKey(ctx context.Context, agentID string) (string, error)
}

// MoltbookAgentRecord represents an agent record for Moltbook operations.
type MoltbookAgentRecord struct {
	ID               string
	MoltbookID       string
	DisplayName      string
	MoltbookVerified bool
	Reputation       int
}

// MoltbookVerifyAgentData represents data from Moltbook verification response.
type MoltbookVerifyAgentData struct {
	MoltbookID  string
	DisplayName string
	Reputation  int
	PostCount   int
}

// MoltbookHandler handles Moltbook authentication endpoints.
// Per SPEC.md Part 5.2: POST /auth/moltbook for agent authentication.
type MoltbookHandler struct {
	config       *MoltbookConfig
	agentService MoltbookAgentServiceInterface
	httpClient   *http.Client
}

// NewMoltbookHandler creates a new MoltbookHandler instance.
func NewMoltbookHandler(config *MoltbookConfig, agentService MoltbookAgentServiceInterface) *MoltbookHandler {
	return &MoltbookHandler{
		config:       config,
		agentService: agentService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewMoltbookHandlerWithDeps creates a MoltbookHandler with all dependencies for testing.
func NewMoltbookHandlerWithDeps(config *MoltbookConfig, agentService MoltbookAgentServiceInterface) *MoltbookHandler {
	return &MoltbookHandler{
		config:       config,
		agentService: agentService,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// MoltbookAuthRequest is the request body for POST /v1/auth/moltbook.
// Per SPEC.md Part 5.2.
type MoltbookAuthRequest struct {
	IdentityToken string `json:"identity_token"`
}

// MoltbookAuthResponseData is the response data for successful Moltbook auth.
type MoltbookAuthResponseData struct {
	Agent  MoltbookAuthAgentData `json:"agent"`
	APIKey string                `json:"api_key"`
}

// MoltbookAuthAgentData represents agent data in auth response.
type MoltbookAuthAgentData struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	MoltbookVerified bool   `json:"moltbook_verified"`
	ImportedReputation int  `json:"imported_reputation"`
}

// MoltbookVerifyResponse is the response from Moltbook's verify endpoint.
type MoltbookVerifyResponse struct {
	Valid bool                        `json:"valid"`
	Error string                      `json:"error,omitempty"`
	Agent *MoltbookVerifyAgentPayload `json:"agent,omitempty"`
}

// MoltbookVerifyAgentPayload is the agent data from Moltbook verification.
type MoltbookVerifyAgentPayload struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Karma       int    `json:"karma"`
	PostCount   int    `json:"post_count"`
}

// Authenticate handles POST /v1/auth/moltbook
// Verifies Moltbook identity token, creates or links agent, returns API key.
// Per SPEC.md Part 5.2: Moltbook Integration.
func (h *MoltbookHandler) Authenticate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req MoltbookAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMoltbookValidationError(w, "invalid JSON body")
		return
	}

	// Validate identity_token is present
	if req.IdentityToken == "" {
		writeMoltbookValidationError(w, "identity_token is required")
		return
	}

	// Verify token with Moltbook API
	moltbookAgent, err := h.verifyWithMoltbook(ctx, req.IdentityToken)
	if err != nil {
		if err == errMoltbookInvalidToken {
			writeMoltbookUnauthorized(w, "INVALID_MOLTBOOK_TOKEN", "Invalid Moltbook identity token")
			return
		}
		// Network or server error
		log.Printf("Moltbook API error: %v", err)
		writeMoltbookBadGateway(w, "Failed to communicate with Moltbook")
		return
	}

	// Check if agent already exists in Solvr
	var agent *MoltbookAgentRecord
	var apiKey string

	if h.agentService != nil {
		existingAgent, err := h.agentService.FindByMoltbookID(ctx, moltbookAgent.MoltbookID)
		if err != nil {
			log.Printf("Agent lookup failed: %v", err)
			writeMoltbookInternalError(w, "Failed to lookup agent")
			return
		}

		if existingAgent != nil {
			// Existing agent - generate new API key
			agent = existingAgent
			apiKey, err = h.agentService.GenerateNewAPIKey(ctx, existingAgent.ID)
			if err != nil {
				log.Printf("API key generation failed: %v", err)
				writeMoltbookInternalError(w, "Failed to generate API key")
				return
			}
		} else {
			// New agent - create from Moltbook data
			agent, apiKey, err = h.agentService.CreateAgentFromMoltbook(ctx, moltbookAgent)
			if err != nil {
				log.Printf("Agent creation failed: %v", err)
				writeMoltbookInternalError(w, "Failed to create agent")
				return
			}
		}
	} else {
		// Fallback for testing without service
		agent = &MoltbookAgentRecord{
			ID:               "mock-agent-" + moltbookAgent.MoltbookID,
			MoltbookID:       moltbookAgent.MoltbookID,
			DisplayName:      moltbookAgent.DisplayName,
			MoltbookVerified: true,
			Reputation:       moltbookAgent.Reputation,
		}
		apiKey = "solvr_" + auth.GenerateRefreshToken()[:32]
	}

	// Return agent and API key
	response := map[string]interface{}{
		"data": MoltbookAuthResponseData{
			Agent: MoltbookAuthAgentData{
				ID:               agent.ID,
				DisplayName:      agent.DisplayName,
				MoltbookVerified: true,
				ImportedReputation: moltbookAgent.Reputation,
			},
			APIKey: apiKey,
		},
	}

	writeMoltbookJSON(w, http.StatusOK, response)
}

// errMoltbookInvalidToken indicates the Moltbook token is invalid.
var errMoltbookInvalidToken = &MoltbookVerifyError{Message: "invalid token"}

// MoltbookVerifyError represents an error from Moltbook verification.
type MoltbookVerifyError struct {
	Message string
}

func (e *MoltbookVerifyError) Error() string {
	return e.Message
}

// verifyWithMoltbook verifies the identity token with Moltbook API.
func (h *MoltbookHandler) verifyWithMoltbook(ctx context.Context, token string) (*MoltbookVerifyAgentData, error) {
	// Build request to Moltbook verify endpoint
	reqBody, _ := json.Marshal(map[string]string{
		"identity_token": token,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.config.MoltbookAPIURL+"/v1/auth/verify", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle unauthorized response
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errMoltbookInvalidToken
	}

	// Handle other error responses
	if resp.StatusCode != http.StatusOK {
		return nil, &MoltbookVerifyError{Message: "moltbook API error: " + resp.Status}
	}

	// Parse response
	var verifyResp MoltbookVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return nil, err
	}

	if !verifyResp.Valid || verifyResp.Agent == nil {
		return nil, errMoltbookInvalidToken
	}

	return &MoltbookVerifyAgentData{
		MoltbookID:  verifyResp.Agent.ID,
		DisplayName: verifyResp.Agent.DisplayName,
		Reputation:  verifyResp.Agent.Karma,
		PostCount:   verifyResp.Agent.PostCount,
	}, nil
}

// Helper functions for writing responses

func writeMoltbookJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeMoltbookValidationError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "VALIDATION_ERROR",
			"message": message,
		},
	})
}

func writeMoltbookUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeMoltbookBadGateway(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "BAD_GATEWAY",
			"message": message,
		},
	})
}

func writeMoltbookInternalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": message,
		},
	})
}
