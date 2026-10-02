import { describe, it, expect, afterEach } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { ApiClient } from "../api.js";
import { createProgram } from "../program.js";
import { VERSION } from "../version.js";
import { json, runCli, startServer } from "./harness.js";
import type { LocalServer } from "./harness.js";

// @solvr/cli 2.0.0 removed the 1.x choices of the legacy knowledge model: a post has no type
// or success criteria, every contribution is a reply, a post's replies are read on their own,
// and search covers every post. A removed command, argument or option still parses, so using
// it fails before any request, naming what replaces it and where the migration notes are.

const read = (p: string) => readFileSync(resolve(__dirname, p), "utf8");
const pkg = JSON.parse(read("../../package.json")) as { version: string };
const KEY = "solvr_sk_migration_test";

// The "Migrating from 1.x to <VERSION>" section of the README.
function migrationNotes(): string {
  const heading = `## Migrating from 1.x to ${VERSION}`;
  const readme = read("../../README.md");
  const start = readme.indexOf(heading);
  if (start < 0) throw new Error(`README has no "${heading}" section`);
  const rest = readme.slice(start + heading.length);
  const next = rest.search(/\n## /);
  return next < 0 ? rest : rest.slice(0, next);
}

// The 1.x surface 2.0.0 removed (git show 0eafb8a0:packages/cli/src/index.ts):
// [the name the error and the notes give it, a 1.x invocation]
const REMOVED: [string, string[]][] = [
  ["solvr answer", ["answer", "post_1", "--content", "Use a mutex"]],
  ["solvr approach", ["approach", "post_1", "--angle", "Retry", "--method", "Backoff", "--assumptions", "a,b"]],
  ["solvr post <type>", ["post", "problem", "--title", "Race in the pool", "--description", "Two workers"]],
  ["--criteria", ["post", "--title", "Race in the pool", "--description", "Two workers", "--criteria", "No race"]],
  ["--include", ["get", "post_1", "--include", "approaches,answers"]],
  ["--include", ["get", "post_1", "-i", "answers"]],
  ["--type", ["search", "pool race", "--type", "problem"]],
  ["--type", ["search", "pool race", "-t", "all"]],
  ["--status", ["search", "pool race", "--status", "open"]],
  ["--status", ["search", "pool race", "-s", "solved"]],
];

let server: LocalServer | undefined;
afterEach(async () => {
  await server?.close();
  server = undefined;
});

describe("@solvr/cli 2.0.0 removed the legacy 1.x choices", () => {
  it.each(REMOVED)("'%s' fails before any request and points at the migration notes", async (name, args) => {
    server = await startServer((_r, w) => json(w, 201, { data: { id: "should-not-be-called" } }));

    for (const asJson of [false, true]) {
      const res = await runCli(asJson ? [...args, "--json"] : args, { baseUrl: server.url, apiKey: KEY });
      expect(res.code).toBe(1);
      expect(res.stderr).toContain(`'${name}' was removed in @solvr/cli ${VERSION}`);
      expect(res.stderr).toContain(`"Migrating from 1.x to ${VERSION}" in the README`);
      expect(res.stdout).toBe("");
    }
    expect(server.sent).toHaveLength(0);
  });

  it("names the refused post type", async () => {
    const res = await runCli(["post", "idea", "--title", "T", "--description", "D"], { baseUrl: "http://127.0.0.1:9", apiKey: KEY });
    expect(res.code).toBe(1);
    expect(res.stderr).toContain('"idea" is not accepted');
  });

  it("refuses a removed choice before asking for an API key", async () => {
    const res = await runCli(["post", "question", "--title", "T", "--description", "D"], { baseUrl: "http://127.0.0.1:9" });
    expect(res.code).toBe(1);
    expect(res.stderr).toContain(`'solvr post <type>' was removed`);
    expect(res.stderr).not.toContain("No API key configured");
  });

  it("search, post and get still run with their 2.0.0 options", async () => {
    server = await startServer((r, w) =>
      r.path === "/v1/search"
        ? json(w, 200, { data: [], meta: { query: "pool race", total: 0, page: 2, per_page: 5, has_more: false } })
        : json(w, r.method === "POST" ? 201 : 200, { data: { id: "p1", title: "T", description: "D", type: "post", status: "open", upvotes: 0, downvotes: 0, created_at: "2026-10-01T00:00:00Z" } })
    );
    const env = { baseUrl: server.url, apiKey: KEY };

    expect((await runCli(["search", "pool race", "--limit", "5", "--page", "2", "--sort", "newest"], env)).code).toBe(0);
    expect((await runCli(["post", "--title", "T", "--description", "D", "--tags", "go"], env)).code).toBe(0);
    expect((await runCli(["get", "p1"], env)).code).toBe(0);

    expect(server.sent.map((r) => [r.method, r.path])).toEqual([
      ["GET", "/v1/search"],
      ["POST", "/v1/posts"],
      ["GET", "/v1/posts/p1"],
    ]);
    expect(server.sent[0].query).toEqual([["q", "pool race"], ["per_page", "5"], ["page", "2"], ["sort", "newest"]]);
    expect(JSON.parse(server.sent[1].body)).toEqual({ title: "T", description: "D", tags: ["go"] });
  });

  it("--help lists none of the removed commands or options", async () => {
    const help = async (...args: string[]) => (await runCli([...args, "--help"], { baseUrl: "http://127.0.0.1:9" })).stdout;

    const root = await help();
    expect(root).not.toMatch(/^\s+answer\b/m);
    expect(root).not.toMatch(/^\s+approach\b/m);
    for (const option of ["--type", "-t,", "--status", "-s,"]) expect(await help("search")).not.toContain(option);
    for (const option of ["--include", "-i,"]) expect(await help("get")).not.toContain(option);
    const post = await help("post");
    expect(post).not.toContain("--criteria");
    expect(post).not.toContain("<type>");
  });

  it("the API client sends no legacy search filter", async () => {
    server = await startServer((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
    const client = new ApiClient(null, server.url);

    await client.search("pool race", { type: "problem", status: "open", limit: 5 } as never);

    expect(server.sent[0].query).toEqual([["q", "pool race"], ["per_page", "5"]]);
    const options = read("../api.ts").match(/interface SearchOptions \{([^}]*)\}/);
    expect(options?.[1]).not.toMatch(/^\s*(type|status)\??:/m);
  });

  it("VERSION is 2.0.0, the package version, and what --version prints", async () => {
    expect(VERSION).toBe("2.0.0");
    expect(pkg.version).toBe(VERSION);
    const res = await runCli(["--version"], { baseUrl: "http://127.0.0.1:9" });
    expect(res.stdout.trim()).toBe(VERSION);
  });

  it("the README migration notes name every removed choice and what replaces it", () => {
    const notes = migrationNotes();
    for (const name of new Set(REMOVED.map(([n]) => n))) {
      expect(notes, `the notes do not name ${name}`).toContain(`\`${name}`);
    }
    for (const short of ["-i", "-t", "-s"]) expect(notes, `the notes do not name ${short}`).toContain(`\`${short}\``);
    for (const use of ["ENDPOINT_RETIRED", "details.replacement", "--json"]) {
      expect(notes, `the notes do not name ${use}`).toContain(use);
    }

    // Every command the notes point to, other than the removed ones, is a 2.0.0 command.
    const commands = new Set(createProgram().commands.filter((c) => !(c as unknown as { _hidden: boolean })._hidden).map((c) => c.name()));
    const named = [...notes.matchAll(/`solvr ([a-z-]+)/g)].map((m) => m[1]);
    expect(named).toContain("reply");
    expect(named).toContain("replies");
    for (const command of named.filter((c) => c !== "answer" && c !== "approach")) {
      expect(commands, `the notes point to solvr ${command}`).toContain(command);
    }
  });
});
