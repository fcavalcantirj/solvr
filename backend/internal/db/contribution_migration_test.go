package db

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestBuildApproachBody_PreservesAllFields is a pure unit test (no DB): it proves the
// approach -> reply body maps every original field into a labeled Markdown section so none
// of the original text is discarded (task step 2) and that a failed approach stays
// identifiable as failed (task step 6). It runs regardless of DATABASE_URL.
func TestBuildApproachBody_PreservesAllFields(t *testing.T) {
	body := buildApproachBody(
		"Try a bloom filter",
		"Sketch it in Go, benchmark against a map",
		[]string{"keys fit in RAM", "false positives acceptable"},
		[]string{"11111111-1111-1111-1111-111111111111"},
		"failed",
		"Memory blew up at 40M keys",
		"Switched to a cuckoo filter instead",
	)
	for _, want := range []string{
		"**Approach:** Try a bloom filter",
		"**Method:** Sketch it in Go, benchmark against a map",
		"**Assumptions:**",
		"- keys fit in RAM",
		"- false positives acceptable",
		"**Differs from:**",
		"11111111-1111-1111-1111-111111111111",
		"**Status:** failed",
		"**Outcome:**",
		"Memory blew up at 40M keys",
		"**Solution:**",
		"Switched to a cuckoo filter instead",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("approach body missing %q\n---\n%s", want, body)
		}
	}

	// Empty optional fields are omitted, but the required Approach and Status remain so a
	// bare approach is still recognizable (e.g. its status).
	bare := buildApproachBody("Just an angle", "", nil, nil, "starting", "", "")
	if !strings.Contains(bare, "**Approach:** Just an angle") || !strings.Contains(bare, "**Status:** starting") {
		t.Errorf("bare approach body missing required sections: %q", bare)
	}
	if strings.Contains(bare, "**Method:**") || strings.Contains(bare, "**Outcome:**") || strings.Contains(bare, "**Solution:**") || strings.Contains(bare, "**Assumptions:**") {
		t.Errorf("bare approach body should omit empty optional sections: %q", bare)
	}
}

