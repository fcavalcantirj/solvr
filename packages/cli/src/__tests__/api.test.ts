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

      await client.search("query", { type: "problem", limit: 5 });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/search?q=query&type=problem&per_page=5",
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

    it("includes include parameter", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: "abc123" } }),
      });

      await client.get("abc123", { include: ["approaches", "answers"] });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts/abc123?include=approaches%2Canswers",
        expect.any(Object)
      );
    });
  });

  describe("createPost", () => {
    it("makes POST request to /v1/posts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "new123", title: "New Post", type: "problem" },
        }),
      });

      const post = await client.createPost({
        type: "problem",
        title: "Test Problem",
        description: "This is a test",
        tags: ["test", "example"],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/posts",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"type":"problem"'),
        })
      );
      expect(post.data.id).toBe("new123");
    });
  });

  describe("createAnswer", () => {
    it("makes POST request to /v1/questions/:id/answers", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "ans123", content: "This is my answer" },
        }),
      });

      const answer = await client.createAnswer("q123", "This is my answer");

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/questions/q123/answers",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"content":"This is my answer"'),
        })
      );
      expect(answer.data.id).toBe("ans123");
    });
  });

  describe("createApproach", () => {
    it("makes POST request to /v1/problems/:id/approaches", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: "apr123", angle: "Testing approach" },
        }),
      });

      const approach = await client.createApproach("p123", {
        angle: "Testing approach",
        method: "Use this method",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "https://api.solvr.dev/v1/problems/p123/approaches",
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining('"angle":"Testing approach"'),
        })
      );
      expect(approach.data.id).toBe("apr123");
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
