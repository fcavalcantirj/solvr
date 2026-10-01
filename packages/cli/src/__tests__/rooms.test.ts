import { describe, it, expect, afterEach } from "vitest";
import * as fs from "fs";
import { freshConfigPath, json, runCli, startServer, within } from "./harness.js";
import type { LocalServer } from "./harness.js";

// The room commands and the credential scopes, against a real local HTTP server.

const KEY = "solvr_sk_rooms_test";
const SLUG = "planner-executor";

const room = {
  id: "r1", slug: SLUG, display_name: "Planner and executor", tags: [], is_private: false,
  message_count: 0, created_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z", last_active_at: "2026-10-01T00:00:00Z",
};
const entry = (id: number, body: string) => ({
  id, room_id: "r1", sequence: id, kind: "message", author_type: "agent", author_id: "agent_a", actor_label: "agent_a",
  body, content_type: "text", extension: {}, created_at: "2026-10-01T00:00:00Z",
});
const frame = (id: number, content: string) =>
  `id: ${id}\nevent: message\ndata: ${JSON.stringify({
    id, sequence: id, type: "message", room_id: "r1", agent_name: "agent_b", timestamp: "2026-10-01T00:00:00Z",
    payload: { id, room_id: "r1", author_type: "agent", agent_name: "agent_b", content, content_type: "text", metadata: {}, sequence_num: id, created_at: "2026-10-01T00:00:00Z" },
  })}\n\n`;

let server: LocalServer | undefined;
afterEach(async () => {
  await server?.close();
  server = undefined;
});

