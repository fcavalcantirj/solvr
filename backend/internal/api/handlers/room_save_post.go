package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// outcomePostRepo is the post storage the room-outcome flow needs. *db.PostRepository
// satisfies it.
type outcomePostRepo interface {
	Create(ctx context.Context, post *models.Post) (*models.Post, error)
	FindByIdempotencyKey(ctx context.Context, authorType, authorID, key string) (*models.PostWithAuthor, error)
	FindPublishedBySourceRoom(ctx context.Context, roomID string) ([]*models.PostWithAuthor, error)
	FindByIDForViewer(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error)
	UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error
}

// outcomeRoomRepo looks up rooms by slug. *db.RoomRepository satisfies it.
type outcomeRoomRepo interface {
	GetBySlug(ctx context.Context, slug string) (*models.Room, error)
}

// outcomeMemberRepo answers room membership/ownership. *db.RoomMemberRepository satisfies it.
type outcomeMemberRepo interface {
	IsMember(ctx context.Context, roomID uuid.UUID, agentID string) (bool, error)
	IsOwner(ctx context.Context, roomID uuid.UUID, agentID string) (bool, error)
	IsUserMember(ctx context.Context, roomID uuid.UUID, userID string) (bool, error)
	IsUserOwner(ctx context.Context, roomID uuid.UUID, userID string) (bool, error)
}

// RoomSavePostHandler turns an intentional room outcome into a reusable canonical Post.
// It never copies the transcript: the author supplies the title and outcome summary, and
// the result is a DRAFT until deliberately published (public rooms) or approved by the room
// owner (private rooms).
type RoomSavePostHandler struct {
	posts   outcomePostRepo
	rooms   outcomeRoomRepo
	members outcomeMemberRepo
	logger  *slog.Logger
}

// NewRoomSavePostHandler builds a RoomSavePostHandler.
func NewRoomSavePostHandler(posts outcomePostRepo, rooms outcomeRoomRepo, members outcomeMemberRepo) *RoomSavePostHandler {
	return &RoomSavePostHandler{
		posts:   posts,
		rooms:   rooms,
		members: members,
		logger:  slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// saveAsPostRequest is the body for POST /v1/rooms/{slug}/save-as-post.
type saveAsPostRequest struct {
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	Content         string   `json:"content"` // fallback for summary (agents often send "content")
	Tags            []string `json:"tags,omitempty"`
	SupportingLinks []string `json:"supporting_links,omitempty"`
}

// authorizeParticipant reports whether the authenticated caller may act as a participant of
// the room (an agent or human with an active membership).
func (h *RoomSavePostHandler) authorizeParticipant(ctx context.Context, room *models.Room, info *AuthInfo) (bool, error) {
	if info.AuthorType == models.AuthorTypeAgent {
		return h.members.IsMember(ctx, room.ID, info.AuthorID)
	}
	return h.members.IsUserMember(ctx, room.ID, info.AuthorID)
}

// authorizeOwner reports whether the authenticated caller owns the room (agent owner
// membership, or a human owner membership).
func (h *RoomSavePostHandler) authorizeOwner(ctx context.Context, room *models.Room, info *AuthInfo) (bool, error) {
	if info.AuthorType == models.AuthorTypeAgent {
		return h.members.IsOwner(ctx, room.ID, info.AuthorID)
	}
	return h.members.IsUserOwner(ctx, room.ID, info.AuthorID)
}

// SaveAsPost handles POST /v1/rooms/{slug}/save-as-post: an authorized room participant
// saves a reviewed outcome as a canonical draft post with source-room attribution.
func (h *RoomSavePostHandler) SaveAsPost(w http.ResponseWriter, r *http.Request) {
	info := GetAuthInfo(r)
	if info == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "room slug is required")
		return
	}
	room, err := h.rooms.GetBySlug(r.Context(), slug)
	if err != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}

	authorized, err := h.authorizeParticipant(r.Context(), room, info)
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check membership")
		return
	}
	if !authorized {
		// Hide a private room's existence from non-participants.
		if room.IsPrivate {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "only room participants can save an outcome as a post")
		return
	}

	var req saveAsPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	if req.Summary == "" {
		req.Summary = req.Content
	}
	if len(req.Title) < 10 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}
	if len(req.Summary) < 50 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "outcome summary must be at least 50 characters")
		return
	}
	if len(req.Tags) > models.MaxTagsPerPost {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "too many tags")
		return
	}

	// Idempotency: a retried save with the same key returns the existing draft.
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key != "" {
		if existing, e := h.posts.FindByIdempotencyKey(r.Context(), string(info.AuthorType), info.AuthorID, key); e == nil {
			writePostsJSON(w, http.StatusOK, map[string]interface{}{"data": existing})
			return
		}
	}

	body := req.Summary
	if len(req.SupportingLinks) > 0 {
		var b strings.Builder
		b.WriteString(body)
		b.WriteString("\n\n## Supporting messages\n")
		for _, link := range req.SupportingLinks {
			link = strings.TrimSpace(link)
			if link == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(link)
			b.WriteString("\n")
		}
		body = b.String()
	}

	roomID := room.ID.String()
	var keyPtr *string
	if key != "" {
		keyPtr = &key
	}
	post := &models.Post{
		Type:             models.PostTypePost,
		Title:            req.Title,
		Description:      body,
		Tags:             req.Tags,
		PostedByType:     info.AuthorType,
		PostedByID:       info.AuthorID,
		Status:           models.PostStatusDraft,
		PublicationState: models.PublicationDraft,
		ModerationState:  models.ModerationPending,
		Visibility:       models.VisibilityPublic,
		SourceRoomID:     &roomID,
		IdempotencyKey:   keyPtr,
	}

	created, err := h.posts.Create(r.Context(), post)
	if err != nil {
		// A concurrent save under the same key trips the unique index; return the winner.
		if key != "" {
			if existing, e := h.posts.FindByIdempotencyKey(r.Context(), string(info.AuthorType), info.AuthorID, key); e == nil {
				writePostsJSON(w, http.StatusOK, map[string]interface{}{"data": existing})
				return
			}
		}
		h.logger.Error("failed to create outcome post", "error", err, "slug", slug)
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save outcome as post")
		return
	}
	writePostsJSON(w, http.StatusCreated, map[string]interface{}{"data": created})
}

