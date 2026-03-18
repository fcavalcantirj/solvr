# Roadmap: Solvr Growth Engine

**Milestone:** v1.1
**Created:** 2026-03-17
**Phases:** 4 (phases 6–9)
**Requirements:** 16

---

## Phase 6: Referral Codes + DB Foundation

**Goal:** Add referral infrastructure to the database and expose the user's code via API.
**Requirements:** REF-01, REF-02, REF-04

### Success Criteria

1. Migration adds `referral_code VARCHAR(8) UNIQUE NOT NULL` to `users` table and creates `referrals` table (referrer_id, referred_id FK to users, created_at).
2. All existing users have a unique 8-char alphanumeric code after the migration backfill runs — verified by querying `SELECT COUNT(*) FROM users WHERE referral_code IS NULL` returning 0.
3. New user registrations automatically receive a generated code on account creation — verified by an integration test that registers a user and checks `referral_code IS NOT NULL`.
4. `GET /v1/users/me/referral` returns `{"referral_code": "...", "referral_count": N}` for the authenticated caller.
5. Integration tests cover code uniqueness enforcement, backfill idempotency, and the API endpoint returning correct counts.

### Notes

- 8-char alphanumeric codes: use `crypto/rand` with charset `[A-Z0-9]` — ~1.7 trillion combinations, collision probability negligible.
- Backfill in the migration itself (DO $$...$$) to keep it atomic with the schema change; no separate script needed.
- `referrals` table columns: `id UUID PK`, `referrer_id UUID NOT NULL REFERENCES users(id)`, `referred_id UUID NOT NULL UNIQUE REFERENCES users(id)`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- `referred_id` is UNIQUE — one user can only be referred once (prevents double-attribution).
- `GET /v1/users/me/referral` counts rows in `referrals WHERE referrer_id = $1` for the referral_count.

---

## Phase 7: Email Personalization

**Goal:** Make the existing admin broadcast handler substitute per-recipient template variables before sending each email.
**Requirements:** EML-01, EML-02, EML-03, EML-04

### Success Criteria

1. `{name}` in `body_html` and `body_text` is replaced with recipient's `display_name` before each send.
2. `{referral_code}` is replaced with the recipient's unique referral code per email.
3. `{referral_link}` is replaced with the full URL `https://solvr.dev/join?ref=CODE` per email.
4. Substitution is applied to both `body_html` and `body_text` fields — verified by a unit test checking both fields after substitution.
5. Dry-run mode shows the substituted body for the first recipient so the admin can verify before live send.

### Notes

- Use `strings.NewReplacer("{name}", name, "{referral_code}", code, "{referral_link}", link)` per recipient — simple, zero deps.
- Fetch `referral_code` alongside `email` + `display_name` in the existing bulk-query loop (update `ListActiveEmails` query or add a new method).
- If `referral_code` is NULL for any user during transition, substitute empty string to avoid literal `{referral_code}` in sent email.
- Extract substitution into a pure helper function `substituteTemplateVars(body, name, code, link string) string` for easy unit testing.
- Phase 6 must be complete before this phase (referral_code column must exist on users).

---

## Phase 8: Referral Tracking + Join Flow

**Goal:** Track signups that arrive via a referral link and attribute them to the referring user.
**Requirements:** REF-03, PAGE-01

### Success Criteria

1. Registration API accepts an optional `ref` field (referral code) in the request body.
2. On successful registration with a valid `ref`, exactly one row is inserted into `referrals` table linking referrer and new user.
3. Invalid or unknown `ref` values are silently ignored — registration still returns 201 with no error.
4. `/join?ref=CODE` frontend page extracts the `ref` query param and forwards it to the registration API call.
5. Integration test: register via the API with a valid `ref` → query `referrals` table → exactly one row for that pair.

### Notes

- Pass `ref` as a JSON field in the registration body (cleaner than threading it as a URL param through the API).
- Use a DB transaction: insert user row + insert referral row atomically; if referral insert fails (e.g. invalid code), log and continue — do not roll back the user creation.
- `/join` page is a thin wrapper over the existing registration UI — only change is reading `?ref=CODE` from URL and including it in the API request payload.
- Phase 6 must be complete before this phase (referrals table must exist).

