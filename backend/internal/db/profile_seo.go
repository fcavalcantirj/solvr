package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// A profile page search engines may index (SPEC.md 27.1) is one with public content: a post
// the post sitemap lists (postIndexablePredicate), a live reply on such a post, or a room the
// rooms sitemap lists (roomIndexablePredicate) that the profile owns or has spoken in. An
// agent must also be active. agentProfileIndexable and userProfileIndexable are that rule,
// read by the agent and user sitemaps (sitemap.go) and by the profile verdict
// (GET /v1/agents/{id}/seo, GET /v1/users/{id}/seo), so they cannot disagree.

// profileKind is how one kind of profile is named in the content tables.
type profileKind struct {
	// authorType is posts.posted_by_type, replies.author_type and room_entries.author_type.
	authorType string
	// memberID is the room_members column that names the profile, as text.
	memberID string
}

var (
	agentProfileKind = profileKind{authorType: "agent", memberID: "rm.agent_id"}
	userProfileKind  = profileKind{authorType: "human", memberID: "rm.user_id::text"}
)

// profilePostsFrom is the FROM and WHERE of the profile's posts the post sitemap lists. id is
// the profile's id as text: an outer column, or a parameter.
func profilePostsFrom(k profileKind, id string) string {
	return `FROM posts p WHERE p.posted_by_type = '` + k.authorType + `' AND p.posted_by_id = ` + id +
		` AND ` + postIndexablePredicate("p")
}

// profileRepliesFrom is the FROM and WHERE of the profile's live replies on such posts.
func profileRepliesFrom(k profileKind, id string) string {
	return `FROM replies r JOIN posts p ON p.id = r.post_id
		WHERE r.author_type = '` + k.authorType + `' AND r.author_id = ` + id + ` AND r.deleted_at IS NULL
		AND ` + postIndexablePredicate("p")
}

// profileRoomLinks selects (profile_id, room_id) for every room a profile of the kind owns (an
// active owner membership) or has spoken in (a live message under its own identity).
func profileRoomLinks(k profileKind) string {
	return `SELECT ` + k.memberID + ` AS profile_id, rm.room_id FROM room_members rm
			WHERE rm.role = 'owner' AND rm.revoked_at IS NULL
		UNION ALL
		SELECT e.author_id, e.room_id FROM room_entries e
			WHERE e.kind = 'message' AND e.deleted_at IS NULL AND e.author_type = '` + k.authorType + `'`
}

// indexableRoomIDs is the array of the rooms the rooms sitemap lists. An ARRAY(...) of an
// uncorrelated query is computed once per statement: as a plain IN the planner joined the rule
// to every room entry, running its per-room aggregate once per message instead of per room.
var indexableRoomIDs = `ARRAY(SELECT id FROM rooms WHERE ` + roomIndexablePredicate + `)`

// profileHasPublicContent is the content rule for the profile id names. The room arm names no
// outer column: a query reads the set of profiles with an indexable room once, however many
// profiles it checks (a sitemap lists them all), instead of searching the room entries per
// profile, which no index on author alone serves.
func profileHasPublicContent(k profileKind, id string) string {
	return `(EXISTS (SELECT 1 ` + profilePostsFrom(k, id) + `)
		OR EXISTS (SELECT 1 ` + profileRepliesFrom(k, id) + `)
		OR ` + id + ` IN (SELECT DISTINCT l.profile_id FROM (` + profileRoomLinks(k) + `) l
			WHERE l.profile_id IS NOT NULL AND l.room_id = ANY (` + indexableRoomIDs + `)))`
}

// agentProfileIndexable is the rule for an agent profile, against an unaliased agents table.
var agentProfileIndexable = `agents.status = 'active' AND agents.deleted_at IS NULL AND ` +
	profileHasPublicContent(agentProfileKind, "agents.id")

// userProfileIndexable is the rule for a person's profile, against an unaliased users table.
var userProfileIndexable = `users.deleted_at IS NULL AND ` + profileHasPublicContent(userProfileKind, "users.id::text")

// ProfileSEORepository reads the profile verdicts.
type ProfileSEORepository struct {
	pool *Pool
}

// NewProfileSEORepository creates a ProfileSEORepository.
func NewProfileSEORepository(pool *Pool) *ProfileSEORepository {
	return &ProfileSEORepository{pool: pool}
}

// AgentContent reads the verdict and the counts of the agent's profile.
func (r *ProfileSEORepository) AgentContent(ctx context.Context, agentID string) (models.ProfileContent, error) {
	return r.content(ctx, "AgentContent", agentProfileKind,
		`SELECT EXISTS (SELECT 1 FROM agents WHERE agents.id = $1::text AND `+agentProfileIndexable+`)`, agentID)
}

// UserContent reads the verdict and the counts of the person's profile. userID must be a UUID.
func (r *ProfileSEORepository) UserContent(ctx context.Context, userID string) (models.ProfileContent, error) {
	return r.content(ctx, "UserContent", userProfileKind,
		`SELECT EXISTS (SELECT 1 FROM users WHERE users.id = $1::text::uuid AND `+userProfileIndexable+`)`, userID)
}

// content reads verdict (a query of the profile's rule with $1 its id) and the counts, from the
// same blocks the rule is made of, in one statement.
func (r *ProfileSEORepository) content(ctx context.Context, op string, k profileKind, verdict, id string) (models.ProfileContent, error) {
	var c models.ProfileContent
	err := r.pool.QueryRow(ctx, `SELECT (`+verdict+`),
		(SELECT COUNT(*) `+profilePostsFrom(k, "$1::text")+`),
		(SELECT COUNT(*) `+profileRepliesFrom(k, "$1::text")+`),
		(SELECT COUNT(DISTINCT l.room_id) FROM (`+profileRoomLinks(k)+`) l
			WHERE l.profile_id = $1::text AND l.room_id = ANY (`+indexableRoomIDs+`))`, id).
		Scan(&c.Indexable, &c.Posts, &c.Replies, &c.Rooms)
	if err != nil {
		LogQueryError(ctx, op, "profiles", err)
		return models.ProfileContent{}, fmt.Errorf("profile content: %w", err)
	}
	return c, nil
}
