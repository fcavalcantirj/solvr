import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { json, runCli, startServer, within } from "./harness.js";
import type { Recorded } from "./harness.js";

// The contract tests serve contract/openapi-examples.json (the recorded examples of the served
// OpenAPI document, each held to the running API by the backend) from a local server and run
// the CLI command of each operation against it: the request the command sends (method, path,
// query, headers, credential, body), what `--json` prints (the API's answer), what the human
// output shows, and how the command reports each recorded error.

const here = dirname(fileURLToPath(import.meta.url));
const fixturePath = join(here, "../../../../contract/openapi-examples.json");

const AGENT_KEY = "solvr_contract_agent_key";
const ROOM_TOKEN = "solvr_rt_contract_room_token";
const DEAD_TOKEN = "solvr_rt_contract_not_live";

interface ContractRequest {
  credential: string;
  path_params: Record<string, string>;
  query: Record<string, string>;
  headers: Record<string, string>;
  request_body: unknown;
  status: number;
  response_body: unknown;
}

interface ContractError extends ContractRequest {
  case: string;
}

interface ContractOperation extends ContractRequest {
  operation_id: string;
  method: string;
  path: string;
  response_media_type: string;
  errors?: ContractError[];
}

function loadContract(): ContractOperation[] {
  const fixture = JSON.parse(readFileSync(fixturePath, "utf8")) as { operations?: ContractOperation[] };
  if (!fixture.operations?.length) {
    throw new Error("the client contract lists no operation");
  }
  return fixture.operations;
}

const operations = loadContract();
// getReply answers the ETag the updateReply example sends back as If-Match (the read before the edit).
const etag = operations.find((op) => op.operation_id === "updateReply")?.headers["If-Match"] ?? "";

/** One example as the CLI user types it. */
class Call {
  constructor(readonly op: ContractOperation, readonly req: ContractRequest) {}

  path(name: string): string {
    const value = this.req.path_params[name];
    if (value === undefined) {
      throw new Error(`${this.op.operation_id}: the example has no path parameter ${name}`);
    }
    return value;
  }

  /** The option with the value of the example's query parameter, or nothing when it has none. */
  query(option: string, name: string): string[] {
    const value = this.req.query[name];
    return value === undefined ? [] : [option, value];
  }

  header(option: string, name: string): string[] {
    const value = this.req.headers[name];
    return value === undefined ? [] : [option, value];
  }

  /**
   * The options that carry the example's request body: each field becomes the option the
   * command declares for it. A field the command has no option for fails the test.
   */
  body(options: Record<string, string>): string[] {
    const body = this.req.request_body;
    if (body === null || body === undefined) return [];
    const args: string[] = [];
    for (const [field, value] of Object.entries(body as Record<string, unknown>)) {
      const option = options[field];
      if (!option) {
        throw new Error(`${this.op.operation_id}: the command has no option for the request field ${field}`);
      }
      if (value === true) args.push(option);
      else if (Array.isArray(value)) args.push(option, value.join(","));
      else if (typeof value === "string" || typeof value === "number") args.push(option, String(value));
      else throw new Error(`${this.op.operation_id}: no option form for ${field}=${JSON.stringify(value)}`);
    }
    return args;
  }
}

interface Command {
  /** The command line of the example (credentials excluded). */
  args: (x: Call) => string[];
  /** What the human output shows of the answer. */
  shows: (answer: Answer) => string[];
}

type Answer = { data: Record<string, unknown> & Array<Record<string, unknown>> };

const slug = (x: Call) => x.path("slug");

