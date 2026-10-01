import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { OPERATION_TOOLS, SolvrTools } from '../tools.js';
import type { ToolDefinition } from '../tools.js';
import { callTool, json, startServer, text } from './harness.js';
import type { Recorded } from './harness.js';

// The contract tests serve contract/openapi-examples.json (the recorded examples of the served
// OpenAPI document, each held to the running API by the backend) from a local server and call
// the MCP tool of each operation through the server's JSON-RPC handler: the request the tool
// sends (method, path, query, headers, credential, body), what its result shows of the answer,
// and how it reports each recorded error (code, message, request id).

const here = dirname(fileURLToPath(import.meta.url));
const fixturePath = join(here, '../../../contract/openapi-examples.json');

const AGENT_KEY = 'solvr_contract_agent_key';
const ROOM_TOKEN = 'solvr_rt_contract_room_token';
const DEAD_TOKEN = 'solvr_rt_contract_not_live';

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
  const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { operations?: ContractOperation[] };
  if (!fixture.operations?.length) {
    throw new Error('the client contract lists no operation');
  }
  return fixture.operations;
}

const operations = loadContract();
const operation = (id: string) => {
  const op = operations.find((o) => o.operation_id === id);
  if (!op) throw new Error(`the contract has no ${id} operation`);
  return op;
};
// getReply answers the ETag the updateReply example sends back as If-Match (the read before the edit).
const etag = operation('updateReply').headers['If-Match'];

const manifest = new SolvrTools(null, 'http://127.0.0.1:1').getManifest();

type Answer = { data: Record<string, unknown> & Array<Record<string, unknown>> };

/** How the tool of an operation takes the example: which argument carries each path, query and header value. */
interface ToolCall {
  tool: string;
  path?: Record<string, string>;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  /** What the result shows of the answer. */
  shows: (answer: Answer) => string[];
}

const slug = { slug: 'slug' };

// One tool per operationId. Rooms: create, join, read, send, ticket, watch.
const calls: Record<string, ToolCall> = {
  createPost: { tool: 'solvr_post', shows: (a) => [String(a.data.id), String(a.data.title)] },
  getPost: { tool: 'solvr_get', path: { id: 'id' }, shows: (a) => [String(a.data.id), String(a.data.title)] },
  search: {
    tool: 'solvr_search',
    query: { q: 'query', per_page: 'limit', page: 'page', sort: 'sort' },
    shows: (a) => [String(a.data[0].id), String(a.data[0].title)],
  },
  createReply: { tool: 'solvr_reply', path: { id: 'post_id' }, shows: (a) => [String(a.data.id)] },
  listReplies: {
    tool: 'solvr_replies',
    path: { id: 'post_id' },
    query: { limit: 'limit', cursor: 'cursor' },
    shows: (a) => [String(a.data[0].id), String(a.data[0].body)],
  },
  getReply: { tool: 'solvr_get_reply', path: { id: 'id' }, shows: (a) => [String(a.data.id), String(a.data.body), etag] },
  updateReply: {
    tool: 'solvr_update_reply',
    path: { id: 'id' },
    headers: { 'If-Match': 'if_match' },
    shows: (a) => [String(a.data.id), etag],
  },
  createRoom: { tool: 'solvr_room_create', shows: (a) => [String(a.data.slug)] },
  handshakeRoom: { tool: 'solvr_room_join', path: slug, shows: (a) => [String(a.data.room_token)] },
  listRoomEntries: {
    tool: 'solvr_room_read',
    path: slug,
    query: { limit: 'limit', cursor: 'cursor', kind: 'kind', issue: 'issue' },
    shows: (a) => [String(a.data[0].body), String(a.data[0].actor_label)],
  },
  createRoomEntry: { tool: 'solvr_room_send', path: slug, shows: (a) => [String(a.data.id)] },
  createRoomStreamTicket: { tool: 'solvr_room_ticket', path: slug, shows: (a) => [String(a.data.ticket)] },
  streamRoom: {
    tool: 'solvr_room_watch',
    path: slug,
    query: { ticket: 'ticket', type: 'event_type', issue: 'issue' },
    headers: { 'Last-Event-ID': 'last_event_id' },
    shows: () => {
      const frame = firstFrameText();
      return [String((JSON.parse(frame.data) as { payload: { content: string } }).payload.content), String(frame.id)];
    },
  },
};

function toolDefinition(name: string): ToolDefinition {
  const tool = manifest.tools.find((t) => t.name === name);
  if (!tool) throw new Error(`tools/list has no ${name}`);
  return tool;
}

/** The argument value of a string the example carries, typed as the tool declares the argument. */
function argument(tool: ToolDefinition, name: string, value: unknown): unknown {
  const declared = tool.inputSchema.properties[name];
  if (!declared) throw new Error(`${tool.name} declares no argument ${name}`);
  if (declared.type === 'number' && typeof value === 'string') return Number(value);
  return value;
}

