package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Post-migration exception kinds (task: migrate every legacy post). These are the
// per-record failures the legacy->canonical migration must never leave behind.
const (
	// PostExceptionStateMismatch: publication_state/moderation_state disagree with the
	// canonical mapping DeriveStates(status) derives from the row's retained legacy status.
	PostExceptionStateMismatch = "state_mismatch"
	// PostExceptionInvalidState: a stored state is outside the allowed enum.
	PostExceptionInvalidState = "invalid_state"
	// PostExceptionMissingTitle: a live (non-deleted) post has no title.
	PostExceptionMissingTitle = "missing_title"
)

// PostMigrationException records one post that failed a migration invariant.
type PostMigrationException struct {
	PostID string
	Kind   string
	Detail string
}

// PostMigrationReport is the inventory plus per-record exceptions produced by verifying
// the legacy->canonical post migration (BART-583 states + this task). It inventories
// posts by legacy type, status and visibility (step 1) and lists every row whose
// canonical states do not derive from its retained legacy status (step 3). Run it
// against a restored production snapshot before and after the migration and require OK().
type PostMigrationReport struct {
	Total        int
	Live         int
	SoftDeleted  int
	ByType       map[string]int
	ByStatus     map[string]int
	ByVisibility map[string]int
	Exceptions   []PostMigrationException
}

// OK reports whether verification found zero exceptions.
func (r *PostMigrationReport) OK() bool { return len(r.Exceptions) == 0 }

// ExceptionsFor returns only the exceptions whose PostID is in ids. It lets a test
// assert on its own seeded rows without being disturbed by unrelated rows already in a
// shared database.
func (r *PostMigrationReport) ExceptionsFor(ids ...string) []PostMigrationException {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []PostMigrationException
	for _, e := range r.Exceptions {
		if want[e.PostID] {
			out = append(out, e)
		}
	}
	return out
}

