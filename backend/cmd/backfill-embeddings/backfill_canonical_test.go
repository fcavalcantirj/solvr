package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/jackc/pgx/v5"
)

// Task idx 76 steps 3 and 5: the backfill embeds canonical posts and replies only. Answers
// and approaches are replies after the cutover (RemapLegacyRelations copies their vectors),
// so the command reads no legacy table and carries no disposition.

func TestLegacyBackfill_ReadsNoLegacyTable(t *testing.T) {
	src, err := db.ScanLegacySourceDependencies("../..")
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}
	for _, dep := range src {
		if strings.HasPrefix(dep.Key, "code:cmd/backfill-embeddings/") {
			t.Errorf("%s still reads the legacy model", dep.Key)
		}
	}
	for key := range db.LegacyDependencyDispositions {
		if strings.HasPrefix(key, "code:cmd/backfill-embeddings/") {
			t.Errorf("%s carries a disposition but the backfill no longer depends on the legacy model", key)
		}
	}
}

func TestParseContentTypes_RejectsRetiredLegacyTypes(t *testing.T) {
	for _, in := range []string{"answers", "approaches", "posts,answers", "replies, approaches "} {
		types, err := parseContentTypes(in)
		if err == nil {
			t.Errorf("parseContentTypes(%q) = %v, want an error naming the retired type", in, types)
			continue
		}
		if !strings.Contains(err.Error(), "replies") {
			t.Errorf("parseContentTypes(%q) error %q does not point at the replies type", in, err)
		}
	}
	types, err := parseContentTypes("all")
	if err != nil || strings.Join(types, ",") != "posts,replies" {
		t.Fatalf("parseContentTypes(all) = %v, %v; want [posts replies]", types, err)
	}
}

