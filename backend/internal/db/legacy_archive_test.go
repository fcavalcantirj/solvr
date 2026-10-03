package db

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cmd/legacy-archive reads the recovery archive without writing: every manifest row beside
// the count and digest recomputed from the archived rows, and a checksummed JSON Lines export
// of every archived row for the production gate (idx 68 step 3).

func archivedScratchDatabase(t *testing.T) *Pool {
	t.Helper()
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	seedLegacyArchive(t, pool)
	_, err := RunKnowledgeCutover(context.Background(), pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	archiveLegacy()
	return pool
}

func TestLegacyArchive_ManifestAgreesWithTheArchivedRows(t *testing.T) {
	pool := archivedScratchDatabase(t)
	entries, err := ReadLegacyArchive(context.Background(), pool)
	require.NoError(t, err)

	var tables []string
	for _, e := range entries {
		tables = append(tables, e.Table)
		assert.True(t, e.OK(), "%s: manifest %d/%s, recomputed %d/%s", e.Table, e.ManifestRows, e.ManifestSHA256, e.Rows, e.SHA256)
		assert.Equal(t, digestOf(t, pool, "legacy_archive."+e.Table), tableDigest{Rows: e.Rows, SHA256: e.SHA256})
	}
	assert.Equal(t, []string{"answers", "approach_relationships", "approaches", "comments", "flags",
		"post_fields", "progress_notes", "reports", "responses", "votes"}, tables)
}

func TestLegacyArchive_ExportIsCompleteDeterministicAndChecksummed(t *testing.T) {
	pool := archivedScratchDatabase(t)
	ctx := context.Background()

	var first bytes.Buffer
	exp, err := ExportLegacyArchive(ctx, pool, &first)
	require.NoError(t, err)
	sum := sha256.Sum256(first.Bytes())
	assert.Equal(t, hex.EncodeToString(sum[:]), exp.SHA256, "the reported checksum is the file's")

	var header struct {
		Format   string `json:"format"`
		Manifest []struct {
			Table  string `json:"table"`
			Rows   int64  `json:"rows"`
			SHA256 string `json:"sha256"`
		} `json:"manifest"`
	}
	perTable := map[string]int64{}
	lines := 0
	sc := bufio.NewScanner(bytes.NewReader(first.Bytes()))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		lines++
		if lines == 1 {
			require.NoError(t, json.Unmarshal(sc.Bytes(), &header))
			continue
		}
		var row struct {
			Table string          `json:"table"`
			Row   json.RawMessage `json:"row"`
		}
		require.NoError(t, json.Unmarshal(sc.Bytes(), &row))
		require.NotEmpty(t, row.Row)
		perTable[row.Table]++
	}
	require.NoError(t, sc.Err())
	assert.Equal(t, "solvr-legacy-archive/1", header.Format)
	require.Len(t, header.Manifest, 10)
	var total int64
	for _, m := range header.Manifest {
		assert.Equal(t, m.Rows, perTable[m.Table], "%s: every archived row is exported once", m.Table)
		total += m.Rows
	}
	assert.EqualValues(t, total+1, lines)
	assert.EqualValues(t, total, exp.Rows)
	assert.Positive(t, perTable["approaches"])

	var second bytes.Buffer
	_, err = ExportLegacyArchive(ctx, pool, &second)
	require.NoError(t, err)
	assert.Equal(t, first.Bytes(), second.Bytes(), "the same archive exports the same bytes")
}

// A changed archived row is a mismatch: the read names the table and the export refuses.
func TestLegacyArchive_DetectsAChangedArchivedRow(t *testing.T) {
	pool := archivedScratchDatabase(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `UPDATE legacy_archive.comments SET content = content || ' (edited)'
		WHERE id = (SELECT id FROM legacy_archive.comments LIMIT 1)`)
	require.NoError(t, err)

	entries, err := ReadLegacyArchive(ctx, pool)
	require.NoError(t, err)
	for _, e := range entries {
		assert.Equal(t, e.Table != "comments", e.OK(), e.Table)
	}
	var buf bytes.Buffer
	_, err = ExportLegacyArchive(ctx, pool, &buf)
	require.ErrorIs(t, err, ErrLegacyArchiveMismatch)
	assert.Contains(t, err.Error(), "comments")
}

func TestLegacyArchive_RefusesADatabaseWithoutTheArchive(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	_, err := ReadLegacyArchive(context.Background(), pool)
	require.ErrorIs(t, err, ErrNoLegacyArchive)
	var buf bytes.Buffer
	_, err = ExportLegacyArchive(context.Background(), pool, &buf)
	require.ErrorIs(t, err, ErrNoLegacyArchive)
}
