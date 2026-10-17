package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
)

// C3 (REVIEW-r1): a post-sourced start that an agent fetched and nobody copied is flagged by
// the job's next run, with no manual call; a start still waiting for its fetch is left alone,
// and so is one whose prompt was copied.
func TestTrafficClassifyJob_FlagsALiveStartWithinOneRun(t *testing.T) {
	url := newMigratedScratchURL(t, "solvr_traffic_classify_")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, execErr := pool.Exec(ctx, sql, args...); execErr != nil {
			t.Fatalf("%s: %v", sql, execErr)
		}
	}
	start := func(flow string, at time.Time) {
		t.Helper()
		exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface, source_kind, source_id,
		                                 traffic_class, traffic_reason, occurred_at)
		      VALUES ($1, 'connection_started', 'browser', 'anonymous', 'connect_page', 'post', $2, 'human', 'browser', $3)`,
			flow, uuid.New(), at)
	}
	fetch := func(flow string, at time.Time) {
		t.Helper()
		exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface,
		                                 traffic_class, traffic_reason, occurred_at)
		      VALUES ($1, 'skill_fetched', 'web_server', 'anonymous', 'agent_fetch', 'agent', 'http_client', $2)`, flow, at)
	}
	label := func(flow, event string) string {
		t.Helper()
		var l string
		if scanErr := pool.QueryRow(ctx, `SELECT traffic_class || '/' || COALESCE(traffic_reason, '') FROM funnel_events
		                               WHERE flow_id = $1 AND event_name = $2`, flow, event).Scan(&l); scanErr != nil {
			t.Fatalf("label %s %s: %v", flow, event, scanErr)
		}
		return l
	}

	now := time.Now()
	start("live0001", now.Add(-5*time.Minute))
	fetch("live0001", now.Add(-5*time.Minute+45*time.Second))
	start("live0002", now.Add(-time.Minute)) // its fetch has not arrived yet
	start("live0003", now.Add(-4*time.Minute))
	exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface, occurred_at)
	      VALUES ('live0003', 'starter_prompt_copied', 'browser', 'anonymous', 'connect_page', $1)`, now.Add(-3*time.Minute))
	fetch("live0003", now.Add(-2*time.Minute))

	run, err := jobs.NewTrafficClassifyJob(db.NewTrafficRules(pool)).RunOnce(ctx)
	if err != nil || run.Busy {
		t.Fatalf("RunOnce: %+v, %v", run, err)
	}
	if got := label("live0001", "connection_started"); got != "automation/fetched_without_copy" {
		t.Errorf("the fetched, uncopied start is %s", got)
	}
	if got := label("live0001", "skill_fetched"); got != "automation/fetched_without_copy" {
		t.Errorf("its fetch is %s", got)
	}
	if got := label("live0002", "connection_started"); got != "human/browser" {
		t.Errorf("the start still waiting for its fetch is %s", got)
	}
	if got := label("live0003", "connection_started"); got != "human/browser" {
		t.Errorf("the copied start is %s", got)
	}
}
