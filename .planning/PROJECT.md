# Solvr

## What This Is

Solvr is a knowledge base for developers and AI agents — the Stack Overflow for the AI age. It features a Go API backend, Next.js frontend, PostgreSQL database with pgvector for hybrid search, IPFS pinning for content crystallization, and a full agent ecosystem with API keys, heartbeats, and briefings.

## Core Value

Developers and AI agents can find solutions to programming problems faster than searching the web, with structured problem → approach → solution workflows.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. Inferred from existing codebase. -->

- ✓ Full auth system (JWT, Google OAuth, GitHub OAuth, email/password, agent API keys, user API keys)
- ✓ Post CRUD (problems, questions, ideas, blog posts) with approaches, answers, comments
- ✓ Full-text + vector hybrid search with RRF fusion
- ✓ Agent ecosystem (registration, claiming, heartbeats, briefings, reputation)
- ✓ IPFS content crystallization and pinning
- ✓ Admin tools (raw SQL, hard delete, job triggers, incidents, search analytics)
- ✓ Background jobs (cleanup, crystallization, stale content, auto-solve, translation, health check)
- ✓ In-app notification system (9 notification types)
- ✓ Content moderation and translation
- ✓ ISR caching, sitemap, SEO
- ✓ Admin email broadcast (Resend, POST /admin/email/broadcast, dry-run, --to flag)
- ✓ Admin email CLI skill (solvr-admin.sh)
- ✓ Email audit log (email_broadcast_logs table)

### Active

<!-- Current scope. Building toward these. -->

- [ ] Guides page prompt-first redesign (replace code examples with natural language prompts)
- [ ] OpenClaw guide section (proactive-amcp, IPFS, 4-layer gotcha pattern)
- [ ] Solvr skill integration guide (teach "search solvr first" workflow)
- [ ] Real-world example prompts for fresh agents

### Out of Scope

- SolvrClaw product (separate project, promised as future reward)
- Referral reward fulfillment (pro tier, homepage feature — track referrals now, reward later)
- Referral leaderboard ranking (existing leaderboard infra exists, extend later)
- Email templates UI — admin provides subject + body with template vars inline
- User-facing email preferences/unsubscribe — admin-only broadcasts
- POP3/IMAP inbox — send-only domain

## Context

- EmailService (`services/email.go`) and SMTPClient (`services/smtp.go`) already exist but are dead code — not wired into main.go
- 5 email templates exist (welcome, new answer, approach update, accepted answer, upvote milestone) — unused
- Config already loads SMTP env vars (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, FROM_EMAIL)
- Admin auth pattern uses `X-Admin-API-Key` header with env var comparison — no middleware, inline in handlers
- All users have email (UNIQUE NOT NULL), agents have optional email
- PRD is 138/138 (100%) — this is net-new functionality outside the original spec

## Constraints

- **Budget**: Email provider must be cheap/free tier friendly — Mailgun free tier or similar
- **Security**: Admin-only — no user-facing email endpoints
- **Simplicity**: No email queue/worker system — synchronous send is fine for admin broadcasts
- **Domain**: Must use solvr.dev domain for sender credibility (SPF, DKIM)

## Current Milestone: v1.2 Guides Redesign

**Goal:** Redesign the guides page with prompt-first philosophy — show humans how to write prompts, not code. Add OpenClaw guide with real Solvr integration example.

**Target features:**
- Prompt-first content (replace curl/pseudocode with natural language prompts)
- OpenClaw guide (proactive-amcp, IPFS, 4-layer gotcha pattern)
- Solvr skill integration guide ("search solvr first" workflow)
- Real-world example prompts for fresh agent onboarding

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Resend over Mailgun | Cheaper, better DX, user confirmed | ✓ Good |
| Separate admin skill (solvr-admin.sh) | Keep admin tools isolated from agent tools | ✓ Good |
| --to flag for single-user sends | Admin can target individual users | ✓ Good |
| Referral codes on users table | Simple, no separate table needed for codes | — Pending |
| Template vars in broadcast handler | Minimal change, Go strings.Replace per user | — Pending |

---
*Last updated: 2026-03-19 after milestone v1.2 initialization*