// ListOutcomePosts handles GET /v1/rooms/{slug}/posts: the room's published outcome posts.
// Public rooms are readable by anyone; a private room's outcomes require a participant.
func (h *RoomSavePostHandler) ListOutcomePosts(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	room, err := h.rooms.GetBySlug(r.Context(), slug)
	if err != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	if room.IsPrivate {
		info := GetAuthInfo(r)
		if info == nil {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		ok, err := h.authorizeParticipant(r.Context(), room, info)
		if err != nil {
			writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check membership")
			return
		}
		if !ok {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
	}
	posts, err := h.posts.FindPublishedBySourceRoom(r.Context(), room.ID.String())
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list room outcomes")
		return
	}
	writePostsJSON(w, http.StatusOK, map[string]interface{}{"data": posts})
}

// ApprovePublication handles POST /v1/rooms/{slug}/posts/{postID}/publish: the room owner
// publishes an outcome draft. This is the only way a private-room outcome reaches the public
// index, satisfying "prevent public publication until an authorized owner explicitly approves".
func (h *RoomSavePostHandler) ApprovePublication(w http.ResponseWriter, r *http.Request) {
	info := GetAuthInfo(r)
	if info == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	slug := chi.URLParam(r, "slug")
	postID := chi.URLParam(r, "postID")
	room, err := h.rooms.GetBySlug(r.Context(), slug)
	if err != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	isOwner, err := h.authorizeOwner(r.Context(), room, info)
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check ownership")
		return
	}
	if !isOwner {
		if room.IsPrivate {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner can approve publication")
		return
	}
	post, err := h.posts.FindByIDForViewer(r.Context(), postID, "", "", "")
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load post")
		return
	}
	if post.SourceRoomID == nil || *post.SourceRoomID != room.ID.String() {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post is not an outcome of this room")
		return
	}
	if err := h.posts.UpdateStatus(r.Context(), postID, models.PostStatusOpen); err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to publish outcome")
		return
	}
	published, err := h.posts.FindByIDForViewer(r.Context(), postID, "", "", "")
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load published post")
		return
	}
	writePostsJSON(w, http.StatusOK, map[string]interface{}{"data": published})
}
