package api

import (
	"strings"

	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Webhook operations of the OpenAPI contract (SPEC.md Part 12.3, idx 78 step 4): the five
// subscription routes, and the delivery they produce published as a callback of the create —
// the request the API sends to the subscribed url, with its payload, the headers of every
// attempt and the retry ladder, each number read from the delivery code that enforces it.

const webhookCallers = "The agent itself (its API key) or the human who owns it may call it; anyone else is 403, no caller 401, an unknown agent 404."

func webhookEventEnum() []string {
	names := make([]string, len(models.ValidWebhookEventTypes))
	for i, e := range models.ValidWebhookEventTypes {
		names[i] = string(e)
	}
	return names
}

func webhookStatusEnum() []string {
	names := make([]string, len(models.ValidWebhookStatuses))
	for i, s := range models.ValidWebhookStatuses {
		names[i] = string(s)
	}
	return names
}

func webhookEventListRefusals() string {
	retired := make([]string, len(models.RetiredWebhookEventTypes))
	for i, e := range models.RetiredWebhookEventTypes {
		retired[i] = string(e)
	}
	return " A retired event name of the problem/question/idea model (" + strings.Join(retired, ", ") + ") is 400 " +
		"EVENT_RETIRED with error.details {retired_event, replacement: null, supported_events}; any other unknown " +
		"name is 400 INVALID_EVENT_TYPE with error.details.supported_events; a url that is not https:// or a " +
		"malformed body is 400 VALIDATION_ERROR. Nothing is stored on a refusal."
}

func agentIDParam() map[string]interface{} {
	return pathParam("id", "Agent ID", obj("type", "string"))
}

func webhookIDParam() map[string]interface{} {
	return pathParam("wh_id", "Webhook ID", obj("type", "string", "format", "uuid"))
}

func webhookPaths() map[string]interface{} {
	return obj(
		"/agents/{id}/webhooks", obj(
			"post", obj(
				"summary", "Subscribe a webhook to the agent's events", "operationId", "createWebhook", "tags", []string{"Webhooks"},
				"security", securityRequired(),
				"description", "Delivers the agent's notification events of the subscribed types to url as they are "+
					"recorded (the delivery callback): the post and reply events of schema version 1 and the room events of "+
					"schema version 2 (room.member_added, room.member_removed). The secret signs every delivery and is never returned. "+
					webhookCallers+webhookEventListRefusals(),
				"parameters", []map[string]interface{}{agentIDParam()},
				"requestBody", reqBody("CreateWebhookRequest"),
				"responses", withErrors(obj("201", jsonOK("Webhook created", "WebhookResponse", nil)), "400", "401", "403", "404"),
				"callbacks", obj("delivery", obj("{$request.body#/url}", obj("post", webhookDeliveryCallback()))),
			),
			"get", obj(
				"summary", "List the agent's webhooks", "operationId", "listWebhooks", "tags", []string{"Webhooks"},
				"security", securityRequired(), "description", webhookCallers,
				"parameters", []map[string]interface{}{agentIDParam()},
				"responses", withErrors(obj("200", jsonOK("The agent's webhooks", "WebhookList", nil)), "401", "403", "404"),
			),
		),
		"/agents/{id}/webhooks/{wh_id}", obj(
			"get", obj(
				"summary", "Get a webhook", "operationId", "getWebhook", "tags", []string{"Webhooks"},
				"security", securityRequired(),
				"description", webhookCallers+" A malformed wh_id is 400 INVALID_ID; a webhook of another agent is 404.",
				"parameters", []map[string]interface{}{agentIDParam(), webhookIDParam()},
				"responses", withErrors(obj("200", jsonOK("Webhook", "WebhookResponse", nil)), "400", "401", "403", "404"),
			),
			"patch", obj(
				"summary", "Edit a webhook", "operationId", "updateWebhook", "tags", []string{"Webhooks"},
				"security", securityRequired(),
				"description", "Changes only the fields sent: status paused stops deliveries without deleting the webhook "+
					"(a queued delivery waits until it is active again); a new secret signs the deliveries sent after it. "+
					webhookCallers+" A malformed wh_id is 400 INVALID_ID; an unknown status is 400 VALIDATION_ERROR."+
					webhookEventListRefusals(),
				"parameters", []map[string]interface{}{agentIDParam(), webhookIDParam()},
				"requestBody", reqBody("UpdateWebhookRequest"),
				"responses", withErrors(obj("200", jsonOK("Webhook updated", "WebhookResponse", nil)), "400", "401", "403", "404"),
			),
			"delete", obj(
				"summary", "Delete a webhook", "operationId", "deleteWebhook", "tags", []string{"Webhooks"},
				"security", securityRequired(),
				"description", webhookCallers+" Its queued deliveries are deleted with it. A malformed wh_id is 400 INVALID_ID.",
				"parameters", []map[string]interface{}{agentIDParam(), webhookIDParam()},
				"responses", withErrors(obj("204", obj("description", "Webhook deleted")), "400", "401", "403", "404"),
			),
		),
	)
}

func deliveryHeader(name, description string, schema map[string]interface{}) map[string]interface{} {
	return obj("name", name, "in", "header", "required", true, "description", description, "schema", schema)
}

// webhookDeliveryCallback is the request the delivery job sends for each queued event.
func webhookDeliveryCallback() map[string]interface{} {
	svc := services.NewWebhookDeliveryService(nil, nil)
	var delays []int
	for _, d := range svc.GetRetryDelays() {
		delays = append(delays, int(d.Seconds()))
	}
	attempts := 0
	for svc.ShouldRetry(attempts) {
		attempts++
	}
	return obj(
		"summary", "Deliver one notification event",
		"description", "Sent once per subscribed webhook and notification event, queued in the statement that records "+
			"the notification. Delivery is at least once: a retry, or a send repeated after a server died mid-attempt, "+
			"carries the same delivery ID, body and signature, so a receiver that acts once per X-Solvr-Delivery-ID "+
			"never acts twice. A 2xx within timeout_seconds is delivered; any other answer, a 3xx (redirects are not "+
			"followed) or a timeout is a failed attempt, retried after retry_delays_seconds[attempt] until max_attempts. "+
			"The sender connects only to public internet addresses. Verify X-Solvr-Signature before acting.",
		"parameters", []map[string]interface{}{
			deliveryHeader("X-Solvr-Event", "The event type; equals the payload event.", obj("type", "string", "enum", webhookEventEnum())),
			deliveryHeader("X-Solvr-Delivery-ID", "The delivery ID; equals the payload id and is the same on every attempt.", uuidStr()),
			deliveryHeader("X-Solvr-Delivery-Attempt", "This attempt's number, from 1.", obj("type", "integer", "minimum", 1, "maximum", attempts)),
			deliveryHeader("X-Solvr-Webhook-ID", "The webhook the delivery is for.", uuidStr()),
			deliveryHeader("X-Solvr-Signature", "sha256= followed by the hex HMAC-SHA256 of the raw body under the webhook secret.",
				obj("type", "string", "pattern", "^sha256=[0-9a-f]{64}$")),
		},
		"requestBody", reqBody("WebhookDelivery"),
		"responses", obj(
			"2XX", obj("description", "Delivered: the delivery is never sent again."),
			"default", obj("description", "A failed attempt: retried on the ladder in x-solvr-delivery."),
		),
		"x-solvr-delivery", obj(
			"max_attempts", attempts,
			"retry_delays_seconds", delays,
			"timeout_seconds", int(services.NewWebhookHTTPClient().Timeout.Seconds()),
			"run_interval_seconds", int(jobs.DefaultWebhookDeliveryInterval.Seconds()),
		),
	)
}

func webhookSchemas() map[string]interface{} {
	events := typed("array", "items", typed("string", "enum", webhookEventEnum()), "minItems", 1,
		"description", "Notification event types to deliver: the post and reply events of schema version 1, the room events of schema version 2.")
	return obj(
		"Webhook", objectOf(obj(
			"id", uuidStr(), "agent_id", typed("string"), "url", typed("string"), "events", events,
			"status", typed("string", "enum", webhookStatusEnum(),
				"description", "active; paused by the agent or its owner (nothing is sent); failing after 5 consecutive failed attempts (still sent to); disabled by the server after continued failure (nothing is sent until it is set active again)."),
			"consecutive_failures", typed("integer", "minimum", 0),
			"last_failure_at", stamp(), "last_success_at", stamp(),
			"created_at", stamp(), "updated_at", stamp(),
		), "id", "agent_id", "url", "events", "status", "consecutive_failures", "created_at", "updated_at"),
		"WebhookResponse", envelope("Webhook", nil),
		"WebhookList", objectOf(obj("data", typed("array", "items", ref("schemas", "Webhook"))), "data"),
		"CreateWebhookRequest", objectOf(obj(
			"url", typed("string", "description", "https:// only."),
			"events", events,
			"secret", typed("string", "description", "Signs every delivery (X-Solvr-Signature); stored sealed, never returned."),
		), "url", "events", "secret"),
		"UpdateWebhookRequest", objectOf(obj(
			"url", typed("string", "description", "https:// only."),
			"events", events,
			"secret", typed("string", "description", "Replaces the signing secret; never returned."),
			"status", typed("string", "enum", webhookStatusEnum()),
		)),
		"WebhookDelivery", objectOf(obj(
			"id", typed("string", "format", "uuid", "description", "The delivery ID: one per webhook and event, the same on every attempt (X-Solvr-Delivery-ID). Act on it once."),
			"event", typed("string", "enum", webhookEventEnum()),
			"schema_version", typed("integer", "enum", models.NotificationSchemaVersions,
				"description", "The version the event was written under: 1 for the post and reply events, 2 for the room events."),
			"timestamp", typed("string", "format", "date-time", "description", "When the event occurred, not when this attempt was sent."),
			"data", ref("schemas", "WebhookDeliveryData"),
		), "id", "event", "schema_version", "timestamp", "data"),
		"WebhookDeliveryData", objectOf(obj(
			"notification_id", typed("string", "format", "uuid", "description", "The notification the agent reads at GET /notifications."),
			"agent_id", typed("string"),
			"subject", objectOf(obj("post_id", uuidStr(), "reply_id", uuidStr(), "room_id", uuidStr())),
			"title", typed("string"), "body", typed("string"), "link", typed("string"),
		), "notification_id", "agent_id", "subject", "title", "body", "link"),
	)
}
