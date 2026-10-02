import { describe, it, expect, afterEach } from "vitest";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { ApiClient } from "../api.js";
import { freshConfigPath, json, runCli, startServer } from "./harness.js";
import type { LocalServer } from "./harness.js";

// A room's participants are a collection its owner reads (`room members`, listRoomMembers) and
// adds to (`room add-member`, addRoomMember) with the API key: a third and any later agent is
// admitted to the SAME room, then joins it with its own `room join`.

const KEY = "solvr_sk_members_test";
const SLUG = "parser-build";

const member = (agent_id: string, role: string, added_by: string) => ({
  room_id: "3f2c9d1e-6a7b-4c8d-9e0f-112233445566", agent_id, role, added_by, created_at: "2026-10-02T12:00:00Z",
});
const participants = {
  data: [
    member("agent_planner", "owner", "system"),
    member("agent_executor", "member", "agent_planner"),
    member("agent_reviewer", "member", "agent_planner"),
  ],
};
const refusal = (code: string, message: string) => ({ error: { code, message, request_id: `req-${code}` } });

let server: LocalServer | undefined;
afterEach(async () => {
  await server?.close();
  server = undefined;
});

describe("room members", () => {
  it("reads the room's participants with the API key and shows them in the order the API answered", async () => {
    server = await startServer((_r, w) => json(w, 200, participants));
    const result = await runCli(["room", "members", "a b/c"], { baseUrl: server.url, apiKey: KEY });

    expect(result.code).toBe(0);
    expect(server.sent).toHaveLength(1);
    expect(server.sent[0].method).toBe("GET");
    expect(server.sent[0].path).toBe("/v1/rooms/a%20b%2Fc/members");
    expect(server.sent[0].query).toEqual([]);
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${KEY}`);
    expect(server.sent[0].body).toBe("");
    const lines = result.stdout.split("\n");
    const row = (agent: string) => lines.findIndex((l) => l.includes(agent));
    expect(lines[row("agent_planner")]).toMatch(/agent_planner\s+owner\s+system/);
    expect(lines[row("agent_executor")]).toMatch(/agent_executor\s+member\s+agent_planner/);
    expect(row("agent_planner")).toBeLessThan(row("agent_executor"));
    expect(row("agent_executor")).toBeLessThan(row("agent_reviewer"));
  });

  it("prints the API's answer with --json", async () => {
    server = await startServer((_r, w) => json(w, 200, participants));
    const result = await runCli(["room", "members", SLUG, "--json"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(JSON.parse(result.stdout)).toEqual(participants);
  });
});

describe("room add-member", () => {
  it("admits an agent to the same room: without --role the body is the agent id alone", async () => {
    server = await startServer((_r, w) => json(w, 201, { data: member("agent_reviewer", "member", "agent_planner") }));
    const result = await runCli(["room", "add-member", "a b/c", "agent_reviewer"], { baseUrl: server.url, apiKey: KEY });

    expect(result.code).toBe(0);
    expect(server.sent).toHaveLength(1);
    expect(server.sent[0].method).toBe("POST");
    expect(server.sent[0].path).toBe("/v1/rooms/a%20b%2Fc/members");
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${KEY}`);
    expect(JSON.parse(server.sent[0].body)).toEqual({ agent_id: "agent_reviewer" });
    expect(result.stdout).toContain("agent_reviewer");
    expect(result.stdout).toContain("member");
    expect(result.stdout).toContain("agent_planner");
    expect(result.stdout).toContain("solvr room join a b/c");
  });

  it("sends --role as given", async () => {
    server = await startServer((_r, w) => json(w, 201, { data: member("agent_reviewer", "owner", "agent_planner") }));
    const result = await runCli(["room", "add-member", SLUG, "agent_reviewer", "--role", "owner"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(JSON.parse(server.sent[0].body)).toEqual({ agent_id: "agent_reviewer", role: "owner" });
    expect(result.stdout).toContain("owner");
  });

  it("leaves the role to the API: a role it does not know is sent, and its refusal reported", async () => {
    server = await startServer((_r, w) => json(w, 400, refusal("VALIDATION_ERROR", "role must be owner or member")));
    const result = await runCli(["room", "add-member", SLUG, "agent_reviewer", "--role", "admin"], { baseUrl: server.url, apiKey: KEY });
    expect(JSON.parse(server.sent[0].body)).toEqual({ agent_id: "agent_reviewer", role: "admin" });
    expect(result.code).toBe(1);
    expect(result.stderr).toContain("VALIDATION_ERROR: role must be owner or member");
  });

  it("prints the API's answer with --json", async () => {
    const answer = { data: member("agent_reviewer", "member", "agent_planner") };
    server = await startServer((_r, w) => json(w, 201, answer));
    const result = await runCli(["room", "add-member", SLUG, "agent_reviewer", "--json"], { baseUrl: server.url, apiKey: KEY });
    expect(result.code).toBe(0);
    expect(JSON.parse(result.stdout)).toEqual(answer);
  });
});

describe("credential of the member commands", () => {
  const commands = [["room", "members", SLUG], ["room", "add-member", SLUG, "agent_reviewer"]];

  it("refuses without an API key, before any request", async () => {
    server = await startServer((_r, w) => json(w, 500, {}));
    for (const args of commands) {
      const result = await runCli(args, { baseUrl: server.url });
      expect(result.code, args.join(" ")).toBe(1);
      expect(result.stderr).toContain("No API key configured");
    }
    expect(server.sent).toHaveLength(0);
  });

  it("presents the API key, not the room token saved for the room", async () => {
    server = await startServer((r, w) => json(w, r.method === "GET" ? 200 : 201, r.method === "GET" ? participants : { data: participants.data[2] }));
    const configPath = freshConfigPath();
    writeFileSync(configPath, JSON.stringify({ roomTokens: { [SLUG]: "solvr_rt_saved" } }));
    for (const args of commands) {
      expect((await runCli(args, { baseUrl: server.url, apiKey: KEY, configPath })).code, args.join(" ")).toBe(0);
    }
    expect(server.sent.map((s) => s.headers.authorization)).toEqual([`Bearer ${KEY}`, `Bearer ${KEY}`]);
  });
});

describe("errors of the member commands", () => {
  const cases: [string, string[], number, string, string][] = [
    ["a caller that does not own the room", ["room", "members", SLUG], 403, "FORBIDDEN", "only a room owner may list its members"],
    ["an agent id that names no agent", ["room", "add-member", SLUG, "agent_nobody"], 400, "INVALID_AGENT", "agent_id names no agent"],
    ["demoting the last owner", ["room", "add-member", SLUG, "agent_planner", "--role", "member"], 409, "LAST_OWNER", "a room must keep an owner"],
  ];

  for (const [name, args, status, code, message] of cases) {
    it(`reports ${status} ${code} (${name}), sent once`, async () => {
      server = await startServer((_r, w) => json(w, status, refusal(code, message)));
      const human = await runCli(args, { baseUrl: server.url, apiKey: KEY });
      expect(human.code).toBe(1);
      expect(human.stderr).toContain(`${code}: ${message}`);
      expect(human.stderr).toContain(`req-${code}`);
      expect(server.sent).toHaveLength(1);

      const asJson = await runCli([...args, "--json"], { baseUrl: server.url, apiKey: KEY });
      expect(asJson.code).toBe(1);
      expect(asJson.stdout).toBe("");
      expect(JSON.parse(asJson.stderr)).toEqual(refusal(code, message));
      expect(server.sent).toHaveLength(2);
    });
  }
});

describe("room help", () => {
  it("lists the member commands with the room's other commands", async () => {
    const result = await runCli(["room", "--help"], { baseUrl: "http://127.0.0.1:1" });
    expect(result.code).toBe(0);
    expect(result.stdout).toMatch(/^  members <slug>/m);
    expect(result.stdout).toMatch(/^  add-member \[options\] <slug> <agentId>/m);
  });
});

describe("ApiClient member methods", () => {
  it("are named after the operationIds and answer the API's answer", async () => {
    server = await startServer((r, w) => json(w, r.method === "GET" ? 200 : 201, r.method === "GET" ? participants : { data: participants.data[1] }));
    const client = new ApiClient(KEY, server.url);
    expect(await client.listRoomMembers(SLUG)).toEqual(participants);
    expect(await client.addRoomMember(SLUG, { agent_id: "agent_executor" })).toEqual({ data: participants.data[1] });
    expect(server.sent.map((s) => [s.method, s.path, s.body])).toEqual([
      ["GET", `/v1/rooms/${SLUG}/members`, ""],
      ["POST", `/v1/rooms/${SLUG}/members`, JSON.stringify({ agent_id: "agent_executor" })],
    ]);
  });
});

// The member types carry the fields of the schemas the API publishes (backend
// openapi_member_paths.go RoomMember and AddRoomMemberRequest, pinned there to the stored model).
describe("member types", () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const source = ts.createSourceFile("room-types.ts", readFileSync(join(here, "../room-types.ts"), "utf8"), ts.ScriptTarget.ES2022);

  function declared(name: string): ts.InterfaceDeclaration | ts.TypeAliasDeclaration {
    const found = source.statements.find(
      (s) => (ts.isInterfaceDeclaration(s) || ts.isTypeAliasDeclaration(s)) && s.name.text === name
    );
    if (!found || !(ts.isInterfaceDeclaration(found) || ts.isTypeAliasDeclaration(found))) {
      throw new Error(`room-types.ts declares no ${name}`);
    }
    return found;
  }

  /** "name" or "name?" for each property of an interface, sorted. */
  function properties(name: string): string[] {
    const decl = declared(name);
    if (!ts.isInterfaceDeclaration(decl)) throw new Error(`${name} is not an interface`);
    return decl.members.map((m) => `${m.name?.getText(source)}${m.questionToken ? "?" : ""}`).sort();
  }

  it("RoomMember has every field of the published RoomMember, each required", () => {
    expect(properties("RoomMember")).toEqual(["added_by", "agent_id", "created_at", "role", "room_id"]);
  });

  it("AddRoomMemberInput is agent_id and an optional role", () => {
    expect(properties("AddRoomMemberInput")).toEqual(["agent_id", "role?"]);
  });

  it("a role is owner or member", () => {
    const decl = declared("RoomRole");
    if (!ts.isTypeAliasDeclaration(decl) || !ts.isUnionTypeNode(decl.type)) throw new Error("RoomRole is not a union");
    expect(decl.type.types.map((t) => t.getText(source)).sort()).toEqual(['"member"', '"owner"']);
  });

  it("the answers are the published envelopes", () => {
    expect(properties("RoomMemberResponse")).toEqual(["data"]);
    expect(properties("RoomMembersResponse")).toEqual(["data"]);
  });
});
