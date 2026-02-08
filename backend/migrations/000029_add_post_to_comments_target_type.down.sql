-- Revert: remove 'post' from comments target_type constraint.
ALTER TABLE comments DROP CONSTRAINT comments_target_type_check;
ALTER TABLE comments ADD CONSTRAINT comments_target_type_check
    CHECK (target_type IN ('approach', 'answer', 'response'));
