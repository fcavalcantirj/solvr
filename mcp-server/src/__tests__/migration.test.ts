import { describe, it, expect, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createRequire } from 'node:module';
import type { ServerResponse } from 'node:http';
import { handleRequest } from '../server.js';
import { SolvrApiClient } from '../api.js';
import { SolvrTools } from '../tools.js';
import { VERSION } from '../version.js';
import { callTool, json, startServer, text } from './harness.js';
import type { LocalServer, Recorded } from './harness.js';

// idx 78 step 5: 2.0.0 removes the tools and arguments of the legacy knowledge model. A 1.x call
// that still uses one is refused before any request, naming what replaces it and the migration
// notes of the README; the notes name every removed choice and only point at tools that exist.

const API_KEY = 'solvr_agent_key';
const README = readFileSync(resolve(__dirname, '../../README.md'), 'utf8');
const PACKAGE = createRequire(import.meta.url)('../../package.json') as { version: string };
const LOCK = createRequire(import.meta.url)('../../package-lock.json') as {
  version: string;
  packages: Record<string, { version: string }>;
};
const NOTES_HEADING = '## Migrating from 1.x to 2.0.0';

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

/** The migration notes: the README section from its heading to the next section. */
function notes(): string {
  const start = README.indexOf(NOTES_HEADING);
  if (start < 0) return '';
  const end = README.indexOf('\n## ', start + NOTES_HEADING.length);
  return README.slice(start, end < 0 ? undefined : end);
}

/** 1.x calls a 2.0.0 server refuses: the tool, its 1.x arguments, the removed name, what replaces it. */
const REMOVED_CALLS: Array<{ tool: string; args: Record<string, unknown>; removed: string; instead: string }> = [
  { tool: 'solvr_answer', args: { post_id: 'post-1', content: 'Use errgroup' }, removed: "'solvr_answer'", instead: 'solvr_reply' },
  {
    tool: 'solvr_answer',
    args: { post_id: 'post-1', content: 'Add the index', approach_angle: 'database' },
    removed: "'solvr_answer'",
    instead: 'solvr_reply',
  },
  ...['problem', 'question', 'idea', 'all'].map((type) => ({
    tool: 'solvr_search',
    args: { query: 'ECONNREFUSED', type },
    removed: "The 'type' argument of solvr_search",
    instead: 'search covers every post',
  })),
  ...['problem', 'question', 'idea'].map((type) => ({
    tool: 'solvr_post',
    args: { type, title: 'Race condition in async code', description: 'Details' },
    removed: "The 'type' argument of solvr_post",
    instead: 'a post has no type',
  })),
  {
    tool: 'solvr_get',
    args: { id: 'post-1', include: ['approaches', 'answers'] },
    removed: "The 'include' argument of solvr_get",
    instead: 'solvr_replies',
  },
];

describe('1.x tools and arguments removed in 2.0.0', () => {
  for (const call of REMOVED_CALLS) {
    it(`${call.tool} ${JSON.stringify(call.args)} fails before any request, naming what replaces it`, async () => {
      const server = await serve((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
      const result = await callTool(new SolvrTools(API_KEY, server.url), call.tool, call.args);

      expect(result.isError).toBe(true);
      const shown = text(result);
      expect(shown).toContain(`${call.removed} was removed in @solvr/mcp-server ${VERSION}`);
      expect(shown).toContain(call.instead);
      expect(shown).toContain(`See "Migrating from 1.x to ${VERSION}" in the @solvr/mcp-server README.`);
      expect(server.sent).toHaveLength(0);
    });
  }

  it('names the type a 1.x solvr_post asked for', async () => {
    const server = await serve((_r, w) => json(w, 201, { data: {} }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_post', {
      type: 'question', title: 't', description: 'd',
    });
    expect(text(result)).toContain('"question"');
    expect(server.sent).toHaveLength(0);
  });

  it('an absent (undefined or null) legacy argument is no choice: the call runs', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
    const tools = new SolvrTools(API_KEY, server.url);
    await callTool(tools, 'solvr_search', { query: 'pool', type: null });
    const result = await tools.executeTool('solvr_search', { query: 'pool', type: undefined });
    expect(result.isError).toBeUndefined();
    expect(server.sent).toHaveLength(2);
  });

  it('a tool that never existed is still an unknown tool', async () => {
    const server = await serve((_r, w) => json(w, 200, {}));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_teleport', {});
    expect(result.isError).toBe(true);
    expect(text(result)).toBe('Unknown tool: solvr_teleport');
    expect(server.sent).toHaveLength(0);
  });
});

describe('the 2.0.0 tools still run with their own arguments', () => {
  it('solvr_search sends exactly q, per_page, page and sort', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_search', {
      query: 'pool race', limit: 3, page: 2, sort: 'newest',
    });
    expect(result.isError).toBeUndefined();
    expect(server.sent).toHaveLength(1);
    expect(server.sent[0].query).toEqual([['q', 'pool race'], ['per_page', '3'], ['page', '2'], ['sort', 'newest']]);
  });

  it('solvr_post sends exactly the canonical body', async () => {
    const server = await serve((_r, w) => json(w, 201, { data: { id: 'post-9', title: 'T', type: 'post' } }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_post', {
      title: 'T', description: 'D', tags: ['go'], visibility: 'family',
    });
    expect(result.isError).toBeUndefined();
    expect(JSON.parse(server.sent[0].body)).toEqual({ title: 'T', description: 'D', tags: ['go'], visibility: 'family' });
  });

  it('solvr_get reads the post and its first replies', async () => {
    const server = await serve((r, w) =>
      r.path.endsWith('/replies')
        ? json(w, 200, { data: [], meta: { total: 0, has_more: false } })
        : json(w, 200, { data: { id: 'post-1', type: 'post', title: 'T', description: 'D' } }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_get', { id: 'post-1' });
    expect(result.isError).toBeUndefined();
    expect(server.sent.map((r) => r.path).sort()).toEqual(['/v1/posts/post-1', '/v1/posts/post-1/replies']);
  });

  it('the API client sends no legacy search filter, even when a JavaScript caller passes one', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: [], meta: { total: 0 } }));
    const client = new SolvrApiClient(API_KEY, server.url);
    await client.search('pool', { type: 'problem', limit: 5 } as Parameters<SolvrApiClient['search']>[1]);
    expect(server.sent[0].query).toEqual([['q', 'pool'], ['per_page', '5']]);
    const api = readFileSync(resolve(__dirname, '../api.ts'), 'utf8');
    const searchOptions = api.slice(api.indexOf('export interface SearchOptions'), api.indexOf('}', api.indexOf('export interface SearchOptions')));
    expect(searchOptions).not.toMatch(/\btype\??:/);
  });
});

