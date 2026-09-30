-- Duplicate pre-check: for every unique index that depends on the default collation, group
-- the table by the index's key expressions (and partial predicate) with index scans OFF, so the
-- answer comes from the heap, not from a possibly mis-ordered index.
SET enable_indexscan = off;
SET enable_bitmapscan = off;
SET enable_indexonlyscan = off;
DO $$
DECLARE
    ix record; keys text; pred text; dups bigint; total bigint := 0; checked int := 0;
BEGIN
    FOR ix IN
        SELECT i.indexrelid, i.indrelid, i.indnkeyatts, i.indpred
        FROM pg_index i
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'public' AND i.indisunique AND 100 = ANY (i.indcollation::oid[])
        ORDER BY i.indexrelid::regclass::text
    LOOP
        SELECT string_agg(pg_get_indexdef(ix.indexrelid, k, true), ', ' ORDER BY k)
          INTO keys FROM generate_series(1, ix.indnkeyatts) AS k;
        pred := CASE WHEN ix.indpred IS NULL THEN 'true' ELSE pg_get_expr(ix.indpred, ix.indrelid) END;
        EXECUTE format('SELECT count(*) FROM (SELECT 1 FROM %s WHERE %s GROUP BY %s HAVING count(*) > 1) d',
                       ix.indrelid::regclass, pred, keys) INTO dups;
        checked := checked + 1;
        IF dups > 0 THEN
            RAISE WARNING 'DUPLICATES in %: % key groups', ix.indexrelid::regclass, dups;
        END IF;
        total := total + dups;
    END LOOP;
    RAISE NOTICE 'duplicate pre-check: % unique indexes checked, % duplicate key groups', checked, total;
END $$;
