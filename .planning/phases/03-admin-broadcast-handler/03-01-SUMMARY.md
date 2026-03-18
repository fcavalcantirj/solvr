---
plan: "03-01"
status: complete
started: 2026-03-17
completed: 2026-03-17
---

# Plan 03-01 Summary: Admin Broadcast Handler with Dry-Run

## What Was Built

Implemented `POST /admin/email/broadcast` on `AdminHandler` supporting both live broadcast (synchronous, sequential sends to all active users) and dry-run preview mode (returns recipient list + count without sending). The `EmailSender` interface was extended with variadic headers support (`...map[string]string`) so `List-Unsubscribe` headers can be passed per email. All repos are wired via setter injection, the route is registered in router.go, and all tests pass.

## Key Files

### Created
- `backend/internal/api/handlers/admin_broadcast_test.go` — 7 unit tests using mock implementations of EmailSender, EmailBroadcastRepo, and UserEmailRepo (no DB required)

### Modified
- `backend/internal/api/handlers/admin.go` — Extended EmailSender interface with variadic headers; added EmailBroadcastRepo and UserEmailRepo interfaces; added emailBroadcastRepo and userEmailRepo fields to AdminHandler; added SetEmailBroadcastRepo and SetUserEmailRepo setters; added broadcastRequest struct and BroadcastEmail handler method (~116 lines added, 536 total)
- `backend/internal/services/resend.go` — Updated Send signature to accept variadic `headers ...map[string]string`; adds `params.Headers = headers[0]` when provided
- `backend/internal/services/resend_test.go` — Added TestResendClient_Send_WithHeaders test verifying List-Unsubscribe header is sent in Resend API request body
- `backend/internal/api/router.go` — Replaced Resend wiring block to also wire emailBroadcastRepo and userEmailRepo; registered `POST /admin/email/broadcast`

## Deviations

None. Plan followed exactly.

## Self-Check

PASSED

- All 7 TestBroadcastEmail_* tests pass
- All 5 TestResendClient_* tests pass (4 existing + 1 new)
- Full test suite passes (all 15 packages OK, no regressions)
- go vet passes on all modified packages
- go build ./cmd/api succeeds
- admin.go: 536 lines (under 900)
- admin_broadcast_test.go: 374 lines (under 900)
- resend.go: 65 lines (under 900)
- resend_test.go: 208 lines (under 900)
- router.go: 1088 lines (pre-existing violation, acknowledged)

## Test Results

```
=== RUN   TestBroadcastEmail_Unauthorized
--- PASS: TestBroadcastEmail_Unauthorized (0.00s)
=== RUN   TestBroadcastEmail_MissingSubject
--- PASS: TestBroadcastEmail_MissingSubject (0.00s)
=== RUN   TestBroadcastEmail_MissingBodyHTML
--- PASS: TestBroadcastEmail_MissingBodyHTML (0.00s)
=== RUN   TestBroadcastEmail_EmailNotConfigured
--- PASS: TestBroadcastEmail_EmailNotConfigured (0.00s)
=== RUN   TestBroadcastEmail_DryRun
--- PASS: TestBroadcastEmail_DryRun (0.00s)
=== RUN   TestBroadcastEmail_Success
--- PASS: TestBroadcastEmail_Success (0.00s)
=== RUN   TestBroadcastEmail_PartialFailure
--- PASS: TestBroadcastEmail_PartialFailure (0.00s)
PASS
ok  	github.com/fcavalcantirj/solvr/internal/api/handlers	0.461s

=== RUN   TestResendClient_Send_Success
--- PASS: TestResendClient_Send_Success (0.00s)
=== RUN   TestResendClient_Send_APIError
--- PASS: TestResendClient_Send_APIError (0.00s)
=== RUN   TestResendClient_Send_EmptyTextBody
--- PASS: TestResendClient_Send_EmptyTextBody (0.00s)
=== RUN   TestResendClient_Send_CustomFromEmail
--- PASS: TestResendClient_Send_CustomFromEmail (0.00s)
=== RUN   TestResendClient_Send_WithHeaders
--- PASS: TestResendClient_Send_WithHeaders (0.00s)
PASS
ok  	github.com/fcavalcantirj/solvr/internal/services	0.501s

Full suite: all 15 packages OK, 0 failures
```
