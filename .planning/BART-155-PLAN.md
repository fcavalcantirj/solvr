# BART-155 — Expose relevance score + a decidable "no match" in GET /search

Repo: `/Users/fcavalcanti/dev/solvr` · Base HEAD: `980b157` (main; BART-151..154 shipped + deployed).
This markdown IS the implementation plan; a fresh agent executes it. Follow the Golden Rules
(TDD, verify-before-declare [REAL]/[TEST], API-first, ~900-line file cap, no stubs).

## Context (the problem)

The learning-wheel gate needs a decidable "has a source already answered this?" signal. Today
`GET /v1/search` cannot answer that:

- `score` IS already returned (`models.SearchResult/SearchResultResponse.Score float64
  json:"score"`, copied by `ToResponse()`), but it is a **raw, tiny, method-dependent** number:
  hybrid returns the real RRF score `hs.rrf_score` (`db/search.go:259`, ~0.008–0.049); fulltext/
  answers/approaches return `ts_rank` (`db/search.go:167,337,410`, ~0.01–0.6). The two scales are
  not comparable, and neither is a probability. SPEC's `"score":0.95` (SPEC.md:795) implies a 0–1
  scale the data does not honor.
- There is **no `min_score` param** and **no "no confident match" signal** in the REST response.
  `buildTsQuery` (`db/search.go:462-499`) joins terms with OR + prefix (`word:*`), so any partial
  word match returns low-relevance rows — real queries **almost never return empty**. The only
  negative signal today is `total:0` from an empty/unmatched tsquery, and a "No results found"
  TEXT string in the MCP tool only (`mcp.go:274-279`).
- Net: an agent cannot decide "answered?" — it sees uninterpretable scores and near-never-empty
  results. The wheel must be able to bias to ASK when borderline.

Intended outcome: expose a **calibrated 0–1 relevance (cosine similarity)** alongside the raw
score, add a caller-set threshold that yields an **honest empty / below-threshold** result, and a
`meta` summary (`top_similarity` + a `confident_match` boolean) so "answered?" is decidable — with
a conservative default that biases to ASK. All additive; no consumer breakage.

## Current state — verified facts (with file:line)

- Response structs + `ToResponse()`: `backend/internal/models/search.go:8-100`. `SearchOptions`
  (no MinScore yet): `search.go:59-73`.
- Score per path: hybrid `hs.rrf_score` (`db/search.go:259`, `ORDER BY hs.rrf_score DESC` :286);
  fulltext `ts_rank` (:167); answers/approaches `ts_rank` (:337,:410). `scanSearchResults` scans
  score as the 11th column (`db/search.go:583`). Merge sorts by `Score` desc across sources
  (`db/search.go:110-113`) — mixes RRF + ts_rank scales when content_types spans sources.
- hybrid_search RRF formula + the semantic gate `embedding <=> query_embedding < 0.85` (cosine
  distance; = similarity > 0.15, loose): `backend/migrations/000080_add_post_visibility.up.sql:32-67`.
  Called `hybrid_search($1,$2,$3,2.0,1.0,60,$5::uuid)` (`db/search.go:267`); `$2` is the query
  vector, available in the outer SELECT already.
- Handler builds envelope + parses params: `backend/internal/api/handlers/search.go:77-195`.
  `SearchResponseMeta{Query,Total,Page,PerPage,HasMore,TookMs,Method}` (`search.go:52-61`) —
  extend this. Repo interface `Search(ctx,query,opts) ([]SearchResult,int,string,error)`
  (`search.go:19-23`).
- Consumers (do NOT break): frontend reads `meta.method/total/has_more` and IGNORES `score`
  (`frontend/hooks/use-posts.ts:221-238`; `lib/api-types.ts:49-50` types it, never renders it) —
  additive fields are safe. Skill prints `score` + meta (`skill/scripts/solvr.sh:75-80`); `--json`
  passes new fields through. MCP prints a MISLEADING `Relevance: int(Score*100)%`
  (`mcp.go:384-386`) and has the only no-match text (`mcp.go:274-279`).
- Tests are thin: only `db/search_test.go:100-103` asserts `Score != 0`; NO handler test asserts
  the response `score`; NO empty-result test anywhere; MCP `executeSearch`/`formatSearchResults`
  untested. `meta.method` well covered (`search_method_test.go`), `has_more` covered
  (`search_test.go:485-532`).

## Locked decisions (from Felipe — do not re-litigate)

1. **Rank by hybrid RRF, decide by cosine.** Keep hybrid RRF for ranking/recall (finding the
   candidate). The confidence signal MUST be normalized cosine similarity (0–1) — the interpretable
   "is this semantically the same question?" number. `similarity` (per result) and
   `meta.top_similarity` are cosine, not RRF.
2. **`confident_match` boolean, ASK-biased.** `false → ASK`. Bake the bias in: a false-skip silently
   drops a real question (the dangerous mode); a false-ask costs one tap. The boundary leans toward
   ASK → use a conservative (high) threshold.
