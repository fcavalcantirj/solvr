package db

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 2.8: the name a public answer gives a person has two forms, PublicDisplayName for
// a name a handler serialises and userPublicName for a name a query reads. The SQL form is
// checked here against the Go form on every case, so the two cannot drift apart.
func TestUserPublicName_AgreesWithPublicDisplayName(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	names := []string{
		"Ana Lima", "felipe@example.com", "  Felipe.C@Example.Com.br  ", "Felipe <felipe@example.com>",
		"", "   ", "\t\n", "ana@home", "@ana", "Ana @ Solvr.dev", "Ação Café", "a.b+c@mail.example.org",
	}
	for _, name := range names {
		var got string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT `+userPublicName("u")+` FROM (SELECT $1::text AS display_name, 'the_username'::text AS username) u`,
			name).Scan(&got))
		assert.Equal(t, models.PublicDisplayName(name, "the_username"), got, "display name %q", name)
	}

	// A LEFT JOIN that finds no user reads NULL, as the bare column did, so COALESCE still
	// falls through to the next name.
	var fallback string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COALESCE(`+userPublicName("u")+`, 'agent name') FROM (SELECT NULL::text AS display_name, NULL::text AS username) u`).Scan(&fallback))
	assert.Equal(t, "agent name", fallback)
}
