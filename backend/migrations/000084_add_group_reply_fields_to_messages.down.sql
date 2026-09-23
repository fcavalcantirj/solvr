-- Rollback: remove group reply and addressing fields.
ALTER TABLE messages DROP COLUMN addressed_member_ids;
ALTER TABLE messages DROP COLUMN reply_to_entry_id;
