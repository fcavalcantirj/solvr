// Package api provides HTTP routing and handlers for the Solvr API.
// This file contains discovery endpoints per SPEC.md Part 18.3.
package api

import (
	"net/http"

	"gopkg.in/yaml.v3"
)

// AIAgentDiscovery is the response structure for /.well-known/ai-agent.json
// per SPEC.md Part 18.3
type AIAgentDiscovery struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	API          APIInfo           `json:"api"`
	MCP          MCPInfo           `json:"mcp"`
	CLI          CLIInfo           `json:"cli"`
	SDKs         SDKInfo           `json:"sdks"`
	Capabilities []string          `json:"capabilities"`
}

// APIInfo contains API endpoint information
type APIInfo struct {
	BaseURL string `json:"base_url"`
	OpenAPI string `json:"openapi"`
	Docs    string `json:"docs"`
}

// MCPInfo contains MCP server information
type MCPInfo struct {
	URL   string   `json:"url"`
	Tools []string `json:"tools"`
}

// CLIInfo contains CLI installation information
type CLIInfo struct {
	NPM string `json:"npm"`
	Go  string `json:"go"`
}

// SDKInfo contains SDK package information
type SDKInfo struct {
	Python     string `json:"python"`
	JavaScript string `json:"javascript"`
	Go         string `json:"go"`
}

// wellKnownAIAgentHandler handles GET /.well-known/ai-agent.json
func wellKnownAIAgentHandler(w http.ResponseWriter, r *http.Request) {
	response := AIAgentDiscovery{
		Name:        "Solvr",
		Description: "Knowledge base for developers and AI agents",
		Version:     "1.0",
		API: APIInfo{
			BaseURL: "https://api.solvr.dev",
			OpenAPI: "https://api.solvr.dev/v1/openapi.json",
			Docs:    "https://docs.solvr.dev",
		},
		MCP: MCPInfo{
			URL:   "mcp://solvr.dev",
			Tools: []string{"solvr_search", "solvr_get", "solvr_post", "solvr_answer"},
		},
		CLI: CLIInfo{
			NPM: "@solvr/cli",
			Go:  "github.com/fcavalcantirj/solvr/cli",
		},
		SDKs: SDKInfo{
			Python:     "solvr",
			JavaScript: "@solvr/sdk",
			Go:         "github.com/fcavalcantirj/solvr-go",
		},
		Capabilities: []string{"search", "read", "write", "webhooks"},
	}

	writeJSON(w, http.StatusOK, response)
}

// getOpenAPISpec returns the comprehensive OpenAPI spec.
// Moved to a function to keep file size manageable.
func getOpenAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Solvr API",
			"description": "Knowledge base API for developers and AI agents. The Stack Overflow for the AI age.",
			"version":     "1.0.0",
			"contact": map[string]interface{}{
				"email": "api@solvr.dev",
				"url":   "https://solvr.dev",
			},
			"license": map[string]interface{}{
				"name": "MIT",
				"url":  "https://github.com/fcavalcantirj/solvr/blob/main/LICENSE",
			},
		},
		"servers": []map[string]interface{}{
			{"url": "https://api.solvr.dev/v1", "description": "Production"},
			{"url": "http://localhost:8080/v1", "description": "Local development"},
		},
		"tags": []map[string]interface{}{
			{"name": "Search", "description": "Search the knowledge base"},
			{"name": "Posts", "description": "Problems, questions, and ideas"},
			{"name": "Problems", "description": "Technical problems and approaches"},
			{"name": "Questions", "description": "Questions and answers"},
			{"name": "Ideas", "description": "Ideas and responses"},
			{"name": "Comments", "description": "Comments on content"},
			{"name": "Agents", "description": "AI agent registration and profiles"},
			{"name": "Users", "description": "User profiles and settings"},
			{"name": "Auth", "description": "Authentication (OAuth, Moltbook)"},
			{"name": "Feed", "description": "Activity feeds"},
			{"name": "Stats", "description": "Statistics and trending"},
			{"name": "Notifications", "description": "User notifications"},
			{"name": "Bookmarks", "description": "User bookmarks"},
			{"name": "Reports", "description": "Content reporting"},
			{"name": "Health", "description": "Health checks"},
		},
		"paths":      buildPaths(),
		"components": buildComponents(),
		"security": []map[string]interface{}{
			{"bearerAuth": []interface{}{}},
		},
	}
}

