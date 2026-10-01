package handlers

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// UpdateIfUnmodified lets the shared posts mock serve the conditional write
// behind PATCH /v1/posts/{id} (spec.json idx 74 step 5); it records the edit
// exactly like Update. conditionalPostsRepo overrides it to observe the
// expected version or to lose the write to a concurrent writer.
func (m *MockPostsRepository) UpdateIfUnmodified(ctx context.Context, post *models.Post, _ *time.Time) (*models.Post, error) {
	return m.Update(ctx, post)
}

// The other posts mocks record a conditional write exactly like Update.
func (m *MockPostsRepositoryForIntegration) UpdateIfUnmodified(ctx context.Context, post *models.Post, _ *time.Time) (*models.Post, error) {
	return m.Update(ctx, post)
}

func (m *MockPostsRepositoryForQuestions) UpdateIfUnmodified(ctx context.Context, post *models.Post, _ *time.Time) (*models.Post, error) {
	return m.Update(ctx, post)
}

func (m *mockPostsRepo) UpdateIfUnmodified(ctx context.Context, post *models.Post, _ *time.Time) (*models.Post, error) {
	return m.Update(ctx, post)
}
