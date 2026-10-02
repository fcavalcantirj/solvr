-- Cutover window G5: break the /v1/stats contribution count (canonical, stats_canonical.go:38) down against
-- the legacy count (c03734ae stats.go:131: live answers + live approaches + all responses). Read-only.
WITH r AS (
  SELECT coalesce(r.legacy_type, '<native>') AS lt,
         (r.deleted_at IS NULL AND r.author_type <> 'system' AND p.deleted_at IS NULL AND p.visibility = 'public') AS counted_new,
         (r.author_type = 'system') AS is_system,
         (p.deleted_at IS NOT NULL) AS on_deleted_post,
         (p.visibility <> 'public') AS on_nonpublic_post,
         (r.deleted_at IS NOT NULL) AS reply_deleted,
         CASE r.legacy_type
           WHEN 'answer' THEN EXISTS (SELECT 1 FROM answers a WHERE a.id = r.legacy_id AND a.deleted_at IS NULL)
           WHEN 'approach' THEN EXISTS (SELECT 1 FROM approaches a WHERE a.id = r.legacy_id AND a.deleted_at IS NULL)
           WHEN 'response' THEN true
           ELSE false END AS counted_legacy
    FROM replies r JOIN posts p ON p.id = r.post_id
)
SELECT jsonb_pretty(jsonb_build_object(
  'new_count', (SELECT count(*) FILTER (WHERE counted_new) FROM r),
  'legacy_count_from_tables', (SELECT
      COALESCE((SELECT COUNT(*) FROM answers WHERE deleted_at IS NULL), 0) +
      COALESCE((SELECT COUNT(*) FROM approaches WHERE deleted_at IS NULL), 0) +
      COALESCE((SELECT COUNT(*) FROM responses), 0)),
  'legacy_count_via_replies', (SELECT count(*) FILTER (WHERE counted_legacy) FROM r),
  'by_legacy_type', (SELECT jsonb_object_agg(lt, jsonb_build_object(
        'replies', n, 'counted_new', cn, 'counted_legacy', cl, 'delta', cn - cl,
        'system_excluded', sys, 'on_deleted_post', del, 'on_nonpublic_post', np, 'reply_deleted', rd))
      FROM (SELECT lt, count(*) n, count(*) FILTER (WHERE counted_new) cn, count(*) FILTER (WHERE counted_legacy) cl,
                   count(*) FILTER (WHERE is_system) sys, count(*) FILTER (WHERE on_deleted_post) del,
                   count(*) FILTER (WHERE on_nonpublic_post) np, count(*) FILTER (WHERE reply_deleted) rd
              FROM r GROUP BY lt) s),
  -- Legacy-counted rows that are not replies at all (orphans): they would be a residual.
  'legacy_counted_without_reply', (SELECT
      (SELECT count(*) FROM answers a WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM replies x WHERE x.legacy_type = 'answer' AND x.legacy_id = a.id))
    + (SELECT count(*) FROM approaches a WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM replies x WHERE x.legacy_type = 'approach' AND x.legacy_id = a.id))
    + (SELECT count(*) FROM responses a WHERE NOT EXISTS (SELECT 1 FROM replies x WHERE x.legacy_type = 'response' AND x.legacy_id = a.id)))
));
