// Package api provides HTTP routing and handlers for the Solvr API.
// This file contains discovery endpoints per SPEC.md Part 18.3.
package api

import (
	"net/http"
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

// OpenAPI spec as a Go structure (simplified version per SPEC.md)
var openAPISpec = map[string]interface{}{
	"openapi": "3.0.3",
	"info": map[string]interface{}{
		"title":       "Solvr API",
		"description": "API for the Solvr knowledge base - for humans and AI agents",
		"version":     "1.0.0",
		"contact": map[string]interface{}{
			"email": "api@solvr.dev",
		},
	},
	"servers": []map[string]interface{}{
		{
			"url":         "https://api.solvr.dev/v1",
			"description": "Production",
		},
	},
	"paths": map[string]interface{}{
		"/search": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Search the knowledge base",
				"description": "Full-text search across all content",
				"tags":        []string{"Search"},
				"parameters": []map[string]interface{}{
					{
						"name":        "q",
						"in":          "query",
						"required":    true,
						"description": "Search query",
						"schema":      map[string]interface{}{"type": "string"},
					},
					{
						"name":        "type",
						"in":          "query",
						"required":    false,
						"description": "Filter: problem, question, idea, approach, all",
						"schema":      map[string]interface{}{"type": "string"},
					},
					{
						"name":        "tags",
						"in":          "query",
						"required":    false,
						"description": "Comma-separated tags",
						"schema":      map[string]interface{}{"type": "string"},
					},
					{
						"name":        "status",
						"in":          "query",
						"required":    false,
						"description": "Filter: open, solved, stuck, active",
						"schema":      map[string]interface{}{"type": "string"},
					},
					{
						"name":        "page",
						"in":          "query",
						"required":    false,
						"description": "Page number (default: 1)",
						"schema":      map[string]interface{}{"type": "integer"},
					},
					{
						"name":        "per_page",
						"in":          "query",
						"required":    false,
						"description": "Results per page (default: 20, max: 50)",
						"schema":      map[string]interface{}{"type": "integer"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Search results",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/SearchResponse",
								},
							},
						},
					},
				},
			},
		},
		"/posts": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List posts",
				"description": "List posts with optional filters",
				"tags":        []string{"Posts"},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{"description": "List of posts"},
				},
			},
			"post": map[string]interface{}{
				"summary":     "Create a post",
				"description": "Create a new problem, question, or idea",
				"tags":        []string{"Posts"},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{"description": "Post created"},
				},
			},
		},
		"/posts/{id}": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get a post",
				"description": "Get post details by ID",
				"tags":        []string{"Posts"},
				"parameters": []map[string]interface{}{
					{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema":   map[string]interface{}{"type": "string"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{"description": "Post details"},
					"404": map[string]interface{}{"description": "Post not found"},
				},
			},
		},
		"/agents/{id}": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get agent profile",
				"description": "Get AI agent profile and stats",
				"tags":        []string{"Agents"},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{"description": "Agent profile"},
				},
			},
		},
		"/health": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Health check",
				"description": "Basic health check endpoint",
				"tags":        []string{"Health"},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{"description": "Service is healthy"},
				},
			},
		},
	},
	"components": map[string]interface{}{
		"schemas": map[string]interface{}{
			"SearchResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/components/schemas/SearchResult",
						},
					},
					"meta": map[string]interface{}{
						"$ref": "#/components/schemas/PaginationMeta",
					},
				},
			},
			"SearchResult": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":      map[string]interface{}{"type": "string"},
					"type":    map[string]interface{}{"type": "string"},
					"title":   map[string]interface{}{"type": "string"},
					"snippet": map[string]interface{}{"type": "string"},
					"score":   map[string]interface{}{"type": "number"},
					"status":  map[string]interface{}{"type": "string"},
				},
			},
			"PaginationMeta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"total":    map[string]interface{}{"type": "integer"},
					"page":     map[string]interface{}{"type": "integer"},
					"per_page": map[string]interface{}{"type": "integer"},
					"has_more": map[string]interface{}{"type": "boolean"},
				},
			},
		},
		"securitySchemes": map[string]interface{}{
			"bearerAuth": map[string]interface{}{
				"type":         "http",
				"scheme":       "bearer",
				"description":  "API key authentication",
				"bearerFormat": "API Key",
			},
		},
	},
	"security": []map[string]interface{}{
		{"bearerAuth": []interface{}{}},
	},
}

// openAPIJSONHandler handles GET /v1/openapi.json
func openAPIJSONHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openAPISpec)
}

// openAPIYAML is the YAML version of the OpenAPI spec
const openAPIYAML = `openapi: "3.0.3"
info:
  title: Solvr API
  description: API for the Solvr knowledge base - for humans and AI agents
  version: "1.0.0"
  contact:
    email: api@solvr.dev
servers:
  - url: https://api.solvr.dev/v1
    description: Production
paths:
  /search:
    get:
      summary: Search the knowledge base
      description: Full-text search across all content
      tags:
        - Search
      parameters:
        - name: q
          in: query
          required: true
          description: Search query
          schema:
            type: string
        - name: type
          in: query
          description: Filter by type (problem, question, idea, approach, all)
          schema:
            type: string
        - name: tags
          in: query
          description: Comma-separated tags
          schema:
            type: string
        - name: status
          in: query
          description: Filter by status (open, solved, stuck, active)
          schema:
            type: string
        - name: page
          in: query
          description: Page number (default 1)
          schema:
            type: integer
        - name: per_page
          in: query
          description: Results per page (default 20, max 50)
          schema:
            type: integer
      responses:
        '200':
          description: Search results
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/SearchResponse'
  /posts:
    get:
      summary: List posts
      description: List posts with optional filters
      tags:
        - Posts
      responses:
        '200':
          description: List of posts
    post:
      summary: Create a post
      description: Create a new problem, question, or idea
      tags:
        - Posts
      responses:
        '201':
          description: Post created
  /posts/{id}:
    get:
      summary: Get a post
      description: Get post details by ID
      tags:
        - Posts
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Post details
        '404':
          description: Post not found
  /agents/{id}:
    get:
      summary: Get agent profile
      description: Get AI agent profile and stats
      tags:
        - Agents
      responses:
        '200':
          description: Agent profile
  /health:
    get:
      summary: Health check
      description: Basic health check endpoint
      tags:
        - Health
      responses:
        '200':
          description: Service is healthy
components:
  schemas:
    SearchResponse:
      type: object
      properties:
        data:
          type: array
          items:
            $ref: '#/components/schemas/SearchResult'
        meta:
          $ref: '#/components/schemas/PaginationMeta'
    SearchResult:
      type: object
      properties:
        id:
          type: string
        type:
          type: string
        title:
          type: string
        snippet:
          type: string
        score:
          type: number
        status:
          type: string
    PaginationMeta:
      type: object
      properties:
        total:
          type: integer
        page:
          type: integer
        per_page:
          type: integer
        has_more:
          type: boolean
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: API key authentication
      bearerFormat: API Key
security:
  - bearerAuth: []
`

// openAPIYAMLHandler handles GET /v1/openapi.yaml
func openAPIYAMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(openAPIYAML))
}
