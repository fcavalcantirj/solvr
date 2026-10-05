// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/contentgate"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// Pagination constants per SPEC.md Part 5.6
const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 50
)

// parsePaginationParams validates and parses page and per_page query parameters.
// FIX-029: Returns error for invalid values instead of silently correcting.
// Per SPEC.md Part 5.6: page >= 1, per_page >= 1 and <= 50.
func parsePaginationParams(r *http.Request) (page, perPage int, err error) {
	page = DefaultPage
	perPage = DefaultPerPage

	// Parse page parameter
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		parsedPage, parseErr := strconv.Atoi(pageStr)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("page must be a valid integer")
		}
		if parsedPage < 1 {
			return 0, 0, fmt.Errorf("page must be >= 1")
		}
		page = parsedPage
	}

	// Parse per_page parameter
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		parsedPerPage, parseErr := strconv.Atoi(perPageStr)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("per_page must be a valid integer")
		}
		if parsedPerPage < 1 {
			return 0, 0, fmt.Errorf("per_page must be >= 1")
		}
		if parsedPerPage > MaxPerPage {
			return 0, 0, fmt.Errorf("per_page must be <= %d", MaxPerPage)
		}
		perPage = parsedPerPage
	}

	return page, perPage, nil
}

// PostsRepositoryInterface defines the database operations for posts.
type PostsRepositoryInterface interface {
	// List returns posts matching the given options.
	List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)

	// FindByID returns a single post by ID.
	FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error)

	// FindByIDForViewer returns a single post by ID with the viewer's vote direction.
	// callerHuman is the caller's family human UUID for visibility scoping ("" = public-only).
	FindByIDForViewer(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error)

	// Create creates a new post and returns it.
	Create(ctx context.Context, post *models.Post) (*models.Post, error)

	// Update updates an existing post and returns it.
	Update(ctx context.Context, post *models.Post) (*models.Post, error)
	// UpdateIfUnmodified writes like Update only while the post is still at
	// expected (its updated_at); nil writes unconditionally. A post that moved
	// is a *models.VersionConflictError.
	UpdateIfUnmodified(ctx context.Context, post *models.Post, expected *time.Time) (*models.Post, error)

	// Delete soft-deletes a post by ID.
	Delete(ctx context.Context, id string) error

	// Vote records a vote on a post.
	Vote(ctx context.Context, postID, voterType, voterID, direction string) error

	// GetUserVote returns the user's current vote on a post, or nil if not voted.
	GetUserVote(ctx context.Context, postID, voterType, voterID string) (*string, error)
}

// PostsHandler handles post-related HTTP requests.
// EmbeddingServiceInterface defines the interface for generating text embeddings.
// Used by PostsHandler to generate embeddings on post creation.
type EmbeddingServiceInterface interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error)
}

// ModerationInput contains the post content to be moderated.
// Mirrors services.ModerationInput to avoid import cycle.
type ModerationInput struct {
	Title              string
	Description        string
	Tags               []string
	AuthorRecentTitles []string // the author's latest other post titles (prompt rule 7)
}

// ModerationResult contains the moderation decision.
// Mirrors services.ModerationResult to avoid import cycle.
type ModerationResult struct {
	Approved         bool
	LanguageDetected string
	RejectionReasons []string
	Confidence       float64
	Explanation      string
}

// RateLimitError is returned when a moderation API is rate limited.
type RateLimitError interface {
	error
	GetRetryAfter() time.Duration
}

// ContentModerationServiceInterface defines the interface for content moderation.
type ContentModerationServiceInterface interface {
	ModerateContent(ctx context.Context, input ModerationInput) (*ModerationResult, error)
}

// PostStatusUpdaterInterface updates post status without requiring full PostsRepositoryInterface.
type PostStatusUpdaterInterface interface {
	UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error
	// UpdateOriginalLanguage sets status='draft' and records the detected language.
	// Called when a post is rejected solely for language, queuing it for auto-translation.
	UpdateOriginalLanguage(ctx context.Context, postID, language string) error
}

// FlagCreatorInterface creates admin flags for moderation failures.
type FlagCreatorInterface interface {
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)
}

// CommentCreatorInterface creates comments for moderation results.
type CommentCreatorInterface interface {
	Create(ctx context.Context, comment *models.Comment) (*models.Comment, error)
}

// NotificationServiceInterface sends notifications on moderation decisions.
type NotificationServiceInterface interface {
	NotifyOnModerationResult(ctx context.Context, postID, postTitle, postType, authorType, authorID string, approved bool, explanation string) error
}

