import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { Output } from "../output.js";

describe("Output", () => {
  let output: Output;
  let consoleSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    output = new Output();
    consoleSpy = vi.spyOn(console, "log").mockImplementation(() => {});
  });

  afterEach(() => {
    consoleSpy.mockRestore();
  });

  describe("json", () => {
    it("outputs data as formatted JSON", () => {
      const data = { id: "123", title: "Test" };
      output.json(data);

      expect(consoleSpy).toHaveBeenCalledWith(JSON.stringify(data, null, 2));
    });

    it("handles arrays", () => {
      const data = [{ id: "1" }, { id: "2" }];
      output.json(data);

      expect(consoleSpy).toHaveBeenCalledWith(JSON.stringify(data, null, 2));
    });
  });

  describe("table", () => {
    it("formats data as a table", () => {
      const data = [
        { id: "1", title: "First", score: 0.9 },
        { id: "2", title: "Second", score: 0.8 },
      ];
      output.table(data, ["id", "title", "score"]);

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("1");
      expect(output_str).toContain("First");
      expect(output_str).toContain("0.9");
    });
  });

  describe("success", () => {
    it("outputs success message with checkmark", () => {
      output.success("Operation completed");

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls[0][0];
      expect(output_str).toContain("Operation completed");
    });
  });

  describe("error", () => {
    let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    });

    afterEach(() => {
      consoleErrorSpy.mockRestore();
    });

    it("outputs error message", () => {
      output.error("Something went wrong");

      expect(consoleErrorSpy).toHaveBeenCalled();
      const output_str = consoleErrorSpy.mock.calls[0][0];
      expect(output_str).toContain("Something went wrong");
    });
  });

  describe("warn", () => {
    it("outputs warning message", () => {
      output.warn("Be careful");

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls[0][0];
      expect(output_str).toContain("Be careful");
    });
  });

  describe("info", () => {
    it("outputs info message", () => {
      output.info("FYI");

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls[0][0];
      expect(output_str).toContain("FYI");
    });
  });

  describe("searchResults", () => {
    it("formats search results nicely", () => {
      const results = {
        data: [
          {
            id: "1",
            type: "problem",
            title: "Test Problem",
            snippet: "This is a test...",
            score: 0.95,
            status: "open",
            votes: 10,
          },
        ],
        meta: { total: 1, took_ms: 23 },
      };
      output.searchResults(results);

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("Test Problem");
      expect(output_str).toContain("0.95");
    });

    it("shows 'no results' message when empty", () => {
      const results = { data: [], meta: { total: 0, took_ms: 5 } };
      output.searchResults(results);

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("No results");
    });
  });

  describe("post", () => {
    it("formats a post nicely", () => {
      const post = {
        data: {
          id: "abc123",
          type: "problem",
          title: "Test Problem",
          description: "This is a detailed description",
          status: "open",
          upvotes: 5,
          downvotes: 1,
          tags: ["test", "example"],
          created_at: "2024-01-01T00:00:00Z",
        },
      };
      output.post(post);

      expect(consoleSpy).toHaveBeenCalled();
      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("Test Problem");
      expect(output_str).toContain("PROBLEM");
    });
  });

  describe("replies", () => {
    it("lists each reply with its id, author, score and body, threaded ones marked", () => {
      output.replies({
        data: [
          { id: "rep1", post_id: "p1", author_type: "agent", author_id: "bot_a", body: "Pin the pool size.", upvotes: 3, downvotes: 0, score: 3, created_at: "2026-09-30T00:00:00Z" },
          { id: "rep2", post_id: "p1", parent_reply_id: "rep1", author_type: "human", author_id: "u_b", body: "Confirmed.", upvotes: 0, downvotes: 0, score: 0, created_at: "2026-09-30T00:01:00Z" },
        ],
        meta: { total: 3, has_more: true, next_cursor: "cur_1" },
      });

      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("3 replies");
      expect(output_str).toContain("rep1");
      expect(output_str).toContain("agent bot_a");
      expect(output_str).toContain("Pin the pool size.");
      expect(output_str).toContain("in reply to rep1");
      expect(output_str).toContain("Confirmed.");
      expect(output_str).toContain("--cursor cur_1");
    });

    it("says there are no replies yet when the page is empty", () => {
      output.replies({ data: [], meta: { total: 0, has_more: false } });

      const output_str = consoleSpy.mock.calls.map((c) => c[0]).join("\n");
      expect(output_str).toContain("No replies yet");
    });

    it("prints the raw page in JSON mode", () => {
      const page = { data: [], meta: { total: 0, has_more: false } };
      output.setJsonMode(true);
      output.replies(page);

      expect(consoleSpy).toHaveBeenCalledWith(JSON.stringify(page, null, 2));
    });
  });
});
