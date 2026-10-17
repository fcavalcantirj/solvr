package db

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 22.7: any UTF-8 text is a valid query. Measured on production (PostgreSQL 17.9,
// 2026-10-16): to_tsquery raised "syntax error in tsquery" (SQLSTATE 42601) for a word holding
// "<" and for a word starting with a quote, because buildTsQuery passed "<" through and doubled
// a leading quote into an empty quoted lexeme. Error messages people paste hold both.
var tsqueryInputClasses = []string{
	"Cannot find module 'express'",
	"KeyError: 'body'",
	"'foo",
	"in <module>",
	"List<String>",
	"<->",
	`a\`,
	`foo\ bar`,
	"rock'n'roll",
	"C++",
	"星弟",
	"' OR 1=1--",
	"<script>alert(1)</script>",
	"TypeError: Cannot read properties of undefined (reading 'map')",
	"ECONNREFUSED 127.0.0.1:5432",
	"a:b|c&d!e(f)g",
}

func TestBuildTsQuery_ParsesEveryInputClass(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	for _, in := range tsqueryInputClasses {
		q := buildTsQuery(in)
		var out string
		err := pool.QueryRow(ctx, `SELECT to_tsquery('english', $1)::text`, q).Scan(&out)
		assert.NoError(t, err, "input %q built %q", in, q)
	}
}

// A seeded corpus of 5,000 strings over letters, digits, every tsquery operator, quotes,
// backslashes, CJK and emoji: one query parses them all, or the whole statement fails.
func TestBuildTsQuery_SeededCorpusParses(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	rng := rand.New(rand.NewSource(20261017))
	alphabet := []rune("abcXYZ019 '\\<>:!&|()-_*\"?.,;@#星弟🙂\t")
	corpus := make([]string, 0, 5000)
	for len(corpus) < 5000 {
		var b strings.Builder
		for i, n := 0, 1+rng.Intn(24); i < n; i++ {
			b.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		if q := buildTsQuery(b.String()); q != "" {
			corpus = append(corpus, q)
		}
	}
	var parsed int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM unnest($1::text[]) AS q WHERE numnode(to_tsquery('english', q)) >= 0`, corpus).Scan(&parsed)
	require.NoError(t, err)
	assert.Equal(t, len(corpus), parsed)
}

// The meaning is kept: a query matches a text holding any of its words, each word as a prefix.
func TestBuildTsQuery_StillMatchesAnyWordByPrefix(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	for _, c := range []struct {
		doc, query string
		want       bool
	}{
		{"webhooks configuration", "webhook config", true},
		{"Cannot find module express", "Cannot find module 'express'", true},
		{"the build failed with ECONNREFUSED", "econnrefused", true},
		{"unrelated text", "webhook", false},
	} {
		var got bool
		require.NoError(t, pool.QueryRow(context.Background(),
			`SELECT to_tsvector('english', $1) @@ to_tsquery('english', $2)`, c.doc, buildTsQuery(c.query)).Scan(&got))
		assert.Equal(t, c.want, got, c.query)
	}
}

// A NUL byte or invalid UTF-8 can't reach Postgres as text; a query of punctuation alone has
// nothing to search.
func TestBuildTsQuery_DropsNULInvalidUTF8AndWordsWithoutLettersOrDigits(t *testing.T) {
	assert.Equal(t, buildTsQuery("ab"), buildTsQuery("a\x00b"))
	assert.Equal(t, buildTsQuery("ab"), buildTsQuery("a\xffb"))
	assert.Equal(t, "", buildTsQuery("((( !!! ::: ''"))
	assert.Equal(t, "", buildTsQuery("   "))
}
