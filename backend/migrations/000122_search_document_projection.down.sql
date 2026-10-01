-- Reverts 000122. The vectors it cleared stay NULL: they described text the rows no longer
-- hold, and cmd/backfill-embeddings rebuilds them from the current text.
DROP TRIGGER IF EXISTS posts_search_document_stale ON posts;
DROP TRIGGER IF EXISTS replies_search_document_stale ON replies;
DROP FUNCTION IF EXISTS search_document_stale();
DROP FUNCTION IF EXISTS search_document_drift();
