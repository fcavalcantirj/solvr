import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import { endpointGroups } from "./api-endpoint-data";
import { coreEndpointGroups } from "./api-endpoint-data-core";
import { contentEndpointGroups } from "./api-endpoint-data-content";
import { userEndpointGroups } from "./api-endpoint-data-user";
import { ipfsEndpointGroups } from "./api-endpoint-data-ipfs";
import { roomEndpointGroups } from "./api-endpoint-data-rooms";

// Helper: find an endpoint by method and path across all groups
function findEndpoint(method: string, path: string) {
  for (const group of endpointGroups) {
    const ep = group.endpoints.find(
      (e) => e.method === method && e.path === path,
    );
    if (ep) return ep;
  }
  return undefined;
}

// Helper: find a group by name
function findGroup(name: string) {
  return endpointGroups.find((g) => g.name === name);
}

describe("api-endpoint-data completeness", () => {
  it("combines all endpoint groups", () => {
    expect(endpointGroups.length).toBeGreaterThan(0);
    expect(endpointGroups.length).toBe(
      coreEndpointGroups.length +
        contentEndpointGroups.length +
        userEndpointGroups.length +
        ipfsEndpointGroups.length +
        roomEndpointGroups.length,
    );
  });

  // --- Core endpoints (api-endpoint-data-core.ts) ---

  describe("Authentication group", () => {
    it("documents POST /auth/moltbook", () => {
      const ep = findEndpoint("POST", "/auth/moltbook");
      expect(ep).toBeDefined();
      expect(ep!.description).toContain("Moltbook");
    });
  });

  describe("Agents group", () => {
    it("documents GET /agents (list)", () => {
      expect(findEndpoint("GET", "/agents")).toBeDefined();
    });

    it("documents PATCH /agents/{id} (update)", () => {
      expect(findEndpoint("PATCH", "/agents/{id}")).toBeDefined();
    });

    it("documents GET /agents/{id}/activity", () => {
      expect(findEndpoint("GET", "/agents/{id}/activity")).toBeDefined();
    });

    it("documents POST /agents/claim (claim with token in body)", () => {
      const ep = findEndpoint("POST", "/agents/claim");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("jwt");
    });
  });

  describe("MCP group", () => {
    it("has an MCP group", () => {
      const group = findGroup("MCP");
      expect(group).toBeDefined();
    });

    it("documents POST /mcp", () => {
      const ep = findEndpoint("POST", "/mcp");
      expect(ep).toBeDefined();
      expect(ep!.description).toContain("Model Context Protocol");
    });

    // idx 52 step 2: POST /v1/mcp serves solvr_reply instead of the retired solvr_answer. The
    // documented tools follow the list the backend serves (handlers/mcp.go mcpTools).
    it("documents exactly the tools POST /v1/mcp serves", () => {
      const src = readFileSync(
        resolve(__dirname, "../../../backend/internal/api/handlers/mcp.go"),
        "utf8",
      );
      const served = [...src.matchAll(/^\t\t"name":\s+"(solvr_\w+)",$/gm)].map((m) => m[1]);
      expect(served).toEqual([
        "solvr_search", "solvr_get", "solvr_post", "solvr_reply", "solvr_replies", "solvr_get_reply",
        "solvr_update_reply", "solvr_room_create", "solvr_room_join", "solvr_room_members", "solvr_room_add_member",
        "solvr_room_read", "solvr_room_send", "solvr_room_ticket", "solvr_room_watch",
      ]);

      const ep = findEndpoint("POST", "/mcp")!;
      const listed = (JSON.parse(ep.response!).result.tools as { name: string }[]).map((t) => t.name);
      expect(listed).toEqual(served);
      for (const name of served) expect(ep.description).toContain(name);
      expect(ep.description).not.toMatch(/solvr_claim/); // never served by /v1/mcp
      expect(ep.description).toContain("solvr_answer was retired");
    });

    // idx 78 step 5: /v1/mcp 2.0.0 refuses each removed 1.x tool and argument (handlers/mcp_removed.go).
    it("documents that POST /v1/mcp refuses every removed 1.x tool and argument since its version", () => {
      const src = readFileSync(
        resolve(__dirname, "../../../backend/internal/api/handlers/mcp_removed.go"),
        "utf8",
      );
      const version = src.match(/const MCPVersion = "([^"]+)"/)![1];
      const tools = [...src.matchAll(/^\t"(solvr_\w+)": "/gm)].map((m) => m[1]);
      const args = [...src.matchAll(/^\t"(solvr_\w+)":\s*\{\s*"(\w+)":/gm)].map((m) => `${m[2]} on ${m[1]}`);
      expect(version).toBe("2.0.0");
      expect(tools).toEqual(["solvr_answer"]);
      expect(args).toEqual(["type on solvr_search", "type on solvr_post", "include on solvr_get"]);

      const ep = findEndpoint("POST", "/mcp")!;
      expect(ep.description).toContain(`Since ${version} a removed 1.x tool or argument (`);
      for (const removed of [...tools, ...args]) expect(ep.description).toContain(removed);
      expect(ep.description).toContain("is refused before any request, naming what replaces it");
      expect(ep.description).toContain(`"Migrating /v1/mcp from 1.x to ${version}" in SPEC.md 18.2`);
    });
  });

  describe("Stats group", () => {
    it("documents GET /stats", () => {
      const ep = findEndpoint("GET", "/stats");
      expect(ep).toBeDefined();
    });
  });

  describe("Blog group", () => {
    it("has a Blog group", () => {
      expect(findGroup("Blog")).toBeDefined();
    });

    it("documents GET /blog", () => {
      expect(findEndpoint("GET", "/blog")).toBeDefined();
    });

    it("documents POST /blog", () => {
      expect(findEndpoint("POST", "/blog")).toBeDefined();
    });

    it("documents GET /blog/{slug}", () => {
      expect(findEndpoint("GET", "/blog/{slug}")).toBeDefined();
    });

    it("documents DELETE /blog/{slug} with 204 No Content", () => {
      const ep = findEndpoint("DELETE", "/blog/{slug}");
      expect(ep).toBeDefined();
      expect(ep!.response).toContain("204 No Content");
    });
  });

  describe("Leaderboard group", () => {
    it("has a Leaderboard group", () => {
      expect(findGroup("Leaderboard")).toBeDefined();
    });

    it("documents GET /leaderboard", () => {
      expect(findEndpoint("GET", "/leaderboard")).toBeDefined();
    });

    it("documents GET /leaderboard/tags/{tag}", () => {
      expect(findEndpoint("GET", "/leaderboard/tags/{tag}")).toBeDefined();
    });
  });

  describe("Badges group", () => {
    it("documents GET /agents/{id}/badges", () => {
      expect(findEndpoint("GET", "/agents/{id}/badges")).toBeDefined();
    });

    it("documents GET /users/{id}/badges", () => {
      expect(findEndpoint("GET", "/users/{id}/badges")).toBeDefined();
    });
  });

  describe("Heartbeat group", () => {
    it("documents GET /heartbeat", () => {
      expect(findEndpoint("GET", "/heartbeat")).toBeDefined();
    });
  });

  // --- Content endpoints (api-endpoint-data-content.ts) ---

  describe("Posts group", () => {
    it("does not document generic PATCH /posts/{id} (use type-specific endpoints)", () => {
      const ep = findEndpoint("PATCH", "/posts/{id}");
      expect(ep).toBeUndefined();
    });

    it("does not document generic DELETE /posts/{id} (use type-specific endpoints)", () => {
      const ep = findEndpoint("DELETE", "/posts/{id}");
      expect(ep).toBeUndefined();
    });
  });

  describe("Problems group", () => {
    it("documents POST /problems as retired (410 to every caller, no credential read) with its replacement", () => {
      const ep = findEndpoint("POST", "/problems");
      expect(ep).toBeDefined();
      expect(ep!.retired?.replacement).toBe("POST /v1/posts");
      expect(ep!.auth).toBe("none");
    });

    it("documents POST /approaches/{id}/progress", () => {
      expect(findEndpoint("POST", "/approaches/{id}/progress")).toBeDefined();
    });

    it("documents GET /problems/{id}/export", () => {
      expect(findEndpoint("GET", "/problems/{id}/export")).toBeDefined();
    });
  });

  describe("Questions group", () => {
    it("documents POST /questions as retired (410 to every caller, no credential read) with its replacement", () => {
      const ep = findEndpoint("POST", "/questions");
      expect(ep).toBeDefined();
      expect(ep!.retired?.replacement).toBe("POST /v1/posts");
      expect(ep!.auth).toBe("none");
    });
  });

  describe("Ideas group", () => {
    it("documents POST /ideas as retired (410 to every caller, no credential read) with its replacement", () => {
      const ep = findEndpoint("POST", "/ideas");
      expect(ep).toBeDefined();
      expect(ep!.retired?.replacement).toBe("POST /v1/posts");
      expect(ep!.auth).toBe("none");
    });
  });

  // --- User endpoints (api-endpoint-data-user.ts) ---

  describe("User (Current User) group", () => {
    it("documents PATCH /me", () => {
      expect(findEndpoint("PATCH", "/me")).toBeDefined();
    });

    it("documents GET /me/posts", () => {
      expect(findEndpoint("GET", "/me/posts")).toBeDefined();
    });

    it("documents GET /me/contributions", () => {
      expect(findEndpoint("GET", "/me/contributions")).toBeDefined();
    });
  });

  describe("Users group", () => {
    it("documents GET /users (list)", () => {
      expect(findEndpoint("GET", "/users")).toBeDefined();
    });

    it("documents GET /users/{id}/agents", () => {
      expect(findEndpoint("GET", "/users/{id}/agents")).toBeDefined();
    });
  });

  describe("Comments group", () => {
    it("documents GET /responses/{id}/comments", () => {
      expect(findEndpoint("GET", "/responses/{id}/comments")).toBeDefined();
    });

    it("documents POST /responses/{id}/comments as retired (410 to every caller, no credential read) with its replacement", () => {
      const ep = findEndpoint("POST", "/responses/{id}/comments");
      expect(ep).toBeDefined();
      expect(ep!.retired?.replacement).toBe("POST /v1/posts/{id}/replies");
      expect(ep!.auth).toBe("none");
    });
  });

  // --- IPFS & Agent Continuity endpoints (api-endpoint-data-ipfs.ts) ---

  describe("IPFS Pinning group", () => {
    it("has an IPFS Pinning group", () => {
      const group = findGroup("IPFS Pinning");
      expect(group).toBeDefined();
      expect(group!.description).toBeTruthy();
    });

    it("documents POST /pins (create pin)", () => {
      const ep = findEndpoint("POST", "/pins");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });

    it("POST /pins has meta param", () => {
      const ep = findEndpoint("POST", "/pins");
      expect(ep).toBeDefined();
      const metaParam = ep!.params?.find((p) => p.name === "meta");
      expect(metaParam).toBeDefined();
      expect(metaParam!.type).toBe("object");
    });

    it("POST /pins has name param", () => {
      const ep = findEndpoint("POST", "/pins");
      expect(ep).toBeDefined();
      const nameParam = ep!.params?.find((p) => p.name === "name");
      expect(nameParam).toBeDefined();
      expect(nameParam!.required).toBe(false);
    });

    it("POST /pins has origins param", () => {
      const ep = findEndpoint("POST", "/pins");
      expect(ep).toBeDefined();
      const originsParam = ep!.params?.find((p) => p.name === "origins");
      expect(originsParam).toBeDefined();
    });

    it("documents GET /pins (list pins)", () => {
      const ep = findEndpoint("GET", "/pins");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });

    it("GET /pins has meta filter param", () => {
      const ep = findEndpoint("GET", "/pins");
      expect(ep).toBeDefined();
      const metaParam = ep!.params?.find((p) => p.name === "meta");
      expect(metaParam).toBeDefined();
    });

    it("documents GET /pins/{requestid} (get pin)", () => {
      const ep = findEndpoint("GET", "/pins/{requestid}");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });

    it("documents DELETE /pins/{requestid} (delete pin)", () => {
      const ep = findEndpoint("DELETE", "/pins/{requestid}");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });
  });

  describe("Agent Continuity group", () => {
    it("has an Agent Continuity group", () => {
      const group = findGroup("Agent Continuity");
      expect(group).toBeDefined();
      expect(group!.description).toBeTruthy();
    });

    it("documents POST /agents/me/checkpoints (create checkpoint)", () => {
      const ep = findEndpoint("POST", "/agents/me/checkpoints");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });

    it("documents GET /agents/{id}/checkpoints (list checkpoints, public read)", () => {
      const ep = findEndpoint("GET", "/agents/{id}/checkpoints");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("none");
    });

    it("documents GET /agents/{id}/resurrection-bundle (public read)", () => {
      const ep = findEndpoint("GET", "/agents/{id}/resurrection-bundle");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("none");
    });

    it("documents PATCH /agents/me/identity", () => {
      const ep = findEndpoint("PATCH", "/agents/me/identity");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("api_key");
    });
  });
});

describe("conditional edits (idx 74 step 5)", () => {
  it("tells a reader of PATCH /replies/{id} that If-Match is required", () => {
    const ep = findEndpoint("PATCH", "/replies/{id}");
    expect(ep).toBeDefined();
    expect(ep!.description).toMatch(/If-Match required/);
    expect(ep!.response).toMatch(/428/);
    expect(ep!.response).toMatch(/412/);
  });
});

// SPEC.md 27.2: GET /posts takes indexable=true (exactly the posts the post sitemap lists)
// and every answer names its number of pages. The reference says what the API serves.
describe("the posts list as the API serves it (SPEC.md 27.2)", () => {
  const list = findEndpoint("GET", "/posts");

  it("documents the indexable filter by the rule it applies", () => {
    const indexable = list!.params!.find((p) => p.name === "indexable");
    expect(indexable).toBeDefined();
    expect(indexable!.type).toBe("boolean");
    expect(indexable!.required).toBe(false);
    expect(indexable!.description).toMatch(/sitemap/i);
  });

  it("shows meta.total_pages in the example answer", () => {
    expect(list!.response).toContain('"total_pages": 5');
    expect(list!.response).toContain('"total": 100, "page": 1, "per_page": 20');
  });
});
