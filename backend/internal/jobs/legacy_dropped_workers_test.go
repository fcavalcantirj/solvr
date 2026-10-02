package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api"
	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The legacy contribution tables are dropped from a scratch database migrated to head,
// and every scheduled job runs there wired the way cmd/api/main.go wires it (only the
// external services are faked). A query tracer records every database error, including
// the ones a job only logs, so a dependency no inventory found shows up here.

type tracedError struct {
	Code    string
	Message string
	SQL     string
}

type dbErrorTracer struct {
	mu         sync.Mutex
	statements []string
	errs       []tracedError
}

type tracedSQLKey struct{}

func (tr *dbErrorTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.statements = append(tr.statements, data.SQL)
	return context.WithValue(ctx, tracedSQLKey{}, data.SQL)
}

func (tr *dbErrorTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	var pgErr *pgconn.PgError
	if !errors.As(data.Err, &pgErr) {
		return
	}
	sql, _ := ctx.Value(tracedSQLKey{}).(string)
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.errs = append(tr.errs, tracedError{Code: pgErr.Code, Message: pgErr.Message, SQL: sql})
}

// take returns what was traced since the previous call and starts a new window.
func (tr *dbErrorTracer) take() ([]string, []tracedError) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	statements, errs := tr.statements, tr.errs
	tr.statements, tr.errs = nil, nil
	return statements, errs
}

func (tr *dbErrorTracer) saw(fragment string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, s := range tr.statements {
		if strings.Contains(s, fragment) {
			return true
		}
	}
	return false
}

var droppedLegacyObjectRe = regexp.MustCompile(`\b(` + strings.Join(db.LegacyTables, "|") + `)\b`)

// missingLegacyObject reports whether an error is a statement reaching for a dropped
// legacy table (or its row type, or a function that went with it).
func missingLegacyObject(e tracedError) (string, bool) {
	switch e.Code {
	case "42P01", "42704", "42883": // undefined_table, undefined_object, undefined_function
		if m := droppedLegacyObjectRe.FindStringSubmatch(e.Message); m != nil {
			return m[1], true
		}
	}
	return "", false
}

type legacyDroppedDatabase struct {
	pool   *db.Pool
	tracer *dbErrorTracer
	url    string
	// dependents are the objects outside the legacy tables that a plain DROP TABLE named
	// as depending on them (the DETAIL of SQLSTATE 2BP01).
	dependents []string
}

// newMigratedScratchURL creates a scratch database named <prefix><nanos>, applies every up
// migration in order, checks the legacy tables are there and returns its URL. The
// database is dropped when the test ends.
func newMigratedScratchURL(t *testing.T, prefix string) string {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set, skipping legacy-dropped database probe")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	name := fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		if _, err := admin.Exec(cctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		admin.Close(cctx)
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	scratchURL := u.String()

	conn, err := pgx.Connect(ctx, scratchURL)
	if err != nil {
		t.Fatalf("connect scratch: %v", err)
	}
	defer conn.Close(ctx)

	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("list migrations: %d files, %v", len(files), err)
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	for _, table := range db.LegacyTables {
		var present bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.'||$1) IS NOT NULL", table).Scan(&present); err != nil || !present {
			t.Fatalf("migrated scratch database must hold legacy table %s before the drop (present=%v, err=%v)", table, present, err)
		}
	}
	return scratchURL
}