// VerifyPostMigration inventories every post (including soft-deleted) and checks, for
// each, that its stored (publication_state, moderation_state) equals the canonical pair
// DeriveStates derives from its retained legacy status and that both states are valid
// enum values. This is the dry-run/reconciliation check: run it against a restored
// production snapshot and require report.OK() before switching reads to the new model.
func VerifyPostMigration(ctx context.Context, pool *Pool) (*PostMigrationReport, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, type, status, visibility, publication_state, moderation_state,
		       (deleted_at IS NOT NULL) AS deleted, title
		FROM posts`)
	if err != nil {
		return nil, fmt.Errorf("verify post migration: query: %w", err)
	}
	defer rows.Close()

	report := &PostMigrationReport{
		ByType:       map[string]int{},
		ByStatus:     map[string]int{},
		ByVisibility: map[string]int{},
	}
	for rows.Next() {
		var id, typ, status, visibility, pub, mod, title string
		var deleted bool
		if err := rows.Scan(&id, &typ, &status, &visibility, &pub, &mod, &deleted, &title); err != nil {
			return nil, fmt.Errorf("verify post migration: scan: %w", err)
		}
		report.Total++
		if deleted {
			report.SoftDeleted++
		} else {
			report.Live++
		}
		report.ByType[typ]++
		report.ByStatus[status]++
		report.ByVisibility[visibility]++

		wantPub, wantMod := models.DeriveStates(models.PostStatus(status))
		if pub != string(wantPub) || mod != string(wantMod) {
			report.Exceptions = append(report.Exceptions, PostMigrationException{
				PostID: id, Kind: PostExceptionStateMismatch,
				Detail: fmt.Sprintf("status %q has states (%s,%s), want (%s,%s)", status, pub, mod, wantPub, wantMod),
			})
		}
		if !validPublicationStateStr(pub) || !validModerationStateStr(mod) {
			report.Exceptions = append(report.Exceptions, PostMigrationException{
				PostID: id, Kind: PostExceptionInvalidState,
				Detail: fmt.Sprintf("invalid states (%s,%s)", pub, mod),
			})
		}
		if !deleted && strings.TrimSpace(title) == "" {
			report.Exceptions = append(report.Exceptions, PostMigrationException{
				PostID: id, Kind: PostExceptionMissingTitle,
				Detail: "live post has empty title",
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("verify post migration: rows: %w", err)
	}
	return report, nil
}

func validPublicationStateStr(s string) bool {
	switch models.PublicationState(s) {
	case models.PublicationDraft, models.PublicationPublished, models.PublicationArchived:
		return true
	}
	return false
}

func validModerationStateStr(s string) bool {
	switch models.ModerationState(s) {
	case models.ModerationPending, models.ModerationApproved, models.ModerationRejected:
		return true
	}
	return false
}

// RemapPostStates re-derives publication_state and moderation_state from each row's
// retained legacy status using models.DeriveStates (the single source of truth, kept
// consistent with migration 000088). It is idempotent — the WHERE guard skips rows that
// are already correct — and touches no content column, so it is safe to resume after an
// interruption. It returns the number of rows actually updated.
func RemapPostStates(ctx context.Context, pool *Pool) (int64, error) {
	statusRows, err := pool.Query(ctx, `SELECT DISTINCT status FROM posts`)
	if err != nil {
		return 0, fmt.Errorf("remap post states: distinct status: %w", err)
	}
	var statuses []string
	for statusRows.Next() {
		var s string
		if err := statusRows.Scan(&s); err != nil {
			statusRows.Close()
			return 0, fmt.Errorf("remap post states: scan status: %w", err)
		}
		statuses = append(statuses, s)
	}
	statusRows.Close()
	if err := statusRows.Err(); err != nil {
		return 0, fmt.Errorf("remap post states: status rows: %w", err)
	}

	var total int64
	for _, s := range statuses {
		pub, mod := models.DeriveStates(models.PostStatus(s))
		tag, err := pool.Exec(ctx, `
			UPDATE posts SET publication_state = $1, moderation_state = $2
			WHERE status = $3
			  AND (publication_state <> $1 OR moderation_state <> $2)`,
			string(pub), string(mod), s)
		if err != nil {
			return total, fmt.Errorf("remap post states: update status %q: %w", s, err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

// PostContentFingerprints returns a stable content hash per post over the fields the
// state migration must never alter: title, body, tags, author, retained legacy status,
// visibility, votes, view count, creation/deletion timestamps, and every problem/idea/
// question and crystallization provenance field. publication_state, moderation_state and
// updated_at are excluded because the migration is allowed to write them. Compare the
// map before and after migration to prove zero unexplained content loss (step 6). Pass
// ids to scope the result; empty ids fingerprints every post.
func PostContentFingerprints(ctx context.Context, pool *Pool, ids ...string) (map[string]string, error) {
	query := `
		SELECT id, type, title, description, tags, posted_by_type, posted_by_id, status,
		       upvotes, downvotes, view_count, success_criteria, weight, accepted_answer_id,
		       evolved_into, created_at, deleted_at, crystallization_cid, crystallized_at,
		       visibility, COALESCE(original_language, ''), COALESCE(original_title, ''),
		       COALESCE(original_description, ''), owner_human_id
		FROM posts`
	var (
		rows pgx.Rows
		err  error
	)
	if len(ids) > 0 {
		query += ` WHERE id = ANY($1)`
		rows, err = pool.Query(ctx, query, ids)
	} else {
		rows, err = pool.Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("post content fingerprints: query: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var p fingerprintedPost
		if err := rows.Scan(
			&p.ID, &p.Type, &p.Title, &p.Description, &p.Tags, &p.PostedByType, &p.PostedByID, &p.Status,
			&p.Upvotes, &p.Downvotes, &p.ViewCount, &p.successCriteria, &p.weight, &p.acceptedAnswerID,
			&p.evolvedInto, &p.CreatedAt, &p.DeletedAt, &p.CrystallizationCID, &p.CrystallizedAt,
			&p.Visibility, &p.OriginalLanguage, &p.OriginalTitle, &p.OriginalDescription, &p.OwnerHumanID,
		); err != nil {
			return nil, fmt.Errorf("post content fingerprints: scan: %w", err)
		}
		out[p.ID] = contentFingerprint(&p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("post content fingerprints: rows: %w", err)
	}
	return out, nil
}

// fingerprintedPost is a post row with the legacy problem-only columns the cutover-time
// fingerprint covers. They are no longer part of models.Post (idx 68); like the rest of the
// cutover tool, the fingerprint runs only on a schema below the legacy archive.
type fingerprintedPost struct {
	models.Post
	successCriteria  []string
	weight           *int
	acceptedAnswerID *string
	evolvedInto      []string
}

func contentFingerprint(p *fingerprintedPost) string {
	var b strings.Builder
	w := func(label, v string) {
		b.WriteString(label)
		b.WriteByte('=')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	w("id", p.ID)
	w("type", string(p.Type))
	w("title", p.Title)
	w("description", p.Description)
	w("tags", strings.Join(p.Tags, "\x1f"))
	w("posted_by_type", string(p.PostedByType))
	w("posted_by_id", p.PostedByID)
	w("status", string(p.Status))
	w("upvotes", strconv.Itoa(p.Upvotes))
	w("downvotes", strconv.Itoa(p.Downvotes))
	w("view_count", strconv.Itoa(p.ViewCount))
	w("success_criteria", strings.Join(p.successCriteria, "\x1f"))
	w("weight", intPtrStr(p.weight))
	w("accepted_answer_id", strPtrStr(p.acceptedAnswerID))
	w("evolved_into", strings.Join(p.evolvedInto, "\x1f"))
	w("created_at", p.CreatedAt.UTC().Format(time.RFC3339Nano))
	w("deleted_at", timePtrStr(p.DeletedAt))
	w("crystallization_cid", strPtrStr(p.CrystallizationCID))
	w("crystallized_at", timePtrStr(p.CrystallizedAt))
	w("visibility", p.Visibility)
	w("original_language", p.OriginalLanguage)
	w("original_title", p.OriginalTitle)
	w("original_description", p.OriginalDescription)
	w("owner_human_id", strPtrStr(p.OwnerHumanID))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func intPtrStr(v *int) string {
	if v == nil {
		return "\x00"
	}
	return strconv.Itoa(*v)
}

func strPtrStr(v *string) string {
	if v == nil {
		return "\x00"
	}
	return *v
}

func timePtrStr(v *time.Time) string {
	if v == nil {
		return "\x00"
	}
	return v.UTC().Format(time.RFC3339Nano)
}