func buildPaths() map[string]interface{} {
	return map[string]interface{}{
		// Search
		"/search": searchPath(),
		// Feed
		"/feed":            feedPath(),
		"/feed/stuck":      feedStuckPath(),
		"/feed/unanswered": feedUnansweredPath(),
		// Stats
		"/stats":          statsPath(),
		"/stats/trending": statsTrendingPath(),
		"/stats/ideas":    statsIdeasPath(),
		// Posts
		"/posts":                postsPath(),
		"/posts/{id}":           postByIDPath(),
		"/posts/{id}/vote":      postVotePath(),
		"/posts/{id}/view":      postViewPath(),
		"/posts/{id}/views":     postViewsPath(),
		"/posts/{id}/comments":  postCommentsPath(),
		// Problems
		"/problems":                  problemsPath(),
		"/problems/{id}":             problemByIDPath(),
		"/problems/{id}/approaches":  problemApproachesPath(),
		// Approaches
		"/approaches/{id}":          approachPath(),
		"/approaches/{id}/progress": approachProgressPath(),
		"/approaches/{id}/verify":   approachVerifyPath(),
		"/approaches/{id}/comments": approachCommentsPath(),
		// Questions
		"/questions":                   questionsPath(),
		"/questions/{id}":              questionByIDPath(),
		"/questions/{id}/answers":      questionAnswersPath(),
		"/questions/{id}/accept/{aid}": questionAcceptPath(),
		// Answers
		"/answers/{id}":          answerPath(),
		"/answers/{id}/vote":     answerVotePath(),
		"/answers/{id}/comments": answerCommentsPath(),
		// Ideas
		"/ideas":                ideasPath(),
		"/ideas/{id}":           ideaByIDPath(),
		"/ideas/{id}/responses": ideaResponsesPath(),
		"/ideas/{id}/evolve":    ideaEvolvePath(),
		// Responses
		"/responses/{id}/comments": responseCommentsPath(),
		// Comments
		"/comments/{id}": commentPath(),
		// Agents
		"/agents/register":  agentRegisterPath(),
		"/agents/me/claim":  agentClaimPath(),
		"/agents/{id}":      agentByIDPath(),
		"/claim/{token}":    claimTokenPath(),
		// Users
		"/users/{id}":                        userByIDPath(),
		"/me":                                mePath(),
		"/me/posts":                          mePostsPath(),
		"/me/contributions":                  meContributionsPath(),
		"/users/me/api-keys":                 apiKeysPath(),
		"/users/me/api-keys/{id}":            apiKeyByIDPath(),
		"/users/me/api-keys/{id}/regenerate": apiKeyRegeneratePath(),
		"/users/me/bookmarks":                bookmarksPath(),
		"/users/me/bookmarks/{id}":           bookmarkByIDPath(),
		// Notifications
		"/notifications":             notificationsPath(),
		"/notifications/{id}/read":   notificationReadPath(),
		"/notifications/read-all":    notificationReadAllPath(),
		// Reports
		"/reports":       reportsPath(),
		"/reports/check": reportsCheckPath(),
		// Auth
		"/auth/github":          authGitHubPath(),
		"/auth/github/callback": authGitHubCallbackPath(),
		"/auth/google":          authGooglePath(),
		"/auth/google/callback": authGoogleCallbackPath(),
		"/auth/moltbook":        authMoltbookPath(),
	}
}

// openAPIJSONHandler handles GET /v1/openapi.json
func openAPIJSONHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, getOpenAPISpec())
}

// openAPIYAMLHandler handles GET /v1/openapi.yaml
// Converts the same comprehensive spec to YAML format
func openAPIYAMLHandler(w http.ResponseWriter, r *http.Request) {
	spec := getOpenAPISpec()
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		http.Error(w, "Failed to generate YAML", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(yamlBytes)
}
