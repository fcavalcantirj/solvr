# Phase 8: Referral Tracking + Join Flow - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire referral attribution into the registration flow: backend accepts optional `ref` field on signup, inserts into `referrals` table. Frontend `/join` page reads `?ref=CODE` query param and forwards to API.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — straightforward plumbing phase.

Key constraints from ROADMAP:
- Registration API accepts optional `ref` field in request body (not URL param)
- Valid ref → insert into referrals table (referrer_id, referred_id)
- Invalid/unknown ref → silently ignored, registration still succeeds
- Use DB transaction: user creation + referral insert atomic; if referral fails, log and continue
- Frontend /join page reads ?ref=CODE from URL, includes in API request payload

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/auth/` — registration handlers (email/password, OAuth flows)
- `backend/internal/db/users.go` — UserRepository.Create
- `backend/internal/db/referral.go` — ReferralRepository (CountByReferrer, GetReferralCode)
- `frontend/app/join/page.tsx` — existing multi-step registration form
- `referrals` table already exists (migration 000070)

### Integration Points
- Registration handler(s) in auth package — need to accept `ref` param
- ReferralRepository needs `CreateReferral(ctx, referrerID, referredID)` method
- Frontend join page needs `useSearchParams()` to read ref query param

</code_context>

<specifics>
## Specific Ideas

No specific requirements — plumbing phase.

</specifics>

<deferred>
## Deferred Ideas

None.

</deferred>

---
*Phase: 08-referral-tracking-join-flow*
*Context gathered: 2026-03-17 via Smart Discuss (infrastructure skip)*
