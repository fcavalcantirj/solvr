import { describe, it, expect } from "vitest";
import { endpointGroups } from "./api-endpoint-data";
import { coreEndpointGroups } from "./api-endpoint-data-core";
import { contentEndpointGroups } from "./api-endpoint-data-content";
import { userEndpointGroups } from "./api-endpoint-data-user";

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
        userEndpointGroups.length,
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
  });

  describe("Stats group", () => {
    it("documents GET /stats/problems", () => {
      expect(findEndpoint("GET", "/stats/problems")).toBeDefined();
    });

    it("documents GET /stats/questions", () => {
      expect(findEndpoint("GET", "/stats/questions")).toBeDefined();
    });

    it("documents GET /stats/ideas", () => {
      expect(findEndpoint("GET", "/stats/ideas")).toBeDefined();
    });
  });

  describe("Sitemap group", () => {
    it("documents GET /sitemap/urls", () => {
      expect(findEndpoint("GET", "/sitemap/urls")).toBeDefined();
    });

    it("documents GET /sitemap/counts", () => {
      expect(findEndpoint("GET", "/sitemap/counts")).toBeDefined();
    });
  });

  // --- Content endpoints (api-endpoint-data-content.ts) ---

  describe("Posts group", () => {
    it("documents PATCH /posts/{id} (update)", () => {
      const ep = findEndpoint("PATCH", "/posts/{id}");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
    });

    it("documents DELETE /posts/{id}", () => {
      const ep = findEndpoint("DELETE", "/posts/{id}");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
    });
  });

  describe("Problems group", () => {
    it("documents POST /problems (create)", () => {
      const ep = findEndpoint("POST", "/problems");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
    });

    it("documents POST /approaches/{id}/progress", () => {
      expect(findEndpoint("POST", "/approaches/{id}/progress")).toBeDefined();
    });

    it("documents GET /problems/{id}/export", () => {
      expect(findEndpoint("GET", "/problems/{id}/export")).toBeDefined();
    });
  });

  describe("Questions group", () => {
    it("documents POST /questions (create)", () => {
      const ep = findEndpoint("POST", "/questions");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
    });
  });

  describe("Ideas group", () => {
    it("documents POST /ideas (create)", () => {
      const ep = findEndpoint("POST", "/ideas");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
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

    it("documents POST /responses/{id}/comments", () => {
      const ep = findEndpoint("POST", "/responses/{id}/comments");
      expect(ep).toBeDefined();
      expect(ep!.auth).toBe("both");
    });
  });
});
