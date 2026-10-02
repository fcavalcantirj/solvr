-- Notification events name the canonical post and reply they are about, under a documented
-- schema version (task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the
-- redesigned product", step 4; SPEC.md Part 5.6).
--
-- schema_version: the event contract a row was written under. Existing rows, and any writer
-- that does not state a version, get 0: outside the contract, a type that may be a retired name
-- and no subject. The repository writes 1 for the events of the documented set.
--
-- post_id / reply_id: the subject, as typed columns with enforced relationships. A reply is
-- named with its post, and must belong to that post (the composite key against
-- replies(id, post_id)). Deleting the subject keeps the notification and clears the column:
-- a reply delete clears reply_id; a post delete clears post_id, and its cascaded replies clear
-- reply_id. A CHECK that reply_id implies post_id is not used: it is evaluated row by row while
-- a post delete clears post_id before its cascaded replies clear reply_id, so it would refuse
-- every hard delete of a post with a reply event (measured on a throwaway database). The
-- repository refuses a reply without its post instead.
ALTER TABLE notifications
    ADD COLUMN schema_version SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    ADD COLUMN reply_id UUID REFERENCES replies(id) ON DELETE SET NULL,
    ADD CONSTRAINT notifications_schema_version_check CHECK (schema_version IN (0, 1)),
    ADD CONSTRAINT notifications_reply_of_post_fkey FOREIGN KEY (reply_id, post_id) REFERENCES replies(id, post_id);

-- The ON DELETE actions look rows up by subject.
CREATE INDEX idx_notifications_post_id ON notifications(post_id) WHERE post_id IS NOT NULL;
CREATE INDEX idx_notifications_reply_id ON notifications(reply_id) WHERE reply_id IS NOT NULL;