---

## Phase 9: Frontend — Dashboard + Pages

**Goal:** Give authenticated users a referral dashboard, build the Chinese promotion page, and add pre-filled share links.
**Requirements:** REF-05, PAGE-02, PAGE-03, DASH-01, DASH-02, DASH-03, DASH-04

### Success Criteria

1. `/referrals` route renders the referral dashboard for authenticated users; unauthenticated visitors are redirected to login.
2. Dashboard displays the user's referral code with a one-click copy-to-clipboard button that shows a "Copied!" confirmation state.
3. Dashboard displays the count of successful referrals fetched from `GET /v1/users/me/referral`.
4. Dashboard shows a pre-filled X/Twitter share link and a copyable referral URL (`https://solvr.dev/join?ref=CODE`).
5. `/zh/promote` page renders a Chinese-language promotion guide; authenticated users see their personal referral link embedded, unauthenticated visitors see the generic `/join` link.
6. Pre-filled tweet link correctly encodes the referral URL and a default message via `encodeURIComponent`.

### Notes

- Tweet intent URL: `https://twitter.com/intent/tweet?text=...&url=https%3A%2F%2Fsolvr.dev%2Fjoin%3Fref%3DCODE`
- Copy button: use `navigator.clipboard.writeText()` with a brief "Copied!" state toggle (setTimeout reset).
- Dashboard fetches `GET /v1/users/me/referral` on mount; show skeleton loader during fetch.
- `/zh/promote` content should be static Chinese text with a dynamic ref link section for logged-in users.
- Vitest tests: copy button interaction, tweet link construction with correct encoding, referral count rendering.
- REF-05 is satisfied by the dashboard — it is the primary surface where users see their code.
- Phase 6 must be complete before this phase (API endpoint must exist); Phase 8 must be complete for `/join` page wiring.

---

## Phase Summary

| Phase | Name | Layer | Requirements | Count |
|-------|------|-------|--------------|-------|
| 6 | Referral Codes + DB Foundation | Backend | REF-01, REF-02, REF-04 | 3 |
| 7 | Email Personalization | Backend | EML-01, EML-02, EML-03, EML-04 | 4 |
| 8 | Referral Tracking + Join Flow | Backend + Frontend | REF-03, PAGE-01 | 2 |
| 9 | Frontend — Dashboard + Pages | Frontend | REF-05, PAGE-02, PAGE-03, DASH-01, DASH-02, DASH-03, DASH-04 | 7 |

**Total:** 16 requirements across 4 phases

---

## Coverage

All 16 v1.1 requirements mapped:

| Requirement | Phase |
|-------------|-------|
| EML-01 | Phase 7 |
| EML-02 | Phase 7 |
| EML-03 | Phase 7 |
| EML-04 | Phase 7 |
| REF-01 | Phase 6 |
| REF-02 | Phase 6 |
| REF-03 | Phase 8 |
| REF-04 | Phase 6 |
| REF-05 | Phase 9 |
| PAGE-01 | Phase 8 |
| PAGE-02 | Phase 9 |
| PAGE-03 | Phase 9 |
| DASH-01 | Phase 9 |
| DASH-02 | Phase 9 |
| DASH-03 | Phase 9 |
| DASH-04 | Phase 9 |

Coverage: **16/16 (100%)**

---

## Dependency Order

Phases must execute in order:

- Phase 6 must precede Phase 7 — email personalization needs `referral_code` column on `users`
- Phase 6 must precede Phase 8 — referral tracking needs `referrals` table
- Phase 6 must precede Phase 9 — dashboard API endpoint (`GET /v1/users/me/referral`) is Phase 6
- Phase 8 must precede Phase 9 — `/join` page behavior depends on tracking being wired in the API

---
*Roadmap created: 2026-03-17*
*Milestone: v1.1 Growth Engine*
