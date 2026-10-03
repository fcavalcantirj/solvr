import { describe, it, expect } from "vitest";
import { endpointGroups } from "./api-endpoint-data";

// idx 92: the docs show the opt-in room events (schema version 3) and the routes that turn
// them on and off.
function findEndpoint(method: string, path: string) {
  for (const group of endpointGroups) {
    const ep = group.endpoints.find((e) => e.method === method && e.path === path);
    if (ep) return ep;
  }
  return undefined;
}

describe("room notification docs (idx 92)", () => {
  it("the type filter and the webhook events name the schema 3 room events", () => {
    const typeParam = findEndpoint("GET", "/notifications")!.params!.find((p) => p.name === "type");
    for (const text of ["room.reply", "room.review_requested", "schema_version 3", "subject.entry_id"]) {
      expect(typeParam!.description).toContain(text);
    }
    const events = findEndpoint("POST", "/agents/{id}/webhooks")!.params!.find((p) => p.name === "events");
    expect(events!.description).toContain("room.reply");
    expect(events!.description).toContain("room.review_requested");
  });

  it("documents the per-room opt-in and the global pause", () => {
    for (const [method, path] of [
      ["GET", "/rooms/{slug}/notifications"],
      ["PUT", "/rooms/{slug}/notifications"],
      ["DELETE", "/rooms/{slug}/notifications"],
      ["GET", "/me/notification-settings"],
      ["PATCH", "/me/notification-settings"],
    ]) {
      expect(findEndpoint(method, path), `${method} ${path}`).toBeDefined();
    }
    expect(findEndpoint("PUT", "/rooms/{slug}/notifications")!.description).toMatch(/off by default/i);
  });
});
