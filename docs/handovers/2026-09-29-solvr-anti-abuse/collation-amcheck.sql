-- B-tree structure and heap-coverage check on every collation-dependent btree index.
CREATE EXTENSION IF NOT EXISTS amcheck;
DO $$
DECLARE ix record; bad int := 0; checked int := 0;
BEGIN
    FOR ix IN
        SELECT i.indexrelid
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_am am ON am.oid = c.relam
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'public' AND am.amname = 'btree' AND 100 = ANY (i.indcollation::oid[])
    LOOP
        checked := checked + 1;
        BEGIN
            PERFORM bt_index_check(ix.indexrelid, true);
        EXCEPTION WHEN OTHERS THEN
            bad := bad + 1;
            RAISE WARNING 'amcheck FAILED on %: %', ix.indexrelid::regclass, SQLERRM;
        END;
    END LOOP;
    RAISE NOTICE 'amcheck: % btree indexes checked, % failed', checked, bad;
END $$;