// PostTranslationTrigger triggers immediate translation + re-moderation
// for posts that were rejected solely for language. Called inline from
// moderatePostAsync when a language-only rejection is detected.
type PostTranslationTrigger interface {
	TranslateAndModerateAsync(postID, title, description string, tags []string, language, postType, authorType, authorID string)
}

// Default retry delays for content moderation (exponential backoff: 2s, 4s, 8s).
var defaultRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

type PostsHandler struct {
	recentTitles       AuthorTitlesReader // the author's recent titles for moderation (nil: none)
	contentGate        *contentgate.Gate  // anti-abuse checks before the insert (nil admits all)
	repo               PostsRepositoryInterface
	logger             *slog.Logger
	embeddingService   EmbeddingServiceInterface
	contentModService  ContentModerationServiceInterface
	statusUpdater      PostStatusUpdaterInterface
	flagCreator        FlagCreatorInterface
	commentRepo        CommentCreatorInterface
	notifService       NotificationServiceInterface
	translationTrigger PostTranslationTrigger
	retryDelays        []time.Duration
	roomPrivacy        RoomPrivacyChecker
}

// RoomPrivacyChecker reports whether a source room is private, gating public publication of
// outcome posts saved from private rooms.
type RoomPrivacyChecker interface {
	IsPrivateRoom(ctx context.Context, roomID string) (bool, error)
}

// SetRoomPrivacyChecker wires the private-room publish gate for outcome posts.
func (h *PostsHandler) SetRoomPrivacyChecker(c RoomPrivacyChecker) { h.roomPrivacy = c }

// NewPostsHandler creates a new PostsHandler.
func NewPostsHandler(repo PostsRepositoryInterface) *PostsHandler {
	return &PostsHandler{
		repo:        repo,
		logger:      slog.New(slog.NewJSONHandler(os.Stderr, nil)),
		retryDelays: defaultRetryDelays,
	}
}

// SetLogger sets a custom logger for the handler.
// This is useful for testing or custom logging configurations.
func (h *PostsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// SetEmbeddingService sets the embedding service for generating post embeddings.
// When set, post creation will generate and store embeddings for semantic search.
func (h *PostsHandler) SetEmbeddingService(svc EmbeddingServiceInterface) {
	h.embeddingService = svc
}

// SetContentModerationService sets the content moderation service.
// When set, post creation triggers async moderation via Groq.
func (h *PostsHandler) SetContentModerationService(svc ContentModerationServiceInterface) {
	h.contentModService = svc
}

// SetPostStatusUpdater sets the post status updater for async moderation.
func (h *PostsHandler) SetPostStatusUpdater(updater PostStatusUpdaterInterface) {
	h.statusUpdater = updater
}

// SetFlagCreator sets the flag creator for moderation failure reporting.
func (h *PostsHandler) SetFlagCreator(creator FlagCreatorInterface) {
	h.flagCreator = creator
}

// SetCommentRepo sets the comment repository for creating moderation comments.
func (h *PostsHandler) SetCommentRepo(repo CommentCreatorInterface) {
	h.commentRepo = repo
}

// SetNotificationService sets the notification service for moderation notifications.
func (h *PostsHandler) SetNotificationService(svc NotificationServiceInterface) {
	h.notifService = svc
}

// SetTranslationTrigger sets the inline translation trigger.
// When set, language-only rejections trigger immediate translation
// instead of waiting for the hourly sweep.
func (h *PostsHandler) SetTranslationTrigger(trigger PostTranslationTrigger) {
	h.translationTrigger = trigger
}

// SetRetryDelays overrides retry delays (useful for testing).
func (h *PostsHandler) SetRetryDelays(delays []time.Duration) {
	h.retryDelays = delays
}

// TriggerModerationAsync implements jobs.PostModerationTrigger.
// Fires off moderatePostAsync in a goroutine so the translation job can trigger re-moderation.
func (h *PostsHandler) TriggerAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	if h.contentModService == nil {
		return
	}
	go h.moderatePostAsync(postID, title, description, tags, postType, authorType, authorID)
}

// CreatePostRequest is the request body for creating a post.
type CreatePostRequest struct {
	Type         string   `json:"type"` // "post" or omitted; anything else is LEGACY_FIELD_RETIRED (idx 68)
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Content      string   `json:"content"` // Fallback for description (agents often send "content")
	Tags         []string `json:"tags,omitempty"`
	Visibility   string   `json:"visibility,omitempty"`     // "public" (default) or "family" (BART-151)
	SourceRoomID *string  `json:"source_room_id,omitempty"` // Optional room provenance (BART-583)
}

