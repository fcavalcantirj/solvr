// Package handlers contains HTTP request handlers for the Solvr API.
// This file contains approach-related methods on ProblemsHandler.
// Split from problems.go to keep file sizes under ~900 lines.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ApproachesListResponse is the response for listing approaches.
type ApproachesListResponse struct {
	Data []models.ApproachWithAuthor `json:"data"`
	Meta ProblemsListMeta            `json:"meta"`
}

// ProgressNoteRequest is the request body for adding a progress note.
type ProgressNoteRequest struct {
	Content string `json:"content"`
}

// VerifyApproachRequest is the request body for verifying an approach.
type VerifyApproachRequest struct {
	Verified bool `json:"verified"`
}

// ListApproaches handles GET /v1/problems/:id/approaches - list approaches for a problem.
// Per FIX-023: Uses findProblem() to find problems from either postsRepo or problemsRepo.
func (h *ProblemsHandler) ListApproaches(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// FIX-023: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	_, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Type and deletion checks are now done in findProblem()

	// Parse query parameters
	opts := models.ApproachListOptions{
		ProblemID: problemID,
		Page:      parseProblemsIntParam(r.URL.Query().Get("page"), 1),
		PerPage:   parseProblemsIntParam(r.URL.Query().Get("per_page"), 20),
	}

	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50
	}

	// Execute query
	approaches, total, err := h.repo.ListApproaches(r.Context(), problemID, opts)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list approaches")
		return
	}

	// Populate progress notes for each approach
	for i := range approaches {
		notes, err := h.repo.GetProgressNotes(r.Context(), approaches[i].ID)
		if err != nil {
			// Log error but don't fail - progress notes are optional
			continue
		}
		approaches[i].ProgressNotes = notes
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := ApproachesListResponse{
		Data: approaches,
		Meta: ProblemsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeProblemsJSON(w, http.StatusOK, response)
}

// CreateApproach handles POST /v1/problems/:id/approaches - create a new approach.
// Per SPEC.md Part 1.4 and FIX-016: Both humans (JWT) and AI agents (API key) can start approaches.
// Per FIX-023: Uses findProblem() to find problems from either postsRepo or problemsRepo.
func (h *ProblemsHandler) CreateApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// FIX-023: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	_, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Type and deletion checks are now done in findProblem()

	// Parse request body
	var req models.CreateApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate angle
	if req.Angle == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "angle is required")
		return
	}
	if len(req.Angle) > 500 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "angle must be at most 500 characters")
		return
	}

	// Validate method
	if len(req.Method) > 500 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "method must be at most 500 characters")
		return
	}

	// Validate assumptions (max 10)
	if len(req.Assumptions) > 10 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 assumptions allowed")
		return
	}

	// Create approach with author info from authentication
	approach := &models.Approach{
		ProblemID:   problemID,
		AuthorType:  authInfo.AuthorType,
		AuthorID:    authInfo.AuthorID,
		Angle:       req.Angle,
		Method:      req.Method,
		Assumptions: req.Assumptions,
		DiffersFrom: req.DiffersFrom,
		Status:      models.ApproachStatusStarting,
	}

	// Synchronous embedding: combine angle + method for semantic search
	if h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := approach.Angle + " " + approach.Method
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to generate embedding for approach", "error", embedErr)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			approach.EmbeddingStr = &vecStr
		}
	}

	createdApproach, err := h.repo.CreateApproach(r.Context(), approach)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create approach")
		return
	}

	writeProblemsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdApproach,
	})
}