// One command per operationId. Rooms: create, join, read, send, ticket, watch.
const commands: Record<string, Command> = {
  createPost: {
    args: (x) => ["post", ...x.body({ title: "--title", description: "--description", tags: "--tags", visibility: "--visibility" })],
    shows: (a) => [String(a.data.id)],
  },
  getPost: { args: (x) => ["get", x.path("id")], shows: (a) => [String(a.data.id), String(a.data.title)] },
  search: {
    args: (x) => [
      "search", x.req.query.q ?? "",
      ...x.query("--limit", "per_page"), ...x.query("--page", "page"), ...x.query("--sort", "sort"),
    ],
    shows: (a) => [String(a.data[0].id), String(a.data[0].title)],
  },
  createReply: {
    args: (x) => ["reply", x.path("id"), ...x.body({ body: "--body", parent_reply_id: "--parent" })],
    shows: (a) => [String(a.data.id)],
  },
  listReplies: {
    args: (x) => ["replies", x.path("id"), ...x.query("--limit", "limit"), ...x.query("--cursor", "cursor")],
    shows: (a) => [String(a.data[0].id), String(a.data[0].body)],
  },
  getReply: { args: (x) => ["get-reply", x.path("id")], shows: (a) => [String(a.data.id), String(a.data.body), etag] },
  updateReply: {
    args: (x) => ["update-reply", x.path("id"), ...x.header("--if-match", "If-Match"), ...x.body({ body: "--body" })],
    shows: (a) => [String(a.data.id)],
  },
  createRoom: {
    args: (x) => [
      "room", "create",
      ...x.body({ display_name: "--display-name", slug: "--slug", description: "--description", tags: "--tags", is_private: "--private" }),
    ],
    shows: (a) => [String(a.data.slug)],
  },
  handshakeRoom: {
    args: (x) => ["room", "join", slug(x), ...x.body({ rotate: "--rotate", ttl_seconds: "--ttl" })],
    shows: (a) => [String(a.data.room_token)],
  },
  listRoomEntries: {
    args: (x) => [
      "room", "read", slug(x),
      ...x.query("--limit", "limit"), ...x.query("--cursor", "cursor"), ...x.query("--kind", "kind"), ...x.query("--issue", "issue"),
    ],
    shows: (a) => [String(a.data[0].body)],
  },
  createRoomEntry: {
    args: (x) => [
      "room", "send", slug(x),
      ...x.body({ body: "--body", client_entry_id: "--client-entry-id", reply_to_entry_id: "--reply-to", addressed_member_ids: "--to" }),
    ],
    shows: (a) => [String(a.data.id)],
  },
  createRoomStreamTicket: { args: (x) => ["room", "ticket", slug(x)], shows: (a) => [String(a.data.ticket)] },
  streamRoom: {
    args: (x) => [
      "room", "watch", slug(x),
      ...x.header("--last-event-id", "Last-Event-ID"), ...x.query("--ticket", "ticket"),
      ...x.query("--type", "type"), ...x.query("--issue", "issue"),
    ],
    shows: () => [String((firstFrame().payload as { content: string }).content)],
  },
};

/** The credential options and the API key of a run, and the Authorization header the CLI must send. */
function credentialFor(op: ContractOperation, credential: string): { args: string[]; apiKey?: string; auth?: string } {
  switch (credential) {
    case "none":
      return { args: [] };
    case "agent_api_key":
      return { args: [], apiKey: AGENT_KEY, auth: `Bearer ${AGENT_KEY}` };
    case "room_token":
      return { args: ["--room-token", ROOM_TOKEN], apiKey: AGENT_KEY, auth: `Bearer ${ROOM_TOKEN}` };
    case "invalid":
      if (op.credential !== "room_token") {
        throw new Error(`${op.operation_id}: an invalid ${op.credential} credential has no CLI case yet`);
      }
      return { args: ["--room-token", DEAD_TOKEN], apiKey: AGENT_KEY, auth: `Bearer ${DEAD_TOKEN}` };
  }
  throw new Error(`${op.operation_id}: unknown credential ${credential}`);
}

/** Serves the example's answer for every request. */
async function serve(op: ContractOperation, req: ContractRequest) {
  return startServer((_r, w) => {
    if (req.status < 400 && op.response_media_type === "text/event-stream") {
      w.writeHead(req.status, { "Content-Type": "text/event-stream" });
      w.end(req.response_body as string);
      return;
    }
    json(w, req.status, req.response_body, op.operation_id === "getReply" || op.operation_id === "updateReply" ? { ETag: etag } : {});
  });
}

