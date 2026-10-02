-- Cutover window G5: reconciliation after the cutover (schema 135, legacy tables still present). Read-only.
SELECT jsonb_pretty(jsonb_build_object(
  'captured_at', now()::text,
  'database', current_database(),
  'schema', (SELECT jsonb_build_object('version', version, 'dirty', dirty) FROM schema_migrations),
  'legacy', jsonb_build_object(
    'approach', (SELECT count(*) FROM approaches), 'answer', (SELECT count(*) FROM answers),
    'response', (SELECT count(*) FROM responses), 'comment', (SELECT count(*) FROM comments),
    'progress_note', (SELECT count(*) FROM progress_notes)),
  'replies_by_legacy_type', (SELECT jsonb_object_agg(coalesce(legacy_type, '<native>'), n) FROM
      (SELECT legacy_type, count(*) n FROM replies GROUP BY 1) s),
  'replies_total', (SELECT count(*) FROM replies),
  'replies_system_author', (SELECT count(*) FROM replies WHERE author_type = 'system'),
  'replies_top_level_vs_child', (SELECT jsonb_build_object('top', count(*) FILTER (WHERE parent_reply_id IS NULL),
      'child', count(*) FILTER (WHERE parent_reply_id IS NOT NULL)) FROM replies),
  -- Every legacy contribution without a reply, by id, with the reason read from the data.
  'orphans', (SELECT coalesce(jsonb_agg(o ORDER BY o->>'legacy_type', o->>'id'), '[]'::jsonb) FROM (
      SELECT jsonb_build_object('legacy_type', 'approach', 'id', a.id, 'parent', a.problem_id,
             'reason', CASE WHEN p.id IS NULL THEN 'parent post hard-deleted (row absent)' ELSE 'post present: UNEXPLAINED' END) o
        FROM approaches a LEFT JOIN posts p ON p.id = a.problem_id
       WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = a.id)
      UNION ALL
      SELECT jsonb_build_object('legacy_type', 'answer', 'id', a.id, 'parent', a.question_id,
             'reason', CASE WHEN p.id IS NULL THEN 'parent post hard-deleted (row absent)' ELSE 'post present: UNEXPLAINED' END)
        FROM answers a LEFT JOIN posts p ON p.id = a.question_id
       WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'answer' AND r.legacy_id = a.id)
      UNION ALL
      SELECT jsonb_build_object('legacy_type', 'response', 'id', a.id, 'parent', a.idea_id,
             'reason', CASE WHEN p.id IS NULL THEN 'parent post hard-deleted (row absent)' ELSE 'post present: UNEXPLAINED' END)
        FROM responses a LEFT JOIN posts p ON p.id = a.idea_id
       WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'response' AND r.legacy_id = a.id)
      UNION ALL
      SELECT jsonb_build_object('legacy_type', 'comment', 'id', c.id, 'parent', c.target_type || ':' || c.target_id,
             'author_type', c.author_type,
             'reason', CASE WHEN CASE c.target_type
                    WHEN 'post' THEN EXISTS (SELECT 1 FROM posts t WHERE t.id = c.target_id)
                    WHEN 'approach' THEN EXISTS (SELECT 1 FROM approaches t WHERE t.id = c.target_id)
                    WHEN 'answer' THEN EXISTS (SELECT 1 FROM answers t WHERE t.id = c.target_id)
                    WHEN 'response' THEN EXISTS (SELECT 1 FROM responses t WHERE t.id = c.target_id)
                    ELSE false END
                  THEN 'target present: UNEXPLAINED' ELSE 'comment target hard-deleted (row absent)' END)
        FROM comments c
       WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'comment' AND r.legacy_id = c.id)
      UNION ALL
      SELECT jsonb_build_object('legacy_type', 'progress_note', 'id', n.id, 'parent', n.approach_id,
             'reason', CASE WHEN NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = n.approach_id)
                  THEN 'approach has no migrated reply' ELSE 'approach migrated: UNEXPLAINED' END)
        FROM progress_notes n
       WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'progress_note' AND r.legacy_id = n.id)) s),
  'drift', jsonb_build_object(
    'vote_score_drift', (SELECT count(*) FROM vote_score_drift()),
    'room_activity_drift', (SELECT count(*) FROM room_activity_drift()),
    'view_count_drift', (SELECT count(*) FROM view_count_drift()),
    'agent_reputation_drift', (SELECT count(*) FROM agent_reputation_drift()),
    'search_document_drift', (SELECT count(*) FROM search_document_drift())),
  'votes_by_target_type', (SELECT jsonb_object_agg(target_type, n) FROM (SELECT target_type, count(*) n FROM votes GROUP BY 1) s),
  -- Owner at 135 = the earliest active human 'owner' membership (000095/000097).
  'rooms_list', (SELECT jsonb_agg(jsonb_build_object('id', r.id, 'slug', r.slug,
      'owner_id', (SELECT m.user_id FROM room_members m WHERE m.room_id = r.id AND m.role = 'owner'
                    AND m.revoked_at IS NULL AND m.user_id IS NOT NULL ORDER BY m.created_at, m.id LIMIT 1),
      'deleted', r.deleted_at IS NOT NULL) ORDER BY r.id) FROM rooms r),
  'room_entries', (SELECT count(*) FROM room_entries),
  'legacy_messages_plus_events', (SELECT count(*) FROM legacy_messages) + (SELECT count(*) FROM legacy_room_events),
  'tombstones', (SELECT jsonb_agg(t ORDER BY t->>'kind', t->>'name') FROM (
      SELECT jsonb_build_object('kind', 'user', 'id', id::text, 'name', username, 'deleted_at', deleted_at::text) t FROM users WHERE deleted_at IS NOT NULL
      UNION ALL
      SELECT jsonb_build_object('kind', 'agent', 'id', id, 'name', display_name, 'deleted_at', deleted_at::text) FROM agents WHERE deleted_at IS NOT NULL) s),
  'trigger', jsonb_build_object(
      'exists', EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'users_refuse_tombstoned_email' AND NOT tgisinternal),
      'triggerdef', (SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgname = 'users_refuse_tombstoned_email'),
      'functiondef', (SELECT pg_get_functiondef(p.oid) FROM pg_trigger t JOIN pg_proc p ON p.oid = t.tgfoid WHERE t.tgname = 'users_refuse_tombstoned_email')),
  'banned_identities', (SELECT jsonb_object_agg(kind, n) FROM (SELECT kind, count(*) n FROM banned_identities GROUP BY 1) s),
  'cutover_ledger', (SELECT jsonb_build_object('runs', count(DISTINCT run_id), 'steps', count(*),
      'errors', count(*) FILTER (WHERE error IS NOT NULL), 'unfinished', count(*) FILTER (WHERE finished_at IS NULL)) FROM cutover_ledger)
));
