package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The recovery archive the legacy archive migration (000138, idx 68) writes: the legacy rows
// moved out of live storage and legacy_archive.manifest, one row count and sha256 per archived
// table. cmd/legacy-archive reads it here, never writing: every read runs in one read-only
// repeatable-read transaction, so the manifest check and the export see the same snapshot.

// LegacyArchiveFormat names the export's line format.
const LegacyArchiveFormat = "solvr-legacy-archive/1"

var (
	// ErrNoLegacyArchive: the database has no legacy_archive.manifest (below 000138).
	ErrNoLegacyArchive = errors.New("no legacy archive: legacy_archive.manifest does not exist (the legacy archive migration has not run)")
	// ErrLegacyArchiveMismatch: an archived table no longer matches its manifest row.
	ErrLegacyArchiveMismatch = errors.New("the legacy archive does not match its manifest")
)

// LegacyArchiveEntry is one manifest row beside the count and digest recomputed from the
// archived rows with the migration's own legacy_archive.digest.
type LegacyArchiveEntry struct {
	Table          string    `json:"table"`
	ManifestRows   int64     `json:"rows"`
	ManifestSHA256 string    `json:"sha256"`
	ArchivedAt     time.Time `json:"archived_at"`
	Rows           int64     `json:"-"`
	SHA256         string    `json:"-"`
}

// OK reports whether the archived rows still match the manifest.
func (e LegacyArchiveEntry) OK() bool {
	return e.Rows == e.ManifestRows && e.SHA256 == e.ManifestSHA256
}

// LegacyArchiveExport describes a written export.
type LegacyArchiveExport struct {
	SchemaVersion *int64
	Rows          int64
	SHA256        string
}

// ReadLegacyArchive returns every manifest row, by table name, with its recomputed digest.
func ReadLegacyArchive(ctx context.Context, pool *Pool) ([]LegacyArchiveEntry, error) {
	var entries []LegacyArchiveEntry
	err := readLegacyArchiveSnapshot(ctx, pool, func(tx pgx.Tx) error {
		var err error
		entries, err = legacyArchiveEntries(ctx, tx)
		return err
	})
	return entries, err
}

// ExportLegacyArchive writes the archive as JSON Lines to w: a header line with the format,
// the schema version and the manifest, then one {"table","row"} line per archived row, tables
// by name and rows in the manifest digest's order, so the same archive gives the same bytes.
// It refuses when the archive does not match its manifest, and checks that each table wrote as
// many lines as its manifest counts.
func ExportLegacyArchive(ctx context.Context, pool *Pool, w io.Writer) (LegacyArchiveExport, error) {
	var exp LegacyArchiveExport
	err := readLegacyArchiveSnapshot(ctx, pool, func(tx pgx.Tx) error {
		entries, err := legacyArchiveEntries(ctx, tx)
		if err != nil {
			return err
		}
		var bad []string
		for _, e := range entries {
			if !e.OK() {
				bad = append(bad, e.Table)
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("%w: %s", ErrLegacyArchiveMismatch, strings.Join(bad, ", "))
		}
		var migrated bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&migrated); err != nil {
			return fmt.Errorf("read schema version: %w", err)
		}
		if migrated {
			if err := tx.QueryRow(ctx, `SELECT version FROM schema_migrations LIMIT 1`).Scan(&exp.SchemaVersion); err != nil {
				return fmt.Errorf("read schema version: %w", err)
			}
		}

		h := sha256.New()
		out := io.MultiWriter(w, h)
		header, err := json.Marshal(struct {
			Format        string               `json:"format"`
			SchemaVersion *int64               `json:"schema_version"`
			Manifest      []LegacyArchiveEntry `json:"manifest"`
		}{LegacyArchiveFormat, exp.SchemaVersion, entries})
		if err != nil {
			return err
		}
		if _, err := out.Write(append(header, '\n')); err != nil {
			return err
		}
		for _, e := range entries {
			n, err := exportLegacyArchiveTable(ctx, tx, out, e.Table)
			if err != nil {
				return err
			}
			if n != e.ManifestRows {
				return fmt.Errorf("%w: %s exported %d rows, the manifest counts %d", ErrLegacyArchiveMismatch, e.Table, n, e.ManifestRows)
			}
			exp.Rows += n
		}
		exp.SHA256 = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return exp, err
}

func readLegacyArchiveSnapshot(ctx context.Context, pool *Pool, read func(pgx.Tx) error) error {
	tx, err := pool.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin read-only snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var present bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('legacy_archive.manifest') IS NOT NULL`).Scan(&present); err != nil {
		return fmt.Errorf("look up the legacy archive: %w", err)
	}
	if !present {
		return ErrNoLegacyArchive
	}
	return read(tx)
}

func legacyArchiveEntries(ctx context.Context, tx pgx.Tx) ([]LegacyArchiveEntry, error) {
	rows, err := tx.Query(ctx, `SELECT m.table_name, m.row_count, m.sha256, m.archived_at, d.row_count, d.sha256
		FROM legacy_archive.manifest m,
		     LATERAL legacy_archive.digest(('legacy_archive.' || quote_ident(m.table_name))::regclass) d
		ORDER BY m.table_name COLLATE "C"`)
	if err != nil {
		return nil, fmt.Errorf("read the legacy archive manifest: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (LegacyArchiveEntry, error) {
		var e LegacyArchiveEntry
		err := row.Scan(&e.Table, &e.ManifestRows, &e.ManifestSHA256, &e.ArchivedAt, &e.Rows, &e.SHA256)
		return e, err
	})
}

func exportLegacyArchiveTable(ctx context.Context, tx pgx.Tx, w io.Writer, table string) (int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(
		`SELECT json_build_object('table', $1::text, 'row', row_to_json(t))::text
		 FROM legacy_archive.%s t ORDER BY t::text COLLATE "C"`, pgx.Identifier{table}.Sanitize()), table)
	if err != nil {
		return 0, fmt.Errorf("export legacy_archive.%s: %w", table, err)
	}
	defer rows.Close()
	var n int64
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return n, fmt.Errorf("export legacy_archive.%s: %w", table, err)
		}
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}
