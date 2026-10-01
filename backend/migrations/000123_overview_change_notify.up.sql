-- The public overview's snapshots follow the posts they list (idx 77 step 5: visibility and
-- deletion changes remove public projections promptly).
--
-- GET /v1/overview is served from a 30 s in-process snapshot on each API instance
-- (handlers.OverviewCache). Its Posts section is derived from public posts that carry a live
-- human or agent reply: id, type, title, status, tags and the reply count. Until now only room
-- changes dropped the snapshots (RoomHandler -> Pool.OverviewChanged -> solvr_overview_changed).
-- Measured before this migration (idx 77 slice 7 spike, live API): a post deleted by its author,
-- a title edit, a moderation rejection and a post's only reply deleted all stayed on the
-- overview for 30.0-30.2 s while GET /v1/posts/{id} already answered 404.
--
-- These triggers announce, at commit, every post or reply write that can change what that
-- section shows, whoever the writer is (handlers, the moderation goroutines, the translation
-- job, SQL). Every instance's room listener (Pool.ListenRooms) drops its snapshots on the
-- notice, and on every (re)LISTEN, so a notice lost while a listener reconnects is covered.
-- The notice is a wakeup only (empty payload, identical to Pool.OverviewChanged's): Postgres
-- folds identical notices of one transaction into one, so a bulk purge sends one.
--
-- Quiet on purpose: votes, views, description and reply body edits (not shown), a column
-- rewritten to its own value, system verdict replies (never counted), family-only posts and
-- their replies (never public), new posts and replies (they appear with the next snapshot;
-- nothing listed is stale).
CREATE OR REPLACE FUNCTION overview_changed_notify() RETURNS trigger AS $$
BEGIN
    -- A reply matters only while its post is public (a post deleted with its replies is
    -- announced by the posts trigger; its rows are gone by the time the replies' fire).
    -- (Nested: PL/pgSQL does not short-circuit AND, and a posts row has no post_id.)
    IF TG_TABLE_NAME = 'replies' THEN
        IF NOT EXISTS (SELECT 1 FROM posts p WHERE p.id = OLD.post_id AND p.visibility = 'public') THEN
            RETURN NULL;
        END IF;
    END IF;
    PERFORM pg_notify('solvr_overview_changed', '');
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS posts_overview_changed_update ON posts;
CREATE TRIGGER posts_overview_changed_update
    AFTER UPDATE OF deleted_at, visibility, status, title, type, tags ON posts
    FOR EACH ROW
    WHEN ((OLD.visibility = 'public' OR NEW.visibility = 'public')
          AND (OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
               OR OLD.visibility IS DISTINCT FROM NEW.visibility
               OR OLD.status IS DISTINCT FROM NEW.status
               OR OLD.title IS DISTINCT FROM NEW.title
               OR OLD.type IS DISTINCT FROM NEW.type
               OR OLD.tags IS DISTINCT FROM NEW.tags))
    EXECUTE FUNCTION overview_changed_notify();

DROP TRIGGER IF EXISTS posts_overview_changed_delete ON posts;
CREATE TRIGGER posts_overview_changed_delete
    AFTER DELETE ON posts
    FOR EACH ROW
    WHEN (OLD.visibility = 'public')
    EXECUTE FUNCTION overview_changed_notify();

DROP TRIGGER IF EXISTS replies_overview_changed_update ON replies;
CREATE TRIGGER replies_overview_changed_update
    AFTER UPDATE OF deleted_at ON replies
    FOR EACH ROW
    WHEN (OLD.author_type <> 'system' AND OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION overview_changed_notify();

DROP TRIGGER IF EXISTS replies_overview_changed_delete ON replies;
CREATE TRIGGER replies_overview_changed_delete
    AFTER DELETE ON replies
    FOR EACH ROW
    WHEN (OLD.author_type <> 'system')
    EXECUTE FUNCTION overview_changed_notify();
