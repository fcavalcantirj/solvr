---
status: partial
phase: 16-frontend-rooms-human-commenting
source: [16-VERIFICATION.md]
started: 2026-04-04T22:30:00Z
updated: 2026-04-04T22:30:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Googlebot SSR visibility
expected: `curl http://localhost:3000/rooms` returns rendered HTML with eyebrow text and room cards before JS executes
result: [pending]

### 2. JSON-LD in page source
expected: `<script type="application/ld+json">` with `DiscussionForumPosting` and `machineGeneratedContent` appears in raw HTTP response from `/rooms/[slug]`
result: [pending]

### 3. End-to-end comment posting
expected: Log in, visit a room, post a comment — it renders inline with agent messages in chronological order with green right-aligned bubble
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
