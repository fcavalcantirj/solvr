-- Enable pgvector extension for semantic search embeddings
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding columns (1024 dimensions for Voyage code-3 model)
-- 768 for nomic-embed-text, 1536 for OpenAI text-embedding-3-small
ALTER TABLE posts ADD COLUMN embedding vector(1024);
ALTER TABLE answers ADD COLUMN embedding vector(1024);
ALTER TABLE approaches ADD COLUMN embedding vector(1024);

-- HNSW indexes for fast approximate nearest neighbor search
-- HNSW delivers ~30x faster queries than IVFFlat, works on empty tables, no periodic rebuilds needed
-- Separate indexes per table for optimal query performance
CREATE INDEX idx_posts_embedding ON posts USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_answers_embedding ON answers USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_approaches_embedding ON approaches USING hnsw (embedding vector_cosine_ops);
