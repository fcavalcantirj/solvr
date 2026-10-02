package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: the webhook routes and the delivery they produce are published in the OpenAPI
// document, so a consumer generates the subscription calls and the receiver from the same
// contract the API answers and sends: the schema-1 event names, the delivery ID preserved
// across retries, the signature and attempt headers, and the retry ladder.

var webhookOperations = []struct{ method, path, operationID string }{
	{"post", "/agents/{id}/webhooks", "createWebhook"},
	{"get", "/agents/{id}/webhooks", "listWebhooks"},
	{"get", "/agents/{id}/webhooks/{wh_id}", "getWebhook"},
	{"patch", "/agents/{id}/webhooks/{wh_id}", "updateWebhook"},
	{"delete", "/agents/{id}/webhooks/{wh_id}", "deleteWebhook"},
}

// webhookDeliveryHeaders are the headers every delivery attempt carries (SPEC.md Part 12.3).
var webhookDeliveryHeaders = []string{"X-Solvr-Event", "X-Solvr-Delivery-ID", "X-Solvr-Delivery-Attempt",
	"X-Solvr-Webhook-ID", "X-Solvr-Signature"}

func webhookEventNames() []string {
	names := make([]string, len(models.ValidWebhookEventTypes))
	for i, e := range models.ValidWebhookEventTypes {
		names[i] = string(e)
	}
	return names
}

func TestOpenAPIWebhooks_TheFiveRoutesArePublishedWithWhoMayCallAndTheirErrors(t *testing.T) {
	spec := servedSpec(t)
	errorsOf := map[string][]string{
		"post /agents/{id}/webhooks":           {"400", "401", "403", "404"},
		"get /agents/{id}/webhooks":            {"401", "403", "404"},
		"get /agents/{id}/webhooks/{wh_id}":    {"400", "401", "403", "404"},
		"patch /agents/{id}/webhooks/{wh_id}":  {"400", "401", "403", "404"},
		"delete /agents/{id}/webhooks/{wh_id}": {"400", "401", "403", "404"},
	}
	for _, o := range webhookOperations {
		key := o.method + " " + o.path
		op := operation(t, spec, o.method, o.path)
		assert.Equal(t, o.operationID, op["operationId"], key)
		assert.Equal(t, []interface{}{"Webhooks"}, op["tags"], key)
		assert.Contains(t, op["description"], "agent itself", "%s names who may call it", key)

		sec, _ := op["security"].([]interface{})
		require.Len(t, sec, 1, "%s requires a credential", key)
		assert.NotEmpty(t, sec[0], "%s requires a credential", key)

		for _, status := range errorsOf[key] {
			assert.Equal(t, "#/components/responses/"+errorRows[status], refName(at(t, op, "responses", status)), "%s: %s", key, status)
		}
		for status, raw := range op["responses"].(map[string]interface{}) {
			resp := deref(t, spec, raw).(map[string]interface{})
			assert.NotEmpty(t, resp["description"], "%s: response %s has no description", key, status)
		}
		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			id := p["in"].(string) + ":" + p["name"].(string)
			assert.False(t, declared[id], "%s declares parameter %s twice", key, id)
			declared[id] = true
		}
		for _, variable := range regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(o.path, -1) {
			assert.True(t, declared["path:"+variable[1]], "%s does not declare path parameter %s", key, variable[1])
		}
	}

	ok := map[string]string{
		"post /agents/{id}/webhooks":          "201 WebhookResponse",
		"get /agents/{id}/webhooks":           "200 WebhookList",
		"get /agents/{id}/webhooks/{wh_id}":   "200 WebhookResponse",
		"patch /agents/{id}/webhooks/{wh_id}": "200 WebhookResponse",
	}
	for key, want := range ok {
		parts := strings.SplitN(key, " ", 2)
		status, schema := strings.SplitN(want, " ", 2)[0], strings.SplitN(want, " ", 2)[1]
		got := at(t, operation(t, spec, parts[0], parts[1]), "responses", status, "content", "application/json", "schema")
		assert.Equal(t, "#/components/schemas/"+schema, refName(got), key)
	}
	_, hasBody := at(t, operation(t, spec, "delete", "/agents/{id}/webhooks/{wh_id}"), "responses", "204").(map[string]interface{})["content"]
	assert.False(t, hasBody, "a delete answers 204 with no body")
	assert.Equal(t, "#/components/schemas/CreateWebhookRequest",
		refName(at(t, operation(t, spec, "post", "/agents/{id}/webhooks"), "requestBody", "content", "application/json", "schema")))
	assert.Equal(t, "#/components/schemas/UpdateWebhookRequest",
		refName(at(t, operation(t, spec, "patch", "/agents/{id}/webhooks/{wh_id}"), "requestBody", "content", "application/json", "schema")))

	tagged := false
	for _, raw := range spec["tags"].([]interface{}) {
		tagged = tagged || raw.(map[string]interface{})["name"] == "Webhooks"
	}
	assert.True(t, tagged, "the Webhooks tag is declared")
}