// UpdatePostRequest is the request body for updating a post.
type UpdatePostRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Status      *string  `json:"status,omitempty"`
}

// VoteRequest is the request body for voting.
type VoteRequest struct {
	Direction string `json:"direction"` // "up" or "down"
}

// PostsListResponse is the response for listing posts.
type PostsListResponse struct {
	Data []models.PostWithAuthor `json:"data"`
	Meta PostsListMeta           `json:"meta"`
}

// PostsListMeta contains metadata for list responses. TotalPages is the number of pages at
// the per_page asked for (SPEC.md 27.2): 0 for an empty list.
type PostsListMeta struct {
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// PostResponse is the response for a single post.
type PostResponse struct {
	Data models.PostWithAuthor `json:"data"`
}

// List handles GET /v1/posts - list posts.
func (h *PostsHandler) List(w http.ResponseWriter, r *http.Request) {
	opts, err := parsePostListOptions(r)
	if err != nil {
		writeFilterError(w, err)
		return
	}

	// Execute query
	posts, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		ctx := response.LogContext{
			Operation: "List",
			Resource:  "posts",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list posts", err, ctx, h.logger)
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := PostsListResponse{
		Data: posts,
		Meta: PostsListMeta{
			Total:      total,
			Page:       opts.Page,
			PerPage:    opts.PerPage,
			TotalPages: (total + opts.PerPage - 1) / opts.PerPage,
			HasMore:    hasMore,
		},
	}

	writePostsJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/posts/:id - get a single post.
func (h *PostsHandler) Get(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Use FindByIDForViewer when authenticated to include user_vote
	var post *models.PostWithAuthor
	var err error
	authInfo := GetAuthInfo(r)
	if authInfo != nil {
		post, err = h.repo.FindByIDForViewer(r.Context(), postID, authInfo.AuthorType, authInfo.AuthorID, callerHumanID(r))
	} else {
		post, err = h.repo.FindByID(r.Context(), postID)
	}
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Check if deleted
	if post.DeletedAt != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}

	// Server-side swap: if viewer is the author (or the human owner of the agent author)
	// and post was translated, show original language content in title/description fields.
	if authInfo != nil && post.OriginalTitle != "" {
		isAuthor := authInfo.AuthorType == post.PostedByType &&
			authInfo.AuthorID == post.PostedByID
		isAgentOwner := authInfo.AuthorType == models.AuthorTypeHuman &&
			post.PostedByType == models.AuthorTypeAgent &&
			post.AgentHumanID != "" &&
			post.AgentHumanID == authInfo.AuthorID

		if isAuthor || isAgentOwner {
			post.Title, post.OriginalTitle = post.OriginalTitle, post.Title
			post.Description, post.OriginalDescription = post.OriginalDescription, post.Description
		}
	}

	// Expose the version validator so a client can send it back as an
	// If-Match precondition on a later edit (idx 73 step 5).
	w.Header().Set("ETag", postETag(post.UpdatedAt))
	writePostsJSON(w, http.StatusOK, PostResponse{Data: *post})
}

// Create handles POST /v1/posts - create a new post.
// Per SPEC.md Part 1.4 and FIX-003: Both humans (JWT) and AI agents (API key) can create posts.
func (h *PostsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreatePostRequest
	retiredField, err := decodePostBody(r, &req)
	if err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Type is optional (BART-583) and, when sent, must be "post": the legacy types and their
	// problem-only fields were retired (idx 68) and are refused, never silently dropped.
	postType := models.PostTypePost
	if models.IsRetiredPostType(models.PostType(req.Type)) {
		writeLegacyFieldRetired(w, "type", req.Type, legacyTypeInstead)
		return
	}
	if req.Type != "" && !models.IsValidPostType(models.PostType(req.Type)) {
		writePostsError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be post or omitted")
		return
	}
	if retiredField != "" {
		writeLegacyFieldRetired(w, retiredField, "", legacyProblemFieldInstead)
		return
	}

	// Validate title
	if req.Title == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}

	// Content → Description fallback (agents often send "content" instead of "description")
	if req.Description == "" && req.Content != "" {
		req.Description = req.Content
	}

	// Validate description
	if req.Description == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description is required")
		return
	}
	if len(req.Description) < 50 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
		return
	}
	if len(req.Description) > models.MaxPostDescriptionLength {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("description must be at most %d characters", models.MaxPostDescriptionLength))
		return
	}

	// Validate tags
	if len(req.Tags) > models.MaxTagsPerPost {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
		return
	}

	// Visibility (BART-151): default "public". A "family" post is owned by the author's
	// human and visible only to that family (the human + agents sharing its human_id).
	visibility := models.VisibilityPublic
	switch req.Visibility {
	case "", models.VisibilityPublic:
		// public
	case models.VisibilityFamily:
		visibility = models.VisibilityFamily
	default:
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "visibility must be 'public' or 'family'")
		return
	}
	// Derive the owning human for family scoping: claimed agent → its human_id; human → user id.
	var ownerHumanID *string
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		ownerHumanID = agent.HumanID // nil for an unclaimed agent
	} else if authInfo.AuthorType == models.AuthorTypeHuman {
		id := authInfo.AuthorID
		ownerHumanID = &id
	}
	if visibility == models.VisibilityFamily && ownerHumanID == nil {
		writePostsError(w, http.StatusBadRequest, "UNCLAIMED_AGENT",
			"claim your agent to a human before creating family-private posts")
		return
	}

	// BART-154: family posts skip moderation — created open (instant read-your-write),
	// never sent to the moderator. Public (and any non-family) posts start pending_review
	// and go through async moderation below.
	initialStatus := models.PostStatusPendingReview
	if visibility == models.VisibilityFamily {
		initialStatus = models.PostStatusOpen
	}

	if refuseContent(w, h.contentGate.CheckPost(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, req.Title)) {
		return
	}

	// Canonical publication/moderation states derived from the initial status (BART-583).
	// The request never carries moderation_state, so an author cannot self-approve: a
	// public post starts published-pending and is not publicly eligible until moderated.
	pubState, modState := models.DeriveStates(initialStatus)

	// Create post with author info from authentication
	post := &models.Post{
		Type:             postType,
		Title:            req.Title,
		Description:      req.Description,
		Tags:             req.Tags,
		PostedByType:     authInfo.AuthorType,
		PostedByID:       authInfo.AuthorID,
		Status:           initialStatus,
		PublicationState: pubState,
		ModerationState:  modState,
		SourceRoomID:     req.SourceRoomID,
		Visibility:       visibility,
		OwnerHumanID:     ownerHumanID,
	}

	// Synchronous embedding adds ~50-100ms latency but ensures post is immediately searchable
	if h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := post.Title + " " + post.Description
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to generate embedding for post", "error", embedErr)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			post.EmbeddingStr = &vecStr
		}
	}

	createdPost, err := h.repo.Create(r.Context(), post)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Create",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"type":       string(postType),
				"authorType": string(authInfo.AuthorType),
				"authorID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to create post", err, ctx, h.logger)
		return
	}

	// Trigger async content moderation for everything EXCEPT family posts (BART-154).
	// Family/private posts are visible only to their owner's family, so they skip the
	// moderation gate entirely and are already 'open'. Fail-safe: any non-family visibility
	// still gets moderated.
	if h.contentModService != nil && visibility != models.VisibilityFamily {
		go h.moderatePostAsync(createdPost.ID, post.Title, post.Description, post.Tags, string(post.Type), string(authInfo.AuthorType), authInfo.AuthorID)
	}

	writePostsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// Update handles PATCH /v1/posts/:id - update a post.
