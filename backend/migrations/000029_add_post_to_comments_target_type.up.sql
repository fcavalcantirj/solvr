-- Add 'post' to comments target_type constraint.
-- Per FIX-019: Comments can target posts directly (problems/questions/ideas).
ALTER TABLE comments DROP CONSTRAINT comments_target_type_check;
ALTER TABLE comments ADD CONSTRAINT comments_target_type_check
    CHECK (target_type IN ('approach', 'answer', 'response', 'post'));
