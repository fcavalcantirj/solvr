-- Drop HNSW indexes
DROP INDEX IF EXISTS idx_posts_embedding;
DROP INDEX IF EXISTS idx_answers_embedding;
DROP INDEX IF EXISTS idx_approaches_embedding;

-- Drop embedding columns
ALTER TABLE posts DROP COLUMN IF EXISTS embedding;
ALTER TABLE answers DROP COLUMN IF EXISTS embedding;
ALTER TABLE approaches DROP COLUMN IF EXISTS embedding;

-- Drop pgvector extension
DROP EXTENSION IF EXISTS vector;
