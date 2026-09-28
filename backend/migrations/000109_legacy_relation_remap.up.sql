-- Task idx 76 step 2: let the cutover retarget legacy dependents onto canonical replies.
-- Flags on approaches, answers, responses and comments move to the reply migrated from
-- them (like votes and reports already can), and progress notes become child replies of
-- their approach's reply, keeping the note id as legacy_id.
ALTER TABLE flags DROP CONSTRAINT flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment', 'reply'));

ALTER TABLE replies DROP CONSTRAINT replies_legacy_type_check;
ALTER TABLE replies ADD CONSTRAINT replies_legacy_type_check
    CHECK (legacy_type IN ('approach', 'answer', 'response', 'comment', 'progress_note'));