/**
 * The arguments of the example: each path, query and header value goes to the argument the
 * call maps it to, and each request body field to the argument of the same name. A value the
 * tool has no argument for fails the test.
 */
function argumentsOf(op: ContractOperation, req: ContractRequest): Record<string, unknown> {
  const call = calls[op.operation_id];
  const tool = toolDefinition(call.tool);
  const args: Record<string, unknown> = {};
  const map = (kind: string, values: Record<string, string>, names: Record<string, string> = {}) => {
    for (const [name, value] of Object.entries(values)) {
      const arg = names[name];
      if (!arg) throw new Error(`${op.operation_id}: no argument carries the ${kind} ${name}`);
      args[arg] = argument(tool, arg, value);
    }
  };
  map('path parameter', req.path_params, call.path);
  map('query parameter', req.query, call.query);
  map('header', req.headers, call.headers);
  const body = req.request_body as Record<string, unknown> | null;
  for (const [field, value] of Object.entries(body ?? {})) {
    args[field] = argument(tool, field, value);
  }
  if (op.operation_id === 'search' && args.query === undefined) {
    args.query = ''; // the tool requires query; the VALIDATION_ERROR example sends none
  }
  return args;
}

/** The API key of the run, the credential arguments, and the Authorization header the tool must send. */
function credentialFor(op: ContractOperation, credential: string): { apiKey: string | null; args: Record<string, string>; auth?: string } {
  switch (credential) {
    case 'none':
      return { apiKey: null, args: {} };
    case 'agent_api_key':
      return { apiKey: AGENT_KEY, args: {}, auth: `Bearer ${AGENT_KEY}` };
    case 'room_token':
      return { apiKey: AGENT_KEY, args: { room_token: ROOM_TOKEN }, auth: `Bearer ${ROOM_TOKEN}` };
    case 'invalid':
      if (op.credential !== 'room_token') {
        throw new Error(`${op.operation_id}: an invalid ${op.credential} credential has no MCP case yet`);
      }
      return { apiKey: AGENT_KEY, args: { room_token: DEAD_TOKEN }, auth: `Bearer ${DEAD_TOKEN}` };
  }
  throw new Error(`${op.operation_id}: unknown credential ${credential}`);
}

function expectedPath(op: ContractOperation, params: Record<string, string>): string {
  let p = op.path;
  for (const [name, value] of Object.entries(params)) {
    p = p.replaceAll(`{${name}}`, encodeURIComponent(value));
  }
  if (p.includes('{')) {
    throw new Error(`${op.operation_id}: path ${p} keeps a variable the example gives no value`);
  }
  return p;
}

/**
 * Serves the example's answer. solvr_get also reads the post's replies: that request gets the
 * listReplies example (or the error of an error example).
 */
async function serve(op: ContractOperation, req: ContractRequest) {
  const path = expectedPath(op, req.path_params);
  return startServer((r, w) => {
    if (req.status < 400 && op.operation_id === 'getPost' && r.path === `${path}/replies`) {
      json(w, 200, operation('listReplies').response_body);
      return;
    }
    if (req.status < 400 && op.response_media_type === 'text/event-stream') {
      w.writeHead(req.status, { 'Content-Type': 'text/event-stream' });
      w.end(req.response_body as string);
      return;
    }
    json(w, req.status, req.response_body, op.operation_id === 'getReply' || op.operation_id === 'updateReply' ? { ETag: etag } : {});
  });
}

