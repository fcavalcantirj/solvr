import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { Solvr, VERSION } from './index.js';

// 2.0.0 removed the 1.x choices of the legacy knowledge model: a post has no
// type, every contribution is a reply, and search no longer filters by the
// legacy type or status. A JavaScript caller still passing a removed option
// gets a TypeError naming it instead of a call the API would read the old way.

const mockFetch = vi.fn();
global.fetch = mockFetch;

const read = (path: string) => readFileSync(resolve(__dirname, path), 'utf8');
const pkg = JSON.parse(read('../package.json')) as { version: string };

// The "Migrating from 1.x to <VERSION>" section of the README.
function migrationNotes(): string {
  const heading = `## Migrating from 1.x to ${VERSION}`;
  const readme = read('../README.md');
  const start = readme.indexOf(heading);
  if (start < 0) throw new Error(`README has no "${heading}" section`);
  const rest = readme.slice(start + heading.length);
  const next = rest.search(/\n## /);
  return next < 0 ? rest : rest.slice(0, next);
}

// The members of an interface in types.ts.
function interfaceKeys(name: string): string[] {
  const body = read('types.ts').match(new RegExp(`interface ${name} \\{([^}]*)\\}`));
  if (!body) throw new Error(`types.ts has no interface ${name}`);
  return Array.from(body[1].matchAll(/^\s*(\w+)\??:/gm), (m) => m[1]);
}

describe('@solvr/sdk 2.0.0 removed the legacy 1.x choices', () => {
  const solvr = new Solvr({ apiKey: 'solvr_sk_test' });

  beforeEach(() => {
    mockFetch.mockReset();
  });

  it.each([
    ['type', { type: 'problem' }],
    ['type', { type: 'all' }],
    ['status', { status: 'open' }],
  ])('search rejects the removed %s option without calling the API', async (name, options) => {
    const call = solvr.search('postgres', options as never);
    await expect(call).rejects.toThrow(TypeError);
    await expect(call).rejects.toThrow(`'${name}' was removed in @solvr/sdk ${VERSION}`);
    await expect(call).rejects.toThrow('Migrating from 1.x');
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it.each(['type', 'success_criteria'])('createPost and post reject the removed %s field', async (name) => {
    const input = { title: 'A title long enough', description: 'A description', [name]: 'problem' };
    await expect(solvr.createPost(input as never)).rejects.toThrow(`'${name}' was removed in @solvr/sdk ${VERSION}`);
    await expect(solvr.post(input as never)).rejects.toThrow(TypeError);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it('an option left undefined is not a removed option', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ data: [], meta: {} }) });
    await solvr.search('postgres', { type: undefined, limit: 5 } as never);
    expect(mockFetch.mock.calls[0][0]).toBe('https://api.solvr.dev/v1/search?q=postgres&per_page=5');
  });

  it('get sends no include: a post and its replies are two reads', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ data: { id: 'p1' } }) });
    await (solvr.get as (id: string, options: unknown) => Promise<unknown>)('p1', { include: ['answers'] });
    expect(mockFetch.mock.calls[0][0]).toBe('https://api.solvr.dev/v1/posts/p1');
  });

  it('declares none of the removed options or methods', () => {
    expect(interfaceKeys('SearchOptions')).not.toContain('type');
    expect(interfaceKeys('SearchOptions')).not.toContain('status');
    expect(interfaceKeys('CreatePostInput')).not.toContain('type');
    expect(interfaceKeys('CreatePostInput')).not.toContain('success_criteria');
    expect('approach' in Solvr.prototype).toBe(false);
    expect('answer' in Solvr.prototype).toBe(false);
  });

  it('VERSION is 2.0.0 and is the published package version', () => {
    expect(VERSION).toBe('2.0.0');
    expect(pkg.version).toBe(VERSION);
  });

  it('the README migration notes name every removed member and what replaces it', () => {
    const notes = migrationNotes();
    for (const removed of ['`approach()`', '`answer()`', '`include`', '`type`', '`success_criteria`', '`status`']) {
      expect(notes, `the notes do not name ${removed}`).toContain(removed);
    }
    for (const use of ['`reply()`', '`replies()`', 'ENDPOINT_RETIRED', 'details.replacement']) {
      expect(notes, `the notes do not name ${use}`).toContain(use);
    }
  });
});