// The create and edit name the codes a rejected event list answers, and the retired names.
func TestOpenAPIWebhooks_TheEventListRefusalsAreDocumented(t *testing.T) {
	spec := servedSpec(t)
	for _, key := range []string{"post /agents/{id}/webhooks", "patch /agents/{id}/webhooks/{wh_id}"} {
		parts := strings.SplitN(key, " ", 2)
		desc := operation(t, spec, parts[0], parts[1])["description"].(string)
		for _, want := range []string{"EVENT_RETIRED", "INVALID_EVENT_TYPE", "VALIDATION_ERROR", "supported_events", "retired_event"} {
			assert.Contains(t, desc, want, key)
		}
		for _, retired := range models.RetiredWebhookEventTypes {
			assert.Contains(t, desc, string(retired), "%s names the retired event %s", key, retired)
		}
	}
	desc := operation(t, spec, "get", "/agents/{id}/webhooks/{wh_id}")["description"].(string)
	assert.Contains(t, desc, "INVALID_ID", "a malformed webhook id is named")
}

// The published schemas list exactly the fields the handlers decode and answer, and the
// payload the delivery job sends.
func TestOpenAPIWebhooks_SchemasDescribeTheJSONTheHandlersAndTheDeliveryUse(t *testing.T) {
	spec := servedSpec(t)
	for schema, typ := range map[string]reflect.Type{
		"Webhook":              reflect.TypeOf(models.Webhook{}),
		"CreateWebhookRequest": reflect.TypeOf(models.CreateWebhookRequest{}),
		"UpdateWebhookRequest": reflect.TypeOf(models.UpdateWebhookRequest{}),
		"WebhookDelivery":      reflect.TypeOf(models.WebhookDeliveryPayload{}),
	} {
		want := jsonFields(typ)
		sort.Strings(want)
		sameSet(t, schema+" properties", propertyNames(t, spec, schema), want)
	}
	sameSet(t, "WebhookDeliveryData properties", propertyNames(t, spec, "WebhookDeliveryData"),
		[]string{"notification_id", "agent_id", "subject", "title", "body", "link"})
	sameSet(t, "WebhookDeliveryData.subject properties",
		mapKeysOf(at(t, spec, "components", "schemas", "WebhookDeliveryData", "properties", "subject", "properties")),
		[]string{"post_id", "reply_id"})

	events := webhookEventNames()
	for _, path := range [][]string{
		{"CreateWebhookRequest", "properties", "events", "items", "enum"},
		{"UpdateWebhookRequest", "properties", "events", "items", "enum"},
		{"Webhook", "properties", "events", "items", "enum"},
		{"WebhookDelivery", "properties", "event", "enum"},
	} {
		sameSet(t, strings.Join(path, "."), strings_(t, at(t, spec, append([]string{"components", "schemas"}, path...)...)), events)
	}
	statuses := make([]string, len(models.ValidWebhookStatuses))
	for i, s := range models.ValidWebhookStatuses {
		statuses[i] = string(s)
	}
	sameSet(t, "Webhook.status enum", strings_(t, at(t, spec, "components", "schemas", "Webhook", "properties", "status", "enum")), statuses)
	sameSet(t, "UpdateWebhookRequest.status enum", strings_(t, at(t, spec, "components", "schemas", "UpdateWebhookRequest", "properties", "status", "enum")), statuses)
	sameSet(t, "CreateWebhookRequest required", strings_(t, at(t, spec, "components", "schemas", "CreateWebhookRequest", "required")),
		[]string{"url", "events", "secret"})
	assert.Equal(t, []interface{}{float64(models.NotificationSchemaVersion)},
		at(t, spec, "components", "schemas", "WebhookDelivery", "properties", "schema_version", "enum"))
	assert.Contains(t, at(t, spec, "components", "schemas", "WebhookDelivery", "properties", "id", "description"), "X-Solvr-Delivery-ID")
	assert.Contains(t, at(t, spec, "components", "schemas", "CreateWebhookRequest", "properties", "secret", "description"), "never returned")
}

