-- Back to 000122's definitions, exactly (which a pg_restore cannot recreate).
DROP TRIGGER IF EXISTS posts_search_document_stale ON posts;
CREATE TRIGGER posts_search_document_stale
    BEFORE UPDATE OF title, description ON posts
    FOR EACH ROW
    WHEN ((OLD.title, OLD.description) IS DISTINCT FROM (NEW.title, NEW.description)
          AND NEW.embedding IS NOT DISTINCT FROM OLD.embedding)
    EXECUTE FUNCTION search_document_stale();

DROP TRIGGER IF EXISTS replies_search_document_stale ON replies;
CREATE TRIGGER replies_search_document_stale
    BEFORE UPDATE OF body ON replies
    FOR EACH ROW
    WHEN (OLD.body IS DISTINCT FROM NEW.body
          AND NEW.embedding IS NOT DISTINCT FROM OLD.embedding)
    EXECUTE FUNCTION search_document_stale();
