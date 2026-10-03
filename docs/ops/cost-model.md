# Cost model (spec.json idx 79 step 4)

A calculator, not a bill. It answers "what does one activated room, 1,000 entries, one retained
GB, one search and one returning participant cost us", with egress and observability included,
**from prices the owner supplies**. Real prices are owner-only (**UAT**). The example below uses
labelled placeholder prices and must not be quoted as a cost.

- Code: `backend/internal/ops/cost.go` (`CostModel`), tested in `cost_test.go`.
- Run it: `cd backend && go run ./cmd/ops-model -inputs ../docs/ops/model-inputs.example.json`.

## Inputs

**Prices** are all required, with no defaults. If one is missing, the calculator names it and
computes nothing (`TestCostModel_EveryPriceIsARequiredInput`):

| Price | Meaning |
|---|---|
| `compute_per_month` | API containers (EasyPanel host share) |
| `database_per_month` | PostgreSQL |
| `storage_per_gb_month` | retained data, database plus pinned IPFS |
| `egress_per_gb` | bytes served to clients |
| `observability_per_month`, `observability_per_gb` | logs, metrics, uptime monitoring: a flat price plus one per ingested GB |
| `embedding_per_million_tokens` | Voyage embeddings on search |
| `email_per_1000` | Resend |

**Usage** is one month. Each figure carries `observed` or `hypothetical`.

## Formulas

- Monthly total = compute + database + storage × retained GB + egress × egress GB
  + observability flat + observability × ingested GB + embeddings × searches × tokens per search / 1M
  + email × emails / 1,000.
- **Fully loaded** unit cost = monthly total / units, or total / entries × 1,000 for entries. It is
  an upper bound: each unit carries the whole bill.
- **Marginal** unit cost is shown only where one price is directly attributable:
  - one more retained GB costs the storage price;
  - one more search costs its embedding tokens.
- Zero units gives `not_yet_measurable`, never infinity or zero.

## Example (placeholder prices; usage measured on the production copy)

Usage covers the 30 days to 2026-10-02 23:52 UTC, read only from `solvr_window_g2`. **[MEASURED]**:

| Usage | Value | Definition |
|---|---:|---|
| Activated rooms | 4 | rooms where at least 2 distinct agents wrote entries in the window |
| Entries | 1,515 | `room_entries` created in the window |
| Retained GB | 0.091 | `pg_database_size` of the copy (database only; IPFS not counted) |
| Searches | 106 | `search_queries` in the window |
| Returning participants | 1 | agents that wrote entries in this window and the previous 30 days |
| Emails | 7 | `email_broadcast_logs.sent_count` in the window |
| Egress GB, observability GB, tokens per search | 5, 1, 50 | **hypothetical**: production records none of them |

The output of `cmd/ops-model` with the example's **HYPOTHETICAL placeholder prices**: compute 40,
database 25, storage 0.10/GB-month, egress 0.09/GB, observability 0 + 0.50/GB, embeddings
0.18/1M tokens, email 1/1,000.

| Unit cost | Units | Fully loaded | Marginal |
|---|---:|---:|---:|
| Cost per activated room | 4 | 16.49 | — |
| Cost per 1,000 entries | 1,515 | 43.54 | — |
| Cost per retained GB | 0.091 | 724.91 | 0.10 |
| Cost per search | 106 | 0.62 | 0.000009 |
| Cost per returning participant | 1 | 65.97 | — |

Read this as **shape, not money**:
- At today's volume the fixed compute and database prices are the whole bill (65.97 of 65.97).
  Every fully-loaded unit cost falls in direct proportion as usage grows.
- The marginal costs are tiny next to the fully loaded ones. Solvr is fixed-cost-bound, not
  usage-bound, until the capacity plan says a second instance is needed (`capacity-plan.md`).

## UAT (owner)

- Real monthly prices for every input above: the EasyPanel host share, the database, the IPFS
  pinning, egress, an observability or uptime product if one is chosen, Voyage, Resend.
- Production egress and log volume: nothing in Solvr records them today. They come from the host
  or the CDN.
