package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// A recorded request keeps the error code of its answer and the class of its client
// (SPEC.md 27.6); a success and an unrecorded class are NULL.
func TestAPIUsageRepository_StoresTheErrorCodeAndTheClientClass(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	failed, served := "r2-"+uuid.NewString(), "r2-"+uuid.NewString()
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM api_request_events WHERE request_id IN ($1, $2)`, failed, served) //nolint:errcheck
	})
	require.NoError(t, db.NewAPIUsageRepository(pool).RecordRequests(ctx, []db.APIRequestEvent{
		{RequestID: failed, RouteTemplate: "/v1/posts/{id}", Method: "GET", Operation: "knowledge.post.read",
			OperationFamily: db.APIFamilyKnowledge, OperationKind: db.APIOperationPoll, ActorType: db.APIActorAnonymous,
			StatusClass: 4, OccurredAt: time.Now(), ErrorCode: "NOT_FOUND", UAClass: "browser"},
		{RequestID: served, RouteTemplate: "/v1/posts/{id}", Method: "GET", Operation: "knowledge.post.read",
			OperationFamily: db.APIFamilyKnowledge, OperationKind: db.APIOperationPoll, ActorType: db.APIActorAnonymous,
			StatusClass: 2, OccurredAt: time.Now()},
	}))
	var code, class string
	require.NoError(t, pool.QueryRow(ctx, `SELECT error_code, ua_class FROM api_request_events WHERE request_id = $1`, failed).Scan(&code, &class))
	require.Equal(t, "NOT_FOUND", code)
	require.Equal(t, "browser", class)
	var nulls bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT error_code IS NULL AND ua_class IS NULL FROM api_request_events WHERE request_id = $1`, served).Scan(&nulls))
	require.True(t, nulls)
}
