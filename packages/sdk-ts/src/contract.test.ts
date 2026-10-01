import { describe, it, expect } from 'vitest';
import { createServer } from 'node:http';
import type { IncomingHttpHeaders } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { Solvr } from './client.js';
import { SolvrError } from './types.js';
import type { RoomEntryKind, RoomStreamEvent, SearchSort } from './types.js';

// The contract tests serve contract/openapi-examples.json (the recorded examples of the
// served OpenAPI document, each held to the running API by the backend) from a local
// server and hold the SDK to it: the request it sends (method, path, query, headers,
// credential, body), what it surfaces (the answer, or the API's error code), and its
// types (every example request and answer is a value of the SDK method's own types).

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
// getReply answers the ETag the updateReply example sends back as If-Match (the read before the edit).
const etag = operations.find(op => op.operation_id === 'updateReply')?.headers['If-Match'] ?? '';

/** One example as the SDK caller sees it. */
class Call {
  constructor(readonly op: ContractOperation, readonly req: ContractRequest) {}

  path(name: string): string {
    const value = this.req.path_params[name];
    if (value === undefined) {
      throw new Error(`${this.op.operation_id}: the example has no path parameter ${name}`);
    }
    return value;
  }

  query(name: string): string | undefined {
    return this.req.query[name];
  }

  queryInt(name: string): number | undefined {
    const value = this.req.query[name];
    if (value === undefined) return undefined;
    const n = Number(value);
    if (!Number.isInteger(n)) {
      throw new Error(`${this.op.operation_id}: query ${name}=${value} is not an integer`);
    }
    return n;
  }

  header(name: string): string | undefined {
    return this.req.headers[name];
  }

  /** The example's request body as the caller passes it; the type test holds it to the SDK's input type. */
  body<T>(): T {
    return this.req.request_body as T;
  }
}

interface Caller {
  call: (client: Solvr, x: Call) => Promise<unknown>;
  /** The argument of the SDK method that carries the request body. */
  bodyArg?: number;
}

// The callers call each operation through the SDK with the example's inputs and return what
// the SDK surfaces. The key is the operationId; the SDK method has the same name.
const callers: Record<string, Caller> = {
  createPost: { bodyArg: 0, call: (c, x) => c.createPost(x.body()) },
  getPost: { call: (c, x) => c.getPost(x.path('id')) },
  search: {
    call: (c, x) => c.search(x.query('q') ?? '', {
      limit: x.queryInt('per_page'), page: x.queryInt('page'), sort: x.query('sort') as SearchSort | undefined,
    }),
  },
  createReply: { bodyArg: 1, call: (c, x) => c.createReply(x.path('id'), x.body()) },
  listReplies: {
    call: (c, x) => c.listReplies(x.path('id'), { cursor: x.query('cursor'), limit: x.queryInt('limit') }),
  },
  getReply: { call: (c, x) => c.getReply(x.path('id')) },
  updateReply: { bodyArg: 2, call: (c, x) => c.updateReply(x.path('id'), x.header('If-Match') ?? '', x.body()) },
  createRoom: { bodyArg: 0, call: (c, x) => c.createRoom(x.body()) },
  handshakeRoom: { bodyArg: 1, call: (c, x) => c.handshakeRoom(x.path('slug'), x.body()) },
  listRoomEntries: {
    call: (c, x) => c.listRoomEntries(x.path('slug'), {
      cursor: x.query('cursor'), limit: x.queryInt('limit'), kind: x.query('kind') as RoomEntryKind | undefined, issue: x.query('issue'),
    }),
  },
  createRoomEntry: { bodyArg: 1, call: (c, x) => c.createRoomEntry(x.path('slug'), x.body()) },
  createRoomStreamTicket: { call: (c, x) => c.createRoomStreamTicket(x.path('slug')) },
  streamRoom: {
    call: async (c, x) => {
      const stream = await c.streamRoom(x.path('slug'), {
        lastEventId: x.header('Last-Event-ID'), ticket: x.query('ticket'), type: x.query('type'), issue: x.query('issue'),
      });
      try {
        return await stream.next();
      } finally {
        await stream.close();
      }
    },
  },
};

interface Recorded {
  method: string;
  path: string;
  query: [string, string][];
  headers: IncomingHttpHeaders;
  body: string;
}

