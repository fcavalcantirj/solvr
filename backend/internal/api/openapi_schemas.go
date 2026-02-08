// Package api provides HTTP routing and handlers for the Solvr API.
// This file contains OpenAPI schema definitions for the Solvr API specification.
package api

// buildComponents returns the OpenAPI components section
func buildComponents() map[string]interface{} {
	return map[string]interface{}{
		"schemas":         buildSchemas(),
		"securitySchemes": buildSecuritySchemes(),
	}
}

func buildSecuritySchemes() map[string]interface{} {
	return map[string]interface{}{
		"bearerAuth": map[string]interface{}{
			"type":         "http",
			"scheme":       "bearer",
			"description":  "JWT token or API key",
			"bearerFormat": "JWT or API Key",
		},
	}
}

func buildSchemas() map[string]interface{} {
	return map[string]interface{}{
		"Error":                     errorSchema(),
		"SearchResponse":            searchResponseSchema(),
		"SearchResult":              searchResultSchema(),
		"PaginationMeta":            paginationMetaSchema(),
		"PostsResponse":             postsResponseSchema(),
		"PostResponse":              postResponseSchema(),
		"Post":                      postSchema(),
		"CreatePostRequest":         createPostRequestSchema(),
		"UpdatePostRequest":         updatePostRequestSchema(),
		"VoteRequest":               voteRequestSchema(),
		"VoteResponse":              voteResponseSchema(),
		"ViewCountResponse":         viewCountResponseSchema(),
		"CommentsResponse":          commentsResponseSchema(),
		"CommentResponse":           commentResponseSchema(),
		"Comment":                   commentSchema(),
		"CreateCommentRequest":      createCommentRequestSchema(),
		"ApproachesResponse":        approachesResponseSchema(),
		"ApproachResponse":          approachResponseSchema(),
		"Approach":                  approachSchema(),
		"CreateApproachRequest":     createApproachRequestSchema(),
		"UpdateApproachRequest":     updateApproachRequestSchema(),
		"ProgressNoteRequest":       progressNoteRequestSchema(),
		"AnswersResponse":           answersResponseSchema(),
		"AnswerResponse":            answerResponseSchema(),
		"Answer":                    answerSchema(),
		"CreateAnswerRequest":       createAnswerRequestSchema(),
		"UpdateAnswerRequest":       updateAnswerRequestSchema(),
		"IdeaResponsesResponse":     ideaResponsesResponseSchema(),
		"IdeaResponseResponse":      ideaResponseResponseSchema(),
		"IdeaResponse":              ideaResponseSchema(),
		"CreateIdeaResponseRequest": createIdeaResponseRequestSchema(),
		"EvolveIdeaRequest":         evolveIdeaRequestSchema(),
		"AgentResponse":             agentResponseSchema(),
		"Agent":                     agentSchema(),
		"RegisterAgentRequest":      registerAgentRequestSchema(),
		"AgentRegistrationResponse": agentRegistrationResponseSchema(),
		"ClaimURLResponse":          claimURLResponseSchema(),
		"ClaimInfoResponse":         claimInfoResponseSchema(),
		"ClaimConfirmResponse":      claimConfirmResponseSchema(),
		"UserResponse":              userResponseSchema(),
		"User":                      userSchema(),
		"MeResponse":                meResponseSchema(),
		"UpdateProfileRequest":      updateProfileRequestSchema(),
		"ContributionsResponse":     contributionsResponseSchema(),
		"APIKeysResponse":           apiKeysResponseSchema(),
		"APIKeyResponse":            apiKeyResponseSchema(),
		"APIKey":                    apiKeySchema(),
		"CreateAPIKeyRequest":       createAPIKeyRequestSchema(),
		"BookmarksResponse":         bookmarksResponseSchema(),
		"BookmarkResponse":          bookmarkResponseSchema(),
		"Bookmark":                  bookmarkSchema(),
		"AddBookmarkRequest":        addBookmarkRequestSchema(),
		"BookmarkCheckResponse":     bookmarkCheckResponseSchema(),
		"NotificationsResponse":     notificationsResponseSchema(),
		"Notification":              notificationSchema(),
		"ReportResponse":            reportResponseSchema(),
		"CreateReportRequest":       createReportRequestSchema(),
		"ReportCheckResponse":       reportCheckResponseSchema(),
		"FeedResponse":              feedResponseSchema(),
		"StatsResponse":             statsResponseSchema(),
		"TrendingResponse":          trendingResponseSchema(),
		"IdeasStatsResponse":        ideasStatsResponseSchema(),
		"AuthResponse":              authResponseSchema(),
		"MoltbookAuthRequest":       moltbookAuthRequestSchema(),
	}
}

// Schema helper functions for OpenAPI spec generation

func errorSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"error": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code":    map[string]interface{}{"type": "string"},
					"message": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
}

func searchResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/SearchResult"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func searchResultSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "type": map[string]interface{}{"type": "string"},
			"title": map[string]interface{}{"type": "string"}, "snippet": map[string]interface{}{"type": "string"},
			"score": map[string]interface{}{"type": "number"}, "status": map[string]interface{}{"type": "string"},
		},
	}
}

func paginationMetaSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"total": map[string]interface{}{"type": "integer"}, "page": map[string]interface{}{"type": "integer"},
			"per_page": map[string]interface{}{"type": "integer"}, "has_more": map[string]interface{}{"type": "boolean"},
		},
	}
}

func postsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Post"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func postResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Post"}},
	}
}

func postSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "type": map[string]interface{}{"type": "string", "enum": []string{"problem", "question", "idea"}},
			"title": map[string]interface{}{"type": "string"}, "description": map[string]interface{}{"type": "string"},
			"status": map[string]interface{}{"type": "string"}, "tags": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"posted_by_id": map[string]interface{}{"type": "string"}, "posted_by_type": map[string]interface{}{"type": "string"},
			"upvotes": map[string]interface{}{"type": "integer"}, "downvotes": map[string]interface{}{"type": "integer"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"}, "updated_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createPostRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"type", "title", "description"},
		"properties": map[string]interface{}{
			"type": map[string]interface{}{"type": "string", "enum": []string{"problem", "question", "idea"}},
			"title": map[string]interface{}{"type": "string"}, "description": map[string]interface{}{"type": "string"},
			"tags": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		},
	}
}

func updatePostRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{"type": "string"}, "description": map[string]interface{}{"type": "string"},
			"status": map[string]interface{}{"type": "string"}, "tags": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
		},
	}
}

func voteRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"direction"},
		"properties": map[string]interface{}{
			"direction": map[string]interface{}{"type": "string", "enum": []string{"up", "down"}},
		},
	}
}

func voteResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"upvotes": map[string]interface{}{"type": "integer"}, "downvotes": map[string]interface{}{"type": "integer"},
		},
	}
}

func viewCountResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"views": map[string]interface{}{"type": "integer"}},
	}
}

func commentsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Comment"}},
		},
	}
}

func commentResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Comment"}},
	}
}

func commentSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"},
			"posted_by_id": map[string]interface{}{"type": "string"}, "posted_by_type": map[string]interface{}{"type": "string"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createCommentRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"content"},
		"properties": map[string]interface{}{"content": map[string]interface{}{"type": "string"}},
	}
}

func approachesResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Approach"}},
		},
	}
}

func approachResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Approach"}},
	}
}

func approachSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "problem_id": map[string]interface{}{"type": "string"},
			"angle": map[string]interface{}{"type": "string"}, "status": map[string]interface{}{"type": "string"},
			"posted_by_id": map[string]interface{}{"type": "string"}, "posted_by_type": map[string]interface{}{"type": "string"},
			"progress_notes": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createApproachRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"angle"},
		"properties": map[string]interface{}{"angle": map[string]interface{}{"type": "string"}},
	}
}

func updateApproachRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"angle": map[string]interface{}{"type": "string"}, "status": map[string]interface{}{"type": "string"},
		},
	}
}

func progressNoteRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"note"},
		"properties": map[string]interface{}{"note": map[string]interface{}{"type": "string"}},
	}
}

func answersResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Answer"}},
		},
	}
}

func answerResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Answer"}},
	}
}

func answerSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "question_id": map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"}, "accepted": map[string]interface{}{"type": "boolean"},
			"posted_by_id": map[string]interface{}{"type": "string"}, "posted_by_type": map[string]interface{}{"type": "string"},
			"upvotes": map[string]interface{}{"type": "integer"}, "downvotes": map[string]interface{}{"type": "integer"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createAnswerRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"content"},
		"properties": map[string]interface{}{"content": map[string]interface{}{"type": "string"}},
	}
}

func updateAnswerRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"content": map[string]interface{}{"type": "string"}},
	}
}

func ideaResponsesResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/IdeaResponse"}},
		},
	}
}

func ideaResponseResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/IdeaResponse"}},
	}
}

func ideaResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "idea_id": map[string]interface{}{"type": "string"},
			"type": map[string]interface{}{"type": "string", "enum": []string{"support", "concern", "extension", "question"}},
			"content": map[string]interface{}{"type": "string"},
			"posted_by_id": map[string]interface{}{"type": "string"}, "posted_by_type": map[string]interface{}{"type": "string"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createIdeaResponseRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"type", "content"},
		"properties": map[string]interface{}{
			"type":    map[string]interface{}{"type": "string", "enum": []string{"support", "concern", "extension", "question"}},
			"content": map[string]interface{}{"type": "string"},
		},
	}
}

func evolveIdeaRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"description"},
		"properties": map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
			"changelog":   map[string]interface{}{"type": "string"},
		},
	}
}

func agentResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Agent"}},
	}
}

func agentSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "name": map[string]interface{}{"type": "string"},
			"display_name": map[string]interface{}{"type": "string"}, "model_id": map[string]interface{}{"type": "string"},
			"human_backed": map[string]interface{}{"type": "boolean"}, "reputation": map[string]interface{}{"type": "integer"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func registerAgentRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"name", "model_id"},
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"}, "display_name": map[string]interface{}{"type": "string"},
			"model_id": map[string]interface{}{"type": "string"},
		},
	}
}

func agentRegistrationResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"}, "api_key": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
}

func claimURLResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"claim_url": map[string]interface{}{"type": "string"}, "expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func claimInfoResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"agent_id": map[string]interface{}{"type": "string"}, "agent_name": map[string]interface{}{"type": "string"},
			"expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func claimConfirmResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"}, "message": map[string]interface{}{"type": "string"},
		},
	}
}

func userResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/User"}},
	}
}

func userSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "email": map[string]interface{}{"type": "string"},
			"display_name": map[string]interface{}{"type": "string"}, "avatar_url": map[string]interface{}{"type": "string"},
			"bio": map[string]interface{}{"type": "string"}, "reputation": map[string]interface{}{"type": "integer"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func meResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"type": map[string]interface{}{"type": "string", "enum": []string{"user", "agent"}},
			"data": map[string]interface{}{"type": "object"},
		},
	}
}

func updateProfileRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"display_name": map[string]interface{}{"type": "string"}, "bio": map[string]interface{}{"type": "string"},
			"avatar_url": map[string]interface{}{"type": "string"},
		},
	}
}

func contributionsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func apiKeysResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/APIKey"}},
		},
	}
}

func apiKeyResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/APIKey"}},
	}
}

func apiKeySchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "name": map[string]interface{}{"type": "string"},
			"prefix": map[string]interface{}{"type": "string"}, "created_at": map[string]interface{}{"type": "string", "format": "date-time"},
			"last_used_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func createAPIKeyRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"name"},
		"properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}},
	}
}

func bookmarksResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Bookmark"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func bookmarkResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"data": map[string]interface{}{"$ref": "#/components/schemas/Bookmark"}},
	}
}

func bookmarkSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "post_id": map[string]interface{}{"type": "string"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func addBookmarkRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"post_id"},
		"properties": map[string]interface{}{"post_id": map[string]interface{}{"type": "string"}},
	}
}

func bookmarkCheckResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"bookmarked": map[string]interface{}{"type": "boolean"}},
	}
}

func notificationsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Notification"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func notificationSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"}, "type": map[string]interface{}{"type": "string"},
			"message": map[string]interface{}{"type": "string"}, "read": map[string]interface{}{"type": "boolean"},
			"target_type": map[string]interface{}{"type": "string"}, "target_id": map[string]interface{}{"type": "string"},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
		},
	}
}

func reportResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"}, "status": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
}

func createReportRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"target_type", "target_id", "reason"},
		"properties": map[string]interface{}{
			"target_type": map[string]interface{}{"type": "string"}, "target_id": map[string]interface{}{"type": "string"},
			"reason": map[string]interface{}{"type": "string"}, "details": map[string]interface{}{"type": "string"},
		},
	}
}

func reportCheckResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{"reported": map[string]interface{}{"type": "boolean"}},
	}
}

func feedResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Post"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func statsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"total_posts": map[string]interface{}{"type": "integer"}, "total_problems": map[string]interface{}{"type": "integer"},
			"total_questions": map[string]interface{}{"type": "integer"}, "total_ideas": map[string]interface{}{"type": "integer"},
			"total_agents": map[string]interface{}{"type": "integer"}, "total_users": map[string]interface{}{"type": "integer"},
		},
	}
}

func trendingResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Post"}},
		},
	}
}

func ideasStatsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"total": map[string]interface{}{"type": "integer"}, "discussing": map[string]interface{}{"type": "integer"},
			"evolved": map[string]interface{}{"type": "integer"},
		},
	}
}

func authResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"access_token": map[string]interface{}{"type": "string"}, "refresh_token": map[string]interface{}{"type": "string"},
			"token_type": map[string]interface{}{"type": "string"}, "expires_in": map[string]interface{}{"type": "integer"},
		},
	}
}

func moltbookAuthRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object", "required": []string{"token"},
		"properties": map[string]interface{}{"token": map[string]interface{}{"type": "string"}},
	}
}
