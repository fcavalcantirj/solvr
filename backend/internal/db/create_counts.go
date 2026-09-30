package db

import (
	"context"
	"fmt"
	"time"
)

// CreateCountRepository counts an author's creates in the authoritative content tables, for
// the hourly create limits (anti-abuse W3). The count survives deploys and is shared by every
// API instance, unlike a per-process counter.
type CreateCountRepository struct {
	pool *Pool
}

// NewCreateCountRepository creates a CreateCountRepository.
func NewCreateCountRepository(pool *Pool) *CreateCountRepository {
	return &CreateCountRepository{pool: pool}
}

// createCountSources lists, per create operation, the rows that count: every row the author
// created, deleted and rejected ones included, so deleting does not buy a new budget.
var createCountSources = map[string]string{
	"posts": `
		SELECT created_at FROM posts WHERE posted_by_type = $1 AND posted_by_id = $2 AND created_at > $3
		UNION ALL SELECT created_at FROM blog_posts WHERE posted_by_type = $1 AND posted_by_id = $2 AND created_at > $3`,
	"contributions": `
		SELECT created_at FROM replies WHERE author_type = $1 AND author_id = $2 AND created_at > $3`,
}

// legacyContributionCountSources extend the contributions count to the legacy contribution
// tables while they exist (their create routes are still mounted until legacy cleanup).
var legacyContributionCountSources = `
		UNION ALL SELECT created_at FROM answers WHERE author_type = $1 AND author_id = $2 AND created_at > $3
		UNION ALL SELECT created_at FROM approaches WHERE author_type = $1 AND author_id = $2 AND created_at > $3
		UNION ALL SELECT created_at FROM responses WHERE author_type = $1 AND author_id = $2 AND created_at > $3
		UNION ALL SELECT created_at FROM comments WHERE author_type = $1 AND author_id = $2 AND created_at > $3
		UNION ALL SELECT pn.created_at FROM progress_notes pn JOIN approaches ap ON ap.id = pn.approach_id
		          WHERE ap.author_type = $1 AND ap.author_id = $2 AND pn.created_at > $3`

// CountRecentCreates returns how many op creates ("posts" or "contributions") the author made
// after since, the oldest of them (zero when none), and when the account was created (zero
// when unknown).
func (r *CreateCountRepository) CountRecentCreates(ctx context.Context, op, authorType, authorID string, since time.Time) (count int, oldest, accountCreatedAt time.Time, err error) {
	source, ok := createCountSources[op]
	if !ok {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("CountRecentCreates: unknown operation %q", op)
	}
	if op == "contributions" {
		legacy, err := legacyContributionTablesPresent(ctx, r.pool)
		if err != nil {
			return 0, time.Time{}, time.Time{}, err
		}
		if legacy {
			source += legacyContributionCountSources
		}
	}
	var oldestPtr, createdPtr *time.Time
	err = r.pool.QueryRow(ctx, `
		WITH c AS (`+source+`)
		SELECT count(*), min(c.created_at),
		       (SELECT a.created_at FROM agents a WHERE $1 = 'agent' AND a.id = $2
		        UNION ALL
		        SELECT u.created_at FROM users u WHERE $1 = 'human' AND u.id::text = $2
		        LIMIT 1)
		FROM c`, authorType, authorID, since).Scan(&count, &oldestPtr, &createdPtr)
	if err != nil {
		LogQueryError(ctx, "CountRecentCreates", op, err)
		return 0, time.Time{}, time.Time{}, fmt.Errorf("count recent creates: %w", err)
	}
	if oldestPtr != nil {
		oldest = *oldestPtr
	}
	if createdPtr != nil {
		accountCreatedAt = *createdPtr
	}
	return count, oldest, accountCreatedAt, nil
}
