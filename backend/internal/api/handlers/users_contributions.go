package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ContribAnswersRepositoryInterface defines the answers repository for contributions.
type ContribAnswersRepositoryInterface interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.AnswerWithContext, int, error)
}

// ContribApproachesRepositoryInterface defines the approaches repository for contributions.
type ContribApproachesRepositoryInterface interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ApproachWithContext, int, error)
}

// ContribResponsesRepositoryInterface defines the responses repository for contributions.
type ContribResponsesRepositoryInterface interface {
	ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ResponseWithContext, int, error)
}

// SetContributionRepositories sets the repositories for listing user contributions.
func (h *UsersHandler) SetContributionRepositories(
	answersRepo ContribAnswersRepositoryInterface,
	approachesRepo ContribApproachesRepositoryInterface,
	responsesRepo ContribResponsesRepositoryInterface,
) {
	h.answersRepo = answersRepo
	h.approachesRepo = approachesRepo
	h.responsesRepo = responsesRepo
}

// ContributionsResponse is the response for GET /v1/users/{id}/contributions.
type ContributionsResponse struct {
	Data []models.ContributionItem `json:"data"`
	Meta ContributionsMeta         `json:"meta"`
}

// ContributionsMeta holds pagination metadata for contributions.
type ContributionsMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// GetUserContributions handles GET /v1/users/{id}/contributions.
// Returns answers, approaches, and responses for a user, unified and sorted by created_at DESC.
// Supports ?type=answers|approaches|responses filter and page/per_page pagination.
func (h *UsersHandler) GetUserContributions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "id")

	if userID == "" {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "user ID is required")
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(userID); err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid user ID format")
		return
	}

	// Look up user to get author type
	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	// Parse pagination
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// Parse type filter
	typeFilter := r.URL.Query().Get("type")

	items, total := h.fetchContributions(ctx, "human", userID, typeFilter, page, perPage)

	resp := ContributionsResponse{
		Data: items,
		Meta: ContributionsMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: total > page*perPage,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// fetchContributions fetches contributions from the three repositories, merges, sorts, and paginates.
func (h *UsersHandler) fetchContributions(
	ctx context.Context,
	authorType, authorID, typeFilter string,
	page, perPage int,
) ([]models.ContributionItem, int) {
	var items []models.ContributionItem
	total := 0

	// We fetch a large batch from each repo to merge and sort.
	// Use perPage * page as the fetch size so we can paginate the merged result.
	fetchSize := perPage * page
	if fetchSize > 100 {
		fetchSize = 100
	}

	// Fetch answers
	if typeFilter == "" || typeFilter == "answers" {
		if h.answersRepo != nil {
			answers, ansTotal, err := h.answersRepo.ListByAuthor(ctx, authorType, authorID, 1, fetchSize)
			if err == nil {
				for _, a := range answers {
					items = append(items, models.ContributionItem{
						Type:           models.ContributionTypeAnswer,
						ID:             a.ID,
						ParentID:       a.QuestionID,
						ParentTitle:    a.QuestionTitle,
						ParentType:     "question",
						ContentPreview: models.TruncateContent(a.Content, 200),
						CreatedAt:      a.CreatedAt,
					})
				}
				if typeFilter == "answers" {
					total = ansTotal
				} else {
					total += ansTotal
				}
			}
		}
	}

	// Fetch approaches
	if typeFilter == "" || typeFilter == "approaches" {
		if h.approachesRepo != nil {
			approaches, appTotal, err := h.approachesRepo.ListByAuthor(ctx, authorType, authorID, 1, fetchSize)
			if err == nil {
				for _, a := range approaches {
					items = append(items, models.ContributionItem{
						Type:           models.ContributionTypeApproach,
						ID:             a.ID,
						ParentID:       a.ProblemID,
						ParentTitle:    a.ProblemTitle,
						ParentType:     "problem",
						ContentPreview: models.TruncateContent(a.Angle, 200),
						Status:         string(a.Status),
						CreatedAt:      a.CreatedAt,
					})
				}
				if typeFilter == "approaches" {
					total = appTotal
				} else {
					total += appTotal
				}
			}
		}
	}

	// Fetch responses
	if typeFilter == "" || typeFilter == "responses" {
		if h.responsesRepo != nil {
			responses, respTotal, err := h.responsesRepo.ListByAuthor(ctx, authorType, authorID, 1, fetchSize)
			if err == nil {
				for _, r := range responses {
					items = append(items, models.ContributionItem{
						Type:           models.ContributionTypeResponse,
						ID:             r.ID,
						ParentID:       r.IdeaID,
						ParentTitle:    r.IdeaTitle,
						ParentType:     "idea",
						ContentPreview: models.TruncateContent(r.Content, 200),
						CreatedAt:      r.CreatedAt,
					})
				}
				if typeFilter == "responses" {
					total = respTotal
				} else {
					total += respTotal
				}
			}
		}
	}

	// Sort by created_at DESC
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	// Apply pagination
	offset := (page - 1) * perPage
	if offset >= len(items) {
		return []models.ContributionItem{}, total
	}
	end := offset + perPage
	if end > len(items) {
		end = len(items)
	}

	return items[offset:end], total
}
