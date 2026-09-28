package db

import (
	"context"
	"fmt"
)

// DiscoverLegacySchemaDependencies reads the PostgreSQL catalog of the public schema and
// returns every legacy table plus every foreign key, view, materialized view, function,
// trigger, index, check constraint and column tied to a legacy table or a legacy
// post/target type. Objects that belong to a legacy table carry it as Owner. Extension
// functions (pgvector) are excluded. It only reads.
func DiscoverLegacySchemaDependencies(ctx context.Context, pool *Pool) ([]LegacyDependency, error) {
	legacy := map[string]bool{}
	for _, t := range LegacyTables {
		legacy[t] = true
	}
	var deps []LegacyDependency
	seen := map[string]bool{}
	add := func(d LegacyDependency) {
		if !seen[d.Key] {
			seen[d.Key] = true
			deps = append(deps, d)
		}
	}
	owner := func(table string) string {
		if legacy[table] {
			return "table:" + table
		}
		return ""
	}
	each := func(label, query string, args []any, fn func(cols []string)) error {
		rows, err := pool.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("discover legacy %s: %w", label, err)
		}
		defer rows.Close()
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return fmt.Errorf("discover legacy %s: scan: %w", label, err)
			}
			cols := make([]string, len(vals))
			for i, v := range vals {
				cols[i] = fmt.Sprint(v)
			}
			fn(cols)
		}
		return rows.Err()
	}

	steps := []struct {
		label, query string
		args         []any
		fn           func(c []string)
	}{
		{"tables", `
			SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'public' AND c.relkind IN ('r','p') AND c.relname = ANY($1)`,
			[]any{LegacyTables}, func(c []string) {
				add(LegacyDependency{Kind: "table", Key: "table:" + c[0], Detail: "legacy contribution table"})
			}},
		{"foreign keys", `
			SELECT src.relname, con.conname, dst.relname, pg_get_constraintdef(con.oid)
			FROM pg_constraint con
			JOIN pg_class src ON src.oid = con.conrelid
			JOIN pg_class dst ON dst.oid = con.confrelid
			JOIN pg_namespace n ON n.oid = src.relnamespace
			WHERE con.contype = 'f' AND n.nspname = 'public'
			  AND (src.relname = ANY($1) OR dst.relname = ANY($1))`,
			[]any{LegacyTables}, func(c []string) {
				add(LegacyDependency{Kind: "fk", Key: "fk:" + c[0] + "." + c[1], Owner: owner(c[0]),
					Detail: c[0] + " -> " + c[2] + ": " + c[3]})
			}},
		{"views", `
			SELECT viewname, definition, 'view' FROM pg_views WHERE schemaname = 'public'
			UNION ALL
			SELECT matviewname, definition, 'materialized view' FROM pg_matviews WHERE schemaname = 'public'`,
			nil, func(c []string) {
				if legacyTableRefRe.MatchString(c[1]) || legacyTypeLiteralRe.MatchString(c[1]) {
					add(LegacyDependency{Kind: "view", Key: "view:" + c[0], Detail: c[2]})
				}
			}},
		{"functions", `
			SELECT p.proname, p.prosrc FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = 'public'
			  AND NOT EXISTS (SELECT 1 FROM pg_depend d
			                  WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e')`,
			nil, func(c []string) {
				if legacyTableRefRe.MatchString(c[1]) || legacyTypeLiteralRe.MatchString(c[1]) {
					add(LegacyDependency{Kind: "function", Key: "function:" + c[0], Detail: "body references the legacy model"})
				}
			}},
		{"triggers", `
			SELECT tbl.relname, t.tgname, p.proname FROM pg_trigger t
			JOIN pg_class tbl ON tbl.oid = t.tgrelid
			JOIN pg_namespace n ON n.oid = tbl.relnamespace
			JOIN pg_proc p ON p.oid = t.tgfoid
			WHERE NOT t.tgisinternal AND n.nspname = 'public'`,
			nil, func(c []string) {
				if legacy[c[0]] || seen["function:"+c[2]] {
					o := owner(c[0])
					if o == "" {
						o = "function:" + c[2]
					}
					add(LegacyDependency{Kind: "trigger", Key: "trigger:" + c[0] + "." + c[1], Owner: o,
						Detail: "executes " + c[2]})
				}
			}},
		{"indexes", `SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = 'public'`,
			nil, func(c []string) {
				if legacy[c[0]] || legacyTypeLiteralRe.MatchString(c[2]) {
					add(LegacyDependency{Kind: "index", Key: "index:" + c[0] + "." + c[1], Owner: owner(c[0]), Detail: c[2]})
				}
			}},
		{"check constraints", `
			SELECT rel.relname, con.conname, pg_get_constraintdef(con.oid) FROM pg_constraint con
			JOIN pg_class rel ON rel.oid = con.conrelid
			JOIN pg_namespace n ON n.oid = rel.relnamespace
			WHERE con.contype = 'c' AND n.nspname = 'public'`,
			nil, func(c []string) {
				if legacy[c[0]] || legacyTypeLiteralRe.MatchString(c[2]) {
					add(LegacyDependency{Kind: "check", Key: "check:" + c[0] + "." + c[1], Owner: owner(c[0]), Detail: c[2]})
				}
			}},
		{"columns", `
			SELECT c.table_name, c.column_name FROM information_schema.columns c
			JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
			WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE' AND NOT (c.table_name = ANY($1))`,
			[]any{LegacyTables}, func(c []string) {
				if legacyColumnRe.MatchString(c[1]) {
					add(LegacyDependency{Kind: "column", Key: "column:" + c[0] + "." + c[1], Detail: "holds a legacy contribution identity"})
				}
			}},
	}
	for _, s := range steps {
		if err := each(s.label, s.query, s.args, s.fn); err != nil {
			return nil, err
		}
	}
	return deps, nil
}
