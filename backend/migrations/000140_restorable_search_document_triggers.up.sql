-- Make 000122's two search-document triggers survive a pg_dump/pg_restore round trip
-- (spec.json idx 79 step 5, found by the restore drill).
--
-- Their WHEN clause compared the pgvector column with IS NOT DISTINCT FROM. pg_dump cannot
-- schema-qualify the "=" that construct uses, and pg_restore runs with search_path = '', so
-- "operator does not exist: public.vector = public.vector": pg_restore skipped both triggers
-- and a restored database stopped clearing a stale vector when the text changed. Comparing the
-- vectors' text forms needs only pg_catalog's text "=", which resolves under any search_path.
-- The behaviour is unchanged: a text change with an unchanged vector clears it; a writer that
-- stores the new text's vector in the same statement keeps it.
DROP TRIGGER IF EXISTS posts_search_document_stale ON posts;
CREATE TRIGGER posts_search_document_stale
    BEFORE UPDATE OF title, description ON posts
    FOR EACH ROW
    WHEN ((OLD.title, OLD.description) IS DISTINCT FROM (NEW.title, NEW.description)
          AND NEW.embedding::text IS NOT DISTINCT FROM OLD.embedding::text)
    EXECUTE FUNCTION search_document_stale();

DROP TRIGGER IF EXISTS replies_search_document_stale ON replies;
CREATE TRIGGER replies_search_document_stale
    BEFORE UPDATE OF body ON replies
    FOR EACH ROW
    WHEN (OLD.body IS DISTINCT FROM NEW.body
          AND NEW.embedding::text IS NOT DISTINCT FROM OLD.embedding::text)
    EXECUTE FUNCTION search_document_stale();