func mapKeysOf(node interface{}) []string {
	m, _ := node.(map[string]interface{})
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// The delivery is published where OpenAPI 3.0 describes a request the API makes: a callback of
// the create, to the subscribed url, with the payload, the headers of every attempt and the
// retry ladder the delivery job runs.
func TestOpenAPIWebhooks_TheDeliveryIsACallbackOfTheCreate(t *testing.T) {
	spec := servedSpec(t)
	send := at(t, operation(t, spec, "post", "/agents/{id}/webhooks"), "callbacks", "delivery", "{$request.body#/url}", "post").(map[string]interface{})
	assert.Equal(t, "#/components/schemas/WebhookDelivery", refName(at(t, send, "requestBody", "content", "application/json", "schema")))
	headers := map[string]map[string]interface{}{}
	for _, raw := range send["parameters"].([]interface{}) {
		p := deref(t, spec, raw).(map[string]interface{})
		require.Equal(t, "header", p["in"])
		assert.Equal(t, true, p["required"], "%s is sent on every attempt", p["name"])
		headers[p["name"].(string)] = p
	}
	sameSet(t, "delivery headers", mapKeysOf(toAny(headers)), webhookDeliveryHeaders)
	assert.Contains(t, headers["X-Solvr-Signature"]["description"], "HMAC-SHA256")
	assert.Contains(t, headers["X-Solvr-Delivery-ID"]["description"], "same on every attempt")
	sameSet(t, "X-Solvr-Event enum", strings_(t, at(t, headers["X-Solvr-Event"], "schema", "enum")), webhookEventNames())
	for _, status := range []string{"2XX", "default"} {
		assert.NotEmpty(t, at(t, send, "responses", status, "description"), status)
	}

	svc := services.NewWebhookDeliveryService(nil, nil)
	var delays []interface{}
	for _, d := range svc.GetRetryDelays() {
		delays = append(delays, float64(d.Seconds()))
	}
	attempts := 0
	for svc.ShouldRetry(attempts) {
		attempts++
	}
	delivery := at(t, send, "x-solvr-delivery").(map[string]interface{})
	assert.Equal(t, delays, delivery["retry_delays_seconds"], "the ladder the job retries on")
	assert.Equal(t, []interface{}{float64(0), float64(60), float64(300), float64(1800), float64(7200)}, delivery["retry_delays_seconds"])
	assert.Equal(t, float64(attempts), delivery["max_attempts"])
	assert.Equal(t, float64(5), delivery["max_attempts"])
	assert.Equal(t, services.NewWebhookHTTPClient().Timeout.Seconds(), delivery["timeout_seconds"])
	assert.Equal(t, jobs.DefaultWebhookDeliveryInterval.Seconds(), delivery["run_interval_seconds"])
}

func toAny(m map[string]map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// The documented webhook operations are served, at the documented methods and paths.
func TestOpenAPIWebhooks_EveryDocumentedOperationIsAServedRoute(t *testing.T) {
	served := servedRoutes(t)
	spec := servedSpec(t)
	for _, o := range webhookOperations {
		operation(t, spec, o.method, o.path)
		route := strings.ToUpper(o.method) + " /v1" + o.path
		assert.True(t, served[route], "%s is documented but not served", route)
	}
}

// The running API answers what is published: each webhook route's answer, a refusal's error,
// and a real delivery's body and headers validate against the documented schemas.
func TestOpenAPIWebhooks_TheRunningAPIAnswersAndSendsWhatIsPublished(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	spec := servedSpec(t)
	agentID, key := moderationAgent(t, ts, pool)
	post := seedOpenPost(t, pool, "post")
	rcv := newWebhookReceiver(t)
	hooks := ts.URL + "/v1/agents/" + agentID + "/webhooks"

	valid := func(what, body, schema string) map[string]interface{} {
		t.Helper()
		var v map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body), &v), "%s: %s", what, body)
		assert.Empty(t, schemaProblems(spec, v, ref("schemas", schema), what), "%s: %s", what, body)
		return v
	}

	created := createWebhookThroughAPI(t, ts, key, agentID, rcv.URL, "whsec-openapi-contract", "reply.removed", "post.approved")
	require.Equal(t, http.StatusCreated, created.status, created.body)
	webhookID := valid("create", created.body, "WebhookResponse")["data"].(map[string]interface{})["id"].(string)

	listed, err := callStatusContract(http.DefaultClient, http.MethodGet, hooks, key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listed.status, listed.body)
	valid("list", listed.body, "WebhookList")

	got, err := callStatusContract(http.DefaultClient, http.MethodGet, hooks+"/"+webhookID, key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, got.body)
	valid("get", got.body, "WebhookResponse")

	patched, err := callStatusContract(http.DefaultClient, http.MethodPatch, hooks+"/"+webhookID, key, `{"events":["reply.removed"]}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, patched.status, patched.body)
	valid("update", patched.body, "WebhookResponse")

	retired := createWebhookThroughAPI(t, ts, key, agentID, rcv.URL, "whsec", "answer.created")
	require.Equal(t, http.StatusBadRequest, retired.status, retired.body)
	valid("EVENT_RETIRED", retired.body, "Error")

	mod.QueueResults(rejected(agentID))
	reply := gateCall(t, ts, key, "/v1/posts/"+post+"/replies", `{"body":"reply sent to the contract receiver `+uuid.NewString()[:8]+`"}`)
	require.Equal(t, http.StatusCreated, reply.status, reply.body)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM webhook_deliveries WHERE webhook_id = $1::uuid`, webhookID)
	_, err = NewWebhookDeliveryJob(pool, rcv.Client()).RunOnce(context.Background())
	require.NoError(t, err)

	sent := rcv.received()
	require.Len(t, sent, 1)
	payload := valid("delivery", string(sent[0].body), "WebhookDelivery")
	assert.Empty(t, schemaProblems(spec, payload["data"], ref("schemas", "WebhookDeliveryData"), "delivery.data"))
	send := at(t, operation(t, spec, "post", "/agents/{id}/webhooks"), "callbacks", "delivery", "{$request.body#/url}", "post").(map[string]interface{})
	for _, raw := range send["parameters"].([]interface{}) {
		name := deref(t, spec, raw).(map[string]interface{})["name"].(string)
		assert.NotEmpty(t, sent[0].header.Get(name), "the delivery carries the documented header %s", name)
	}

	deleted, err := callStatusContract(http.DefaultClient, http.MethodDelete, hooks+"/"+webhookID, key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleted.status, deleted.body)
}
