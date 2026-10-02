import { describe, it, expect, beforeEach, vi } from "vitest";
import { ApiClient, ApiError } from "../api.js";

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe("ApiClient", () => {
  let client: ApiClient;

  beforeEach(() => {
    mockFetch.mockReset();
    client = new ApiClient("solvr_sk_test", "https://api.solvr.dev");
  });

  describe("search", () => {
    it("makes GET request to /v1/search with query", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: [{ id: "1", title: "Test", score: 0.9 }],
          meta: { total: 1 },
        }),
      });

      const results = await client.search("test query");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/search?q=test+query",
        expect.objectContaining({
          method: "GET",
          headers: expect.objectContaining({
            Authorization: "Bearer solvr_sk_test",
          }),
        })
      );
      expect(results.data).toHaveLength(1);
      expect(results.data[0].title).toBe("Test");
    });

    it("includes optional parameters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], meta: { total: 0 } }),
      });

      // 2.0.0: no legacy type/status filter (migration.test.ts "the API client sends no legacy search filter")
      await client.search("query", { limit: 5, page: 2, sort: "newest" });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/search?q=query&per_page=5&page=2&sort=newest",
        expect.any(Object)
      );
    });

    it("throws ApiError on non-ok response", async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: { code: "UNAUTHORIZED", message: "Invalid API key" } }),
      });

      await expect(client.search("test")).rejects.toThrow(ApiError);

      // Need fresh mock for second assertion
      mockFetch.mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: { code: "UNAUTHORIZED", message: "Invalid API key" } }),
      });

      await expect(client.search("test")).rejects.toMatchObject({
        status: 401,
        code: "UNAUTHORIZED",
      });
    });
  });

  describe("get", () => {
    it("makes GET request to /v1/posts/:id", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "abc123", title: "Test Post", type: "problem" },
        }),
      });

      const post = await client.get("abc123");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/abc123",
        expect.objectContaining({
          method: "GET",
        })
      );
      expect(post.data.id).toBe("abc123");
    });

    it("requests the post alone (its replies are read with replies())", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: "abc123" } }),
      });

      // An old caller may still pass an include list; nothing is appended to the URL.
      await (client.get as (id: string, options?: unknown) => Promise<unknown>)("abc123", {
        include: ["approaches", "answers"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/abc123",
        expect.objectContaining({ method: "GET" })
      );
    });
  });

  describe("createPost", () => {
    it("makes POST request to /v1/posts with a canonical post (no type)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "new123", title: "Test Post", type: "post" },
        }),
      });

      const post = await client.createPost({
        title: "Test Post",
        description: "This is a test",
        tags: ["test", "example"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts",
        expect.objectContaining({ method: "POST" })
      );
      const [, init] = mockFetch.mock.calls[0];
      expect(JSON.parse(init.body)).toEqual({
        title: "Test Post",
        description: "This is a test",
        tags: ["test", "example"],
      });
      expect(post.data.id).toBe("new123");
      expect(post.data.type).toBe("post");
    });

    it("sends visibility when given", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: "p1", type: "post" } }),
      });

      await client.createPost({ title: "T", description: "D", visibility: "family" });

      const [, init] = mockFetch.mock.calls[0];
      expect(JSON.parse(init.body)).toEqual({ title: "T", description: "D", visibility: "family" });
    });
  });

  describe("reply", () => {
    it("makes POST request to /v1/posts/:id/replies with the body", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "rep123", post_id: "p123", body: "This is my reply" },
        }),
      });

      const reply = await client.reply("p123", "This is my reply");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/p123/replies",
        expect.objectContaining({ method: "POST" })
      );
      const [, init] = mockFetch.mock.calls[0];
      expect(JSON.parse(init.body)).toEqual({ body: "This is my reply" });
      expect(reply.data.id).toBe("rep123");
    });

    it("threads a reply under a parent reply", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: "rep2", parent_reply_id: "rep1" } }),
      });

      await client.reply("p123", "Confirmed", "rep1");

      const [, init] = mockFetch.mock.calls[0];
      expect(JSON.parse(init.body)).toEqual({ body: "Confirmed", parent_reply_id: "rep1" });
    });
  });

  describe("replies", () => {
    it("makes GET request to /v1/posts/:id/replies", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [{ id: "rep1", body: "First" }], meta: { total: 1, has_more: false } }),
      });

      const page = await client.replies("p123");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/p123/replies",
        expect.objectContaining({ method: "GET" })
      );
      expect(page.data[0].id).toBe("rep1");
      expect(page.meta.total).toBe(1);
    });

    it("pages with a cursor and a limit", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], meta: { total: 0, has_more: false } }),
      });

      await client.replies("p123", { cursor: "abc", limit: 10 });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/p123/replies?cursor=abc&limit=10",
        expect.any(Object)
      );
    });
  });

  describe("legacy contribution routes", () => {
    it("has no method that calls a retired answer or approach route", () => {
      const methods = client as unknown as Record<string, unknown>;
      expect(methods.createAnswer).toBeUndefined();
      expect(methods.createApproach).toBeUndefined();
    });

    it("exposes the migration details of a retired route", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 410,
        json: async () => ({
          error: {
            code: "ENDPOINT_RETIRED",
            message:
              "POST /v1/questions/{id}/answers was retired with the canonical knowledge model; use POST /v1/posts/{id}/replies instead.",
            details: {
              retired_route: "POST /v1/questions/{id}/answers",
              replacement: "POST /v1/posts/{id}/replies",
            },
          },
        }),
      });

      await expect(client.reply("p123", "x")).rejects.toMatchObject({
        status: 410,
        code: "ENDPOINT_RETIRED",
        details: {
          retired_route: "POST /v1/questions/{id}/answers",
          replacement: "POST /v1/posts/{id}/replies",
        },
      });
    });
  });

  describe("vote", () => {
    it("makes POST request to /v1/posts/:id/vote", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { upvotes: 10, downvotes: 2 } }),
      });

      const result = await client.vote("post123", "up");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/post123/vote",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"direction":"up"'),
        })
      );
      expect(result.data.upvotes).toBe(10);
    });
  });
});
