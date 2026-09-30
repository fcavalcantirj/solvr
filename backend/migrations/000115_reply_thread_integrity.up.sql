-- Reply threads are enforced by the database (idx 68 step 2: enforceable relationships).
-- Until now only ReplyRepository.Create checked that a parent reply belongs to the same
-- post, and nothing stopped a parent reference from making a reply its own ancestor. The
-- cutover, the legacy relation remap and any operator wrote replies around that check.
--
--   * Same post: the parent reference is (parent_reply_id, post_id) -> replies (id, post_id),
--     so a parent in another post is refused on insert and on update, and a parent cannot be
--     moved to another post while its children stay behind. Deleting a parent still deletes
--     its thread (ON DELETE CASCADE, as before).
--   * No cycles: a constraint trigger walks a new or changed parent's ancestors after the
--     statement and refuses the write if the reply is among them (constraint name
--     replies_parent_acyclic). It runs after the statement, so replies that name each other
--     in one multi-row INSERT are caught too. It serializes on a per-post advisory lock
--     before it reads, so two transactions that each add one half of a cycle cannot both
--     commit: the second waits and then sees the first one's parent (READ COMMITTED).
--
-- Measured before writing this (local restore of the production dump, after the cutover):
-- 3510 replies, 183 threaded, 0 with a parent in another post, 0 cycles, depth at most 2.
ALTER TABLE replies ADD CONSTRAINT replies_id_post_key UNIQUE (id, post_id);

ALTER TABLE replies DROP CONSTRAINT replies_parent_reply_id_fkey;
ALTER TABLE replies ADD CONSTRAINT replies_parent_same_post_fkey
    FOREIGN KEY (parent_reply_id, post_id) REFERENCES replies (id, post_id) ON DELETE CASCADE;

CREATE OR REPLACE FUNCTION replies_refuse_parent_cycle() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    loops boolean;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('replies.thread:' || NEW.post_id::text, 0));
    WITH RECURSIVE ancestors(id, parent_reply_id) AS (
        SELECT r.id, r.parent_reply_id FROM replies r WHERE r.id = NEW.parent_reply_id
        UNION
        SELECT r.id, r.parent_reply_id FROM replies r JOIN ancestors a ON r.id = a.parent_reply_id
    )
    SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = NEW.id) INTO loops;
    IF loops THEN
        RAISE EXCEPTION 'reply % cannot be its own ancestor', NEW.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'replies_parent_acyclic';
    END IF;
    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER replies_parent_acyclic
    AFTER INSERT OR UPDATE OF parent_reply_id ON replies
    FOR EACH ROW WHEN (NEW.parent_reply_id IS NOT NULL)
    EXECUTE FUNCTION replies_refuse_parent_cycle();
