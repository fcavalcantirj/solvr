package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// profileFixture holds the agents and users of the profile-verdict test, by name.
type profileFixture struct {
	pool   *Pool
	agents map[string]string // label -> agent id
	users  map[string]string // label -> user id
}

func (f profileFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err, sql)
}

// post inserts a post by (kind, id); listed is whether the post sitemap lists it.
func (f profileFixture) post(t *testing.T, kind, id string, listed bool) string {
	t.Helper()
	moderation := "approved"
	if !listed {
		moderation = "pending"
	}
	var postID string
	require.NoError(t, f.pool.QueryRow(context.Background(), `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state)
		VALUES ('post', $1, 'A fixture post body.', $2, $3, 'open', 'published', $4) RETURNING id::text`,
		"Profile fixture post "+uuid.NewString(), kind, id, moderation).Scan(&postID))
	return postID
}

func (f profileFixture) room(t *testing.T, slug string, private bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO rooms (slug, display_name, is_private) VALUES ($1, $2, $3) RETURNING id`, slug, slug, private).Scan(&id))
	return id
}

func (f profileFixture) say(t *testing.T, room uuid.UUID, name, kind string, authorID *string) {
	t.Helper()
	f.exec(t, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content) VALUES ($1, $2, $3, $4, 'a message')`,
		room, name, kind, authorID)
}

func newProfileFixture(t *testing.T) profileFixture {
	t.Helper()
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	sfx := time.Now().Format("150405.000000")
	f := profileFixture{pool: pool, agents: map[string]string{}, users: map[string]string{}}
	for _, label := range []string{"post", "hiddenPost", "reply", "hiddenReply", "roomOwner", "roomSpeaker",
		"thinRoom", "privateRoom", "empty", "suspended", "other"} {
		id := "agent_prof_" + label + "_" + sfx
		insertRemapAgent(t, pool, ctx, id)
		f.agents[label] = id
	}
	f.exec(t, `UPDATE agents SET status = 'suspended' WHERE id = $1`, f.agents["suspended"])
	n := time.Now().UnixNano() % 1000000000
	for _, label := range []string{"post", "roomOwner", "speaker", "empty"} {
		u, err := NewUserRepository(pool).Create(ctx, &models.User{
			Username: fmt.Sprintf("prof%s%d", label, n), DisplayName: "Profile " + label, Email: fmt.Sprintf("prof%s%d@example.com", label, n),
			AuthProvider: models.AuthProviderGitHub, AuthProviderID: fmt.Sprintf("gh_prof%s%d", label, n), Role: models.UserRoleUser,
		})
		require.NoError(t, err)
		f.users[label] = u.ID
	}

	listedPost := f.post(t, "agent", f.agents["post"], true)
	hiddenPost := f.post(t, "agent", f.agents["hiddenPost"], false)
	f.post(t, "agent", f.agents["suspended"], true)
	f.post(t, "human", f.users["post"], true)
	f.exec(t, `INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1::uuid, 'agent', $2, 'a reply')`, listedPost, f.agents["reply"])
	f.exec(t, `INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1::uuid, 'agent', $2, 'gone', NOW())`, listedPost, f.agents["reply"])
	f.exec(t, `INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1::uuid, 'agent', $2, 'a reply')`, hiddenPost, f.agents["hiddenReply"])

	// A two-way public room, owned by an agent and by a person; one agent and one person spoke there.
	slugSfx := strings.ReplaceAll(sfx, ".", "")
	twoWay := f.room(t, "profile-two-way-"+slugSfx, false)
	other, speaker, owner, person := f.agents["other"], f.agents["roomSpeaker"], f.agents["roomOwner"], f.users["speaker"]
	f.say(t, twoWay, "other", "agent", &other)
	f.say(t, twoWay, "speaker", "agent", &speaker)
	f.say(t, twoWay, "owner", "agent", &owner)
	f.say(t, twoWay, "human:"+person, "human", &person)
	f.exec(t, `INSERT INTO room_members (room_id, agent_id, role, added_by) VALUES ($1, $2, 'owner', 'system')`, twoWay, owner)
	f.exec(t, `INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2::uuid, 'owner', 'system')`, twoWay, f.users["roomOwner"])

	// A one-sided public room (thin, not in the rooms sitemap) and a private two-way room.
	thin, thinAgent := f.room(t, "profile-thin-"+slugSfx, false), f.agents["thinRoom"]
	f.say(t, thin, "thin", "agent", &thinAgent)
	f.exec(t, `INSERT INTO room_members (room_id, agent_id, role, added_by) VALUES ($1, $2, 'owner', 'system')`, thin, thinAgent)
	private, privateAgent := f.room(t, "profile-private-"+slugSfx, true), f.agents["privateRoom"]
	f.say(t, private, "private", "agent", &privateAgent)
	f.say(t, private, "other", "agent", &other)
	return f
}

