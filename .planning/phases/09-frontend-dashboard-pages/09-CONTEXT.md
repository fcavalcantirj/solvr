# Phase 9: Frontend — Dashboard + Pages - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the referral dashboard page (/referrals), Chinese promotion guide page (/zh/promote), and pre-filled tweet link construction. All frontend work.

</domain>

<decisions>
## Implementation Decisions

### Dashboard Layout
- Simple card layout: referral code with copy button, count, share links
- Minimal UI — no heavy components, just clean cards
- Copy button uses navigator.clipboard.writeText() with "Copied!" state toggle
- Share links: X/Twitter pre-filled tweet + copy referral URL
- Fetches GET /v1/users/me/referral on mount, skeleton loader during fetch
- Redirect to /join if not authenticated

### Chinese Promotion Page
- Static Chinese-language page at /zh/promote
- Content: intro to Solvr, where to share (juejin.cn, CSDN, V2EX, zhihu, Gitee)
- Logged-in users see personalized referral link embedded
- Anonymous visitors see generic /join link
- No i18n framework needed — single static page

### Claude's Discretion
- Component styling (follow existing Tailwind patterns)
- Exact tweet text (reasonable default)
- Error states for API failures

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- frontend/components/ — existing UI components (cards, buttons, etc.)
- frontend/lib/api.ts — API client with auth headers
- frontend/hooks/use-auth.tsx — authentication hook (user, isAuthenticated)
- frontend/app/leaderboard/ — example of authenticated page pattern

### Established Patterns
- Next.js 15 App Router, server components default, 'use client' for interactivity
- Tailwind v4 for styling
- Vitest for testing (NOT Jest)
- ISR caching with revalidation

### Integration Points
- New route: /referrals (authenticated)
- New route: /zh/promote (public, optional auth for personalization)
- API call: GET /v1/users/me/referral (from Phase 6)
- Navigation: add link to referral dashboard from user menu or profile

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard frontend implementation.

</specifics>

<deferred>
## Deferred Ideas

None.

</deferred>

---
*Phase: 09-frontend-dashboard-pages*
*Context gathered: 2026-03-17 via Smart Discuss*
