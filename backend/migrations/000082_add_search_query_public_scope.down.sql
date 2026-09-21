DROP INDEX IF EXISTS idx_search_queries_public_scope;
ALTER TABLE search_queries DROP COLUMN IF EXISTS public_scope;
