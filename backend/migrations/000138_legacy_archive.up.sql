-- Task idx 68 step 5: the legacy contribution tables leave live storage. Nothing is dropped:
-- every legacy row MOVES into the legacy_archive schema, with a manifest (row count and sha256
-- per table, taken before the move), and the down migration restores it exactly.
--
--  * approaches, answers, responses, comments, approach_relationships and progress_notes move
--    with ALTER TABLE ... SET SCHEMA: rows, columns, indexes and the constraints among them are
--    unchanged. The functions typed by their rows and their foreign keys to live tables are
--    recorded in legacy_archive.dropped_objects and dropped, so the archive never blocks a post
--    delete and nothing in public reads it.
--  * posts: every post is type 'post' and the legacy-only statuses become 'open'. Each post's
--    type, status, success_criteria, weight, accepted_answer_id and evolved_into are kept in
--    legacy_archive.post_fields first; the four problem-only columns are dropped.
--  * votes, reports and flags rows whose target is a legacy type move to legacy_archive, and
--    the target-type checks admit only the canonical targets. replies_legacy_type_check stays:
--    replies.legacy_type is provenance.
--
-- Refused while a legacy contribution has no reply: run cmd/cutover below this migration first.

DO $$
DECLARE pending bigint;
BEGIN
    SELECT
        (SELECT count(*) FROM approaches a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.problem_id)
            AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = a.id)) +
        (SELECT count(*) FROM answers a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.question_id)
            AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'answer' AND r.legacy_id = a.id)) +
        (SELECT count(*) FROM responses a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.idea_id)
            AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'response' AND r.legacy_id = a.id)) +
        (SELECT count(*) FROM comments c
            WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'comment' AND r.legacy_id = c.id)
            AND CASE c.target_type
                WHEN 'post'     THEN EXISTS (SELECT 1 FROM posts t WHERE t.id = c.target_id)
                WHEN 'approach' THEN EXISTS (SELECT 1 FROM approaches t WHERE t.id = c.target_id)
                WHEN 'answer'   THEN EXISTS (SELECT 1 FROM answers t WHERE t.id = c.target_id)
                WHEN 'response' THEN EXISTS (SELECT 1 FROM responses t WHERE t.id = c.target_id)
                ELSE false END) +
        (SELECT count(*) FROM progress_notes n
            WHERE EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = n.approach_id)
            AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'progress_note' AND r.legacy_id = n.id))
    INTO pending;
    IF pending > 0 THEN
        RAISE EXCEPTION '000138: % legacy contributions have no reply yet: run cmd/cutover below the legacy archive migration first', pending;
    END IF;
END $$;

CREATE SCHEMA legacy_archive;

COMMENT ON SCHEMA legacy_archive IS
    'idx 68 recovery archive: legacy rows moved out of live storage by 000138; read only, restored by its down migration';

CREATE TABLE legacy_archive.manifest (
    table_name  TEXT PRIMARY KEY,
    row_count   BIGINT      NOT NULL,
    sha256      TEXT        NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The definitions dropped to move the tables and narrow the checks, as defined on this
-- database, replayed by the down migration. A foreign key also names its single column and the
-- live table and column it references.
CREATE TABLE legacy_archive.dropped_objects (
    ord        INT  PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('function', 'foreign_key', 'check')),
    name       TEXT NOT NULL,
    on_table   TEXT,
    definition TEXT NOT NULL,
    fk_column  TEXT,
    ref_table  TEXT,
    ref_column TEXT
);