async function runExample(op: ContractOperation, req: ContractRequest) {
  const call = calls[op.operation_id];
  const cred = credentialFor(op, req.credential);
  const server = await serve(op, req);
  try {
    const tools = new SolvrTools(cred.apiKey, server.url);
    const args = { ...argumentsOf(op, req), ...cred.args };
    const result = await callTool(tools, call.tool, args);
    return { result, sent: server.sent, auth: cred.auth, args };
  } finally {
    await server.close();
  }
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/**
 * Compares two JSON values. exact also fails on a field only got has; either way a field the
 * example has must be in got with the same value (null and absent agree).
 */
function jsonDiff(want: unknown, got: unknown, exact: boolean, at = '$'): string {
  if (isObject(want)) {
    if (!isObject(got)) return `${at}: want an object, got ${JSON.stringify(got)}`;
    for (const key of Object.keys(want).sort()) {
      if (want[key] === null && (got[key] === null || got[key] === undefined)) continue;
      if (got[key] === undefined) return `${at}.${key}: the example has it, the tool lost it`;
      const diff = jsonDiff(want[key], got[key], exact, `${at}.${key}`);
      if (diff) return diff;
    }
    if (exact) {
      for (const key of Object.keys(got)) {
        if (!(key in want) && got[key] !== null && got[key] !== undefined) {
          return `${at}.${key}: the tool sent it, the example does not`;
        }
      }
    }
    return '';
  }
  if (Array.isArray(want)) {
    if (!Array.isArray(got) || got.length !== want.length) {
      return `${at}: want ${want.length} items, got ${JSON.stringify(got)}`;
    }
    for (let i = 0; i < want.length; i++) {
      const diff = jsonDiff(want[i], got[i], exact, `${at}[${i}]`);
      if (diff) return diff;
    }
    return '';
  }
  return Object.is(want, got) ? '' : `${at}: want ${JSON.stringify(want)}, got ${JSON.stringify(got)}`;
}

/** Holds the request of the operation the tool sent to the example. */
function checkRequest(op: ContractOperation, req: ContractRequest, sent: Recorded[], auth?: string) {
  const id = op.operation_id;
  const path = expectedPath(op, req.path_params);
  // solvr_get sends the post's replies read beside it; every other tool sends one request.
  expect(sent.length, `${id}: requests sent ${sent.map((r) => `${r.method} ${r.path}`)}`).toBe(id === 'getPost' ? 2 : 1);
  const matching = sent.filter((r) => r.method === op.method && r.path === path);
  expect(matching, `${id}: ${op.method} ${path} among ${sent.map((r) => `${r.method} ${r.path}`)}`).toHaveLength(1);
  for (const got of sent) {
    expect(got.headers.authorization, `${id}: Authorization of ${got.method} ${got.path}`).toBe(auth);
  }
  const got = matching[0];
  const names = got.query.map(([name]) => name);
  expect(new Set(names).size, `${id}: a query parameter sent twice: ${names}`).toBe(names.length);
  expect(Object.fromEntries(got.query), `${id}: query`).toEqual(req.query);
  for (const [name, value] of Object.entries(req.headers)) {
    expect(got.headers[name.toLowerCase()], `${id}: header ${name}`).toBe(value);
  }
  for (const name of ['If-Match', 'Last-Event-ID']) {
    if (!(name in req.headers)) {
      expect(got.headers[name.toLowerCase()], `${id}: sent ${name} the example does not`).toBeUndefined();
    }
  }
  if (req.request_body === null || req.request_body === undefined) {
    expect(got.body, `${id}: sent a body the example does not`).toBe('');
    return;
  }
  const diff = got.body === '' ? '$: the tool sent no body' : jsonDiff(req.request_body, JSON.parse(got.body), true);
  expect(diff, `${id}: request body ${got.body}; example ${JSON.stringify(req.request_body)}`).toBe('');
}

interface StreamFrameText {
  id?: string;
  event?: string;
  data: string;
}

/** The first event of the streamRoom example. */
function firstFrameText(): StreamFrameText {
  const frame: StreamFrameText = { data: '' };
  for (const line of String(operation('streamRoom').response_body).split('\n')) {
    if (line.startsWith('id: ')) frame.id = line.slice(4);
    else if (line.startsWith('event: ')) frame.event = line.slice(7);
    else if (line.startsWith('data: ')) frame.data = line.slice(6);
    else if (line === '' && frame.data) break;
  }
  return frame;
}

describe('MCP contract (contract/openapi-examples.json)', () => {
  it('has a tool for every operation of the contract', () => {
    const ids = operations.map((op) => op.operation_id).sort();
    expect(Object.keys(calls).sort()).toEqual(ids);
    expect(Object.keys(OPERATION_TOOLS).sort(), 'OPERATION_TOOLS keys').toEqual(ids);
    for (const id of ids) {
      expect(OPERATION_TOOLS[id], `the tool of ${id}`).toBe(calls[id].tool);
      expect(manifest.tools.map((t) => t.name), `tools/list serves ${calls[id].tool}`).toContain(calls[id].tool);
    }
  });

  for (const op of operations) {
    const call = calls[op.operation_id];

    it(`${op.operation_id}: ${call?.tool} sends the example's request and shows the answer`, async () => {
      const { result, sent, auth, args } = await runExample(op, op);
      const required = toolDefinition(call.tool).inputSchema.required ?? [];
      for (const name of required) {
        expect(args, `${call.tool}: the example gives the required argument ${name}`).toHaveProperty(name);
      }
      expect(result.isError, `${call.tool} ${JSON.stringify(args)}: ${text(result)}`).toBeFalsy();
      checkRequest(op, op, sent, auth);
      for (const shown of call.shows(op.response_body as Answer)) {
        expect(text(result), `${call.tool}: the result`).toContain(shown);
      }
    });

    for (const error of op.errors ?? []) {
      it(`${op.operation_id}: ${call?.tool} reports ${error.status} (${error.case})`, async () => {
        const answer = error.response_body as { error: { code: string; message: string; request_id?: string } };
        const { result, sent, auth } = await runExample(op, error);
        checkRequest(op, error, sent, auth);
        expect(result.isError, `${call.tool}: isError`).toBe(true);
        expect(text(result)).toContain(`${answer.error.code}: ${answer.error.message}`);
        if (answer.error.request_id) {
          expect(text(result)).toContain(`request id: ${answer.error.request_id}`);
        }
      });
    }
  }

  it('records error examples to hold the tools to', () => {
    expect(operations.flatMap((op) => op.errors ?? []).length).toBeGreaterThan(0);
  });
});
