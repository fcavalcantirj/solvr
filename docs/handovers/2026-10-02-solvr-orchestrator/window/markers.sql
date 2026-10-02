-- Cutover window P1: read-only pre-window checks on production (schema 84, no schema_migrations).
-- Run through PSQL_RO (default_transaction_read_only=on). Expected values are in the runbook.
SELECT jsonb_pretty(jsonb_build_object(
  'database', current_database(),
  'server_version', current_setting('server_version'),
  'ssl', current_setting('ssl'),
  'collation', (SELECT jsonb_build_object('datcollversion', datcollversion,
      'actual', pg_database_collation_actual_version(oid)) FROM pg_database WHERE datname = current_database()),
  'schema_migrations_present', to_regclass('public.schema_migrations') IS NOT NULL,
  -- Present at 84 (82: search_queries.public_scope, 83: api_request_events, 84: messages columns).
  'markers_84_present', jsonb_build_object(
      'search_queries.public_scope', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'search_queries' AND column_name = 'public_scope'),
      'api_request_events', to_regclass('public.api_request_events') IS NOT NULL,
      'messages.reply_to_entry_id', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'messages' AND column_name = 'reply_to_entry_id'),
      'messages.addressed_member_ids', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'messages' AND column_name = 'addressed_member_ids')),
  -- Absent at 84 (85, 86, 87, 89) and never present on production (000024's tags tables).
  'markers_85_plus_present', jsonb_build_object(
      'messages.client_entry_id', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'messages' AND column_name = 'client_entry_id'),
      'rooms.archived_at', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'rooms' AND column_name = 'archived_at'),
      'messages.pinned_at', EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'messages' AND column_name = 'pinned_at'),
      'replies', to_regclass('public.replies') IS NOT NULL,
      'tags', to_regclass('public.tags') IS NOT NULL,
      'post_tags', to_regclass('public.post_tags') IS NOT NULL),
  'rate_limit_config', (SELECT jsonb_object_agg(key, value) FROM rate_limit_config),
  'other_sessions', (SELECT coalesce(jsonb_agg(jsonb_build_object('user', usename, 'client', client_addr::text,
      'application', application_name, 'state', state) ORDER BY backend_start), '[]'::jsonb)
      FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid())
));