-- Row count and sha256 of a table's rows: each row's text form, sorted bytewise and joined by
-- newlines. The same rows give the same digest in either schema.
CREATE FUNCTION legacy_archive.digest(rel regclass, OUT row_count bigint, OUT sha256 text)
LANGUAGE plpgsql STABLE AS $$
BEGIN
    EXECUTE format(
        'SELECT count(*), encode(sha256(convert_to(coalesce(string_agg(t::text, E''\n'' ORDER BY t::text COLLATE "C"), ''''), ''UTF8'')), ''hex'') FROM %s t',
        rel)
    INTO row_count, sha256;
END $$;

INSERT INTO legacy_archive.manifest (table_name, row_count, sha256)
SELECT t, d.row_count, d.sha256
FROM unnest(ARRAY['approaches', 'answers', 'responses', 'comments', 'approach_relationships', 'progress_notes']) t,
     LATERAL legacy_archive.digest(('public.' || t)::regclass) d;

-- Functions typed by a legacy row (hybrid_search_answers/approaches return SETOF it) and
-- foreign keys from a legacy table to a live one, found in the catalog.
DO $$
DECLARE
    legacy text[] := ARRAY['approaches', 'answers', 'responses', 'comments', 'approach_relationships', 'progress_notes'];
    rec record;
    n int := 0;
BEGIN
    FOR rec IN
        SELECT p.oid, p.oid::regprocedure::text AS sig, pg_get_functiondef(p.oid) AS def
        FROM pg_proc p
        WHERE p.pronamespace = 'public'::regnamespace
          AND EXISTS (SELECT 1 FROM pg_class c
                      WHERE c.relnamespace = 'public'::regnamespace AND c.relname = ANY (legacy)
                        AND (p.prorettype = c.reltype OR c.reltype = ANY (p.proargtypes)))
        ORDER BY 2
    LOOP
        n := n + 1;
        INSERT INTO legacy_archive.dropped_objects (ord, kind, name, definition)
        VALUES (n, 'function', rec.sig, rec.def);
        EXECUTE 'DROP FUNCTION ' || rec.sig;
    END LOOP;

    FOR rec IN
        SELECT con.conname, con.conrelid::regclass::text AS tbl, pg_get_constraintdef(con.oid) AS def,
               con.conkey, con.confkey, con.conrelid, con.confrelid,
               con.confrelid::regclass::text AS ref
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_class r ON r.oid = con.confrelid
        WHERE con.contype = 'f'
          AND c.relnamespace = 'public'::regnamespace AND c.relname = ANY (legacy)
          AND NOT (r.relnamespace = 'public'::regnamespace AND r.relname = ANY (legacy))
        ORDER BY 2, 1
    LOOP
        IF array_length(rec.conkey, 1) <> 1 THEN
            RAISE EXCEPTION '000138: foreign key % on % has more than one column', rec.conname, rec.tbl;
        END IF;
        n := n + 1;
        INSERT INTO legacy_archive.dropped_objects (ord, kind, name, on_table, definition, fk_column, ref_table, ref_column)
        VALUES (n, 'foreign_key', rec.conname, rec.tbl, rec.def,
                (SELECT attname FROM pg_attribute WHERE attrelid = rec.conrelid AND attnum = rec.conkey[1]),
                rec.ref,
                (SELECT attname FROM pg_attribute WHERE attrelid = rec.confrelid AND attnum = rec.confkey[1]));
        EXECUTE format('ALTER TABLE public.%I DROP CONSTRAINT %I', rec.tbl, rec.conname);
    END LOOP;
END $$;

-- The checks the narrow replaces, and the weight check that drops with its column, recorded
-- verbatim so the down migration re-adds exactly what this database had.
INSERT INTO legacy_archive.dropped_objects (ord, kind, name, on_table, definition)
SELECT (SELECT COALESCE(max(ord), 0) FROM legacy_archive.dropped_objects)
         + row_number() OVER (ORDER BY c.relname, con.conname),
       'check', con.conname, c.relname, pg_get_constraintdef(con.oid)
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
WHERE c.relnamespace = 'public'::regnamespace AND con.contype = 'c'
  AND (c.relname, con.conname) IN (('posts', 'posts_type_check'), ('posts', 'posts_status_check'),
       ('posts', 'posts_weight_check'), ('votes', 'votes_target_type_check'),
       ('reports', 'reports_target_type_check'), ('flags', 'flags_target_type_check'));

DO $$
BEGIN
    IF (SELECT count(*) FROM legacy_archive.dropped_objects WHERE kind = 'check') <> 6 THEN
        RAISE EXCEPTION '000138: expected the six legacy checks (posts type/status/weight, votes/reports/flags target type)';
    END IF;
END $$;

ALTER TABLE public.approaches SET SCHEMA legacy_archive;
ALTER TABLE public.answers SET SCHEMA legacy_archive;
ALTER TABLE public.responses SET SCHEMA legacy_archive;
ALTER TABLE public.comments SET SCHEMA legacy_archive;
ALTER TABLE public.approach_relationships SET SCHEMA legacy_archive;
ALTER TABLE public.progress_notes SET SCHEMA legacy_archive;

-- Posts: the legacy fields of every post, deleted ones included, before the narrow.
CREATE TABLE legacy_archive.post_fields AS
SELECT id AS post_id, type, status, success_criteria, weight, accepted_answer_id, evolved_into
FROM posts;
ALTER TABLE legacy_archive.post_fields ADD PRIMARY KEY (post_id);

INSERT INTO legacy_archive.manifest (table_name, row_count, sha256)
SELECT 'post_fields', d.row_count, d.sha256 FROM legacy_archive.digest('legacy_archive.post_fields') d;

ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts DROP CONSTRAINT posts_status_check;
UPDATE posts SET type = 'post' WHERE type <> 'post';
UPDATE posts SET status = 'open' WHERE status IN ('in_progress', 'solved', 'answered', 'active', 'dormant', 'evolved');
ALTER TABLE posts ADD CONSTRAINT posts_type_check CHECK (type IN ('post'));
ALTER TABLE posts ALTER COLUMN type SET DEFAULT 'post';
ALTER TABLE posts ADD CONSTRAINT posts_status_check
    CHECK (status IN ('draft', 'open', 'closed', 'stale', 'pending_review', 'rejected'));
ALTER TABLE posts DROP COLUMN success_criteria, DROP COLUMN weight, DROP COLUMN accepted_answer_id, DROP COLUMN evolved_into;

-- Votes, reports and flags on legacy targets. Deleting a legacy-target vote moves no score
-- (votes_score_projection only scores posts, blog posts and replies, whose ids no legacy row
-- shares); the trigger is still off for the move so it is exact by construction.
CREATE TABLE legacy_archive.votes AS
SELECT * FROM votes WHERE target_type NOT IN ('post', 'blog_post', 'reply');
CREATE TABLE legacy_archive.reports AS
SELECT * FROM reports WHERE target_type NOT IN ('post', 'reply');
CREATE TABLE legacy_archive.flags AS
SELECT * FROM flags WHERE target_type NOT IN ('post', 'reply');

INSERT INTO legacy_archive.manifest (table_name, row_count, sha256)
SELECT t, d.row_count, d.sha256
FROM unnest(ARRAY['votes', 'reports', 'flags']) t,
     LATERAL legacy_archive.digest(('legacy_archive.' || t)::regclass) d;

ALTER TABLE votes DISABLE TRIGGER votes_score_delete;
DELETE FROM votes WHERE target_type NOT IN ('post', 'blog_post', 'reply');
ALTER TABLE votes ENABLE TRIGGER votes_score_delete;
DELETE FROM reports WHERE target_type NOT IN ('post', 'reply');
DELETE FROM flags WHERE target_type NOT IN ('post', 'reply');

ALTER TABLE votes DROP CONSTRAINT votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check CHECK (target_type IN ('post', 'blog_post', 'reply'));
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'reply'));
ALTER TABLE flags DROP CONSTRAINT flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check CHECK (target_type IN ('post', 'reply'));
