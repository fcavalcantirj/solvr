package jobs_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
)

// retiredLifecycleRows are rows each retired lifecycle job would act on.
type retiredLifecycleRows struct {
	staleApproach   string // 'working', idle 31 days: StaleContentJob abandons it
	warnedApproach  string // 'starting', idle 25 days: StaleContentJob warns its author
	quietProblem    string // open problem, 61 days old, no approaches: StaleContentJob marks it dormant
	solvedProblem   string // open problem, succeeded approach 15 days old: AutoSolveJob solves it
	warnedProblem   string // open problem, succeeded approach 10 days old: AutoSolveJob warns its owner
	approachProblem string // the open problem holding the two stale approaches
}

func seedRetiredLifecycleRows(ctx context.Context, t *testing.T, pool *db.Pool, agentID string) retiredLifecycleRows {
	t.Helper()
	problem := func(title string, age string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, created_at, updated_at)
			VALUES ('problem', $1, 'A problem the retired lifecycle jobs would act on', 'agent', $2, 'open',
			        NOW() - $3::interval, NOW() - $3::interval)
			RETURNING id::text`, title, agentID, age).Scan(&id); err != nil {
			t.Fatalf("seed problem %q: %v", title, err)
		}
		return id
	}
	approach := func(problemID, status, idle string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO approaches (problem_id, author_type, author_id, angle, status, created_at, updated_at)
			VALUES ($1, 'agent', $2, 'Retired lifecycle angle', $3, NOW() - INTERVAL '90 days', NOW() - $4::interval)
			RETURNING id::text`, problemID, agentID, status, idle).Scan(&id); err != nil {
			t.Fatalf("seed %s approach: %v", status, err)
		}
		return id
	}
	var rows retiredLifecycleRows
	rows.approachProblem = problem("Retired lifecycle problem with stale approaches", "90 days")
	rows.staleApproach = approach(rows.approachProblem, "working", "31 days")
	rows.warnedApproach = approach(rows.approachProblem, "starting", "25 days")
	rows.quietProblem = problem("Retired lifecycle problem nobody approached", "61 days")
	rows.solvedProblem = problem("Retired lifecycle problem with an old success", "90 days")
	approach(rows.solvedProblem, "succeeded", "15 days")
	rows.warnedProblem = problem("Retired lifecycle problem with a recent success", "90 days")
	approach(rows.warnedProblem, "succeeded", "10 days")
	return rows
}

func lifecycleStatus(ctx context.Context, t *testing.T, pool *db.Pool, table, id string) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM "+table+" WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatalf("read %s %s status: %v", table, id, err)
	}
	return status
}

func lifecycleNotifications(ctx context.Context, t *testing.T, pool *db.Pool) map[string]int {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT type, COUNT(*) FROM notifications
		WHERE type IN ('approach_abandonment_warning', 'auto_solve_warning', 'auto_solved')
		GROUP BY type`)
	if err != nil {
		t.Fatalf("count lifecycle notifications: %v", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			t.Fatalf("scan lifecycle notifications: %v", err)
		}
		out[typ] = n
	}
	return out
}

// The approach and problem lifecycles are retired: replies carry no status workflow, so
// nothing cmd/api/main.go schedules may abandon or warn about approaches, mark problems
// dormant or auto-solve them, and no lifecycle notification goes out. Checked on a
// migrated database that still holds the legacy tables (between the cutover deploy and
// schema cleanup), with rows inside each retired job's window.
func TestRetiredLegacyLifecycles_NoScheduledJobAbandonsSolvesOrMarksDormant(t *testing.T) {
	pool, tracer := newMigratedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	workers := scheduledWorkers(pool, tracer)
	scheduled := scheduledJobKeys(t)
	var workerKeys, scheduledKeys []string
	for _, w := range workers {
		workerKeys = append(workerKeys, w.key)
	}
	for k := range scheduled {
		scheduledKeys = append(scheduledKeys, k)
	}
	sort.Strings(workerKeys)
	sort.Strings(scheduledKeys)
	if len(workerKeys) != len(scheduledKeys) {
		t.Fatalf("the probe workers %v must be exactly the jobs the sources schedule %v", workerKeys, scheduledKeys)
	}
	for i := range workerKeys {
		if workerKeys[i] != scheduledKeys[i] {
			t.Fatalf("the probe workers %v must be exactly the jobs the sources schedule %v", workerKeys, scheduledKeys)
		}
	}
	for _, retired := range []string{"job:StaleContentJob", "job:AutoSolveJob"} {
		if scheduled[retired] {
			t.Errorf("%s is retired but the sources still schedule it", retired)
		}
	}

	agentID := seedScheduledWorkerData(ctx, t, pool)
	rows := seedRetiredLifecycleRows(ctx, t, pool, agentID)
	tracer.take()

	for _, w := range workers {
		tracer.take() // a worker's waits look only at its own statements
		w.run(ctx, t)
	}

	for _, c := range []struct{ table, id, want string }{
		{"approaches", rows.staleApproach, "working"},
		{"approaches", rows.warnedApproach, "starting"},
		{"posts", rows.quietProblem, "open"},
		{"posts", rows.solvedProblem, "open"},
		{"posts", rows.warnedProblem, "open"},
		{"posts", rows.approachProblem, "open"},
	} {
		if got := lifecycleStatus(ctx, t, pool, c.table, c.id); got != c.want {
			t.Errorf("a scheduled job changed %s %s to %q, want it left %q", c.table, c.id, got, c.want)
		}
	}
	if got := lifecycleNotifications(ctx, t, pool); len(got) != 0 {
		t.Errorf("scheduled jobs sent retired lifecycle notifications: %v", got)
	}

	// Positive control: the retired jobs, run directly, still act on exactly these rows, so
	// the rows sit inside their windows and the checks above are not vacuous.
	notifRepo := db.NewNotificationsRepository(pool)
	staleRepo := db.NewStaleContentRepository(pool, notifRepo)
	if got, want := jobs.NewStaleContentJob(staleRepo, staleRepo, staleRepo).RunOnce(ctx),
		(jobs.StaleContentResult{Abandoned: 1, Warned: 1, Dormant: 1}); got != want {
		t.Errorf("retired StaleContentJob run directly = %+v, want %+v", got, want)
	}
	autoRepo := db.NewAutoSolveRepository(pool, notifRepo)
	if got, want := jobs.NewAutoSolveJob(autoRepo, autoRepo).RunOnce(ctx),
		(jobs.AutoSolveResult{Warned: 1, Solved: 1}); got != want {
		t.Errorf("retired AutoSolveJob run directly = %+v, want %+v", got, want)
	}
	if got := lifecycleStatus(ctx, t, pool, "approaches", rows.staleApproach); got != "abandoned" {
		t.Errorf("control: stale approach = %q, want abandoned", got)
	}
	if got := lifecycleStatus(ctx, t, pool, "posts", rows.quietProblem); got != "dormant" {
		t.Errorf("control: quiet problem = %q, want dormant", got)
	}
	if got := lifecycleStatus(ctx, t, pool, "posts", rows.solvedProblem); got != "solved" {
		t.Errorf("control: problem with an old success = %q, want solved", got)
	}
	want := map[string]int{"approach_abandonment_warning": 1, "auto_solve_warning": 1, "auto_solved": 1}
	got := lifecycleNotifications(ctx, t, pool)
	for typ, n := range want {
		if got[typ] != n {
			t.Errorf("control: %s notifications = %d, want %d (all: %v)", typ, got[typ], n, got)
		}
	}
}
