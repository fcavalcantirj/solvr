package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The pre-archive scratch database carries the head router: every probe on it runs
// api.NewRouter, whose API-usage recorder writes each request's served duration
// (db.APIRequestEvent.DurationMs) on the probe's traced pool. That write must succeed there,
// or its errors land inside whichever probe call is running and are judged as that call's.
func TestPreArchiveScratchDatabase_AcceptsTheRoutersAPIUsageWrite(t *testing.T) {
	scratchURL := newPreArchiveScratchURL(t, "solvr_legacy_usage_")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, scratchURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	served := 12
	err = db.NewAPIUsageRepository(pool).RecordRequests(ctx, []db.APIRequestEvent{{
		RouteTemplate: "/v1/posts", Method: "GET", Operation: "posts.list",
		OperationFamily: db.APIFamilyKnowledge, OperationKind: "search", ActorType: "anonymous",
		StatusClass: 2, DurationMs: &served,
	}})
	if err != nil {
		t.Fatalf("the router's API-usage write fails on the pre-archive database: %v", err)
	}
}