3. **Both a client `?min_similarity=` param AND a server `SEARCH_CONFIDENCE_THRESHOLD` config
   default.** The caller must pass its own per-query threshold (SPIKE-2 will calibrate the real
   cutoff by seeding known family Q&A and measuring false-skip rate). `0.85` is a fine default but
   MUST be overridable per request.
4. **Honest empty = empty.** The old behavior ("never returns empty; returns 20/60 fuzzy prefix-OR
   fallbacks") is exactly what made the decision impossible. When nothing clears the bar, return a
   TRUE empty result (`data:[]`, `total:0`) — no fuzzy fallbacks. This is the critical fix.
5. Solvr exposes the calibrated cosine + `confident_match`; SPIKE-2 (downstream, not this ticket)
   tunes the final cutoff. This ticket is the last gate on the wheel — once score-returned +
   honest-empty is live, Felipe verifies end-to-end and SPIKE-2 becomes runnable.

## Design (locked to the decisions above)

Expose the calibrated signal as **cosine similarity (0–1)** — the correct "is this the same
question?" measure — and let the caller threshold on it, returning honest empty below the bar.
Keep the raw `score` for RANKING only. Migration-free (compute similarity in the existing hybrid
SELECT).

1. **Per-result `similarity` (0–1 cosine).** Add `Similarity *float64 json:"similarity,omitempty"`
   to `SearchResult` + `SearchResultResponse` (+ `ToResponse`). Populate in `searchPostsHybrid` by
   adding `CASE WHEN p.embedding IS NOT NULL THEN 1 - (p.embedding <=> $2::vector) END AS similarity`
   to the outer SELECT (the query vector `$2` is already bound) and scanning it. It is the true
   semantic closeness of each returned post to the query. NULL for fulltext-only path
   (`searchPosts`, no query vector) and for answers/approaches (ts_rank-only) — absent field, and
   `meta.method=="fulltext"` tells the caller "keyword-only, no semantic confidence → bias to ASK".

2. **`min_similarity` query param** (float 0–1) + **`SEARCH_CONFIDENCE_THRESHOLD` config default**
   (both — decision #3). Add `MinSimilarity float64` to `SearchOptions`; parse in the handler after
   per_page (`search.go:143-149`); when the param is absent, fall back to the config default. Apply
   the filter in the repo `Search()` AFTER the per-source queries + merge sort, BEFORE pagination:
   **keep only results with `Similarity != nil && *Similarity >= threshold`.** Per decision #4
   (honest empty) + #2 (ASK-bias): results we CANNOT measure semantically — nil similarity
   (keyword-only fulltext, and answers/approaches which are ts_rank-only) — are DROPPED when a
   threshold is active, because an unmeasurable match must not be presented as confident (bias to
   ASK). When nothing clears the bar → `data:[]`, `total:0` (true empty, no fuzzy fallbacks).
   Note the trade-off in docs: with a threshold set, results are semantic-confident posts only.
   Threshold `0` (or a documented `min_similarity=0`) preserves the old recall-first behavior for
   callers that want it.

3. **`meta` decision summary.** Extend `SearchResponseMeta` with:
   - `TopSimilarity *float64 json:"top_similarity,omitempty"` — max similarity across ALL matches
     BEFORE min_similarity filtering + pagination (so the caller sees "best was 0.72" even on empty).
   - `ConfidentMatch bool json:"confident_match"` — `TopSimilarity != nil && *TopSimilarity >=
     confidenceThreshold`. This is the server-provided "answered?" decision; when false → ASK.
   Compute in the repo `Search()` (it has the unfiltered, sorted results) and thread out. Because
   the interface signature is `([]SearchResult,int,string,error)`, either (a) widen it to return a
   small `SearchMeta{Total int; Method string; TopSimilarity *float64}` struct, or (b) add a
   `TopSimilarity` return value. Recommended: introduce `type SearchMeta struct{...}` and change
   `Search` to `(...,SearchMeta,error)`; update the interface + the mock `MockSearchRepository`
   (search_test.go) + MCP caller. Keep it minimal.

4. **Calibrated threshold, config-driven, biased to ASK.** Add env `SEARCH_CONFIDENCE_THRESHOLD`
   (default `0.85` cosine ≈ near-duplicate question; conservative → confident_match false unless
   clearly the same, per decision #2). Load in `config/env.go`; pass to the search handler/repo. It
   is BOTH the `min_similarity` fallback default AND the `confident_match` cutoff. Do NOT tune the
   value empirically here (Golden Rule 8 — no solo research; SPIKE-2 owns calibration). Ship the
   conservative default + the env knob + the per-request override.

5. **Fix the MCP relevance display + no-match.** In `mcp.go:378-396`, replace the misleading
   `Relevance: int(Score*100)%` with the cosine `similarity` when present (e.g. `Similarity: 84%`),
   and include the `confident_match`/no-match guidance so an MCP agent gets the same decidable
   signal. Keep the existing `len(results)==0` "No results found" branch; also surface
   `confident_match:false` as "no confident match — consider asking".

6. **Docs (API-first).** SPEC.md §5.5/§5.6 GET /search: document `min_similarity` param, the new
   `similarity` result field (0–1 cosine, semantic only), `meta.top_similarity` +
   `meta.confident_match`, and the score/similarity semantics per method (fix the drifted
   `"score":0.95` note). Add the "answered? = confident_match && data non-empty; else ASK" recipe.
   Mirror to `skill/references/api.md` + `skill/scripts/solvr.sh` cmd_search (print `similarity` +
   a "confident match: yes/no" line) and re-run `scripts/sync-skill.sh`.

## Files to touch
- `backend/internal/models/search.go` — Similarity field on both structs + ToResponse; MinSimilarity on SearchOptions; optional SearchMeta struct.
- `backend/internal/db/search.go` — similarity column in searchPostsHybrid outer SELECT + scan; min_similarity filter + top_similarity compute in Search(); interface return shape.
- `backend/internal/api/handlers/search.go` — parse min_similarity; SearchResponseMeta += TopSimilarity + ConfidentMatch; thread confidence threshold.
- `backend/internal/api/handlers/mcp.go` — similarity-based relevance + no-confident-match text.
- `backend/internal/config/env.go` — SEARCH_CONFIDENCE_THRESHOLD (default 0.85).
- `SPEC.md`, `skill/references/api.md`, `skill/scripts/solvr.sh` (+ sync-skill.sh), version bumps.
- Tests: `db/search*_test.go`, `api/handlers/search*_test.go`, `mcp_test.go` (see below).

## Tests (TDD, RED first)
- DB (`internal/db`, real DATABASE_URL, VOYAGE_API_KEY or fake embedder — see search_hybrid_test.go
  patterns): hybrid result populates `similarity` in (0,1]; a highly-relevant query → high
  top_similarity + confident_match true; an off-topic/gibberish query → results filtered by a high
  min_similarity → `data:[]`, and top_similarity below threshold → confident_match false.
- Handler (`internal/api/handlers`, MockSearchRepository): response body includes `similarity` +
  `meta.top_similarity` + `meta.confident_match`; `min_similarity` param parsed + forwarded;
  fulltext method → similarity absent + confident_match false.
- MCP (`mcp_test.go` — currently zero coverage of executeSearch): the relevance line uses
  similarity; the no-confident-match branch renders guidance.
- Keep `meta.method/total/has_more` assertions green (don't rename fields).

## Verification (end-to-end)
- [TEST] `cd backend && DATABASE_URL=postgres://solvr:solvr_dev@localhost:5434/solvr?sslmode=disable
  go test ./internal/db/... ./internal/api/... -run 'Search|Mcp|Similarity' -count=1` (RED→GREEN);
  `go vet ./...`, `go build ./...`. Full-suite failures remain the known pre-existing/environmental
  set (referral_code, service_checks, room-slug) — confirm via the stash method (stash only the
  changed source files, re-run the failing tests, confirm identical failures on clean code).
- [REAL] local: `go run ./cmd/api` (with VOYAGE_API_KEY for hybrid). Seed one family/public post
  with a distinctive phrase, then: (a) query the SAME phrase → high `similarity`, `confident_match:
  true`; (b) query an unrelated phrase with `min_similarity=0.85` → `data:[]`, `top_similarity`
  low, `confident_match:false`. Prove the decidable no-match. Clean up.
- [REAL] prod (after deploy): repeat (a)/(b) against api.solvr.dev via a claimed test agent +
  admin-query cleanup. Deploy-completion probe = presence of `meta.confident_match` in the response.

## Non-breaking / risk notes
- All new fields are additive; `similarity`/`top_similarity` are `omitempty` pointers. Do NOT rename
  `score`, `meta.method/total/has_more/took_ms` (frontend + skill consume them).
- Local DB (`solvr-postgres`, port 5434, user/db `solvr`, pass `solvr_dev`) is fully migrated.
- Deploy = EasyPanel webhooks in `.env` (SOLVR_DEPLOY_API backend, SOLVR_DEPLOY_WEB frontend);
  push does NOT auto-deploy. Prod DB writes via `/admin/query` (X-Admin-API-Key in `.env`).
- Commit/deploy only on explicit user go-ahead. Version bump convention: frontend
  `frontend/package.json` + skill `skill/skill.json` (last shipped 0.3.53 / skill 3.7.12).
- Decisions already locked (see "Locked decisions"): cosine for confidence; ASK-biased
  confident_match; both param + config; honest empty; and min_similarity DROPS nil-similarity
  (keyword-only) results when a threshold is active. Nothing here is left for the executor to
  re-decide except mechanical choices (interface return shape, exact test fixtures).

## Session process notes (how BART-151..154 were shipped — mirror this)
- TDD: write the RED test first, confirm it fails for the right reason, implement, confirm GREEN.
- Two independent verification agents confirmed each plan before coding (adversarial + design).
- Label every claim [REAL] (verified on a running system) / [TEST] (tests only) / [UNVERIFIED].
- Stage ONLY the ticket's files (never `.planning/STATE.md` or `PROJECT_KNOWLEDGE.md`).
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- After merge: bump versions, `bash scripts/sync-skill.sh`, push, trigger both deploy webhooks,
  apply any data change via `/admin/query`, poll a deploy-completion probe, run the [REAL] prod
  drill, clean up all test fixtures.
