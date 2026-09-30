-- Indexes whose ordering depends on the database's default collation (collation oid 100).
SELECT count(*) AS collation_indexes,
       count(*) FILTER (WHERE ix.indisunique) AS unique_indexes
FROM pg_index ix
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = 'public' AND 100 = ANY (ix.indcollation::oid[]);
