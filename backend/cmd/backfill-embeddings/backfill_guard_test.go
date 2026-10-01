package main

import (
	"context"
	"sort"
	"testing"
	"time"
)

// The backfill is the rebuild path of the search documents (idx 77, migration 000122): it
// embeds the live posts and replies that have no vector. Its write lands only on the text
// the vector was computed from, only where no vector exists yet, and never moves updated_at
// (the If-Match validator). Measured on HEAD before it (idx 77 slice 6 spike): the post write
// moved updated_at, so an owner's edit from the version read before the backfill got 412, and
// an edit landing while the embedder ran ended with the old text's vector on the new text.

func TestBackfillWorker_ARowChangedSinceItWasReadIsSkippedNotCounted(t *testing.T) {
	mdb := &mockDB{
		posts:          []postRow{{ID: "p1", Title: "a"}, {ID: "p2", Title: "b"}, {ID: "p3", Title: "c"}},
		replies:        []replyRow{{ID: "r1", Body: "x"}, {ID: "r2", Body: "y"}},
		changedPosts:   map[string]bool{"p2": true},
		changedReplies: map[string]bool{"r1": true},
	}
	embSvc := &mockEmbeddingService{}
	worker := &backfillWorker{db: mdb, embeddingService: embSvc, batchSize: 2, contentTypes: []string{"posts", "replies"}}

	result, err := worker.run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.postsEmbedded != 2 || result.postsSkipped != 1 || result.repliesEmbedded != 1 || result.repliesSkipped != 1 {
		t.Fatalf("posts embedded=%d skipped=%d, replies embedded=%d skipped=%d; want 2/1/1/1",
			result.postsEmbedded, result.postsSkipped, result.repliesEmbedded, result.repliesSkipped)
	}
	if result.embedded != 3 || result.skipped != 2 || result.errors != 0 {
		t.Fatalf("totals embedded=%d skipped=%d errors=%d, want 3/2/0", result.embedded, result.skipped, result.errors)
	}
	if embSvc.callCount != 5 {
		t.Fatalf("embedder called %d times, want 5 (each row once; a skipped row is not retried this run)", embSvc.callCount)
	}
}