// newMigratedDatabase returns a traced pool on a scratch database migrated to head that
// still holds the legacy tables: the state between the cutover deploy and schema cleanup.
func newMigratedDatabase(t *testing.T) (*db.Pool, *dbErrorTracer) {
	t.Helper()
	scratchURL := newMigratedScratchURL(t, "solvr_legacy_present_")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tracer := &dbErrorTracer{}
	pool, err := db.NewPool(ctx, scratchURL, db.WithQueryTracer(tracer))
	if err != nil {
		t.Fatalf("open traced pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, tracer
}

// newLegacyDroppedDatabase creates a scratch database, applies every up migration in
// order, runs beforeDrop on the fully migrated schema, drops the legacy tables and returns
// a traced pool on it. The database is dropped when the test ends.
func newLegacyDroppedDatabase(t *testing.T, beforeDrop ...func(ctx context.Context, conn *pgx.Conn)) *legacyDroppedDatabase {
	t.Helper()
	scratchURL := newMigratedScratchURL(t, "solvr_legacy_dropped_")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, err := pgx.Connect(ctx, scratchURL)
	if err != nil {
		t.Fatalf("connect scratch: %v", err)
	}
	defer conn.Close(ctx)

	for _, hook := range beforeDrop {
		hook(ctx, conn)
	}

	d := &legacyDroppedDatabase{tracer: &dbErrorTracer{}, url: scratchURL}
	list := strings.Join(db.LegacyTables, ", ")
	_, err = conn.Exec(ctx, "DROP TABLE "+list)
	var pgErr *pgconn.PgError
	switch {
	case err == nil:
	case errors.As(err, &pgErr) && pgErr.Code == "2BP01": // dependent_objects_still_exist
		for line := range strings.SplitSeq(pgErr.Detail, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				d.dependents = append(d.dependents, line)
			}
		}
		if _, err := conn.Exec(ctx, "DROP TABLE "+list+" CASCADE"); err != nil {
			t.Fatalf("drop legacy tables with cascade: %v", err)
		}
	default:
		t.Fatalf("drop legacy tables: %v", err)
	}
	for _, table := range db.LegacyTables {
		var present bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('public.'||$1) IS NOT NULL", table).Scan(&present); err != nil || present {
			t.Fatalf("legacy table %s must be gone (present=%v, err=%v)", table, present, err)
		}
	}

	d.pool, err = db.NewPool(ctx, scratchURL, db.WithQueryTracer(d.tracer))
	if err != nil {
		t.Fatalf("open traced pool: %v", err)
	}
	t.Cleanup(d.pool.Close)
	return d
}

// dependentKeyRes map one DETAIL line of a refused DROP TABLE to a registry key.
var dependentKeyRes = []struct {
	re     *regexp.Regexp
	format func(m []string) string
}{
	{regexp.MustCompile(`^function (\w+)\(`), func(m []string) string { return "function:" + m[1] }},
	{regexp.MustCompile(`^(?:materialized )?view (\w+) `), func(m []string) string { return "view:" + m[1] }},
	{regexp.MustCompile(`^constraint (\w+) on table (\w+) `), func(m []string) string { return "fk:" + m[2] + "." + m[1] }},
}

func dependentKey(line string) (string, bool) {
	for _, r := range dependentKeyRes {
		if m := r.re.FindStringSubmatch(line); m != nil {
			return r.format(m), true
		}
	}
	return "", false
}

type probeIPFS struct{}

func (probeIPFS) Add(context.Context, io.Reader) (string, error) { return "bafyprobe", nil }
func (probeIPFS) Pin(context.Context, string) error              { return nil }
func (probeIPFS) NodeInfo(context.Context) (*services.NodeInfoResult, error) {
	return &services.NodeInfoResult{PeerID: "probe"}, nil
}

type probeTranslator struct{}

func (probeTranslator) TranslateContent(_ context.Context, in services.TranslationInput) (*services.TranslationResult, error) {
	return &services.TranslationResult{Title: "Translated " + in.Title, Description: "Translated " + in.Description}, nil
}

type probeModeration struct{}

func (probeModeration) ModerateContent(context.Context, handlers.ModerationInput) (*handlers.ModerationResult, error) {
	return &handlers.ModerationResult{Approved: true, LanguageDetected: "en"}, nil
}

type probeWorker struct {
	key string
	run func(ctx context.Context, t *testing.T)
}

