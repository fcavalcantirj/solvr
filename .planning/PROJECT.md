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

### Active

<!-- Current scope. Building toward these. -->

- [ ] Email provider infrastructure (Mailgun + DNS for solvr.dev)
- [ ] Admin email broadcast to all active users
- [ ] Admin email skill for Claude Code (solvr-admin.sh)
- [ ] Email audit logging

### Out of Scope

- User-facing email preferences/unsubscribe — not needed for admin-only broadcasts yet
- Per-user targeted emails — announcements only for v1
- Email templates UI — admin provides subject + body inline
- Stats/analytics dashboard — audit log is sufficient for now
- POP3/IMAP inbox — solvr.dev doesn't need to receive email, only send

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

## Current Milestone: v1.0 Admin Email System

**Goal:** Enable admin to send email announcements to all Solvr users via Claude Code skill, with proper email infrastructure for solvr.dev.

**Target features:**
- Email provider setup (Mailgun + DNS)
- Wire existing EmailService into production
- Admin broadcast endpoint
- Admin email skill (solvr-admin.sh)
- Email audit log

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Mailgun over SES/SendGrid | Cheapest for low-volume transactional, simple API, generous free tier | — Pending |
| Separate admin skill (not extend solvr.sh) | Keep admin tools isolated from agent tools, different auth model | — Pending |
| Broadcast only (no targeting) | Start simple, all active users, add filtering later if needed | — Pending |
| Sync send (no queue) | Admin broadcasts are infrequent, no need for worker infrastructure | — Pending |

---
*Last updated: 2026-03-17 after milestone v1.0 initialization*
