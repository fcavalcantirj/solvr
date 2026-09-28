-- Reply embeddings are derived data: migrated ones are copies of the answer/approach
-- embeddings, which stay in their tables, so dropping the column loses nothing authoritative.
DROP FUNCTION IF EXISTS hybrid_search_replies(text, vector(1024), int, float, float, int, uuid);
DROP INDEX IF EXISTS idx_replies_body_tsvector;
DROP INDEX IF EXISTS idx_replies_embedding;
ALTER TABLE replies DROP COLUMN IF EXISTS embedding;
