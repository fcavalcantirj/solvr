package db

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// searchFailureQueryMax is the search_failures.query column rule, in characters (000144).
const searchFailureQueryMax = 500

// SearchFailureRepository stores searches that did not answer 2xx (SPEC.md 22.8, migration
// 000144). Operator diagnostics: no reader publishes them.
type SearchFailureRepository struct {
	pool *Pool
}

// NewSearchFailureRepository creates the store of failed searches.
func NewSearchFailureRepository(pool *Pool) *SearchFailureRepository {
	return &SearchFailureRepository{pool: pool}
}

// RecordFailure stores one failed search. Its query is stored as text Postgres accepts: invalid
// UTF-8 becomes U+FFFD, NUL bytes are removed and it is cut to 500 characters. An empty request
// id or SQLSTATE is stored as NULL.
func (r *SearchFailureRepository) RecordFailure(ctx context.Context, f *models.SearchFailure) error {
	q := strings.ToValidUTF8(strings.ReplaceAll(f.Query, "\x00", ""), "�")
	if utf8.RuneCountInString(q) > searchFailureQueryMax {
		q = string([]rune(q)[:searchFailureQueryMax])
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO search_failures (request_id, query, status, error_code, sqlstate, searcher_type, duration_ms, occurred_at)
		VALUES (NULLIF($1, ''), $2, $3, $4, NULLIF($5, ''), $6, $7, $8)`,
		f.RequestID, q, f.Status, f.ErrorCode, f.SQLState, f.SearcherType, f.DurationMs, f.OccurredAt)
	if err != nil {
		LogQueryError(ctx, "RecordFailure", "search_failures", err)
		return fmt.Errorf("record search failure: %w", err)
	}
	return nil
}
