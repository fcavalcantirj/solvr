# Runbook — unpin the purged content from IPFS (anti-abuse W5)

The 2026-09-29 purge removed the rows, but the IPFS node (`solvr-ipfs-01`, Kubo v0.39.0) still pins their content. `ipfs-unpin-cids.csv` holds the 87 CIDs: 67 pins (`agent_openclaw_mack` 62, `agent_frogtrader` 5) and 20 crystallized posts. On 2026-09-29 no surviving `pins` row or post referenced any of them. The endpoint re-checks that at run time.

**Preconditions:** v1.3 is deployed (cutover G6 done), so `POST /admin/ipfs/unpin` and `POST /admin/ipfs/gc` exist. `ADMIN_API_KEY` comes from `.env` (name only; never paste the value).

## What the endpoint does

`POST /admin/ipfs/unpin` takes `{"cids": [...up to 500], "dry_run": bool}` and reports per CID:

| Status | Meaning |
|---|---|
| `invalid` | not a CIDv0 (`Qm…`) or base32 CIDv1 (`b…`); never sent to the node |
| `in_use_pin` | a `pins` row still names it (any owner, any state); never unpinned |
| `in_use_post` | a post's `crystallization_cid` still names it (deleted posts included); never unpinned |
| `would_unpin` | dry run: eligible |
| `unpinned` | the node removed the pin |
| `not_pinned` | the node had no direct pin (Kubo: `not pinned or pinned indirectly`) |
| `error` | anything else, with the node's message |

Kubo's pin set is node-wide, which is why any row naming a CID blocks it. The CID is URL-escaped in every request to the node.

## Steps

```bash
set -a; source .env; set +a
CSV=docs/handovers/2026-09-29-solvr-anti-abuse/ipfs-unpin-cids.csv
tail -n +2 "$CSV" | cut -d, -f3 | jq -R . | jq -s '{cids: ., dry_run: true}'  > /tmp/unpin-dry.json
tail -n +2 "$CSV" | cut -d, -f3 | jq -R . | jq -s '{cids: ., dry_run: false}' > /tmp/unpin-run.json
jq '.cids | length' /tmp/unpin-dry.json   # expect 87
```

**1. Dry run.** It is read-only and makes no call to the node.
```bash
curl -s -X POST https://api.solvr.dev/admin/ipfs/unpin -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" --data @/tmp/unpin-dry.json | tee /tmp/unpin-dry-report.json | jq .data.summary
```
Expect `{"would_unpin": 87}`. **Any `in_use_*`, `invalid` or `error`: STOP** and look at those CIDs first.

**2. [Felipe gate] The real run.**
```bash
curl -s -X POST https://api.solvr.dev/admin/ipfs/unpin -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" --data @/tmp/unpin-run.json | tee /tmp/unpin-report.json | jq .data.summary
```
Expect `unpinned` for every CID, or `not_pinned` for any the node never pinned directly. Keep `/tmp/unpin-report.json` as the record.

**3. [Separate Felipe gate] Garbage collection.** Unpinning only marks blocks collectable; `repo/gc` deletes them. It can take minutes and is not reversible. After it, the content cannot be re-pinned from this node.
```bash
curl -s -X POST https://api.solvr.dev/admin/ipfs/gc -H "X-Admin-API-Key: $ADMIN_API_KEY" --max-time 700 | jq .
```
Expect `{"data": {"removed": N}}`.

## Verification status (2026-09-29)

- **[TEST]** Handler unit tests with fakes: dry run, both in-use refusals, invalid CIDs (including `&arg=` and `+/`), unpinned, not_pinned, gc.
- **[TEST]** Escaping of `arg=` at every site (`pin/add`, `pin/rm`, `pin/ls`, `dag/stat`) against a recording HTTP server.
- **[REAL, local node]** Kubo 0.33.2 in `solvr-ipfs`, probed from inside the container: the first `pin/rm` returns 200 `{"Pins":[…]}`; the second returns 500 and `Error: not pinned or pinned indirectly`, which the endpoint maps to `not_pinned`.
- **[UNVERIFIED]** The end-to-end Go test against a live node (`TestAdminIPFS_UnpinOnAKuboNode`) could not run from the host: Docker's port proxy on `localhost:5001` returned empty replies while the node answered inside the container. Production is Kubo 0.39.0 and was not touched.
