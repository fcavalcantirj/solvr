package handlers

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Shared test helpers, kept from the legacy handler tests retired with the legacy handlers
// (idx 68): other handler tests still use them.

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func intPtr(i int) *int {
	return &i
}

// MockPostsRepositoryForQuestions implements PostsRepositoryInterface for testing FIX-023.
// This mock simulates posts created via the main posts endpoint.
type MockPostsRepositoryForQuestions struct {
	posts map[string]*models.PostWithAuthor
}

func NewMockPostsRepositoryForQuestions() *MockPostsRepositoryForQuestions {
	return &MockPostsRepositoryForQuestions{
		posts: make(map[string]*models.PostWithAuthor),
	}
}

func (m *MockPostsRepositoryForQuestions) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	var result []models.PostWithAuthor
	for _, post := range m.posts {
		if opts.Type != "" && post.Type != opts.Type {
			continue
		}
		result = append(result, *post)
	}
	return result, len(result), nil
}

func (m *MockPostsRepositoryForQuestions) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if post, exists := m.posts[id]; exists {
		return post, nil
	}
	return nil, ErrPostNotFound
}

func (m *MockPostsRepositoryForQuestions) FindByIDForViewer(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error) {
	return m.FindByID(ctx, id)
}

func (m *MockPostsRepositoryForQuestions) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	postWithAuthor := &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
	}
	m.posts[post.ID] = postWithAuthor
	return post, nil
}

func (m *MockPostsRepositoryForQuestions) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	return post, nil
}

func (m *MockPostsRepositoryForQuestions) Delete(ctx context.Context, id string) error {
	delete(m.posts, id)
	return nil
}

func (m *MockPostsRepositoryForQuestions) Vote(ctx context.Context, postID, voterType, voterID, direction string) error {
	return nil
}

func (m *MockPostsRepositoryForQuestions) GetUserVote(ctx context.Context, postID, voterType, voterID string) (*string, error) {
	return nil, nil
}

func (m *MockPostsRepositoryForQuestions) AddPost(post *models.PostWithAuthor) {
	m.posts[post.ID] = post
}
