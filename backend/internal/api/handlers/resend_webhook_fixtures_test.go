package handlers

// Resend's documented example payloads (C7 of the 2026-10-12 fix-first review), copied verbatim
// from each event's page of Resend's webhook reference as fetched on 2026-10-17: the JSON of the
// page's example, byte for byte. Tests change a field of one only where they say so.
// Index of the events: https://resend.com/docs/webhooks/event-types
//
// Bounce types (https://resend.com/docs/dashboard/emails/email-bounces): "Permanent" is the hard
// bounce ("the recipient's mail server rejects the email and will never be delivered"),
// "Transient" the soft bounce, and "Undetermined" a bounce whose message did not say why.

// resendBouncedExample: email.bounced, https://resend.com/docs/webhooks/emails/bounced
const resendBouncedExample = `{
  "type": "email.bounced",
  "created_at": "2026-11-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-11-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "bounce": {
      "message": "The recipient's email address is on the suppression list because it has a recent history of producing hard bounces.",
      "subType": "Suppressed",
      "type": "Permanent"
    },
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendComplainedExample: email.complained, https://resend.com/docs/webhooks/emails/complained
const resendComplainedExample = `{
  "type": "email.complained",
  "created_at": "2026-02-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-02-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendDeliveredExample: email.delivered, https://resend.com/docs/webhooks/emails/delivered
const resendDeliveredExample = `{
  "type": "email.delivered",
  "created_at": "2026-02-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-02-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendDeliveryDelayedExample: email.delivery_delayed, https://resend.com/docs/webhooks/emails/delivery-delayed
const resendDeliveryDelayedExample = `{
  "type": "email.delivery_delayed",
  "created_at": "2026-02-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-02-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendFailedExample: email.failed, https://resend.com/docs/webhooks/emails/failed
const resendFailedExample = `{
  "type": "email.failed",
  "created_at": "2026-11-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-11-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "failed": {
      "reason": "reached_daily_quota"
    },
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendSentExample: email.sent, https://resend.com/docs/webhooks/emails/sent
const resendSentExample = `{
  "type": "email.sent",
  "created_at": "2026-02-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-02-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendSuppressedExample: email.suppressed, https://resend.com/docs/webhooks/emails/suppressed
const resendSuppressedExample = `{
  "type": "email.suppressed",
  "created_at": "2026-11-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-11-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "suppressed": {
      "message": "Resend has suppressed sending to this address because it is on the account-level suppression list. This does not count toward your bounce rate metric",
      "type": "OnAccountSuppressionList"
    },
    "tags": {
      "category": "confirm_email"
    }
  }
}`

// resendOpenedExample: email.opened, https://resend.com/docs/webhooks/emails/opened
const resendOpenedExample = `{
  "type": "email.opened",
  "created_at": "2026-02-22T23:41:12.126Z",
  "data": {
    "broadcast_id": "8b146471-e88e-4322-86af-016cd36fd216",
    "created_at": "2026-02-22T23:41:11.894Z",
    "email_id": "56761188-7520-42d8-8898-ff6fc54ce618",
    "message_id": "<111-222-333@email.example.com>",
    "from": "Acme <onboarding@resend.dev>",
    "to": ["delivered@resend.dev"],
    "subject": "Sending this example",
    "template_id": "43f68331-0622-4e15-8202-246a0388854b",
    "tags": {
      "category": "confirm_email"
    }
  }
}`