// scheduledWorkers wires every job cmd/api/main.go schedules against the given pool.
func scheduledWorkers(pool *db.Pool, tracer *dbErrorTracer) []probeWorker {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifRepo := db.NewNotificationsRepository(pool)
	postRepo := db.NewPostRepository(pool)
	return []probeWorker{
		{"job:CleanupJob", func(ctx context.Context, _ *testing.T) {
			job := jobs.NewCleanupJob(db.NewClaimTokenRepository(pool)).WithIdempotencyPruner(db.NewIdempotencyRepository(pool))
			_, _ = job.CleanupExpiredTokens(ctx)
			_, _ = job.PruneExpiredIdempotencyKeys(ctx)
		}},
		{"job:CrystallizationJob", func(ctx context.Context, t *testing.T) {
			repo := db.NewPostCrystallizationRepository(pool)
			svc := services.NewPostCrystallizationService(repo, postRepo, db.NewReplyRepository(pool), probeIPFS{}, probeIPFS{})
			result := jobs.NewCrystallizationJob(repo, svc, jobs.DefaultCrystallizationStabilityPeriod).RunOnce(ctx)
			if result.Crystallized != 1 {
				t.Errorf("crystallization job = %+v, want the 1 seeded stable post crystallized", result)
			}
		}},
		{"job:TranslationJob", func(ctx context.Context, t *testing.T) {
			trigger := handlers.NewModerationTrigger(probeModeration{}, postRepo, logger)
			trigger.SetCommentRepo(db.NewModerationReplyWriter(pool))
			trigger.SetNotificationService(api.NewModerationNotificationService(notifRepo.Create))
			translated, _ := jobs.NewTranslationJob(postRepo, postRepo, probeTranslator{}, trigger, jobs.DefaultTranslationBatchSize, 0).RunOnce(ctx)
			if translated != 1 {
				t.Errorf("translation job translated %d posts, want the 1 seeded draft", translated)
				return
			}
			// Moderation runs asynchronously; its last step is the author notification.
			deadline := time.Now().Add(15 * time.Second)
			for !tracer.saw("INSERT INTO notifications") {
				if time.Now().After(deadline) {
					t.Errorf("translation moderation never reached the author notification")
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			if !tracer.saw("INSERT INTO replies") {
				t.Errorf("translation moderation did not record its verdict as a reply")
			}
		}},
		{"job:HealthCheckJob", func(ctx context.Context, _ *testing.T) {
			svc := services.NewHealthCheckerService(pool, probeIPFS{})
			jobs.NewHealthCheckJob(svc, db.NewServiceCheckRepository(pool)).RunOnce(ctx)
		}},
		{"job:PresenceReaperJob", func(ctx context.Context, _ *testing.T) {
			registry := hub.NewPresenceRegistry()
			mgr := hub.NewHubManager(ctx, registry, logger, 0)
			jobs.NewPresenceReaperJob(db.NewAgentPresenceRepository(pool), db.NewRoomRepository(pool), registry, mgr).RunOnce(ctx)
		}},
		{"job:SearchDocumentJob", func(ctx context.Context, t *testing.T) {
			// The seeded posts and reply carry no vector.
			job := jobs.NewSearchDocumentJob(db.NewSearchDocumentQueue(pool), newTextEmbedder(), jobs.DefaultSearchDocumentBatchSize)
			if result, err := job.RunOnce(ctx); err != nil || result.Embedded == 0 || result.Failed != 0 {
				t.Errorf("search document job = %+v, %v; want the seeded rows embedded", result, err)
			}
		}},
		{"job:WebhookDeliveryJob", func(ctx context.Context, t *testing.T) {
			// Nothing is queued in the seed: the claim runs its SQL and finds nothing due.
			if run, err := api.NewWebhookDeliveryJob(pool, nil).RunOnce(ctx); err != nil || run.Claimed != 0 {
				t.Errorf("webhook delivery job = %+v, %v; want a clean run with nothing due", run, err)
			}
		}},
		{"job:CounterReconcileJob", func(ctx context.Context, t *testing.T) {
			job := jobs.NewCounterReconcileJob(db.NewCounterReconciler(pool), jobs.DefaultCounterReconcileLimit)
			if run, err := job.RunOnce(ctx); err != nil || run.Busy || len(run.Counters) == 0 {
				t.Errorf("counter reconcile job = %+v, %v; want every counter reconciled", run, err)
			}
		}},
	}
}

// seedScheduledWorkerData seeds the rows the data-gated worker paths need and returns the
// seeding agent's id.
func seedScheduledWorkerData(ctx context.Context, t *testing.T, pool *db.Pool) string {
	t.Helper()
	// Data the data-gated paths need: one draft awaiting translation by a real agent.
	agentID := fmt.Sprintf("probe_agent_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, 'Probe Agent')`, agentID); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, original_language)
		VALUES ('question', 'Titulo da pergunta de teste', 'Descricao longa o bastante para a pergunta de teste',
		        'agent', $1, 'draft', 'pt')`, agentID); err != nil {
		t.Fatalf("seed translation draft: %v", err)
	}
	// And one stable public post with an agent reply, so crystallization runs past its candidate list.
	var crystalPost string
	if err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status,
		                   publication_state, moderation_state, visibility, created_at, updated_at)
		VALUES ('post', 'Probe crystallization post', 'A stable public post with one reply', 'agent', $1, 'open',
		        'published', 'approved', 'public', NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days')
		RETURNING id::text`, agentID).Scan(&crystalPost); err != nil {
		t.Fatalf("seed crystallization post: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'Probe reply', NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days')`,
		crystalPost, agentID); err != nil {
		t.Fatalf("seed crystallization reply: %v", err)
	}
	return agentID
}

// scheduledJobKeys returns the jobs the non-test sources schedule (jobs.New<Name>Job), i.e.
// what cmd/api/main.go runs.
func scheduledJobKeys(t *testing.T) map[string]bool {
	t.Helper()
	src, err := db.ScanLegacySourceDependencies("../..")
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}
	scheduled := map[string]bool{}
	for _, dep := range src {
		if dep.Kind == "job" {
			scheduled[dep.Key] = true
		}
	}
	return scheduled
}

// Every scheduled job runs against a database without the legacy tables. A job that
// reaches for a dropped legacy relation must carry a pending (non-keep, not done)
// disposition: that is the dependency the migration still owes. A job recorded as keep
// or done must run clean. Objects a plain DROP TABLE refuses to drop without CASCADE
// must already be replaced (done) or reviewed (keep).
func TestLegacyDroppedDatabase_ScheduledJobsExposeOnlyRegisteredDependencies(t *testing.T) {
	d := newLegacyDroppedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Logf("plain DROP TABLE refused because of: %q", d.dependents)
	for _, line := range d.dependents {
		key, ok := dependentKey(line)
		if !ok {
			t.Errorf("DROP TABLE named a dependent this probe cannot map to a registry key: %q", line)
			continue
		}
		disp, ok := db.LegacyDependencyDispositions[key]
		switch {
		case !ok:
			t.Errorf("dropping the legacy tables destroys %s (%q), which has no disposition", key, line)
		case disp.Action != db.LegacyActionKeep && !disp.Done:
			t.Errorf("dropping the legacy tables destroys %s, whose %s is still pending", key, disp.Action)
		}
	}

	seedScheduledWorkerData(ctx, t, d.pool)
	d.tracer.take()

	scheduled := scheduledJobKeys(t)

	for _, w := range scheduledWorkers(d.pool, d.tracer) {
		if !scheduled[w.key] {
			t.Errorf("%s is exercised by this probe but the sources do not schedule it", w.key)
		}
		delete(scheduled, w.key)
		w.run(ctx, t)
		statements, errs := d.tracer.take()
		if len(statements) == 0 {
			t.Errorf("%s ran no SQL: the probe exercised nothing", w.key)
		}
		missing := map[string]bool{}
		for _, e := range errs {
			if rel, ok := missingLegacyObject(e); ok {
				missing[rel] = true
				continue
			}
			t.Errorf("%s hit a database error unrelated to the dropped tables: %s %s (sql: %s)", w.key, e.Code, e.Message, e.SQL)
		}
		var rels []string
		for rel := range missing {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		t.Logf("%s: %d statements, dropped legacy relations reached: %v", w.key, len(statements), rels)

		disp, ok := db.LegacyDependencyDispositions[w.key]
		if !ok {
			t.Errorf("%s has no disposition", w.key)
			continue
		}
		if len(rels) == 0 {
			continue
		}
		if disp.Action == db.LegacyActionKeep || disp.Done {
			t.Errorf("%s reaches dropped legacy relations %v but is recorded as %s (done=%v): a hidden legacy dependency",
				w.key, rels, disp.Action, disp.Done)
		}
	}
	for key := range scheduled {
		t.Errorf("%s is scheduled in the sources but not exercised by this probe", key)
	}

	// The drop and the tracing work: a statement on each dropped table is traced as reaching
	// it. The jobs that still reached the legacy tables used to show this; every scheduled
	// job now runs clean, so it is checked directly.
	for _, table := range db.LegacyTables {
		_, _ = d.pool.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 1")
		_, errs := d.tracer.take()
		reached := false
		for _, e := range errs {
			if rel, ok := missingLegacyObject(e); ok && rel == table {
				reached = true
			}
		}
		if !reached {
			t.Errorf("a statement on dropped legacy table %s was not traced as reaching it (errors %+v): the drop or the tracing is not working", table, errs)
		}
	}
}
