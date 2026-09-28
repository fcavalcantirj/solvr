-- Flags retargeted to replies go back to the legacy row the reply was migrated from; only
-- a flag on a reply with no legacy origin (none are created before cutover) is removed.
UPDATE flags SET target_type = r.legacy_type, target_id = r.legacy_id
FROM replies r
WHERE flags.target_type = 'reply' AND r.id = flags.target_id
  AND r.legacy_type IN ('approach', 'answer', 'response', 'comment');
DELETE FROM flags WHERE target_type = 'reply';
ALTER TABLE flags DROP CONSTRAINT flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment'));

-- Replies migrated from progress notes are derived copies: the notes stay in progress_notes
-- until the legacy tables are dropped, so removing the copies loses nothing.
DELETE FROM replies WHERE legacy_type = 'progress_note';
ALTER TABLE replies DROP CONSTRAINT replies_legacy_type_check;
ALTER TABLE replies ADD CONSTRAINT replies_legacy_type_check
    CHECK (legacy_type IN ('approach', 'answer', 'response', 'comment'));