// Per SPEC.md Part 15.2 and FIX-003: Users can edit their own content (humans and agents).
func (h *PostsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Get existing post
	existingPost, err := h.repo.FindByIDForViewer(r.Context(), postID, "", "", callerHumanID(r)) // BART-151: owner/family can find their own private post
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Update"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Check ownership - only owner can update (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.AuthorType && existingPost.PostedByID == authInfo.AuthorID
	if !isOwner {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own posts")
		return
	}

	// Required If-Match (idx 74 step 5): 428 without it, 412 when stale, so a
	// retry cannot overwrite a newer revision. Checked after ownership so a
	// non-owner never learns the version.
	expected, ok := enforceIfMatch(w, r, existingPost.UpdatedAt)
	if !ok {
		return
	}

	// Status guard — only allow editing if status is editable
	switch existingPost.Status {
	case models.PostStatusOpen, models.PostStatusRejected, models.PostStatusPendingReview, models.PostStatusDraft:
		// Allowed
	default:
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			fmt.Sprintf("Cannot edit post with status %s", existingPost.Status))
		return
	}

	// Parse request body
	var req UpdatePostRequest
	retiredField, err := decodePostBody(r, &req)
	if err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	if retiredField != "" {
		writeLegacyFieldRetired(w, retiredField, "", legacyProblemFieldInstead)
		return
	}

	// Apply updates
	updatedPost := existingPost.Post

	if req.Title != nil {
		if len(*req.Title) < 10 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
			return
		}
		if len(*req.Title) > 200 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
			return
		}
		updatedPost.Title = *req.Title
	}

	if req.Description != nil {
		if len(*req.Description) < 50 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
			return
		}
		if len(*req.Description) > models.MaxPostDescriptionLength {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("description must be at most %d characters", models.MaxPostDescriptionLength))
			return
		}
		updatedPost.Description = *req.Description
	}

	if req.Tags != nil {
		if len(req.Tags) > models.MaxTagsPerPost {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
			return
		}
		updatedPost.Tags = req.Tags
	}

	if req.Status != nil {
		newStatus := models.PostStatus(*req.Status)
		if models.IsRetiredPostStatus(newStatus) {
			writeLegacyFieldRetired(w, "status", *req.Status, legacyStatusInstead)
			return
		}
		if !models.IsValidPostStatus(newStatus) {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
			return
		}
		updatedPost.Status = newStatus

		// Private-room outcome gate: a post saved from a PRIVATE room must not reach the
		// public index through an ordinary author edit. Only the room owner may publish it,
		// via POST /v1/rooms/{slug}/posts/{id}/publish. Public-room outcomes and non-room
		// posts publish through this normal flow unchanged. A source room that no longer
		// exists cannot prove it was public, so its outcome stays unpublished; only a failed
		// lookup (DB error) lets the edit through.
		if h.roomPrivacy != nil && existingPost.SourceRoomID != nil &&
			existingPost.PublicationState != models.PublicationPublished {
			if newPub, _ := models.DeriveStates(newStatus); newPub == models.PublicationPublished {
				priv, perr := h.roomPrivacy.IsPrivateRoom(r.Context(), *existingPost.SourceRoomID)
				if (perr == nil && priv) || errors.Is(perr, db.ErrRoomNotFound) {
					writePostsError(w, http.StatusForbidden, "ROOM_OWNER_APPROVAL_REQUIRED",
						"a private-room outcome can only be published by the room owner")
					return
				}
			}
		}
	}

	// Determine if content (title/description) was changed
	contentChanged := req.Title != nil || req.Description != nil

	// Re-moderation: changed content on an open, rejected or pending_review post, or a status
	// edit that would publish a post moderation has not approved (D5a), goes back to
	// pending_review and through async moderation — the latter even without a moderator.
	unapprovedPublish := publishesUnapproved(existingPost.Post, updatedPost)
	needsReModeration := existingPost.Visibility != models.VisibilityFamily && // BART-154: family posts are never re-moderated
		(unapprovedPublish || contentChanged && (existingPost.Status == models.PostStatusOpen ||
			existingPost.Status == models.PostStatusRejected || existingPost.Status == models.PostStatusPendingReview))
	if needsReModeration && (h.contentModService != nil || unapprovedPublish) {
		updatedPost.Status = models.PostStatusPendingReview
	}
	needsReModeration = needsReModeration && h.contentModService != nil

	// Regenerate embedding if title or description changed
	if contentChanged && h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := updatedPost.Title + " " + updatedPost.Description
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to regenerate embedding for post", "error", embedErr, "postID", postID)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			updatedPost.EmbeddingStr = &vecStr
		}
	}

	// The write lands only at the version the precondition checked, so an
	// edit that lost a race to another writer is 412, not a lost update.
	result, err := h.repo.UpdateIfUnmodified(r.Context(), &updatedPost, expected)
	if answerVersionConflict(w, err, "post", writePostsError) {
		return
	}
	if err != nil {
		ctx := response.LogContext{
			Operation: "Update",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to update post", err, ctx, h.logger)
		return
	}

	// Trigger async re-moderation if content was changed
	if needsReModeration {
		go h.moderatePostAsync(postID, updatedPost.Title, updatedPost.Description, updatedPost.Tags, string(updatedPost.Type), string(authInfo.AuthorType), authInfo.AuthorID)
	}

	// Echo the new version validator so the client's next If-Match is current.
	w.Header().Set("ETag", postETag(result.UpdatedAt))
	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}
