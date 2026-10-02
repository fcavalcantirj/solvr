import { describe, it, expect } from "vitest";
import { endpointGroups } from "./api-endpoint-data";

// The API docs list the webhook routes the API serves (SPEC.md Part 12.3): who may call them,
// the schema-1 events a webhook subscribes to, the refusal of retired event names, and what
// every delivery carries — the delivery ID a retry preserves and the signature.

const schemaOneEvents = ["post.approved", "post.rejected", "reply.removed", "reply.flagged", "blog_post_rejected"];
const retiredEvents = ["answer.created", "comment.created", "approach.stuck", "problem.solved"];

function webhooksGroup() {
  return endpointGroups.find((g) => g.name === "Webhooks");
}

function findEndpoint(method: string, path: string) {
  return webhooksGroup()?.endpoints.find((e) => e.method === method && e.path === path);
}

describe("webhook docs follow the served routes and the delivery contract", () => {
  it("lists the five webhook routes, each for the agent itself or its owner", () => {
    const group = webhooksGroup();
    expect(group).toBeDefined();
    expect(group!.endpoints.map((e) => `${e.method} ${e.path}`).sort()).toEqual([
      "DELETE /agents/{id}/webhooks/{wh_id}",
      "GET /agents/{id}/webhooks",
      "GET /agents/{id}/webhooks/{wh_id}",
      "PATCH /agents/{id}/webhooks/{wh_id}",
      "POST /agents/{id}/webhooks",
    ]);
    for (const ep of group!.endpoints) {
      expect(ep.auth, `${ep.method} ${ep.path}`).toBe("both");
      expect(ep.retired, `${ep.method} ${ep.path}`).toBeUndefined();
    }
    expect(group!.description).toContain("agent itself");
  });

  it("create subscribes to schema-1 events, never returns the secret, and names EVENT_RETIRED", () => {
    const ep = findEndpoint("POST", "/agents/{id}/webhooks");
    expect(ep).toBeDefined();
    const events = ep!.params!.find((p) => p.name === "events");
    for (const name of schemaOneEvents) {
      expect(events!.description).toContain(name);
    }
    expect(ep!.params!.find((p) => p.name === "secret")!.required).toBe(true);
    const created = JSON.parse(ep!.response);
    expect(created.data.events.every((e: string) => schemaOneEvents.includes(e))).toBe(true);
    expect(Object.keys(created.data)).not.toContain("secret");
    expect(ep!.description).toContain("EVENT_RETIRED");
  });

  it("documents the delivery: the preserved delivery ID, the signature and the canonical subject", () => {
    const group = webhooksGroup()!;
    const text = `${group.description} ${group.endpoints.map((e) => e.description).join(" ")}`;
    for (const header of ["X-Solvr-Delivery-ID", "X-Solvr-Signature", "X-Solvr-Event", "X-Solvr-Delivery-Attempt"]) {
      expect(text).toContain(header);
    }
    expect(text).toContain("same on every retry");
    expect(text).toContain("HMAC-SHA256");
    expect(text).toContain("openapi.json");
  });

  it("no example subscribes to a retired event", () => {
    for (const ep of webhooksGroup()!.endpoints) {
      const text = `${ep.response} ${JSON.stringify(ep.params ?? [])}`;
      for (const retired of retiredEvents) {
        expect(text, `${ep.method} ${ep.path}`).not.toContain(retired);
      }
    }
  });
});
