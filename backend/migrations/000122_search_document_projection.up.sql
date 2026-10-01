-- Search documents are projections of the text they describe (idx 77: counters and projections
-- are rebuildable from authoritative records). The text is the record; what search reads is
-- derived from it:
--
--   keyword documents  = the GIN expression indexes over to_tsvector(title || ' ' || description)
--                        and to_tsvector(body); Postgres maintains them in the row's own write,
--                        so they cannot drift and have nothing to rebuild (REINDEX at most)
--   posts.embedding    = the vector of title || ' ' || description, or NULL
--   replies.embedding  = the vector of body, or NULL (system replies are never embedded)
--
-- A vector needs the embedding service, so the database cannot compute it; what it can
-- guarantee is that a stored vector describes the text the row holds now. Measured on the
-- API before this migration (idx 77 slice 6 spike): a post PATCH that changed the title and
-- description without a fresh vector (no embedder configured, or the embedder failed) kept
-- the old text's vector, and the translation job (ApplyTranslation) kept the original-language
-- vector under the English text it wrote; only a reply edit cleared it. A wrong vector ranks
-- the row for text it no longer has and nothing can find it again; a missing one is listed
-- by the drift check and rebuilt by the backfill. So:
--
--   * a write that changes the text and does not store a new vector clears the old one, in
--     the same row write (BEFORE UPDATE triggers below), whoever the writer is;
--   * the vectors of posts the translation job already rewrote are cleared once here (they
--     were computed from the original-language text at creation; nothing re-embedded them);
--   * cmd/backfill-embeddings writes a vector only onto the text it was computed from, only
--     where none exists yet, and without moving updated_at (the If-Match validator).
--
-- REBUILD PATH (operator):
--   SELECT * FROM search_document_drift();   -- live rows without a vector (kind, id)
--   go run ./cmd/backfill-embeddings         -- embeds exactly those from their current text
--                                              (needs VOYAGE_API_KEY; -dry-run counts them)
--   SELECT * FROM search_document_drift();   -- 0 rows. A row whose text changed while the
--                                              command ran stays listed; run it again.
-- Replaying the command is idempotent: a row that has a vector is not read again.

CREATE OR REPLACE FUNCTION search_document_stale() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.embedding := NULL;
    RETURN NEW;
END;
$$;

-- Only writes that name the text fire these (view counts, votes and status moves do not),
-- and only a changed text with an unchanged vector clears it: a writer that stores the new
-- text's vector in the same statement keeps it.
CREATE TRIGGER posts_search_document_stale
    BEFORE UPDATE OF title, description ON posts
    FOR EACH ROW
    WHEN ((OLD.title, OLD.description) IS DISTINCT FROM (NEW.title, NEW.description)
          AND NEW.embedding IS NOT DISTINCT FROM OLD.embedding)
    EXECUTE FUNCTION search_document_stale();

CREATE TRIGGER replies_search_document_stale
    BEFORE UPDATE OF body ON replies
    FOR EACH ROW
    WHEN (OLD.body IS DISTINCT FROM NEW.body
          AND NEW.embedding IS NOT DISTINCT FROM OLD.embedding)
    EXECUTE FUNCTION search_document_stale();

-- The live rows the backfill embeds (the same predicates as cmd/backfill-embeddings).
CREATE OR REPLACE FUNCTION search_document_drift()
RETURNS TABLE (kind text, id uuid)
LANGUAGE sql STABLE AS $$
    SELECT 'post'::text, p.id FROM posts p
     WHERE p.deleted_at IS NULL AND p.embedding IS NULL
    UNION ALL
    SELECT 'reply'::text, r.id FROM replies r
     WHERE r.deleted_at IS NULL AND r.embedding IS NULL AND r.author_type <> 'system'
$$;

-- original_title is set only by ApplyTranslation; a translation that returned the same text
-- left the vector describing it. Sets only the vector: no trigger fires, updated_at stays.
UPDATE posts
   SET embedding = NULL
 WHERE embedding IS NOT NULL
   AND original_title IS NOT NULL
   AND (title, description) IS DISTINCT FROM (original_title, original_description);
