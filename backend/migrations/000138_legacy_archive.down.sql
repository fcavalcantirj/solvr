-- Reverse 000138: restore the legacy tables, the posts' legacy fields and the legacy-target
-- votes, reports and flags exactly as they were archived, then drop the emptied schema.
--
-- Refused when an archived table no longer matches its manifest (row count and sha256).
-- A legacy row whose live parent was hard-deleted after the archive cannot take its foreign
-- key back: it is ARCHIVED into rollback_archive with its legacy dependents (the convention of
-- the 000088/000089/000109/000114 down files) and the counts are raised as NOTICEs.
-- A post's status is restored only where it still holds the 'open' 000138 wrote, so a status
-- changed after the archive survives. The checks come back as 000138 recorded them on this
-- database, verbatim. The four problem-only columns come back last in the column order.

DO $$
DECLARE rec record; d record;
BEGIN
    FOR rec IN SELECT table_name, row_count, sha256 FROM legacy_archive.manifest ORDER BY table_name LOOP
        SELECT * INTO d FROM legacy_archive.digest(('legacy_archive.' || quote_ident(rec.table_name))::regclass);
        IF d.row_count <> rec.row_count OR d.sha256 <> rec.sha256 THEN
            RAISE EXCEPTION '000138 down: legacy_archive.% has % rows (sha256 %), the manifest says % (sha256 %): refusing to restore',
                rec.table_name, d.row_count, d.sha256, rec.row_count, rec.sha256;
        END IF;
    END LOOP;
END $$;

