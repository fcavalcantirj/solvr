package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A backup must restore exactly (spec.json idx 79 step 5). pg_restore recreates every object
// with an empty search_path and from the definition pg_dump printed, so a trigger whose WHEN
// clause leans on an operator pg_dump cannot schema-qualify — IS [NOT] DISTINCT FROM on a
// pgvector column resolves "=" through search_path — fails to come back, and pg_restore skips
// it. This re-creates every trigger the way a restore does, on a scratch database at head.
func TestEveryTrigger_IsRecreatedUnderTheEmptySearchPathARestoreUses(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()

	tx, err := pool.pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `SET LOCAL search_path = ''`)
	require.NoError(t, err)

	type trigger struct{ name, table, def string }
	rows, err := tx.Query(ctx, `
		SELECT t.tgname, t.tgrelid::regclass::text, pg_catalog.pg_get_triggerdef(t.oid)
		  FROM pg_catalog.pg_trigger t
		 WHERE NOT t.tgisinternal
		 ORDER BY 2, 1`)
	require.NoError(t, err)
	var triggers []trigger
	for rows.Next() {
		var tr trigger
		require.NoError(t, rows.Scan(&tr.name, &tr.table, &tr.def))
		triggers = append(triggers, tr)
	}
	rows.Close()
	require.NoError(t, rows.Err())
	require.NotEmpty(t, triggers)

	var failed []string
	for _, tr := range triggers {
		_, err := tx.Exec(ctx, `SAVEPOINT recreate`)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `DROP TRIGGER "`+tr.name+`" ON `+tr.table)
		require.NoError(t, err, tr.name)
		if _, err := tx.Exec(ctx, tr.def); err != nil {
			failed = append(failed, tr.table+"."+tr.name+": "+err.Error())
		}
		_, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT recreate`)
		require.NoError(t, err)
	}
	require.Empty(t, failed, "these triggers would be lost by a pg_dump/pg_restore round trip")
}
