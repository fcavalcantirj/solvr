package db

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A native reply names an existing account (000116: replies_author_agent_fkey,
// replies_author_human_fkey), so a fixture that writes one as an agent or a human creates
// that author first.

// authorAgent registers id as an agent unless it already is one, and returns it.
func authorAgent(ctx context.Context, t *testing.T, pool *Pool, id string) string {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')
		ON CONFLICT (id) DO NOTHING`, id)
	require.NoError(t, err, "register author agent %s", id)
	return id
}

// authorHuman creates a human account labelled label and returns its id.
func authorHuman(ctx context.Context, t *testing.T, pool *Pool, label string) string {
	t.Helper()
	tag := strings.ReplaceAll(uuid.NewString(), "-", "")
	var id string
	err := pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, $2, $3, 'email', $3, $4) RETURNING id::text`,
		"au_"+tag[:20], label, "au_"+tag+"@example.test", strings.ToUpper(tag[:8])).Scan(&id)
	require.NoError(t, err, "create author human %s", label)
	return id
}