// Integration test on a scratch database migrated to head (needs DATABASE_URL):
//
//	DATABASE_URL="postgres://solvr:solvr_dev@localhost:5435/solvr_test" go test ./cmd/backfill-embeddings/ -count=1 -v
func TestBackfill_EmbedsMigratedContributionsOnceTheLegacyTablesAreGone(t *testing.T) {
	pool, archiveLegacy := newBackfillScratchDatabase(t)
	ctx := context.Background()
	scan := func(sql string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	stored := make([]float32, 1024)
	stored[7] = 1
	storedVec := float32SliceToVectorString(stored)

	agent := "backfill_cutover_agent"
	scan(`INSERT INTO agents (id, display_name) VALUES ($1, 'Backfill Cutover Probe') RETURNING id`, agent)
	post := func(postType, status, title string) string {
		t.Helper()
		return scan(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
			VALUES ($1, $2, 'the description', 'agent', $3, $4) RETURNING id::text`, postType, title, agent, status)
	}
	question := post("question", "open", "Backfill cutover question")
	problem := post("problem", "open", "Backfill cutover problem")
	if _, err := pool.Exec(ctx, `UPDATE posts SET embedding = $1::vector WHERE id = $2`, storedVec, problem); err != nil {
		t.Fatalf("embed the problem: %v", err)
	}
	answer := scan(`INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'agent', $2, 'the answer the legacy backfill never embedded') RETURNING id::text`, question, agent)
	approach := func(angle string) string {
		t.Helper()
		return scan(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, outcome)
			VALUES ($1, 'agent', $2, $3, 'method one', 'succeeded', 'outcome one') RETURNING id::text`, problem, agent, angle)
	}
	bare := approach("angle without a vector")
	embedded := approach("angle with a vector")
	if _, err := pool.Exec(ctx, `UPDATE approaches SET embedding = $1::vector WHERE id = $2`, storedVec, embedded); err != nil {
		t.Fatalf("embed the approach: %v", err)
	}

	inputs := []string{}
	embSvc := &mockEmbeddingService{generateFunc: func(_ context.Context, text string) ([]float32, error) {
		inputs = append(inputs, text)
		v := make([]float32, 1024)
		v[0] = 1
		return v, nil
	}}
	types, err := parseContentTypes("all")
	if err != nil {
		t.Fatalf("parseContentTypes(all): %v", err)
	}
	worker := func(dryRun bool) *backfillWorker {
		return &backfillWorker{db: &pgBackfillDB{pool: pool}, embeddingService: embSvc, batchSize: 1, dryRun: dryRun, contentTypes: types}
	}

	// Before the cutover the legacy rows are not replies yet: nothing of theirs is pending.
	result, err := worker(true).run(ctx)
	if err != nil {
		t.Fatalf("dry run before the cutover: %v", err)
	}
	if result.postsFound != 1 || result.repliesFound != 0 || result.totalFound != 1 {
		t.Fatalf("before the cutover found posts=%d replies=%d total=%d, want 1/0/1", result.postsFound, result.repliesFound, result.totalFound)
	}

	if _, err := db.MigrateContributions(ctx, pool); err != nil {
		t.Fatalf("MigrateContributions: %v", err)
	}
	if _, err := db.RemapLegacyRelations(ctx, pool); err != nil {
		t.Fatalf("RemapLegacyRelations: %v", err)
	}
	replyOf := func(legacyType, legacyID string) (id, body string) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT id::text, body FROM replies WHERE legacy_type = $1 AND legacy_id = $2`,
			legacyType, legacyID).Scan(&id, &body); err != nil {
			t.Fatalf("reply migrated from %s %s: %v", legacyType, legacyID, err)
		}
		return id, body
	}
	answerReply, answerBody := replyOf("answer", answer)
	bareReply, bareBody := replyOf("approach", bare)
	embeddedReply, _ := replyOf("approach", embedded)
	archiveLegacy()

	result, err = worker(false).run(ctx)
	if err != nil {
		t.Fatalf("backfill without the legacy tables: %v", err)
	}
	if result.postsEmbedded != 1 || result.repliesEmbedded != 2 || result.errors != 0 || result.embedded != 3 {
		t.Fatalf("embedded posts=%d replies=%d total=%d errors=%d, want 1/2/3/0",
			result.postsEmbedded, result.repliesEmbedded, result.embedded, result.errors)
	}
	sort.Strings(inputs)
	want := []string{"Backfill cutover question the description", answerBody, bareBody}
	sort.Strings(want)
	if strings.Join(inputs, "\n--\n") != strings.Join(want, "\n--\n") {
		t.Fatalf("embedding inputs = %q, want the pending post and the two migrated reply bodies %q", inputs, want)
	}
	for _, part := range []string{"angle without a vector", "method one", "outcome one"} {
		if !strings.Contains(bareBody, part) {
			t.Errorf("the migrated approach body %q lacks %q, so its vector would not cover it", bareBody, part)
		}
	}
	var dims int
	if err := pool.QueryRow(ctx, `SELECT MIN(vector_dims(embedding)) FROM replies WHERE id IN ($1, $2)`,
		answerReply, bareReply).Scan(&dims); err != nil || dims != 1024 {
		t.Errorf("migrated replies embedded with %d dims (%v), want 1024", dims, err)
	}
	if kept := scan(`SELECT (embedding = $1::vector)::text FROM replies WHERE id = $2`, storedVec, embeddedReply); kept != "true" {
		t.Error("the reply whose vector the cutover copied was embedded again")
	}
	pg := &pgBackfillDB{pool: pool}
	posts, err := pg.CountPostsWithoutEmbedding(ctx)
	if err != nil || posts != 0 {
		t.Errorf("posts still pending = %d, %v", posts, err)
	}
	replies, err := pg.CountRepliesWithoutEmbedding(ctx)
	if err != nil || replies != 0 {
		t.Errorf("replies still pending = %d, %v", replies, err)
	}
}

// newBackfillScratchDatabase creates an empty database next to DATABASE_URL's, applies the up
// migrations below the legacy archive migration and returns a pool on it; the database is
// dropped when the test ends. archiveLegacy applies the legacy archive migration and every
// later one, which move the legacy contribution tables (db.LegacyTables) out of public.
func newBackfillScratchDatabase(t *testing.T) (*db.Pool, func()) {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	name := fmt.Sprintf("solvr_scratch_backfill_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 30*time.Second)
		defer cc()
		if _, err := admin.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		admin.Close(c)
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect scratch: %v", err)
	}
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		conn.Close(ctx)
		t.Fatalf("list migrations: %d files, %v", len(files), err)
	}
	sort.Strings(files)
	split := len(files)
	for i, f := range files {
		if strings.HasSuffix(f, "_legacy_archive.up.sql") {
			split = i
		}
	}
	if split == len(files) {
		conn.Close(ctx)
		t.Fatal("no *_legacy_archive.up.sql migration")
	}
	apply := func(conn *pgx.Conn, files []string) {
		t.Helper()
		for _, f := range files {
			sql, err := os.ReadFile(f)
			if err == nil {
				_, err = conn.Exec(context.Background(), string(sql))
			}
			if err != nil {
				t.Fatalf("apply %s: %v", filepath.Base(f), err)
			}
		}
	}
	apply(conn, files[:split])
	conn.Close(ctx)

	pool, err := db.NewPool(ctx, u.String())
	if err != nil {
		t.Fatalf("pool on scratch database: %v", err)
	}
	t.Cleanup(pool.Close) // registered after the drop, so it runs before it
	scratchURL := u.String()
	return pool, func() {
		t.Helper()
		conn, err := pgx.Connect(context.Background(), scratchURL)
		if err != nil {
			t.Fatalf("connect scratch: %v", err)
		}
		defer conn.Close(context.Background())
		apply(conn, files[split:])
	}
}