/** Serves one example's answer and records what the SDK sent. */
async function serve(op: ContractOperation, req: ContractRequest, servedEtag: string) {
  const sent: Recorded[] = [];
  const server = createServer((r, w) => {
    const chunks: Buffer[] = [];
    r.on('data', (chunk: Buffer) => chunks.push(chunk));
    r.on('end', () => {
      const url = new URL(r.url ?? '/', 'http://contract.local');
      sent.push({
        method: r.method ?? '', path: url.pathname, query: [...url.searchParams.entries()],
        headers: r.headers, body: Buffer.concat(chunks).toString('utf8'),
      });
      if (op.operation_id === 'getReply' && servedEtag) {
        w.setHeader('ETag', servedEtag);
      }
      if (req.status < 400 && op.response_media_type === 'text/event-stream') {
        w.writeHead(req.status, { 'Content-Type': 'text/event-stream' });
        w.end(req.response_body as string);
        return;
      }
      w.writeHead(req.status, { 'Content-Type': 'application/json' });
      w.end(JSON.stringify(req.response_body));
    });
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  const close = () => new Promise<void>(resolve => {
    server.closeAllConnections();
    server.close(() => resolve());
  });
  return { url: `http://127.0.0.1:${port}`, sent, close };
}

/** The SDK client that presents the example's credential, and the Authorization header it must send. */
function clientFor(op: ContractOperation, credential: string, baseUrl: string): { client: Solvr; auth?: string } {
  const config = { baseUrl, retries: 1 };
  if (credential === 'invalid') {
    if (op.credential !== 'room_token') {
      throw new Error(`${op.operation_id}: an invalid ${op.credential} credential has no SDK case yet`);
    }
    return { client: new Solvr({ ...config, apiKey: AGENT_KEY }).withRoomToken(DEAD_TOKEN), auth: `Bearer ${DEAD_TOKEN}` };
  }
  switch (credential) {
    case 'none':
      return { client: new Solvr({ ...config, apiKey: null }) };
    case 'agent_api_key':
      return { client: new Solvr({ ...config, apiKey: AGENT_KEY }), auth: `Bearer ${AGENT_KEY}` };
    case 'room_token':
      return { client: new Solvr({ ...config, apiKey: AGENT_KEY }).withRoomToken(ROOM_TOKEN), auth: `Bearer ${ROOM_TOKEN}` };
  }
  throw new Error(`${op.operation_id}: unknown credential ${credential}`);
}

function expectedPath(op: ContractOperation, params: Record<string, string>): string {
  let path = op.path;
  for (const [name, value] of Object.entries(params)) {
    path = path.replaceAll(`{${name}}`, encodeURIComponent(value));
  }
  if (path.includes('{')) {
    throw new Error(`${op.operation_id}: path ${path} keeps a variable the example gives no value`);
  }
  return path;
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
      if (got[key] === undefined) return `${at}.${key}: the example has it, the SDK lost it`;
      const diff = jsonDiff(want[key], got[key], exact, `${at}.${key}`);
      if (diff) return diff;
    }
    if (exact) {
      for (const key of Object.keys(got)) {
        if (!(key in want) && got[key] !== null && got[key] !== undefined) {
          return `${at}.${key}: the SDK sent it, the example does not`;
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

/** Holds what the SDK sent to the example. */
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
  for (const name of ['If-Match', 'Last-Event-ID']) {
    if (!(name in req.headers)) {
      expect(got.headers[name.toLowerCase()], `${id}: sent ${name} the example does not`).toBeUndefined();
    }
  }
  if (req.request_body === null || req.request_body === undefined) {
    expect(got.body, `${id}: sent a body the example does not`).toBe('');
    return;
  }
  const diff = got.body === '' ? '$: the SDK sent no body' : jsonDiff(req.request_body, JSON.parse(got.body), true);
  expect(diff, `${id}: request body ${got.body}; example ${JSON.stringify(req.request_body)}`).toBe('');
}

interface StreamFrameText {
  id?: string;
  event?: string;
  data: string;
}

/** The first event of an event-stream example. */
function firstFrame(text: string): StreamFrameText {
  const frame: StreamFrameText = { data: '' };
  for (const line of text.split('\n')) {
    if (line.startsWith('id: ')) frame.id = line.slice(4);
    else if (line.startsWith('event: ')) frame.event = line.slice(7);
    else if (line.startsWith('data: ')) frame.data = line.slice(6);
    else if (line === '' && frame.data) break;
  }
  return frame;
}

/** Holds what the SDK surfaced to the example's answer. */
function checkResult(op: ContractOperation, result: unknown) {
  const id = op.operation_id;
  if (op.response_media_type === 'text/event-stream') {
    const want = firstFrame(op.response_body as string);
    const event = result as RoomStreamEvent | null;
    expect(event, `${id}: the SDK read no event`).not.toBeNull();
    expect({ id: event?.id, event: event?.event }, `${id}: event id and name`).toEqual({ id: want.id, event: want.event });
    const frame = JSON.parse(want.data) as { payload?: unknown };
    expect(jsonDiff(frame, event?.frame, false), `${id}: the SDK frame`).toBe('');
    expect(jsonDiff(frame.payload, event?.message, false), `${id}: the SDK message`).toBe('');
    return;
  }
  expect(jsonDiff(op.response_body, result, false), `${id}: the SDK result ${JSON.stringify(result)}`).toBe('');
  if (id === 'getReply') {
    expect((result as { etag?: string }).etag, 'getReply: the ETag the edit sends back').toBe(etag);
  }
}

// --- the type check: every example is a value of the SDK's own types ----------------------

const typeHeader = [
  "import type { Solvr, RoomStreamFrame, RoomStreamMessage, SolvrErrorResponse } from './index.js';",
  'type Answer<K extends keyof Solvr> = Solvr[K] extends (...args: any[]) => Promise<infer R> ? R : never;',
  'type Input<K extends keyof Solvr, I extends number> = Solvr[K] extends (...args: infer P) => unknown ? NonNullable<P[I]> : never;',
];

/** Compiles declarations (one per line) against the SDK's sources; answers each error as "<name>: <message>". */
function typeErrors(declarations: string[]): string[] {
  const source = [...typeHeader, ...declarations].join('\n');
  const configPath = join(here, '..', 'tsconfig.json');
  const config = ts.readConfigFile(configPath, ts.sys.readFile);
  const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, dirname(configPath));
  const options = { ...parsed.options, noEmit: true };
  const fileName = join(here, '__contract_types__.ts');
  const host = ts.createCompilerHost(options);
  const getSourceFile = host.getSourceFile.bind(host);
  host.getSourceFile = (name, language, ...rest) =>
    name === fileName ? ts.createSourceFile(name, source, language) : getSourceFile(name, language, ...rest);
  const fileExists = host.fileExists.bind(host);
  host.fileExists = name => name === fileName || fileExists(name);
  const program = ts.createProgram([fileName], options, host);
  const file = program.getSourceFile(fileName);
  if (!file) throw new Error('the type check lost its source');
  const lines = source.split('\n');
  return ts.getPreEmitDiagnostics(program, file).map(d => {
    const line = d.start === undefined ? -1 : file.getLineAndCharacterOfPosition(d.start).line;
    const name = lines[line]?.match(/^const (\w+)/)?.[1] ?? `line ${line + 1}`;
    return `${name}: ${ts.flattenDiagnosticMessageText(d.messageText, ' ')}`;
  });
}

function contractDeclarations(): string[] {
  const out: string[] = [];
  for (const op of operations) {
    const id = op.operation_id;
    if (op.request_body !== null && op.request_body !== undefined) {
      const arg = callers[id]?.bodyArg;
      if (arg === undefined) throw new Error(`${id}: the example sends a body and no SDK argument carries it`);
      out.push(`const request_${id}: Input<'${id}', ${arg}> = ${JSON.stringify(op.request_body)};`);
    }
    if (op.response_media_type === 'text/event-stream') {
      const frame = JSON.parse(firstFrame(op.response_body as string).data) as { type?: string; payload?: unknown };
      out.push(`const frame_${id}: RoomStreamFrame = ${JSON.stringify(frame)};`);
      if (frame.type === 'message') {
        out.push(`const message_${id}: RoomStreamMessage = ${JSON.stringify(frame.payload)};`);
      }
    } else {
      out.push(`const answer_${id}: Answer<'${id}'> = ${JSON.stringify(op.response_body)};`);
    }
    for (const e of op.errors ?? []) {
      out.push(`const error_${id}_${e.status}: SolvrErrorResponse = ${JSON.stringify(e.response_body)};`);
    }
  }
  return out;
}

describe('contract: contract/openapi-examples.json', () => {
  it('every operation is a client method named after its operationId', () => {
    for (const op of operations) {
      expect(callers[op.operation_id], `${op.operation_id}: no SDK caller exercises it`).toBeDefined();
      const method = (Solvr.prototype as unknown as Record<string, unknown>)[op.operation_id];
      expect(typeof method, `${op.operation_id}: the SDK has no method of that name`).toBe('function');
    }
  });

  describe('each example is what the SDK sends and surfaces', () => {
    for (const op of operations) {
      it(op.operation_id, async () => {
        const caller = callers[op.operation_id];
        expect(caller, `no SDK caller for ${op.operation_id}`).toBeDefined();
        const server = await serve(op, op, etag);
        try {
          const { client, auth } = clientFor(op, op.credential, server.url);
          const result = await caller.call(client, new Call(op, op));
          checkRequest(op, op, server.sent, auth);
          checkResult(op, result);
        } finally {
          await server.close();
        }
      });
    }
  });

  describe('each error example surfaces the API error code', () => {
    const cases = operations.flatMap(op => (op.errors ?? []).map(e => ({ op, e })));

    it('the contract carries error examples', () => {
      expect(cases.length).toBeGreaterThan(0);
    });

    for (const { op, e } of cases) {
      it(`${op.operation_id}/${e.status}: ${e.case}`, async () => {
        const server = await serve(op, e, '');
        try {
          const { client, auth } = clientFor(op, e.credential, server.url);
          const error = await callers[op.operation_id].call(client, new Call(op, e)).then(
            () => new Error('the SDK answered no error'),
            (err: unknown) => err,
          );
          checkRequest(op, e, server.sent, auth);
          expect(error).toBeInstanceOf(SolvrError);
          const got = error as SolvrError;
          const want = (e.response_body as { error: unknown }).error;
          const surfaced = { code: got.code, message: got.message, request_id: got.requestId, details: got.details };
          expect(jsonDiff(want, surfaced, false), `the SDK error ${JSON.stringify(surfaced)}`).toBe('');
          expect(got.status).toBe(e.status);
        } finally {
          await server.close();
        }
      });
    }
  });

  it('every example request and answer is a value of the SDK method types', () => {
    expect(typeErrors(contractDeclarations())).toEqual([]);
  }, 60_000);

  it('the type check catches a lost answer field, a wrong value, a forbidden null and an unknown request field', () => {
    const reply = operations.find(op => op.operation_id === 'getReply')?.response_body as { data: Record<string, unknown> };
    const room = operations.find(op => op.operation_id === 'createRoom')?.request_body as Record<string, unknown>;
    const errors = typeErrors([
      `const intact: Answer<'getReply'> = ${JSON.stringify(reply)};`,
      `const extra: Answer<'getReply'> = ${JSON.stringify({ data: { ...reply.data, not_a_field: 1 } })};`,
      `const wrong: Answer<'getReply'> = ${JSON.stringify({ data: { ...reply.data, upvotes: '0' } })};`,
      `const nulled: Answer<'getReply'> = ${JSON.stringify({ data: { ...reply.data, body: null } })};`,
      `const unknownField: Input<'createRoom', 0> = ${JSON.stringify({ ...room, not_a_field: true })};`,
      `const noMethod: Answer<'notAnOperation'> = {};`,
    ]);
    const failing = new Set(errors.map(e => e.split(':')[0]));
    expect([...failing].sort()).toEqual(['extra', 'noMethod', 'nulled', 'unknownField', 'wrong']);
  }, 60_000);

  it('jsonDiff catches a lost field, a value and an extra request field', () => {
    const cases: [unknown, unknown, boolean, boolean][] = [
      [{ a: 1, b: { c: 'x' } }, { a: 1, b: { c: 'x' }, d: 2 }, false, false],
      [{ a: 1, b: null }, { a: 1 }, true, false],
      [{ a: 1, b: { c: 'x' } }, { a: 1, b: {} }, false, true],
      [{ a: [1, 2] }, { a: [1] }, false, true],
      [{ a: '2026-10-01T18:41:37.518736Z' }, { a: '2026-10-01T18:41:37.518737Z' }, false, true],
      [{ a: 1 }, { a: 1, rotate: false }, true, true],
      [{ a: {} }, {}, false, true],
    ];
    for (const [want, got, exact, fails] of cases) {
      expect(jsonDiff(want, got, exact) !== '', JSON.stringify({ want, got, exact })).toBe(fails);
    }
  });
});
