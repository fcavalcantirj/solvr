import { describe, it, expect, afterEach } from 'vitest';
import type { ServerResponse } from 'node:http';
import { handleRequest } from '../server.js';
import { OPERATION_TOOLS, SolvrTools } from '../tools.js';
import { callTool, json, startServer, text } from './harness.js';
import type { LocalServer, Recorded } from './harness.js';

// The reply and search tools against a real local server, and the JSON-RPC handler itself.

const API_KEY = 'solvr_agent_key';
const servers: LocalServer[] = [];

afterEach(async () => {
  while (servers.length > 0) {
    await servers.pop()?.close();
  }
});

async function serve(handler: (req: Recorded, res: ServerResponse) => void): Promise<LocalServer> {
  const server = await startServer(handler);
  servers.push(server);
  return server;
}

const reply = (body: string) => ({
  id: 'rep/1',
  post_id: 'post-1',
  author_type: 'agent',
  author_id: 'agent_planner',
  body,
  upvotes: 0,
  downvotes: 0,
  score: 0,
  created_at: '2026-10-01T00:00:00Z',
});

describe('solvr_get_reply and solvr_update_reply', () => {
  it('get_reply escapes the id and shows the body and the ETag to edit with', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: reply('the fix') }, { ETag: '"v1"' }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_get_reply', { id: 'rep/1' });
    expect(server.sent[0].path).toBe('/v1/replies/rep%2F1');
    const shown = text(result);
    expect(shown).toContain('the fix');
    expect(shown).toContain('ETag: "v1"');
    expect(shown).toContain('solvr_update_reply');
  });

  it('update_reply sends If-Match and the body, and shows the new ETag', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: reply('the fix, edited') }, { ETag: '"v2"' }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_update_reply', {
      id: 'rep/1', if_match: '"v1"', body: 'the fix, edited',
    });
    const req = server.sent[0];
    expect(req).toMatchObject({ method: 'PATCH', path: '/v1/replies/rep%2F1' });
    expect(req.headers['if-match']).toBe('"v1"');
    expect(req.headers.authorization).toBe(`Bearer ${API_KEY}`);
    expect(JSON.parse(req.body)).toEqual({ body: 'the fix, edited' });
    expect(text(result)).toContain('ETag: "v2"');
  });

  it('an empty if_match sends no If-Match and reports the API answer (428) with its request id', async () => {
    const server = await serve((_r, w) =>
      json(w, 428, { error: { code: 'PRECONDITION_REQUIRED', message: 'If-Match is required', request_id: 'req-428' } })
    );
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_update_reply', { id: 'rep-1', if_match: '', body: 'x' });
    expect(server.sent[0].headers['if-match']).toBeUndefined();
    expect(result.isError).toBe(true);
    expect(text(result)).toContain('PRECONDITION_REQUIRED: If-Match is required');
    expect(text(result)).toContain('request id: req-428');
  });
});

describe('solvr_replies', () => {
  it('sends the cursor and limit and shows the replies and the next cursor', async () => {
    const server = await serve((_r, w) =>
      json(w, 200, { data: [reply('first reply')], meta: { total: 3, has_more: true, next_cursor: 'cur-2' } })
    );
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_replies', { post_id: 'post-1', limit: 1, cursor: 'cur-1' });
    expect(server.sent[0].path).toBe('/v1/posts/post-1/replies');
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ limit: '1', cursor: 'cur-1' });
    expect(text(result)).toContain('first reply');
    expect(text(result)).toContain('cursor cur-2');
  });
});

describe('solvr_search', () => {
  it('an empty query sends no q; page and sort are sent as given', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: [], meta: { total: 0, page: 2, per_page: 5 } }));
    const tools = new SolvrTools(null, server.url);
    await callTool(tools, 'solvr_search', { query: '', page: 2, sort: 'newest' });
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ page: '2', sort: 'newest' });
    expect(server.sent[0].headers.authorization).toBeUndefined();
  });
});

describe('JSON-RPC handler', () => {
  it('tools/list serves the tool of every contract operation and solvr_claim', async () => {
    const response = await handleRequest({ jsonrpc: '2.0', id: 1, method: 'tools/list' }, new SolvrTools(API_KEY, 'http://127.0.0.1:1'));
    const names = (response.result as { tools: Array<{ name: string }> }).tools.map((t) => t.name);
    expect(names.sort()).toEqual([...Object.values(OPERATION_TOOLS), 'solvr_claim'].sort());
  });

  it('answers initialize, rejects a call without a tool name and an unknown method', async () => {
    const tools = new SolvrTools(API_KEY, 'http://127.0.0.1:1');
    const init = await handleRequest({ jsonrpc: '2.0', id: 1, method: 'initialize' }, tools);
    expect(init.result).toMatchObject({ name: 'solvr', capabilities: { tools: {} } });
    const noName = await handleRequest({ jsonrpc: '2.0', id: 2, method: 'tools/call', params: {} }, tools);
    expect(noName.error?.code).toBe(-32602);
    const unknown = await handleRequest({ jsonrpc: '2.0', id: 3, method: 'nope' }, tools);
    expect(unknown.error).toEqual({ code: -32601, message: 'Method not found: nope' });
  });
});
