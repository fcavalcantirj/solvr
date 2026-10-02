-- Cutover window G3: sessions on the database other than this one. After solvr-api is stopped,
-- expect none from the API. Read-only.
SELECT usename, client_addr, application_name, state, backend_start, left(query, 60) AS last_query
  FROM pg_stat_activity
 WHERE datname = current_database() AND pid <> pg_backend_pid()
 ORDER BY backend_start;
