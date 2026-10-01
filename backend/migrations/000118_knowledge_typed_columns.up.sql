-- The knowledge schema keeps its domain in typed columns and JSON only as bounded provenance
-- (idx 68 step 4). Audited before writing this (\d posts, \d replies; information_schema):
--
--   * posts: body (description text), visibility, publication_state and moderation_state
--     (varchar NOT NULL, each with its CHECK), ownership (owner_human_id -> users, and the
--     author links of 000117) are typed. Its created_at and updated_at were the only nullable
--     domain timestamps: they become NOT NULL here. posts has no JSON column.
--   * replies: body, timestamps and the author links of 000116 are typed and NOT NULL. Its one
--     JSON column is provenance, the origin of a reply migrated from a legacy contribution.
--     Nothing bounded it: any reply could hold any JSON. Now:
--       - replies_legacy_pair: legacy_type and legacy_id are set together or not at all.
--       - replies_provenance_bounded: only a migrated reply (legacy_id set) has provenance; it
--         is a JSON object; and it holds only the keys its legacy type's migration writes
--         (contribution_migration.go, legacy_relation_remap.go). A native reply has none, so
--         no new domain field can live in JSON.
--
-- Measured before writing this (read-only): on the local restores of the production dump
-- after the purge (at 113) and before it (cutover rehearsal), and on the local test database,
-- 0 posts with a NULL created_at or updated_at, 0 replies with only one of legacy_type and
-- legacy_id, 0 native replies with provenance and 0 provenance keys outside the lists below.
ALTER TABLE posts
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;

ALTER TABLE replies
    ADD CONSTRAINT replies_legacy_pair CHECK ((legacy_type IS NULL) = (legacy_id IS NULL)),
    ADD CONSTRAINT replies_provenance_bounded CHECK (
        provenance IS NULL OR (
            legacy_id IS NOT NULL
            AND jsonb_typeof(provenance) = 'object'
            AND CASE legacy_type
                WHEN 'approach' THEN provenance - ARRAY['legacy_table', 'angle', 'method',
                    'assumptions', 'differs_from', 'status', 'outcome', 'solution', 'is_latest',
                    'archived_cid', 'approach_relationships'] = '{}'::jsonb
                WHEN 'answer' THEN provenance - ARRAY['legacy_table', 'is_accepted'] = '{}'::jsonb
                WHEN 'response' THEN provenance - ARRAY['legacy_table', 'response_type'] = '{}'::jsonb
                WHEN 'comment' THEN provenance - ARRAY['legacy_table', 'target_type',
                    'target_id'] = '{}'::jsonb
                WHEN 'progress_note' THEN provenance - ARRAY['legacy_table',
                    'approach_id'] = '{}'::jsonb
                ELSE false
            END));
