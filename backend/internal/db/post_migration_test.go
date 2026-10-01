package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// insertLegacyPost inserts a post directly with a chosen legacy status, visibility and
// deletion, and deliberately wrong canonical states ('archived','rejected') so the
// migration verifier and remapper have something to detect and fix. It carries
// distinct content plus problem-only provenance (success_criteria, weight) so the
// content-fingerprint checks are meaningful. Returns the new post ID.
func insertLegacyPost(t *testing.T, pool *Pool, ctx context.Context, i int, sfx, status, visibility string, deleted bool) string {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id,
			status, visibility, publication_state, moderation_state, deleted_at,
			success_criteria, weight)
		VALUES ('post', $1, $2, $3, 'agent', $4, $5, $6, 'archived', 'rejected', $7, $8, $9)
		RETURNING id`,
		fmt.Sprintf("migtest %s #%d", sfx, i),
		fmt.Sprintf("migration verification body %s #%d", sfx, i),
		[]string{"migtest_" + sfx},
		authorAgent(ctx, t, pool, "agent_migtest_"+sfx),
		status, visibility, deletedAt,
		[]string{"crit-a", "crit-b"}, 3,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert legacy post (status=%s): %v", status, err)
	}
	return id
}

// TestPostMigration_RemapFixesStatesAndPreservesContent proves the legacy->canonical
// post migration is lossless: every retained legacy status remaps to the contract
// (publication_state, moderation_state) pair, no content-bearing field changes, and
// soft-deleted and family rows stay out of public eligibility (task BART: migrate every
// legacy post; steps 3, 5, 6).
func TestPostMigration_RemapFixesStatesAndPreservesContent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	type seed struct {
		status     string
		visibility string
		deleted    bool
		wantPub    models.PublicationState
		wantMod    models.ModerationState
	}
	seeds := []seed{
		{"draft", "public", false, models.PublicationDraft, models.ModerationPending},
		{"pending_review", "public", false, models.PublicationDraft, models.ModerationPending},
		{"rejected", "public", false, models.PublicationDraft, models.ModerationRejected},
		{"closed", "public", false, models.PublicationArchived, models.ModerationApproved},
		{"open", "public", false, models.PublicationPublished, models.ModerationApproved},
		{"solved", "public", false, models.PublicationPublished, models.ModerationApproved},
		{"answered", "public", false, models.PublicationPublished, models.ModerationApproved},
		{"active", "family", false, models.PublicationPublished, models.ModerationApproved},
		{"open", "public", true, models.PublicationPublished, models.ModerationApproved},  // soft-deleted
		{"open", "family", false, models.PublicationPublished, models.ModerationApproved}, // family
	}

	ids := make([]string, len(seeds))
	wantStates := map[string][2]string{}
	hiddenIDs := map[string]bool{} // must never be publicly eligible
	for i, s := range seeds {
		id := insertLegacyPost(t, pool, ctx, i, sfx, s.status, s.visibility, s.deleted)
		ids[i] = id
		wantStates[id] = [2]string{string(s.wantPub), string(s.wantMod)}
		if s.deleted || s.visibility == "family" {
			hiddenIDs[id] = true
		}
	}

	// Baseline content fingerprints, taken before the migration (step 6 "before").
	before, err := PostContentFingerprints(ctx, pool, ids...)
	if err != nil {
		t.Fatalf("PostContentFingerprints before: %v", err)
	}
	if len(before) != len(ids) {
		t.Fatalf("fingerprinted %d posts, want %d", len(before), len(ids))
	}

	// RED: every seeded row was inserted with wrong states, so the verifier must flag
	// exactly one state mismatch per seeded row.
	pre, err := VerifyPostMigration(ctx, pool)
	if err != nil {
		t.Fatalf("VerifyPostMigration (pre): %v", err)
	}
	preEx := pre.ExceptionsFor(ids...)
	if len(preEx) != len(ids) {
		t.Fatalf("pre-migration exceptions for seeded rows = %d, want %d: %+v", len(preEx), len(ids), preEx)
	}
	for _, e := range preEx {
		if e.Kind != PostExceptionStateMismatch {
			t.Errorf("unexpected exception kind %q for %s: %s", e.Kind, e.PostID, e.Detail)
		}
	}
	if pre.Total < len(ids) {
		t.Errorf("report Total = %d, want >= %d", pre.Total, len(ids))
	}

	// Apply the migration operation (idempotent, content-preserving).
	n, err := RemapPostStates(ctx, pool)
	if err != nil {
		t.Fatalf("RemapPostStates: %v", err)
	}
	if n < int64(len(ids)) {
		t.Errorf("RemapPostStates updated %d rows, want >= %d", n, len(ids))
	}

	// GREEN: no seeded row has any exception now.
	post, err := VerifyPostMigration(ctx, pool)
	if err != nil {
		t.Fatalf("VerifyPostMigration (post): %v", err)
	}
	if ex := post.ExceptionsFor(ids...); len(ex) != 0 {
		t.Fatalf("post-migration exceptions for seeded rows = %+v, want none", ex)
	}

	// Step 3: each seeded row now carries the exact contract mapping.
	for id, want := range wantStates {
		var pub, mod string
		if err := pool.QueryRow(ctx, `SELECT publication_state, moderation_state FROM posts WHERE id = $1`, id).Scan(&pub, &mod); err != nil {
			t.Fatalf("read states for %s: %v", id, err)
		}
		if pub != want[0] || mod != want[1] {
			t.Errorf("post %s states = (%s,%s), want (%s,%s)", id, pub, mod, want[0], want[1])
		}
	}

	// Step 6: no content-bearing field changed across the migration.
	after, err := PostContentFingerprints(ctx, pool, ids...)
	if err != nil {
		t.Fatalf("PostContentFingerprints after: %v", err)
	}
	for _, id := range ids {
		if before[id] != after[id] {
			t.Errorf("content fingerprint changed for %s: before=%s after=%s", id, before[id], after[id])
		}
	}

	// Step 5: soft-deleted and family rows are excluded from public eligibility.
	eligible := map[string]bool{}
	rows, err := pool.Query(ctx, `
		SELECT id FROM posts
		WHERE id = ANY($1)
		  AND publication_state = 'published' AND moderation_state = 'approved'
		  AND visibility = 'public' AND deleted_at IS NULL`, ids)
	if err != nil {
		t.Fatalf("public-eligible query: %v", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("scan eligible id: %v", err)
		}
		eligible[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("eligible rows: %v", err)
	}
	for id := range hiddenIDs {
		if eligible[id] {
			t.Errorf("hidden (deleted/family) post %s became publicly eligible", id)
		}
	}
}

// TestPostContentFingerprint_SensitiveToContentNotStates proves the fingerprint used to
// certify a lossless migration reacts to real content changes but ignores the migration's
// own publication_state/moderation_state writes (task BART step 6).
func TestPostContentFingerprint_SensitiveToContentNotStates(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	id := insertLegacyPost(t, pool, ctx, 0, sfx, "open", "public", false)

	fps1, err := PostContentFingerprints(ctx, pool, id)
	if err != nil {
		t.Fatalf("fingerprint 1: %v", err)
	}

	// Changing only the canonical states must NOT change the fingerprint.
	if _, err := pool.Exec(ctx, `UPDATE posts SET publication_state='published', moderation_state='approved' WHERE id=$1`, id); err != nil {
		t.Fatalf("update states: %v", err)
	}
	fps2, err := PostContentFingerprints(ctx, pool, id)
	if err != nil {
		t.Fatalf("fingerprint 2: %v", err)
	}
	if fps1[id] != fps2[id] {
		t.Errorf("fingerprint changed after a states-only update: %s -> %s", fps1[id], fps2[id])
	}

	// Changing a content field (title) MUST change the fingerprint.
	if _, err := pool.Exec(ctx, `UPDATE posts SET title=$2 WHERE id=$1`, id, "migtest changed "+sfx); err != nil {
		t.Fatalf("update title: %v", err)
	}
	fps3, err := PostContentFingerprints(ctx, pool, id)
	if err != nil {
		t.Fatalf("fingerprint 3: %v", err)
	}
	if fps3[id] == fps1[id] {
		t.Errorf("fingerprint unchanged after a content change: %s", fps3[id])
	}
}
