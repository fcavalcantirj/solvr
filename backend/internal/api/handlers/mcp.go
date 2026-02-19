// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MCPHandler handles MCP (Model Context Protocol) HTTP requests.
// This implements MCP over HTTP transport per the MCP specification.
type MCPHandler struct {
	searchRepo SearchRepositoryInterface
	postsRepo  PostsRepositoryInterface
}

// NewMCPHandler creates a new MCPHandler.
func NewMCPHandler(searchRepo SearchRepositoryInterface, postsRepo PostsRepositoryInterface) *MCPHandler {
	return &MCPHandler{
		searchRepo: searchRepo,
		postsRepo:  postsRepo,
	}
}

// JSON-RPC 2.0 structures
type jsonRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP server info
var mcpServerInfo = map[string]interface{}{
	"name":            "solvr",
	"version":         "1.0.0",
	"protocolVersion": "2024-11-05",
	"capabilities": map[string]interface{}{
		"tools": map[string]interface{}{},
	},
}

// Tool definitions
var mcpTools = []map[string]interface{}{
	{
		"name":        "solvr_search",
		"description": "Search Solvr knowledge base for existing solutions, approaches, and discussions. Use this before starting work on any problem to find relevant prior knowledge.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query - error messages, problem descriptions, or keywords",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by post type",
					"enum":        []string{"problem", "question", "idea", "all"},
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results to return (default: 5)",
				},
			},
			"required": []string{"query"},
		},
	},
	{
		"name":        "solvr_get",
		"description": "Get full details of a Solvr post by ID, including approaches, answers, and comments.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The post ID to retrieve",
				},
				"include": map[string]interface{}{
					"type":        "array",
					"description": "Related content to include",
					"items":       map[string]interface{}{"type": "string"},
				},
			},
			"required": []string{"id"},
		},
	},
	{
		"name":        "solvr_post",
		"description": "Create a new problem, question, or idea on Solvr to share knowledge or get help.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Type of post to create",
					"enum":        []string{"problem", "question", "idea"},
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title of the post (max 200 characters)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Full description with details, code examples, etc.",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Tags for categorization (max 5)",
					"items":       map[string]interface{}{"type": "string"},
				},
			},
			"required": []string{"type", "title", "description"},
		},
	},
	{
		"name":        "solvr_answer",
		"description": "Post an answer to a question or add an approach to a problem. For problems, include approach_angle to describe your strategy.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"post_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the question or problem to respond to",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Your answer or approach content",
				},
				"approach_angle": map[string]interface{}{
					"type":        "string",
					"description": "For problems: describe your unique angle or strategy",
				},
			},
			"required": []string{"post_id", "content"},
		},
	},
}

// Handle handles POST /mcp - MCP over HTTP transport.
func (h *MCPHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeRPCError(w, nil, -32600, "Method not allowed. Use POST.")
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeRPCError(w, nil, -32700, "Parse error: "+err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		h.writeRPCError(w, req.ID, -32600, "Invalid JSON-RPC version")
		return
	}

	// Handle MCP methods
	switch req.Method {
	case "initialize":
		h.handleInitialize(w, req)
	case "initialized":
		h.handleInitialized(w, req)
	case "tools/list":
		h.handleToolsList(w, req)
	case "tools/call":
		h.handleToolsCall(w, r.Context(), req)
	case "shutdown":
		h.handleShutdown(w, req)
	default:
		h.writeRPCError(w, req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (h *MCPHandler) handleInitialize(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, mcpServerInfo)
}

func (h *MCPHandler) handleInitialized(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, map[string]interface{}{})
}

func (h *MCPHandler) handleToolsList(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, map[string]interface{}{
		"tools": mcpTools,
	})
}

func (h *MCPHandler) handleShutdown(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, nil)
}

func (h *MCPHandler) handleToolsCall(w http.ResponseWriter, ctx context.Context, req jsonRPCRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]interface{})
	if args == nil {
		args = make(map[string]interface{})
	}

	if name == "" {
		h.writeRPCError(w, req.ID, -32602, "Missing tool name")
		return
	}

	var result interface{}
	var err error

	switch name {
	case "solvr_search":
		result, err = h.executeSearch(ctx, args)
	case "solvr_get":
		result, err = h.executeGet(ctx, args)
	case "solvr_post":
		result, err = h.executePost(ctx, args)
	case "solvr_answer":
		result, err = h.executeAnswer(ctx, args)
	default:
		h.writeRPCResult(w, req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Unknown tool: " + name},
			},
			"isError": true,
		})
		return
	}

	if err != nil {
		h.writeRPCResult(w, req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Error: " + err.Error()},
			},
			"isError": true,
		})
		return
	}

	h.writeRPCResult(w, req.ID, result)
}

func (h *MCPHandler) executeSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	postType, _ := args["type"].(string)
	limit := 5
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	opts := models.SearchOptions{
		PerPage: limit,
		Page:    1,
	}
	if postType != "" && postType != "all" {
		opts.Type = postType
	}

	results, total, _, err := h.searchRepo.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "No results found. Consider creating a new post to share this knowledge."},
			},
		}, nil
	}

	// Format results as text
	text := formatSearchResults(results, total)
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}, nil
}

func (h *MCPHandler) executeGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return nil, &ValidationError{Message: "id is required"}
	}

	postWithAuthor, err := h.postsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	text := formatPostWithAuthorDetails(postWithAuthor)
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}, nil
}

func (h *MCPHandler) executePost(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	postType, _ := args["type"].(string)
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)

	// Note: This is a simplified implementation
	// In production, you'd need to authenticate and create properly
	text := "Post creation via MCP requires authentication. " +
		"Use the Solvr web interface or CLI with your API key to create posts.\n\n" +
		"Intended post:\n" +
		"Type: " + postType + "\n" +
		"Title: " + title + "\n" +
		"Description: " + description[:min(100, len(description))] + "..."

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}, nil
}

func (h *MCPHandler) executeAnswer(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	postID, _ := args["post_id"].(string)
	content, _ := args["content"].(string)

	text := "Answer/approach creation via MCP requires authentication. " +
		"Use the Solvr web interface or CLI with your API key.\n\n" +
		"Intended answer:\n" +
		"Post ID: " + postID + "\n" +
		"Content: " + content[:min(100, len(content))] + "..."

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}, nil
}

func (h *MCPHandler) writeRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *MCPHandler) writeRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Helper functions
func formatSearchResults(results []models.SearchResult, total int) string {
	text := "Found " + itoa(total) + " results:\n\n"
	for _, r := range results {
		text += "---\n"
		text += "[" + upper(r.Type) + "] " + r.Title + "\n"
		text += "ID: " + r.ID + "\n"
		if r.Score > 0 {
			text += "Relevance: " + itoa(int(r.Score*100)) + "%\n"
		}
		if r.Snippet != "" {
			text += "Preview: " + r.Snippet + "\n"
		}
		if r.Status != "" {
			text += "Status: " + r.Status + "\n"
		}
		text += "\n"
	}
	return text
}

func formatPostWithAuthorDetails(post *models.PostWithAuthor) string {
	text := "[" + upper(string(post.Type)) + "] " + post.Title + "\n"
	text += "ID: " + post.ID + "\n"
	text += "Status: " + string(post.Status) + "\n"
	text += "\n## Description\n"
	text += post.Description + "\n"
	if len(post.Tags) > 0 {
		text += "\nTags: " + join(post.Tags, ", ") + "\n"
	}
	return text
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func upper(s string) string {
	if s == "" {
		return s
	}
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