async function runExample(op: ContractOperation, req: ContractRequest, mode: "json" | "human") {
  const command = commands[op.operation_id];
  const cred = credentialFor(op, req.credential);
  const server = await serve(op, req);
  try {
    const args = [...command.args(new Call(op, req)), ...cred.args, ...(mode === "json" ? ["--json"] : [])];
    const result = await within(10_000, runCli(args, { baseUrl: server.url, apiKey: cred.apiKey }));
    return { result, sent: server.sent, auth: cred.auth, args };
  } finally {
    await server.close();
  }
}

function expectedPath(op: ContractOperation, params: Record<string, string>): string {
  let p = op.path;
  for (const [name, value] of Object.entries(params)) {
    p = p.replaceAll(`{${name}}`, encodeURIComponent(value));
  }
  if (p.includes("{")) {
    throw new Error(`${op.operation_id}: path ${p} keeps a variable the example gives no value`);
  }
  return p;
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

/**
 * Compares two JSON values. exact also fails on a field only got has; either way a field the
 * example has must be in got with the same value (null and absent agree).
 */
function jsonDiff(want: unknown, got: unknown, exact: boolean, at = "$"): string {
  if (isObject(want)) {
    if (!isObject(got)) return `${at}: want an object, got ${JSON.stringify(got)}`;
    for (const key of Object.keys(want).sort()) {
      if (want[key] === null && (got[key] === null || got[key] === undefined)) continue;
      if (got[key] === undefined) return `${at}.${key}: the example has it, the CLI lost it`;
      const diff = jsonDiff(want[key], got[key], exact, `${at}.${key}`);
      if (diff) return diff;
    }
    if (exact) {
      for (const key of Object.keys(got)) {
        if (!(key in want) && got[key] !== null && got[key] !== undefined) {
          return `${at}.${key}: the CLI sent it, the example does not`;
        }
      }
    }
    return "";
  }
  if (Array.isArray(want)) {
    if (!Array.isArray(got) || got.length !== want.length) {
      return `${at}: want ${want.length} items, got ${JSON.stringify(got)}`;
    }
    for (let i = 0; i < want.length; i++) {
      const diff = jsonDiff(want[i], got[i], exact, `${at}[${i}]`);
      if (diff) return diff;
    }
    return "";
  }
  return Object.is(want, got) ? "" : `${at}: want ${JSON.stringify(want)}, got ${JSON.stringify(got)}`;
}

/** Holds what the CLI sent to the example. */
function checkRequest(op: ContractOperation, req: ContractRequest, sent: Recorded[], auth?: string) {
  const id = op.operation_id;
  expect(sent, `${id}: requests sent`).toHaveLength(1);
  const got = sent[0];
  expect(got.method, `${id}: method`).toBe(op.method);
  expect(got.path, `${id}: path`).toBe(expectedPath(op, req.path_params));
  expect(got.headers.authorization, `${id}: Authorization`).toBe(auth);
  const names = got.query.map(([name]) => name);
  expect(new Set(names).size, `${id}: a query parameter sent twice: ${names}`).toBe(names.length);
  expect(Object.fromEntries(got.query), `${id}: query`).toEqual(req.query);
  for (const [name, value] of Object.entries(req.headers)) {
    expect(got.headers[name.toLowerCase()], `${id}: header ${name}`).toBe(value);
  }
  for (const name of ["If-Match", "Last-Event-ID"]) {
    if (!(name in req.headers)) {
      expect(got.headers[name.toLowerCase()], `${id}: sent ${name} the example does not`).toBeUndefined();
    }
  }
  if (req.request_body === null || req.request_body === undefined) {
    expect(got.body, `${id}: sent a body the example does not`).toBe("");
    return;
  }
  const diff = got.body === "" ? "$: the CLI sent no body" : jsonDiff(req.request_body, JSON.parse(got.body), true);
  expect(diff, `${id}: request body ${got.body}; example ${JSON.stringify(req.request_body)}`).toBe("");
}

interface StreamFrameText {
  id?: string;
  event?: string;
  data: string;
}

/** The first event of the streamRoom example. */
function firstFrameText(): StreamFrameText {
  const op = operations.find((o) => o.operation_id === "streamRoom");
  const frame: StreamFrameText = { data: "" };
  for (const line of String(op?.response_body ?? "").split("\n")) {
    if (line.startsWith("id: ")) frame.id = line.slice(4);
    else if (line.startsWith("event: ")) frame.event = line.slice(7);
    else if (line.startsWith("data: ")) frame.data = line.slice(6);
    else if (line === "" && frame.data) break;
  }
  return frame;
}

function firstFrame(): { payload?: unknown } {
  return JSON.parse(firstFrameText().data) as { payload?: unknown };
}

/** Holds what `--json` printed to the example's answer. */
function checkJsonOutput(op: ContractOperation, stdout: string) {
  const id = op.operation_id;
  if (op.response_media_type === "text/event-stream") {
    const lines = stdout.split("\n").filter((l) => l.trim() !== "");
    expect(lines, `${id}: one line per stream event`).toHaveLength(1);
    const event = JSON.parse(lines[0]) as { id?: string; event?: string; frame?: unknown };
    const want = firstFrameText();
    expect({ id: event.id, event: event.event }, `${id}: event id and name`).toEqual({ id: want.id, event: want.event });
    expect(jsonDiff(JSON.parse(want.data), event.frame, false), `${id}: the printed frame`).toBe("");
    return;
  }
  const printed = JSON.parse(stdout) as unknown;
  expect(jsonDiff(op.response_body, printed, false), `${id}: --json printed ${stdout}`).toBe("");
  if (id === "getReply" || id === "updateReply") {
    expect((printed as { data: { etag?: string } }).data.etag, `${id}: the ETag the edit sends back`).toBe(etag);
  }
}

describe("CLI contract (contract/openapi-examples.json)", () => {
  it("has a command for every operation of the contract", () => {
    const ids = operations.map((op) => op.operation_id).sort();
    expect(Object.keys(commands).sort()).toEqual(ids);
  });

  for (const op of operations) {
    it(`${op.operation_id}: --json sends the example's request and prints the API's answer`, async () => {
      const { result, sent, auth, args } = await runExample(op, op, "json");
      expect(result.stderr, `${op.operation_id}: solvr ${args.join(" ")}`).toBe("");
      expect(result.code, `${op.operation_id}: exit code`).toBe(0);
      checkRequest(op, op, sent, auth);
      checkJsonOutput(op, result.stdout);
    });

    it(`${op.operation_id}: the human output shows the answer`, async () => {
      const { result, sent, auth, args } = await runExample(op, op, "human");
      expect(result.stderr, `${op.operation_id}: solvr ${args.join(" ")}`).toBe("");
      expect(result.code, `${op.operation_id}: exit code`).toBe(0);
      checkRequest(op, op, sent, auth);
      for (const shown of commands[op.operation_id].shows(op.response_body as Answer)) {
        expect(result.stdout, `${op.operation_id}: human output`).toContain(shown);
      }
    });

    for (const error of op.errors ?? []) {
      it(`${op.operation_id}: reports ${error.status} (${error.case})`, async () => {
        const answer = error.response_body as { error: { code: string; message: string; request_id?: string } };

        const asJson = await runExample(op, error, "json");
        checkRequest(op, error, asJson.sent, asJson.auth);
        expect(asJson.result.code, `${op.operation_id}: exit code`).toBe(1);
        expect(asJson.result.stdout, `${op.operation_id}: nothing on stdout`).toBe("");
        const printed = JSON.parse(asJson.result.stderr) as unknown;
        expect(jsonDiff(error.response_body, printed, true), `${op.operation_id}: --json error ${asJson.result.stderr}`).toBe("");

        const human = await runExample(op, error, "human");
        expect(human.result.code, `${op.operation_id}: exit code`).toBe(1);
        expect(human.result.stderr).toContain(`${answer.error.code}: ${answer.error.message}`);
        if (answer.error.request_id) {
          expect(human.result.stderr).toContain(answer.error.request_id);
        }
      });
    }
  }

  it("records error examples to hold the CLI to", () => {
    expect(operations.flatMap((op) => op.errors ?? []).length).toBeGreaterThan(0);
  });
});