// UpdateApproach handles PATCH /v1/approaches/:id - update an approach.
// Per FIX-016: Both humans (JWT) and AI agents (API key) can update their approaches.
func (h *ProblemsHandler) UpdateApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	existingApproach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Check ownership - only author can update (works for both humans and agents)
	if existingApproach.AuthorType != authInfo.AuthorType || existingApproach.AuthorID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own approaches")
		return
	}

	// Parse request body
	var req models.UpdateApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Apply updates
	updatedApproach := existingApproach.Approach
	contentChanged := false

	if req.Status != nil {
		newStatus := models.ApproachStatus(*req.Status)
		if !models.IsValidApproachStatus(newStatus) {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
			return
		}
		updatedApproach.Status = newStatus
	}

	if req.Outcome != nil {
		if len(*req.Outcome) > 10000 {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "outcome must be at most 10000 characters")
			return
		}
		updatedApproach.Outcome = *req.Outcome
		contentChanged = true
	}

	if req.Method != nil {
		if len(*req.Method) > 500 {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "method must be at most 500 characters")
			return
		}
		updatedApproach.Method = *req.Method
		contentChanged = true
	}

	// Regenerate embedding if method or outcome changed (content that affects semantic meaning)
	if contentChanged && h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := updatedApproach.Angle + " " + updatedApproach.Method
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to regenerate embedding for approach", "error", embedErr)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			updatedApproach.EmbeddingStr = &vecStr
		}
	}

	result, err := h.repo.UpdateApproach(r.Context(), &updatedApproach)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update approach")
		return
	}

	writeProblemsJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}

// AddProgressNote handles POST /v1/approaches/:id/progress - add a progress note.
// Per FIX-016: Both humans (JWT) and AI agents (API key) can add progress notes.
func (h *ProblemsHandler) AddProgressNote(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	existingApproach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Check ownership - only author can add progress notes (works for both humans and agents)
	if existingApproach.AuthorType != authInfo.AuthorType || existingApproach.AuthorID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "you can only add progress notes to your own approaches")
		return
	}

	// Parse request body
	var req ProgressNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate content
	if req.Content == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}

	// Create progress note
	note := &models.ProgressNote{
		ApproachID: approachID,
		Content:    req.Content,
	}

	createdNote, err := h.repo.AddProgressNote(r.Context(), note)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add progress note")
		return
	}

	writeProblemsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdNote,
	})
}

// VerifyApproach handles POST /v1/approaches/:id/verify - verify an approach solution.
// Per FIX-016: Both humans (JWT) and AI agents (API key) who own the problem can verify.
func (h *ProblemsHandler) VerifyApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	approach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Get the problem to verify ownership
	// FIX-024: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	problem, err := h.findProblem(r.Context(), approach.ProblemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Check ownership - only problem owner can verify (works for both humans and agents)
	if problem.PostedByType != authInfo.AuthorType || problem.PostedByID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "only the problem owner can verify approaches")
		return
	}

	// Parse request body
	var req VerifyApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// If verified and approach succeeded, update problem status to solved
	if req.Verified && approach.Status == models.ApproachStatusSucceeded {
		// FIX-025: Try postsRepo first (where most problems are stored), then fall back to problemsRepo
		var updateErr error
		if h.postsRepo != nil {
			// Update via postsRepo.Update (requires full post object)
			updatedPost := models.Post{
				ID:           problem.ID,
				Type:         problem.Type,
				Title:        problem.Title,
				Description:  problem.Description,
				Tags:         problem.Tags,
				PostedByType: problem.PostedByType,
				PostedByID:   problem.PostedByID,
				Status:       models.PostStatusSolved,
				Upvotes:      problem.Upvotes,
				Downvotes:    problem.Downvotes,
				ViewCount:    problem.ViewCount,
				CreatedAt:    problem.CreatedAt,
			}
			_, updateErr = h.postsRepo.Update(r.Context(), &updatedPost)
		}
		if updateErr != nil || h.postsRepo == nil {
			// Fall back to problemsRepo
			updateErr = h.repo.UpdateProblemStatus(r.Context(), problem.ID, models.PostStatusSolved)
		}
		if updateErr != nil {
			writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update problem status")
			return
		}
	}

	writeProblemsJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "approach verified",
		"verified": req.Verified,
	})
}

// GetApproachHistory handles GET /v1/problems/:id/approaches/:approachId/history.
// Returns the version chain for an approach (current + history + relationships).
// Public endpoint (no auth required).
func (h *ProblemsHandler) GetApproachHistory(w http.ResponseWriter, r *http.Request) {
	if h.relRepo == nil {
		writeProblemsError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "version history not available")
		return
	}

	approachID := chi.URLParam(r, "approachId")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Verify approach exists
	_, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Parse optional depth parameter
	depth := 0
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d > 0 {
			depth = d
		}
	}

	history, err := h.relRepo.GetVersionChain(r.Context(), approachID, depth)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get version history")
		return
	}

	writeProblemsJSON(w, http.StatusOK, map[string]interface{}{
		"data": history,
	})
}