// Integration test on a scratch database migrated to head (needs DATABASE_URL):
//
//	DATABASE_URL="postgres://solvr:solvr_dev@localhost:5435/solvr_test" go test ./cmd/backfill-embeddings/ -count=1 -v
func TestPGBackfillDB_WritesAVectorOnlyOntoTheTextItWasComputedFrom(t *testing.T) {
	pool, _ := newBackfillScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	scan := func(sql string, args ...any) string {
		t.Helper()
		var v string
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	agent := scan(`INSERT INTO agents (id, display_name) VALUES ('backfill_guard_agent', 'Backfill Guard') RETURNING id`)
	post := func(title string) string {
		t.Helper()
		return scan(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, updated_at)
			VALUES ('question', $1, 'the description', 'agent', $2, 'open', NOW() - INTERVAL '1 day') RETURNING id::text`, title, agent)
	}
	edited, taken, fresh, gone := post("Text A the backfill read"), post("A post another writer embeds"),
		post("A post only the backfill embeds"), post("A post deleted after the read")
	reply := func(body string) string {
		t.Helper()
		return scan(`INSERT INTO replies (post_id, author_type, author_id, body, updated_at)
			VALUES ($1, 'agent', $2, $3, NOW() - INTERVAL '1 day') RETURNING id::text`, fresh, agent, body)
	}
	editedReply, takenReply, freshReply := reply("reply text A the backfill read"), reply("a reply another writer embeds"),
		reply("a reply only the backfill embeds")

	pg := &pgBackfillDB{pool: pool}
	postRows, err := pg.GetPostsWithoutEmbedding(ctx, 100, 0)
	if err != nil {
		t.Fatalf("GetPostsWithoutEmbedding: %v", err)
	}
	replyRows, err := pg.GetRepliesWithoutEmbedding(ctx, 100, 0)
	if err != nil {
		t.Fatalf("GetRepliesWithoutEmbedding: %v", err)
	}
	readPost, readReply := map[string]postRow{}, map[string]replyRow{}
	for _, p := range postRows {
		readPost[p.ID] = p
	}
	for _, r := range replyRows {
		readReply[r.ID] = r
	}
	if len(readPost) != 4 || len(readReply) != 3 {
		t.Fatalf("read %d posts and %d replies pending, want 4 and 3", len(readPost), len(readReply))
	}

	// While the embedder runs: one row's text changes, another writer embeds one row, one is deleted.
	exec(`UPDATE posts SET title = 'Text B written while the embedder ran' WHERE id = $1`, edited)
	exec(`UPDATE replies SET body = 'reply text B written while the embedder ran' WHERE id = $1`, editedReply)
	other := make([]float32, 1024)
	other[9] = 1
	exec(`UPDATE posts SET embedding = $2::vector WHERE id = $1`, taken, float32SliceToVectorString(other))
	exec(`UPDATE replies SET embedding = $2::vector WHERE id = $1`, takenReply, float32SliceToVectorString(other))
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, gone)
	stamps := func() string {
		t.Helper()
		return scan(`SELECT string_agg(id::text || ':' || updated_at::text, ',' ORDER BY id::text)
			FROM (SELECT id, updated_at FROM posts UNION ALL SELECT id, updated_at FROM replies) s`)
	}
	before := stamps()

	vec := make([]float32, 1024)
	vec[3] = 1
	for id, want := range map[string]bool{edited: false, taken: false, gone: false, fresh: true} {
		written, err := pg.UpdatePostEmbedding(ctx, readPost[id], vec)
		if err != nil || written != want {
			t.Errorf("UpdatePostEmbedding(%s) = %v, %v; want %v", id, written, err, want)
		}
	}
	for id, want := range map[string]bool{editedReply: false, takenReply: false, freshReply: true} {
		written, err := pg.UpdateReplyEmbedding(ctx, readReply[id], vec)
		if err != nil || written != want {
			t.Errorf("UpdateReplyEmbedding(%s) = %v, %v; want %v", id, written, err, want)
		}
	}
	vectorOf := func(table, id string) string {
		t.Helper()
		return scan(`SELECT COALESCE(embedding::text, 'NULL') FROM `+table+` WHERE id = $1`, id)
	}
	ours, theirs := float32SliceToVectorString(vec), float32SliceToVectorString(other)
	for _, c := range []struct{ table, id, want string }{
		{"posts", edited, "NULL"}, {"posts", taken, theirs}, {"posts", gone, "NULL"}, {"posts", fresh, ours},
		{"replies", editedReply, "NULL"}, {"replies", takenReply, theirs}, {"replies", freshReply, ours},
	} {
		if got := vectorOf(c.table, c.id); got != c.want {
			t.Errorf("%s %s stores %.20s..., want %.20s...", c.table, c.id, got, c.want)
		}
	}
	if after := stamps(); after != before {
		t.Errorf("the backfill moved updated_at:\nbefore %s\nafter  %s", before, after)
	}

	// The rows still pending are exactly the drift check's, and a run embeds them from the
	// current text; a second run (a replayed worker) finds nothing and calls no embedder.
	pending := func() []string {
		t.Helper()
		var out []string
		posts, _ := pg.GetPostsWithoutEmbedding(ctx, 100, 0)
		for _, p := range posts {
			out = append(out, "post:"+p.ID)
		}
		replies, _ := pg.GetRepliesWithoutEmbedding(ctx, 100, 0)
		for _, r := range replies {
			out = append(out, "reply:"+r.ID)
		}
		sort.Strings(out)
		return out
	}
	drift := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT kind || ':' || id::text FROM search_document_drift()`)
		if err != nil {
			t.Fatalf("search_document_drift: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatalf("scan drift: %v", err)
			}
			out = append(out, s)
		}
		sort.Strings(out)
		return out
	}
	if want, got := []string{"post:" + edited, "reply:" + editedReply}, pending(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("pending after the guarded writes = %v, want %v", got, want)
	}
	if got, want := drift(), pending(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("search_document_drift() = %v, the backfill's pending rows = %v", got, want)
	}

	inputs := []string{}
	embSvc := &mockEmbeddingService{generateFunc: func(_ context.Context, text string) ([]float32, error) {
		inputs = append(inputs, text)
		return vec, nil
	}}
	worker := &backfillWorker{db: pg, embeddingService: embSvc, batchSize: 10, contentTypes: []string{"posts", "replies"}}
	result, err := worker.run(ctx)
	if err != nil || result.embedded != 2 || result.skipped != 0 || result.errors != 0 {
		t.Fatalf("run = embedded %d skipped %d errors %d, %v; want 2/0/0", result.embedded, result.skipped, result.errors, err)
	}
	sort.Strings(inputs)
	if len(inputs) != 2 || inputs[0] != "Text B written while the embedder ran the description" ||
		inputs[1] != "reply text B written while the embedder ran" {
		t.Fatalf("embedded texts = %q, want the current text of each", inputs)
	}
	calls := embSvc.callCount
	result, err = worker.run(ctx)
	if err != nil || result.embedded != 0 || result.totalFound != 0 || embSvc.callCount != calls {
		t.Fatalf("second run = found %d embedded %d, embedder calls %d -> %d, %v; want nothing",
			result.totalFound, result.embedded, calls, embSvc.callCount, err)
	}
	if got := drift(); len(got) != 0 {
		t.Fatalf("search_document_drift() after the rebuild = %v, want empty", got)
	}
	if after := stamps(); after != before {
		t.Errorf("the rebuild moved updated_at:\nbefore %s\nafter  %s", before, after)
	}
}