describe('tools/list', () => {
  it('offers none of the removed tools or arguments', async () => {
    const response = await handleRequest({ jsonrpc: '2.0', id: 1, method: 'tools/list' }, new SolvrTools(API_KEY, 'http://127.0.0.1:9'));
    const tools = (response.result as { tools: Array<{ name: string; inputSchema: { properties: Record<string, unknown> } }> }).tools;
    expect(tools.map((t) => t.name)).not.toContain('solvr_answer');
    for (const tool of tools) {
      for (const removed of ['type', 'include', 'approach_angle']) {
        expect(tool.inputSchema.properties, `${tool.name} offers ${removed}`).not.toHaveProperty(removed);
      }
    }
  });
});

describe('the version the migration notes describe', () => {
  it('is 2.0.0 in the package, the lock file, the code and the initialize answer', async () => {
    expect(VERSION).toBe('2.0.0');
    expect(PACKAGE.version).toBe(VERSION);
    expect(LOCK.version).toBe(VERSION);
    expect(LOCK.packages[''].version).toBe(VERSION);
    const response = await handleRequest(
      { jsonrpc: '2.0', id: 1, method: 'initialize', params: {} },
      new SolvrTools(API_KEY, 'http://127.0.0.1:9')
    );
    expect((response.result as { version: string }).version).toBe(VERSION);
    expect(README).toContain(NOTES_HEADING);
  });
});

describe('the migration notes', () => {
  it('name every removed tool and argument, the refusal, and the retired routes', () => {
    const section = notes();
    expect(section).not.toBe('');
    for (const name of ['`solvr_answer`', '`approach_angle`', '`content`', '`solvr_search`', '`solvr_post`', '`solvr_get`', '`type`', '`include`']) {
      expect(section, `the notes do not name ${name}`).toContain(name);
    }
    expect(section).toContain("'solvr_answer' was removed in @solvr/mcp-server 2.0.0");
    expect(section).toContain('ENDPOINT_RETIRED');
    expect(section).toContain('error.details.replacement');
  });

  it('point only at tools 2.0.0 serves (besides the removed solvr_answer)', async () => {
    const response = await handleRequest({ jsonrpc: '2.0', id: 1, method: 'tools/list' }, new SolvrTools(API_KEY, 'http://127.0.0.1:9'));
    const served = (response.result as { tools: Array<{ name: string }> }).tools.map((t) => t.name);
    const named = [...new Set([...notes().matchAll(/\bsolvr_[a-z_]+/g)].map((m) => m[0]))].filter((n) => n !== 'solvr_answer');
    expect(named.length).toBeGreaterThan(3);
    for (const name of named) {
      expect(served, `the notes point at ${name}`).toContain(name);
    }
  });

  it('every refusal points only at tools 2.0.0 serves', async () => {
    const tools = new SolvrTools(API_KEY, 'http://127.0.0.1:9');
    const served = tools.getManifest().tools.map((t) => t.name);
    for (const call of REMOVED_CALLS) {
      const shown = text(await tools.executeTool(call.tool, call.args));
      for (const name of shown.matchAll(/\bsolvr_[a-z_]+/g)) {
        if (name[0] !== call.tool) expect(served, `the refusal of ${call.tool} points at ${name[0]}`).toContain(name[0]);
      }
    }
  });
});