CREATE TABLE IF NOT EXISTS rollback_archive (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT        NOT NULL,
    reason       TEXT        NOT NULL,
    row_data     JSONB       NOT NULL,
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Orphans: rows whose recorded live parent is gone, then, until nothing changes, the legacy
-- rows that reference an orphan through a foreign key inside the archive.
CREATE TEMP TABLE legacy_orphans (tbl TEXT NOT NULL, id UUID NOT NULL, PRIMARY KEY (tbl, id));

DO $$
DECLARE rec record; added bigint; total bigint;
BEGIN
    FOR rec IN SELECT on_table, fk_column, ref_table, ref_column FROM legacy_archive.dropped_objects
               WHERE kind = 'foreign_key' ORDER BY ord LOOP
        EXECUTE format(
            'INSERT INTO legacy_orphans SELECT %L, t.id FROM legacy_archive.%I t
             WHERE t.%I IS NOT NULL AND NOT EXISTS (SELECT 1 FROM public.%I r WHERE r.%I = t.%I)
             ON CONFLICT DO NOTHING',
            rec.on_table, rec.on_table, rec.fk_column, rec.ref_table, rec.ref_column, rec.fk_column);
    END LOOP;
    LOOP
        added := 0;
        FOR rec IN
            SELECT c.relname AS child, a.attname AS col, p.relname AS parent
            FROM pg_constraint con
            JOIN pg_class c ON c.oid = con.conrelid
            JOIN pg_class p ON p.oid = con.confrelid
            JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = con.conkey[1]
            WHERE con.contype = 'f'
              AND c.relnamespace = 'legacy_archive'::regnamespace
              AND p.relnamespace = 'legacy_archive'::regnamespace
        LOOP
            EXECUTE format(
                'INSERT INTO legacy_orphans SELECT %L, t.id FROM legacy_archive.%I t
                 JOIN legacy_orphans o ON o.tbl = %L AND o.id = t.%I
                 ON CONFLICT DO NOTHING',
                rec.child, rec.child, rec.parent, rec.col);
            GET DIAGNOSTICS total = ROW_COUNT;
            added := added + total;
        END LOOP;
        EXIT WHEN added = 0;
    END LOOP;

    FOR rec IN SELECT tbl, count(*) AS n FROM legacy_orphans GROUP BY tbl ORDER BY tbl LOOP
        RAISE NOTICE '000138 down: archived and removed % % rows whose post was hard-deleted after the archive', rec.n, rec.tbl;
        EXECUTE format(
            'INSERT INTO rollback_archive (source_table, reason, row_data)
             SELECT %L, ''000138 down: post hard-deleted after the archive'', to_jsonb(t)
             FROM legacy_archive.%I t JOIN legacy_orphans o ON o.tbl = %L AND o.id = t.id',
            rec.tbl, rec.tbl, rec.tbl);
    END LOOP;
    -- Children first: a table goes once no other table with orphans left references it.
    WHILE EXISTS (SELECT 1 FROM legacy_orphans) LOOP
        FOR rec IN
            SELECT DISTINCT o.tbl FROM legacy_orphans o
            WHERE NOT EXISTS (
                SELECT 1 FROM pg_constraint con
                JOIN pg_class c ON c.oid = con.conrelid
                JOIN pg_class p ON p.oid = con.confrelid
                WHERE con.contype = 'f' AND p.relname = o.tbl AND c.relname <> o.tbl
                  AND c.relnamespace = 'legacy_archive'::regnamespace
                  AND p.relnamespace = 'legacy_archive'::regnamespace
                  AND EXISTS (SELECT 1 FROM legacy_orphans o2 WHERE o2.tbl = c.relname))
        LOOP
            EXECUTE format('DELETE FROM legacy_archive.%I t USING legacy_orphans o WHERE o.tbl = %L AND o.id = t.id',
                rec.tbl, rec.tbl);
            DELETE FROM legacy_orphans WHERE tbl = rec.tbl;
        END LOOP;
    END LOOP;
END $$;

DROP TABLE legacy_orphans;

ALTER TABLE legacy_archive.approaches SET SCHEMA public;
ALTER TABLE legacy_archive.answers SET SCHEMA public;
ALTER TABLE legacy_archive.responses SET SCHEMA public;
ALTER TABLE legacy_archive.comments SET SCHEMA public;
ALTER TABLE legacy_archive.approach_relationships SET SCHEMA public;
ALTER TABLE legacy_archive.progress_notes SET SCHEMA public;

-- The recorded foreign keys and functions, in the order they were dropped.
DO $$
DECLARE rec record;
BEGIN
    FOR rec IN SELECT * FROM legacy_archive.dropped_objects WHERE kind <> 'check' ORDER BY ord LOOP
        IF rec.kind = 'foreign_key' THEN
            EXECUTE format('ALTER TABLE public.%I ADD CONSTRAINT %I %s', rec.on_table, rec.name, rec.definition);
        ELSE
            EXECUTE rec.definition;
        END IF;
    END LOOP;
END $$;

-- The checks 000138 replaced or dropped, re-added as this database had them. PostgreSQL
-- renders `col IN ('a', 'b')` as `((col)::text = ANY ((ARRAY['a'::character varying, ...])::text[]))`
-- and re-renders that text, when it is replayed, as `ANY (ARRAY[('a'::character varying)::text,
-- ...])`: a definition in the first form is re-added from its IN list, any other verbatim, so
-- each comes back byte-identical (a NOTICE says so if one does not).
CREATE TEMP TABLE legacy_checks AS
SELECT on_table, name, definition FROM legacy_archive.dropped_objects WHERE kind = 'check';

CREATE FUNCTION pg_temp.legacy_check_restore(tbl text, cname text, def text) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE col text; vals text; src text := def; got text;
BEGIN
    IF def ~ '^CHECK \(\(\(\w+\)::text = ANY \(\(ARRAY\[.*\]\)::text\[\]\)\)\)$' THEN
        col := substring(def from '^CHECK \(\(\((\w+)\)::text');
        SELECT string_agg(quote_literal(m[1]), ', ' ORDER BY n) INTO vals
        FROM regexp_matches(def, '''([^'']*)''::character varying', 'g') WITH ORDINALITY AS r(m, n);
        src := format('CHECK (%I IN (%s))', col, vals);
    END IF;
    EXECUTE format('ALTER TABLE public.%I ADD CONSTRAINT %I %s', tbl, cname, src);
    SELECT pg_get_constraintdef(oid) INTO got FROM pg_constraint
    WHERE conrelid = ('public.' || quote_ident(tbl))::regclass AND conname = cname;
    IF got IS DISTINCT FROM def THEN
        RAISE NOTICE '000138 down: % on % restored as % (recorded %)', cname, tbl, got, def;
    END IF;
END $$;

-- Votes, reports and flags on legacy targets, with the checks that admit them.
ALTER TABLE votes DROP CONSTRAINT votes_target_type_check;
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE flags DROP CONSTRAINT flags_target_type_check;
DO $$
DECLARE rec record;
BEGIN
    FOR rec IN SELECT * FROM legacy_checks WHERE on_table IN ('votes', 'reports', 'flags') ORDER BY on_table LOOP
        PERFORM pg_temp.legacy_check_restore(rec.on_table, rec.name, rec.definition);
    END LOOP;
END $$;

ALTER TABLE votes DISABLE TRIGGER votes_score_insert;
INSERT INTO votes SELECT * FROM legacy_archive.votes;
ALTER TABLE votes ENABLE TRIGGER votes_score_insert;
INSERT INTO reports SELECT * FROM legacy_archive.reports;
INSERT INTO flags SELECT * FROM legacy_archive.flags;

-- Posts: the problem-only columns, the original type and status, the original checks.
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts DROP CONSTRAINT posts_status_check;
ALTER TABLE posts ALTER COLUMN type DROP DEFAULT;
ALTER TABLE posts
    ADD COLUMN success_criteria TEXT[],
    ADD COLUMN weight INT,
    ADD COLUMN accepted_answer_id UUID,
    ADD COLUMN evolved_into UUID[];

UPDATE posts p
SET type = f.type,
    status = CASE WHEN f.status IN ('in_progress', 'solved', 'answered', 'active', 'dormant', 'evolved')
                   AND p.status = 'open' THEN f.status ELSE p.status END,
    success_criteria = f.success_criteria,
    weight = f.weight,
    accepted_answer_id = f.accepted_answer_id,
    evolved_into = f.evolved_into
FROM legacy_archive.post_fields f
WHERE f.post_id = p.id;

DO $$
DECLARE rec record;
BEGIN
    FOR rec IN SELECT * FROM legacy_checks WHERE on_table = 'posts' ORDER BY name LOOP
        PERFORM pg_temp.legacy_check_restore('posts', rec.name, rec.definition);
    END LOOP;
END $$;
DROP FUNCTION pg_temp.legacy_check_restore(text, text, text);
DROP TABLE legacy_checks;

DROP SCHEMA legacy_archive CASCADE;