// SPEC.md 27.1: a profile is indexable when it has public content (a post the post sitemap
// lists, a live reply on such a post, a room the rooms sitemap lists that it owns or spoke in)
// and, for an agent, is active. The verdict's counts follow the same rule, and the agent and
// user sitemaps list exactly the profiles whose verdict is indexable.
func TestProfileVerdict_IndexableExactlyWithPublicContent(t *testing.T) {
	f := newProfileFixture(t)
	ctx := context.Background()
	repo := NewProfileSEORepository(f.pool)

	wantAgents := map[string]models.ProfileContent{
		"post":        {Indexable: true, Posts: 1},
		"hiddenPost":  {},
		"reply":       {Indexable: true, Replies: 1},
		"hiddenReply": {},
		"roomOwner":   {Indexable: true, Rooms: 1}, // owner and speaker of one room: one room
		"roomSpeaker": {Indexable: true, Rooms: 1},
		"thinRoom":    {},
		"privateRoom": {},
		"empty":       {},
		"suspended":   {Posts: 1}, // its post counts, but a suspended agent is not indexed
		"other":       {Indexable: true, Rooms: 1},
	}
	for label, want := range wantAgents {
		got, err := repo.AgentContent(ctx, f.agents[label])
		require.NoError(t, err)
		assert.Equal(t, want, got, "agent %s", label)
	}
	wantUsers := map[string]models.ProfileContent{
		"post":      {Indexable: true, Posts: 1},
		"roomOwner": {Indexable: true, Rooms: 1},
		"speaker":   {Indexable: true, Rooms: 1},
		"empty":     {},
	}
	for label, want := range wantUsers {
		got, err := repo.UserContent(ctx, f.users[label])
		require.NoError(t, err)
		assert.Equal(t, want, got, "user %s", label)
	}

	sitemap := NewSitemapRepository(f.pool)
	indexable := func(want map[string]models.ProfileContent, ids map[string]string) []string {
		out := []string{}
		for label, c := range want {
			if c.Indexable {
				out = append(out, ids[label])
			}
		}
		sort.Strings(out)
		return out
	}
	agentPage, err := sitemap.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: "agents", Page: 1, PerPage: 100})
	require.NoError(t, err)
	userPage, err := sitemap.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: "users", Page: 1, PerPage: 100})
	require.NoError(t, err)
	all, err := sitemap.GetSitemapURLs(ctx)
	require.NoError(t, err)
	listed := func(agents []models.SitemapAgent, users []models.SitemapUser) ([]string, []string) {
		a, u := []string{}, []string{}
		for _, x := range agents {
			a = append(a, x.ID)
		}
		for _, x := range users {
			u = append(u, x.ID)
		}
		sort.Strings(a)
		sort.Strings(u)
		return a, u
	}
	pagedAgents, pagedUsers := listed(agentPage.Agents, userPage.Users)
	allAgents, allUsers := listed(all.Agents, all.Users)
	assert.Equal(t, indexable(wantAgents, f.agents), pagedAgents, "the agent sitemap lists exactly the indexable agents")
	assert.Equal(t, indexable(wantUsers, f.users), pagedUsers, "the user sitemap lists exactly the indexable people")
	assert.Equal(t, pagedAgents, allAgents, "the flat listing agrees on agents")
	assert.Equal(t, pagedUsers, allUsers, "the flat listing agrees on people")

	counts, err := sitemap.GetSitemapCounts(ctx)
	require.NoError(t, err)
	assert.Equal(t, len(pagedAgents), counts.Agents)
	assert.Equal(t, len(pagedUsers), counts.Users)
	require.NotNil(t, counts.Lastmod.Agents)
	// The users sub-sitemap is dated like the agents one: its newest listed entry.
	require.NotNil(t, counts.Lastmod.Users)
	newest := userPage.Users[0].UpdatedAt
	for _, u := range userPage.Users {
		if u.UpdatedAt.After(newest) {
			newest = u.UpdatedAt
		}
	}
	assert.True(t, counts.Lastmod.Users.Equal(newest), "the users lastmod is the newest listed person's")
}
