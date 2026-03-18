# Phase 7: Email Personalization - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Add per-recipient template variable substitution to the existing BroadcastEmail handler. Variables: `{name}` → display_name, `{referral_code}` → user's code, `{referral_link}` → full URL. Applied to both body_html and body_text before each send.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — pure infrastructure phase.

Key constraints from ROADMAP:
- Use `strings.NewReplacer` per recipient — simple, zero deps
- Need referral_code in the email loop — update ListActiveEmails or add new method
- Handle NULL referral_code gracefully (substitute empty string)
- Extract substitution into pure helper `substituteTemplateVars` for easy testing
- Dry-run should show substituted body for first recipient as preview

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/api/handlers/admin.go` — BroadcastEmail handler (lines 92-190)
- `backend/internal/db/users.go` — ListActiveEmails returns []models.EmailRecipient
- `backend/internal/models/email.go` — EmailRecipient struct (id, email, display_name)
- Phase 6 added: referral_code on User model, referrals table

### Established Patterns
- BroadcastEmail loops over recipients, sends one by one
- EmailRecipient currently has: ID, Email, DisplayName
- Need to add ReferralCode to EmailRecipient (or create extended struct)

### Integration Points
- Modify BroadcastEmail handler in admin.go
- Modify ListActiveEmails query to include referral_code
- Update EmailRecipient model to include ReferralCode

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---
*Phase: 07-email-personalization*
*Context gathered: 2026-03-17 via Smart Discuss (infrastructure skip)*
