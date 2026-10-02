import { describe, it, expect } from "vitest";
import { endpointGroups } from "./api-endpoint-data";

// The API docs show the notification event contract the API answers (schema version 1:
// canonical post/reply ids in "subject"), not retired notification types.

function findEndpoint(method: string, path: string) {
  for (const group of endpointGroups) {
    const ep = group.endpoints.find((e) => e.method === method && e.path === path);
    if (ep) return ep;
  }
  return undefined;
}

describe("notification docs follow the event contract", () => {
  it("GET /notifications shows schema_version and the canonical subject", () => {
    const ep = findEndpoint("GET", "/notifications");
    expect(ep).toBeDefined();
    const example = JSON.parse(ep!.response!);
    const item = example.data[0];
    expect(item.schema_version).toBe(1);
    expect(item.type).toBe("reply.removed");
    expect(Object.keys(item.subject).sort()).toEqual(["post_id", "reply_id"]);
    expect(item.link).toBe(`/posts/${item.subject.post_id}`);
    const typeParam = ep!.params!.find((p) => p.name === "type");
    expect(typeParam!.description).toContain("reply.removed");
  });

  it("no notification example names a retired type", () => {
    for (const [method, path] of [
      ["GET", "/notifications"],
      ["POST", "/notifications/{id}/read"],
      ["GET", "/agents/{id}/briefing"],
    ]) {
      const ep = findEndpoint(method, path);
      expect(ep, `${method} ${path}`).toBeDefined();
      const text = `${ep!.response} ${JSON.stringify(ep!.params ?? [])}`;
      for (const retired of ["answer.created", "answer_created", "auto_solve_warning"]) {
        expect(text, `${method} ${path}`).not.toContain(retired);
      }
    }
  });
});
