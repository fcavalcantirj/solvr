-- Rehearsal aid: committed transactions so far on this database (round trips between two readings).
SELECT xact_commit + xact_rollback AS xacts FROM pg_stat_database WHERE datname = current_database();
