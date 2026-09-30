-- Back to the 000089 parent reference: any existing reply, no same-post or cycle check in
-- the database (ReplyRepository.Create still checks the post).
DROP TRIGGER IF EXISTS replies_parent_acyclic ON replies;
DROP FUNCTION IF EXISTS replies_refuse_parent_cycle();
ALTER TABLE replies DROP CONSTRAINT IF EXISTS replies_parent_same_post_fkey;
ALTER TABLE replies DROP CONSTRAINT IF EXISTS replies_id_post_key;
ALTER TABLE replies ADD CONSTRAINT replies_parent_reply_id_fkey
    FOREIGN KEY (parent_reply_id) REFERENCES replies (id) ON DELETE CASCADE;
