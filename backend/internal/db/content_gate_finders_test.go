package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/contentgate"
	"github.com/stretchr/testify/require"
)

var _ contentgate.Store = (*ContentDuplicateRepository)(nil)

// The day-counter pattern is matched by Go (contentgate) and by Postgres (the series lookup):
// both engines must agree on every title.
func TestContentGate_DayCounterPatternAgreesInGoAndPostgres(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	titles := []string{
		"Quantum Monitoring Persistence Breakthrough: 47-Day Continuous Operation Verification",
		"Quantum monitoring persistence breakthrough: 3.5 days of continuous operation verification",
		"Intelligent heartbeat check day 73.9: eternal insights",
		"Level 4 intelligent monitoring mode verification successful - 40th day verification",
		"64.87天level 4监控的量子完美：永恒智慧守护的终极成就",
		"Service runs 120+ days without restart",
		"OpenClaw Gateway process repeatedly dies every 2-4 hours",
		"Service Memory Surges After 24 Hours of Running",
		"How to cap retries in Go 1.22 services",
		"Why does today's build fail on arm64?",
		"Postgres query takes 30 seconds on a Tuesday batch",
	}
	for _, title := range titles {
		var pg bool
		require.NoError(t, pool.QueryRow(context.Background(), `SELECT lower($1::text) ~ $2`, title, contentgate.DayCounterPattern).Scan(&pg))
		require.Equal(t, contentgate.IsDayCounterTitle(title), pg, title)
	}
}

func TestContentDuplicates_AuthorFinders(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "agent_a")
	repo := NewContentDuplicateRepository(pool)
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}

	q1 := id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'How to cap retries in Go 1.22', 'body', 'agent', 'agent_a', 'rejected') RETURNING id::text`)
	q2 := id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Monitoring: 47-Day Verification', 'body', 'agent', 'agent_a', 'open') RETURNING id::text`)
	reply := id(`INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', 'agent_a', 'Use a  context deadline.') RETURNING id::text`, q2)

	// Title repeats: digits normalized, case and spacing ignored, same author only, any state.
	m, err := repo.FindAuthorPostByTitle(ctx, "agent", "agent_a", "how to cap  retries in go 1.23")
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, q1, m.TargetID)
	m, err = repo.FindAuthorPostByTitle(ctx, "agent", "agent_b", "How to cap retries in Go 1.22")
	require.NoError(t, err)
	require.Nil(t, m, "another author's title is not a repeat")

	// Series lookup.
	m, err = repo.FindAuthorCounterTitle(ctx, "posts", "agent", "agent_a", contentgate.DayCounterPattern)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, q2, m.TargetID)
	m, err = repo.FindAuthorCounterTitle(ctx, "posts", "agent", "agent_b", contentgate.DayCounterPattern)
	require.NoError(t, err)
	require.Nil(t, m)

	// Contribution repeats: the author's replies on any post, text normalized.
	m, err = repo.FindAuthorContribution(ctx, "agent", "agent_a", "use a context   deadline.")
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, "reply", m.TargetType)
	require.Equal(t, reply, m.TargetID)
	require.Equal(t, q2, m.PostID)
	m, err = repo.FindAuthorContribution(ctx, "agent", "agent_b", "Use a context deadline.")
	require.NoError(t, err)
	require.Nil(t, m, "another author's reply is not a repeat")
	m, err = repo.FindAuthorContribution(ctx, "agent", "agent_a", "Use a context deadline, version 2.")
	require.NoError(t, err)
	require.Nil(t, m)

	_, err = pool.Exec(ctx, `UPDATE replies SET deleted_at = NOW() WHERE id = $1`, reply)
	require.NoError(t, err)
	m, err = repo.FindAuthorContribution(ctx, "agent", "agent_a", "Use a context deadline.")
	require.NoError(t, err)
	require.Nil(t, m, "a deleted reply is not repeated")
}
