package db

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SPEC.md 22.8: a failed search is stored with its input, outcome and request id. The input is
// stored as text Postgres accepts (valid UTF-8, no NUL) and within the 500-character column rule;
// an absent request id or SQLSTATE is NULL.
func TestSearchFailureRepository_StoresTheInputAsValidTextWithItsOutcome(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewSearchFailureRepository(pool)
	at := time.Now().UTC().Truncate(time.Millisecond)
	marker := "c1-probe-" + at.Format("150405.000")
	long := marker + " bad\xffbyte nul\x00here " + strings.Repeat("é", 600)

	require.NoError(t, repo.RecordFailure(ctx, &models.SearchFailure{
		RequestID: "req-fail-1", Query: long, Status: 500, ErrorCode: "INTERNAL_ERROR",
		SQLState: "42601", SearcherType: "anonymous", DurationMs: 312, OccurredAt: at,
	}))
	require.NoError(t, repo.RecordFailure(ctx, &models.SearchFailure{
		Query: marker + " gone", Status: 499, ErrorCode: "CLIENT_CLOSED_REQUEST", SearcherType: "agent", OccurredAt: at,
	}))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM search_failures WHERE query LIKE $1`, marker+"%") }) //nolint:errcheck

	var q, code string
	var status, dur int
	var rid, state *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT query, status, error_code, request_id, sqlstate, duration_ms
		FROM search_failures WHERE query LIKE $1 AND status = 500`, marker+"%").Scan(&q, &status, &code, &rid, &state, &dur))
	assert.True(t, utf8.ValidString(q))
	assert.NotContains(t, q, "\x00")
	assert.Equal(t, 500, utf8.RuneCountInString(q))
	assert.Contains(t, q, "bad�byte nulhere", "invalid bytes become U+FFFD; NUL is removed")
	assert.Equal(t, "INTERNAL_ERROR", code)
	assert.Equal(t, 312, dur)
	require.NotNil(t, rid)
	assert.Equal(t, "req-fail-1", *rid)
	require.NotNil(t, state)
	assert.Equal(t, "42601", *state)

	require.NoError(t, pool.QueryRow(ctx, `SELECT request_id, sqlstate FROM search_failures
		WHERE query = $1`, marker+" gone").Scan(&rid, &state))
	assert.Nil(t, rid, "no request id is NULL")
	assert.Nil(t, state, "no SQLSTATE is NULL")
}
