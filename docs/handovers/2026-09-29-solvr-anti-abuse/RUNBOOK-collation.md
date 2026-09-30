# Runbook — production collation mismatch (anti-abuse W6)

Production's `slvr` database was created under glibc 2.41. The server now runs on glibc 2.36 (`17.9 (Debian 17.9-1.pgdg12+1)`), so every connection and `pg_dump` warns about a collation version mismatch. This runbook repairs it. **Every production step below is FELIPE'S CALL and runs only inside the v1.3 cutover's write pause (gate G3), before the migrations (G4).**

## What was rehearsed (2026-09-29, local, measured)

**Faithful reproduction.** Stamping `datcollversion` only fakes the warning. Instead:
1. The pre-purge dump (`db-backups/solvr_prod_2026-09-29_19-28-15_prepurge.dump`) was restored into a throwaway `pgvector/pgvector:pg17-trixie` container, glibc `2.41-12+deb13u3`. That gave `slvr`, `en_US.utf8`, `datcollversion` 2.41, 2,225 posts, and no restore errors.
2. The container was stopped, and the **same data directory** was started under `pgvector/pgvector:pg17` (bookworm, glibc `2.36-9+deb12u14`). The indexes were built under 2.41 and are now read under 2.36, which is production's exact condition.
3. Connecting printed production's warning verbatim: `database "slvr" has a collation version mismatch … created using collation version 2.41, but the operating system provides version 2.36`.

**Measurements, under 2.36, before any repair:**

| Check | Script | Result |
|---|---|---|
| Indexes depending on the default collation | `collation-inventory.sql` | **92, of which 34 unique** (matches the handover) |
| Duplicate pre-check on the 34 unique indexes, index scans off | `collation-dup-precheck.sql` | **0 duplicate key groups** (0.34 s) |
| `amcheck` `bt_index_check(idx, heapallindexed => true)` | `collation-amcheck.sql` | **89 btree indexes checked, 0 failed** (0.59 s); the other 3 are not btree |
| Targeted `REINDEX INDEX` of the 92 | — | **1.8 s** |
| `REINDEX DATABASE slvr` | — | **3.7 s** (includes the HNSW vector indexes) |
| `ALTER DATABASE slvr REFRESH COLLATION VERSION` | — | **0.1 s** (`changing version from 2.41 to 2.36`) |
| After the repair | — | `datcollversion` 2.36, no warning, `amcheck` 89/0 |

**Reading of the results** (MEASURED on the pre-purge data; INFERRED for production today): no index is mis-ordered on this data under the 2.41 → 2.36 change, so the repair is a precaution plus the warning's removal, not a data fix. Production today holds less data (632 posts), so the timings above are an upper bound. That holds unless it changed after the dump; the read-only pre-check in step (b) is what decides.

**Recommendation:** `REINDEX DATABASE slvr`. It takes seconds, needs no index list, and matches the server's own HINT.

## Production steps (inside G3, the API stopped, each on Felipe's "yes")

Connection: the same route the cutover uses, the `psql` client in the local `solvr-postgres` container with the `SOLVR_DB_*` keys from `.env` (names only here; never paste values):

```bash
set -a; source .env; set +a
PSQL=(docker exec -i -e PGPASSWORD="$SOLVR_DB_PASSWORD" solvr-postgres
      psql -h "$SOLVR_DB_HOST" -p "$SOLVR_DB_PORT" -U "$SOLVR_DB_USER" -d "$SOLVR_DB_NAME" -v ON_ERROR_STOP=1)
```

**(a) Read-only: which databases are affected.**
```bash
"${PSQL[@]}" -c "SELECT datname, datcollversion, pg_database_collation_actual_version(oid) FROM pg_database"
```
Expect `slvr | 2.41 | 2.36`. In the rehearsal, `postgres` and `template1` were mismatched too because they were created under 2.41. If production shows them, repeat (d) and (e) for each; they hold almost no objects.

**(b) Read-only: duplicate pre-check.**
```bash
"${PSQL[@]}" < docs/handovers/2026-09-29-solvr-anti-abuse/collation-dup-precheck.sql
```
Expect `NOTICE: duplicate pre-check: 34 unique indexes checked, 0 duplicate key groups`. **Any `WARNING: DUPLICATES in …` means STOP.** A unique index would fail to rebuild. What to keep is FELIPE'S CALL.

**(c) Optional, its own gate: amcheck.** This needs `CREATE EXTENSION amcheck`, which is a write.
```bash
"${PSQL[@]}" < docs/handovers/2026-09-29-solvr-anti-abuse/collation-amcheck.sql
```
Expect `0 failed`. It is informational: step (d) rebuilds every index either way.

**(d) Rebuild.** Expect seconds.
```bash
"${PSQL[@]}" -c "REINDEX DATABASE slvr"
```

**(e) Record the new version.**
```bash
"${PSQL[@]}" -c "ALTER DATABASE slvr REFRESH COLLATION VERSION"
```
Expect `NOTICE: changing version from 2.41 to 2.36`. Re-run (a) to confirm there is no mismatch.

**(f)** Continue with G4 (migrations 85 → 114).

## Keep it from happening again

Pin the production Postgres image to a Debian-suffixed tag or a digest in EasyPanel (for example `pgvector/pgvector:0.8.x-pg17-bookworm`, or `@sha256:…`), so a base-image change cannot swap glibc under the data directory again. The mismatch came from exactly such a switch: data created under Debian 13 (glibc 2.41), server now on Debian 12 (2.36). The cause is INFERRED from the version strings. Changing the image is FELIPE'S CALL, made in EasyPanel.
