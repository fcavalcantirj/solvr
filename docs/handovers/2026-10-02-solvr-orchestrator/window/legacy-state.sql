-- Cutover window G3 and rollback R4: legacy (schema 84) state snapshot as one JSON document. Read-only.
-- Taken after the write pause (G3) and again after `migrate goto 84`, then compared.
SELECT jsonb_pretty(jsonb_build_object(
  'captured_at', now()::text,
  'database', current_database(),
  'counts', jsonb_build_object(
    'posts', (SELECT count(*) FROM posts),
    'posts_live', (SELECT count(*) FROM posts WHERE deleted_at IS NULL),
    'posts_by_type', (SELECT jsonb_object_agg(type, n) FROM (SELECT type, count(*) n FROM posts GROUP BY type) s),
    'answers', (SELECT count(*) FROM answers),
    'answers_live', (SELECT count(*) FROM answers WHERE deleted_at IS NULL),
    'approaches', (SELECT count(*) FROM approaches),
    'approaches_live', (SELECT count(*) FROM approaches WHERE deleted_at IS NULL),
    'responses', (SELECT count(*) FROM responses),
    'comments', (SELECT count(*) FROM comments),
    'comments_live', (SELECT count(*) FROM comments WHERE deleted_at IS NULL),
    'comments_by_target_type', (SELECT jsonb_object_agg(target_type, n) FROM (SELECT target_type, count(*) n FROM comments GROUP BY 1) s),
    'comments_by_author_type', (SELECT jsonb_object_agg(author_type, n) FROM (SELECT author_type, count(*) n FROM comments GROUP BY 1) s),
    'votes', (SELECT count(*) FROM votes),
    'votes_by_target_type', (SELECT jsonb_object_agg(target_type, n) FROM (SELECT target_type, count(*) n FROM votes GROUP BY 1) s),
    'rooms', (SELECT count(*) FROM rooms),
    'rooms_live', (SELECT count(*) FROM rooms WHERE deleted_at IS NULL),
    'rooms_with_token_hash', (SELECT count(*) FROM rooms WHERE token_hash IS NOT NULL),
    'room_agent_tokens', (SELECT count(*) FROM room_agent_tokens),
    'room_agent_tokens_unexpired', (SELECT count(*) FROM room_agent_tokens WHERE expires_at IS NULL OR expires_at > now()),
    'room_members', (SELECT count(*) FROM room_members),
    'messages', (SELECT count(*) FROM messages),
    'room_events', (SELECT count(*) FROM room_events),
    'users', (SELECT count(*) FROM users),
    'agents', (SELECT count(*) FROM agents)
  ),
  -- c03734ae backend/internal/db/stats.go:131 GetTotalContributionsCount, verbatim.
  'legacy_public_contributions', (SELECT
      COALESCE((SELECT COUNT(*) FROM answers WHERE deleted_at IS NULL), 0) +
      COALESCE((SELECT COUNT(*) FROM approaches WHERE deleted_at IS NULL), 0) +
      COALESCE((SELECT COUNT(*) FROM responses), 0)),
  'rooms_list', (SELECT jsonb_agg(jsonb_build_object('id', id, 'slug', slug, 'owner_id', owner_id,
      'token_md5', md5(token_hash), 'deleted', deleted_at IS NOT NULL) ORDER BY id) FROM rooms),
  'room_agent_tokens_list', (SELECT jsonb_agg(jsonb_build_object('room_id', room_id, 'agent_id', agent_id,
      'token_md5', md5(token_hash), 'expires_at', expires_at::text) ORDER BY room_id, agent_id, token_hash) FROM room_agent_tokens),
  'tombstones', (SELECT jsonb_agg(t ORDER BY t->>'kind', t->>'name') FROM (
      SELECT jsonb_build_object('kind', 'user', 'id', id::text, 'name', username, 'deleted_at', deleted_at::text) t FROM users WHERE deleted_at IS NOT NULL
      UNION ALL
      SELECT jsonb_build_object('kind', 'agent', 'id', id, 'name', display_name, 'deleted_at', deleted_at::text) FROM agents WHERE deleted_at IS NOT NULL) s),
  'trigger', jsonb_build_object(
      'exists', EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'users_refuse_tombstoned_email' AND NOT tgisinternal),
      'triggerdef', (SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgname = 'users_refuse_tombstoned_email'),
      'functiondef', (SELECT pg_get_functiondef(p.oid) FROM pg_trigger t JOIN pg_proc p ON p.oid = t.tgfoid WHERE t.tgname = 'users_refuse_tombstoned_email'))
));