// seedContribution inserts one legacy contribution row of the given kind and returns its id.
func insertApproach(t *testing.T, pool *Pool, ctx context.Context, problemID, authorID, status string, deleted bool) string {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, method, assumptions,
			differs_from, status, outcome, solution, deleted_at)
		VALUES ($1,'agent',$2,$3,'Method text',ARRAY['assume one','assume two'],
			'{}'::uuid[],$4,'Outcome text','Solution text',$5)
		RETURNING id`, problemID, authorID, "Angle for "+authorID, status, deletedAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert approach: %v", err)
	}
	return id
}

// TestContributionMigration_ConvertsAllTypesAndIsResumable is the end-to-end proof for the
// legacy-contribution -> Reply migration: every kind converts with preserved content and
// provenance (steps 1-3), the legacy-id map drives accepted-answer / vote remap (step 4),
// genuine orphans and contributions on non-live posts are reported (step 5), and a second
// run duplicates nothing (step 6).
func TestContributionMigration_ConvertsAllTypesAndIsResumable(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	author := "agent_migc_" + sfx

	// Posts: a live problem, a question, an idea, and a soft-deleted problem.
	probID := insertTestPostWithAuthor(t, pool, ctx, "problem", "migc problem "+sfx, "body", []string{"migc"}, "open", "agent", author)
	qID := insertTestPostWithAuthor(t, pool, ctx, "question", "migc question "+sfx, "body", []string{"migc"}, "open", "agent", author)
	ideaID := insertTestPostWithAuthor(t, pool, ctx, "idea", "migc idea "+sfx, "body", []string{"migc"}, "active", "agent", author)
	delID := insertTestPostDeleted(t, pool, ctx, "problem", "migc deleted "+sfx, "body", []string{"migc"}, "open")

	// Contributions.
	a1 := insertApproach(t, pool, ctx, probID, author, "failed", false)
	a2 := insertApproach(t, pool, ctx, probID, author, "succeeded", false)
	a3 := insertApproach(t, pool, ctx, delID, author, "working", false) // non-deleted approach on a non-live post

	var an1 string
	if err := pool.QueryRow(ctx, `
		INSERT INTO answers (question_id, author_type, author_id, content, is_accepted, upvotes, downvotes)
		VALUES ($1,'agent',$2,'The accepted answer body',TRUE,5,1) RETURNING id`, qID, author).Scan(&an1); err != nil {
		t.Fatalf("insert answer: %v", err)
	}
	// Mark it accepted at the post level so the accepted-answer remap has something to update.
	if _, err := pool.Exec(ctx, `UPDATE posts SET accepted_answer_id=$1 WHERE id=$2`, an1, qID); err != nil {
		t.Fatalf("set accepted_answer_id: %v", err)
	}

	var r1 string
	if err := pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type, upvotes, downvotes)
		VALUES ($1,'agent',$2,'A critique response','critique',3,0) RETURNING id`, ideaID, author).Scan(&r1); err != nil {
		t.Fatalf("insert response: %v", err)
	}

	// Comments: one on the post (top-level), one on approach a1 (child), one orphan.
	cPost := insertComment(t, pool, ctx, "post", probID, author, "comment on the post")
	cChild := insertComment(t, pool, ctx, "approach", a1, author, "comment on the approach")
	orphanTarget := "99999999-9999-9999-9999-999999999999"
	cOrphan := insertComment(t, pool, ctx, "approach", orphanTarget, author, "comment on a ghost")

	// A vote on the failed approach, to prove the vote remap (step 4).
	if _, err := pool.Exec(ctx, `
		INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('approach',$1,'agent',$2,'up',TRUE)`, a1, author); err != nil {
		t.Fatalf("insert vote: %v", err)
	}

	// Step 5 (pre): the orphan comment and the approach on the non-live post are reported.
	pre, err := VerifyContributionMigration(ctx, pool)
	if err != nil {
		t.Fatalf("VerifyContributionMigration (pre): %v", err)
	}
	if ex := pre.ExceptionsFor(cOrphan); len(ex) != 1 || ex[0].Kind != ContribExceptionOrphanComment {
		t.Fatalf("want one orphan_comment for %s, got %+v", cOrphan, ex)
	}
	if ex := pre.ExceptionsFor(a3); len(ex) != 1 || ex[0].Kind != ContribExceptionContributionOnDeletedPost {
		t.Fatalf("want one contribution_on_deleted_post for %s, got %+v", a3, ex)
	}
	if pre.ApproachesByStatus["failed"] < 1 {
		t.Errorf("inventory missed the failed approach: %+v", pre.ApproachesByStatus)
	}

	// Convert (steps 2, 3).
	created, err := MigrateContributions(ctx, pool)
	if err != nil {
		t.Fatalf("MigrateContributions: %v", err)
	}
	// a1,a2,a3,an1,r1,cPost,cChild = 7 migratable; cOrphan is skipped.
	if created < 7 {
		t.Fatalf("MigrateContributions created %d replies, want >= 7", created)
	}

	// The failed approach converts with its status visible and preserved in provenance.
	repA1 := getReplyByLegacy(t, pool, ctx, "approach", a1)
	if !strings.Contains(repA1.body, "**Status:** failed") {
		t.Errorf("failed approach reply body lost its status: %q", repA1.body)
	}
	if got := provField(t, repA1.provenance, "status"); got != "failed" {
		t.Errorf("approach provenance status = %q, want failed", got)
	}
	if repA1.postID != probID {
		t.Errorf("approach reply post_id = %s, want %s", repA1.postID, probID)
	}

	// The accepted answer stays recognizably accepted, with its inline vote count preserved.
	repAn1 := getReplyByLegacy(t, pool, ctx, "answer", an1)
	if provField(t, repAn1.provenance, "is_accepted") != "true" {
		t.Errorf("answer provenance is_accepted not true: %s", repAn1.provenance)
	}
	if repAn1.upvotes != 5 {
		t.Errorf("answer reply upvotes = %d, want 5", repAn1.upvotes)
	}

	// The post comment becomes a top-level reply; the approach comment becomes its child.
	repCPost := getReplyByLegacy(t, pool, ctx, "comment", cPost)
	if repCPost.parentReplyID != nil {
		t.Errorf("post comment reply should be top-level, got parent %v", *repCPost.parentReplyID)
	}
	repCChild := getReplyByLegacy(t, pool, ctx, "comment", cChild)
	if repCChild.parentReplyID == nil || *repCChild.parentReplyID != repA1.id {
		t.Errorf("approach comment reply parent = %v, want %s", repCChild.parentReplyID, repA1.id)
	}
	if repCChild.postID != probID {
		t.Errorf("child reply post_id = %s, want %s (inherited from parent)", repCChild.postID, probID)
	}

	// Step 4: accepted-answer reference and the vote are remapped to the canonical reply.
	if n, err := RemapAcceptedAnswerReferences(ctx, pool); err != nil || n < 1 {
		t.Fatalf("RemapAcceptedAnswerReferences n=%d err=%v", n, err)
	}
	var acc string
	if err := pool.QueryRow(ctx, `SELECT accepted_answer_id FROM posts WHERE id=$1`, qID).Scan(&acc); err != nil {
		t.Fatalf("read accepted_answer_id: %v", err)
	}
	if acc != repAn1.id {
		t.Errorf("accepted_answer_id = %s, want reply %s", acc, repAn1.id)
	}
	if v, _, err := RemapContributionVotesAndReports(ctx, pool); err != nil || v < 1 {
		t.Fatalf("RemapContributionVotesAndReports votes=%d err=%v", v, err)
	}
	var vt, vid string
	if err := pool.QueryRow(ctx, `SELECT target_type, target_id FROM votes WHERE target_id=$1 OR target_id=$2`, repA1.id, a1).Scan(&vt, &vid); err != nil {
		t.Fatalf("read vote: %v", err)
	}
	if vt != "reply" || vid != repA1.id {
		t.Errorf("vote target = (%s,%s), want (reply,%s)", vt, vid, repA1.id)
	}

	// Step 6: resumable — a second run creates nothing and does not duplicate.
	again, err := MigrateContributions(ctx, pool)
	if err != nil {
		t.Fatalf("MigrateContributions (rerun): %v", err)
	}
	if again != 0 {
		t.Errorf("second MigrateContributions created %d, want 0 (idempotent)", again)
	}
	var dup int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM replies WHERE legacy_id IN ($1,$2,$3,$4,$5,$6,$7)`,
		a1, a2, a3, an1, r1, cPost, cChild).Scan(&dup); err != nil {
		t.Fatalf("count migrated replies: %v", err)
	}
	if dup != 7 {
		t.Errorf("migrated reply count = %d, want exactly 7 (no duplicates)", dup)
	}
}

// --- test helpers ---

type migReply struct {
	id            string
	postID        string
	parentReplyID *string
	body          string
	upvotes       int
	provenance    []byte
}

func getReplyByLegacy(t *testing.T, pool *Pool, ctx context.Context, legacyType, legacyID string) migReply {
	t.Helper()
	var r migReply
	err := pool.QueryRow(ctx, `
		SELECT id, post_id, parent_reply_id, body, upvotes, provenance
		FROM replies WHERE legacy_type=$1 AND legacy_id=$2`, legacyType, legacyID).
		Scan(&r.id, &r.postID, &r.parentReplyID, &r.body, &r.upvotes, &r.provenance)
	if err != nil {
		t.Fatalf("getReplyByLegacy(%s,%s): %v", legacyType, legacyID, err)
	}
	return r
}

func insertComment(t *testing.T, pool *Pool, ctx context.Context, targetType, targetID, authorID, content string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ($1,$2,'agent',$3,$4) RETURNING id`, targetType, targetID, authorID, content).Scan(&id); err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	return id
}

func provField(t *testing.T, raw []byte, key string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal provenance: %v (%s)", err, raw)
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s := string(v)
	s = strings.Trim(s, `"`)
	return s
}