describe("room join", () => {
  it("prints the room token and saves it for the room's other commands", async () => {
    server = await startServer((r, w) => {
      if (r.path.endsWith("/handshake")) {
        json(w, 201, { data: { agent_id: "agent_a", room_slug: SLUG, room_token: "solvr_rt_saved", rotated: false } });
      } else {
        json(w, 201, { data: entry(7, "hello"), meta: { idempotent_replay: false } });
      }
    });
    const configPath = freshConfigPath();

    const joined = await runCli(["room", "join", SLUG], { baseUrl: server.url, apiKey: KEY, configPath });
    expect(joined.code).toBe(0);
    expect(joined.stdout).toContain("solvr_rt_saved");
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${KEY}`);
    expect(JSON.parse(server.sent[0].body)).toEqual({});

    const sent = await runCli(["room", "send", SLUG, "--body", "hello"], { baseUrl: server.url, apiKey: KEY, configPath });
    expect(sent.code).toBe(0);
    expect(server.sent[1].path).toBe(`/v1/rooms/${SLUG}/entries`);
    expect(server.sent[1].headers.authorization).toBe("Bearer solvr_rt_saved");
    expect(JSON.parse(server.sent[1].body)).toEqual({ body: "hello" });
  });

  it("keeps a token per room and replaces it on a later join", async () => {
    let n = 0;
    server = await startServer((r, w) => {
      n++;
      const slug = r.path.split("/")[3];
      json(w, 201, { data: { agent_id: "agent_a", room_slug: slug, room_token: `solvr_rt_${slug}_${n}`, rotated: false } });
    });
    const configPath = freshConfigPath();
    const env = { baseUrl: server.url, apiKey: KEY, configPath };

    await runCli(["room", "join", "alpha"], env);
    await runCli(["room", "join", "beta"], env);
    await runCli(["room", "join", "alpha", "--rotate", "--ttl", "600"], env);

    expect(JSON.parse(server.sent[2].body)).toEqual({ rotate: true, ttl_seconds: 600 });
    const saved = JSON.parse(fs.readFileSync(configPath, "utf-8"));
    expect(saved.roomTokens).toEqual({ alpha: "solvr_rt_alpha_3", beta: "solvr_rt_beta_2" });
  });

  it("needs an API key", async () => {
    server = await startServer((_r, w) => json(w, 500, {}));
    const result = await runCli(["room", "join", SLUG], { baseUrl: server.url });
    expect(result.code).toBe(1);
    expect(result.stderr).toContain("No API key configured");
    expect(server.sent).toHaveLength(0);
  });
});

describe("room token scope", () => {
  it("refuses a room command without a room token for that room, before any request", async () => {
    server = await startServer((_r, w) => json(w, 500, {}));
    for (const args of [["read", SLUG], ["send", SLUG, "--body", "x"], ["ticket", SLUG], ["watch", SLUG]]) {
      const result = await runCli(["room", ...args], { baseUrl: server.url, apiKey: KEY });
      expect(result.code, args.join(" ")).toBe(1);
      expect(result.stderr).toContain(`No room token for ${SLUG}. Run: solvr room join ${SLUG}`);
    }
    expect(server.sent).toHaveLength(0);
  });

  it("presents --room-token instead of the API key", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { limit: 50, has_more: false, next_cursor: null } }));
    const result = await runCli(["room", "read", SLUG, "--room-token", "solvr_rt_flag"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(server.sent[0].headers.authorization).toBe("Bearer solvr_rt_flag");
  });

  it("escapes the slug in the path", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { limit: 50, has_more: false, next_cursor: null } }));
    await runCli(["room", "read", "a b/c", "--room-token", "solvr_rt_x"], { baseUrl: server.url });
    expect(server.sent[0].path).toBe("/v1/rooms/a%20b%2Fc/entries");
  });
});

describe("room create", () => {
  it("sends only the given fields; --private makes it private", async () => {
    server = await startServer((_r, w) => json(w, 201, { data: { ...room, is_private: true } }));
    const result = await runCli(["room", "create", "--display-name", "Planner and executor", "--private"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(JSON.parse(server.sent[0].body)).toEqual({ display_name: "Planner and executor", is_private: true });
    expect(result.stdout).toContain(SLUG);
  });
});

describe("room read", () => {
  it("lists the entries and the command for the next page", async () => {
    server = await startServer((_r, w) =>
      json(w, 200, { data: [entry(1, "plan"), entry(2, "built")], meta: { limit: 2, has_more: true, next_cursor: "c2" } })
    );
    const result = await runCli(
      ["room", "read", SLUG, "--room-token", "solvr_rt_x", "--limit", "2", "--kind", "message", "--issue", "i1", "--cursor", "c1"],
      { baseUrl: server.url }
    );
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ limit: "2", kind: "message", issue: "i1", cursor: "c1" });
    expect(result.stdout).toContain("agent_a");
    expect(result.stdout).toContain("plan");
    expect(result.stdout).toContain("built");
    expect(result.stdout).toContain(`solvr room read ${SLUG} --cursor c2`);
  });

  it("accepts --json after the subcommand's arguments", async () => {
    const page = { data: [entry(1, "plan")], meta: { limit: 50, has_more: false, next_cursor: null } };
    server = await startServer((_r, w) => json(w, 200, page));
    const result = await runCli(["room", "read", SLUG, "--room-token", "solvr_rt_x", "--json"], { baseUrl: server.url });
    expect(JSON.parse(result.stdout)).toEqual(page);
  });
});

describe("room send", () => {
  it("addresses a reply to a list of members", async () => {
    server = await startServer((_r, w) => json(w, 201, { data: entry(9, "on it"), meta: { idempotent_replay: false } }));
    const result = await runCli(
      ["room", "send", SLUG, "--room-token", "solvr_rt_x", "--body", "on it", "--reply-to", "7", "--to", "agent_b, agent_c"],
      { baseUrl: server.url }
    );
    expect(result.code).toBe(0);
    expect(JSON.parse(server.sent[0].body)).toEqual({ body: "on it", reply_to_entry_id: 7, addressed_member_ids: ["agent_b", "agent_c"] });
  });

  it("says when the API replayed an earlier client entry id", async () => {
    server = await startServer((_r, w) => json(w, 201, { data: entry(9, "on it"), meta: { idempotent_replay: true } }));
    const result = await runCli(["room", "send", SLUG, "--room-token", "solvr_rt_x", "--body", "on it", "--client-entry-id", "c-1"], { baseUrl: server.url });
    expect(result.stdout).toContain("already sent");
  });
});

describe("room watch", () => {
  it("prints each message, skips heartbeats, and stops after --max messages of a stream the server keeps open", async () => {
    server = await startServer((r, w) => {
      w.writeHead(200, { "Content-Type": "text/event-stream" });
      w.write(": heartbeat\n\n");
      w.write(frame(5, "first"));
      w.write(frame(6, "second"));
      // left open: only --max ends the command
    });
    const result = await within(5000, runCli(["room", "watch", SLUG, "--room-token", "solvr_rt_x", "--max", "2"], { baseUrl: server.url }));
    expect(result.code).toBe(0);
    expect(result.stdout).toContain("agent_b: first");
    expect(result.stdout).toContain("agent_b: second");
    expect(server.sent[0].headers.accept).toBe("text/event-stream");
  });

  it("prints one JSON line per event with --json", async () => {
    server = await startServer((_r, w) => {
      w.writeHead(200, { "Content-Type": "text/event-stream" });
      w.end(frame(5, "first") + frame(6, "second"));
    });
    const result = await runCli(["room", "watch", SLUG, "--room-token", "solvr_rt_x", "--json"], { baseUrl: server.url });
    const lines = result.stdout.split("\n").map((l) => JSON.parse(l) as { id: string; frame: { payload: { content: string } } });
    expect(lines.map((l) => [l.id, l.frame.payload.content])).toEqual([["5", "first"], ["6", "second"]]);
  });

  it("watches anonymously with a ticket", async () => {
    server = await startServer((_r, w) => {
      w.writeHead(200, { "Content-Type": "text/event-stream" });
      w.end(frame(5, "first"));
    });
    const result = await runCli(["room", "watch", SLUG, "--ticket", "solvr_st_t"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(server.sent[0].headers.authorization).toBeUndefined();
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ ticket: "solvr_st_t" });
  });

  it("fails with the code of a stream the API ended because the token was rotated", async () => {
    server = await startServer((_r, w) => {
      w.writeHead(200, { "Content-Type": "text/event-stream" });
      w.end(frame(5, "first") + `event: credential_rotated\ndata: ${JSON.stringify({ code: "CREDENTIAL_ROTATED", message: "handshake again" })}\n\n`);
    });
    const result = await runCli(["room", "watch", SLUG, "--room-token", "solvr_rt_x"], { baseUrl: server.url });
    expect(result.code).toBe(1);
    expect(result.stdout).toContain("first");
    expect(result.stderr).toContain("CREDENTIAL_ROTATED: handshake again");
  });
});

describe("credential scopes of the post and reply commands", () => {
  it("reads anonymously when no API key is configured", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { total: 0, has_more: false } }));
    const result = await runCli(["replies", "p1"], { baseUrl: server.url });
    expect(result.code).toBe(0);
    expect(server.sent[0].headers.authorization).toBeUndefined();
  });

  it("reads with the API key when one is configured", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { total: 0, has_more: false } }));
    await runCli(["replies", "p1"], { baseUrl: server.url, apiKey: KEY });
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${KEY}`);
  });

  it("refuses a write without an API key, before any request", async () => {
    server = await startServer((_r, w) => json(w, 500, {}));
    for (const args of [["post", "--title", "t", "--description", "d"], ["reply", "p1", "--body", "b"], ["update-reply", "r1", "--if-match", '"1"', "--body", "b"], ["room", "create", "--display-name", "x"]]) {
      const result = await runCli(args, { baseUrl: server.url });
      expect(result.code, args.join(" ")).toBe(1);
      expect(result.stderr).toContain("No API key configured");
    }
    expect(server.sent).toHaveLength(0);
  });

  it("sends search only the options given (the API picks the defaults)", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
    await runCli(["search", "pool exhaustion"], { baseUrl: server.url });
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ q: "pool exhaustion" });
  });
});

describe("update-reply", () => {
  it("sends no If-Match for an empty --if-match and reports the API's answer", async () => {
    server = await startServer((_r, w) =>
      json(w, 428, { error: { code: "PRECONDITION_REQUIRED", message: "If-Match is required", request_id: "req-428" } })
    );
    const result = await runCli(["update-reply", "r1", "--if-match", "", "--body", "b"], { baseUrl: server.url, apiKey: KEY });
    expect(server.sent[0].headers["if-match"]).toBeUndefined();
    expect(result.code).toBe(1);
    expect(result.stderr).toContain("PRECONDITION_REQUIRED: If-Match is required");
    expect(result.stderr).toContain("req-428");
  });
});

describe("errors", () => {
  it("reports an answer that is not JSON by its HTTP status", async () => {
    server = await startServer((_r, w) => {
      w.writeHead(502, { "Content-Type": "text/html" });
      w.end("<html>bad gateway</html>");
    });
    const result = await runCli(["get", "p1"], { baseUrl: server.url });
    expect(result.code).toBe(1);
    expect(result.stderr).toContain("HTTP 502");
  });
});
