import { describe, it, expect, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { ServerResponse } from 'node:http';
import ts from 'typescript';
import { OPERATION_TOOLS, SolvrTools } from '../tools.js';
import { callTool, json, startServer, text } from './harness.js';
import type { LocalServer, Recorded } from './harness.js';

// A room's participants are a collection its owner reads (solvr_room_members, listRoomMembers)
// and adds to (solvr_room_add_member, addRoomMember) with the API key: a third and any later
// agent is admitted to the SAME room, then joins it with its own solvr_room_join.

const API_KEY = 'solvr_agent_owner_key';
const SLUG = 'parser-build';
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

const member = (agent_id: string, role: string, added_by: string) => ({
  room_id: '3f2c9d1e-6a7b-4c8d-9e0f-112233445566', agent_id, role, added_by, created_at: '2026-10-02T12:00:00Z',
});
const refusal = (code: string, message: string) => ({ error: { code, message, request_id: `req-${code}` } });

/** A handshake answers a room token; members answers the participants in an order no sort gives. */
function roomApi(req: Recorded, res: ServerResponse) {
  if (req.path.endsWith('/handshake')) {
    json(res, 201, { data: { agent_id: 'agent_planner', room_slug: SLUG, room_token: 'solvr_rt_kept', rotated: false } });
    return;
  }
  if (req.method === 'GET') {
    json(res, 200, {
      data: [
        member('agent_planner', 'owner', 'system'),
        member('zeta_executor', 'member', 'agent_planner'),
        member('alpha_reviewer', 'member', 'agent_planner'),
      ],
    });
    return;
  }
  const body = JSON.parse(req.body) as { agent_id: string; role?: string };
  json(res, 201, { data: member(body.agent_id, body.role ?? 'member', 'agent_planner') });
}

describe('solvr_room_members', () => {
  it("reads the room's participants with the API key and shows them in the order the API answered", async () => {
    const server = await serve(roomApi);
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_members', { slug: 'a b/c' });

    expect(result.isError).toBeFalsy();
    expect(server.sent).toHaveLength(1);
    expect(server.sent[0]).toMatchObject({ method: 'GET', path: '/v1/rooms/a%20b%2Fc/members', query: [], body: '' });
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${API_KEY}`);
    const shown = text(result);
    expect(shown).toContain('3 participants of a b/c');
    const rows = shown.split('\n').filter((line) => /^(agent_planner|zeta_executor|alpha_reviewer) /.test(line));
    expect(rows.map((line) => line.split(' ')[0])).toEqual(['agent_planner', 'zeta_executor', 'alpha_reviewer']);
    expect(rows[0]).toContain('owner');
    expect(rows[0]).toContain('added by system');
    expect(rows[1]).toContain('member');
    expect(rows[1]).toContain('added by agent_planner');
    expect(rows[1]).toContain('2026-10-02T12:00:00Z');
  });

  it("is refused by the API for a caller that does not own the room: FORBIDDEN and the request id", async () => {
    const server = await serve((_r, w) => json(w, 403, refusal('FORBIDDEN', 'only a room owner may list its participants')));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_members', { slug: SLUG });

    expect(result.isError).toBe(true);
    expect(server.sent).toHaveLength(1);
    expect(text(result)).toContain('FORBIDDEN: only a room owner may list its participants');
    expect(text(result)).toContain('request id: req-FORBIDDEN');
  });

  it('refuses a call without slug before any request', async () => {
    const server = await serve(roomApi);
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_members', {});

    expect(result.isError).toBe(true);
    expect(text(result)).toContain('slug is required');
    expect(server.sent).toHaveLength(0);
  });
});

describe('solvr_room_add_member', () => {
  it('admits an agent with the API key: the body is exactly the agent id, and the result says how it joins', async () => {
    const server = await serve(roomApi);
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_add_member', {
      slug: 'a b/c', agent_id: 'agent_executor',
    });

    expect(result.isError).toBeFalsy();
    expect(server.sent).toHaveLength(1);
    expect(server.sent[0]).toMatchObject({ method: 'POST', path: '/v1/rooms/a%20b%2Fc/members', query: [] });
    expect(server.sent[0].body).toBe(JSON.stringify({ agent_id: 'agent_executor' }));
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${API_KEY}`);
    expect(server.sent[0].headers['content-type']).toBe('application/json');
    const shown = text(result);
    expect(shown).toContain('agent_executor is in a b/c as member (added by agent_planner)');
    expect(shown).toContain('solvr_room_join');
  });

  it('sends the role it was given', async () => {
    const server = await serve(roomApi);
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_add_member', {
      slug: SLUG, agent_id: 'agent_reviewer', role: 'owner',
    });

    expect(JSON.parse(server.sent[0].body)).toEqual({ agent_id: 'agent_reviewer', role: 'owner' });
    expect(text(result)).toContain(`agent_reviewer is in ${SLUG} as owner`);
  });

  it('passes a role it does not know to the API, which decides: the API refusal is reported', async () => {
    const server = await serve((_r, w) => json(w, 400, refusal('VALIDATION_ERROR', 'role must be owner or member')));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_add_member', {
      slug: SLUG, agent_id: 'agent_reviewer', role: 'admin',
    });

    expect(server.sent).toHaveLength(1);
    expect(JSON.parse(server.sent[0].body)).toEqual({ agent_id: 'agent_reviewer', role: 'admin' });
    expect(result.isError).toBe(true);
    expect(text(result)).toContain('VALIDATION_ERROR: role must be owner or member');
  });

  for (const [status, code, message] of [
    [400, 'INVALID_AGENT', 'agent_id names no agent'],
    [403, 'FORBIDDEN', 'only a room owner may admit participants'],
    [409, 'LAST_OWNER', 'a room keeps at least one owner'],
  ] as const) {
    it(`reports the API's ${code} with its message and request id, sent once`, async () => {
      const server = await serve((_r, w) => json(w, status, refusal(code, message)));
      const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_add_member', {
        slug: SLUG, agent_id: 'agent_x', role: 'member',
      });

      expect(result.isError).toBe(true);
      expect(server.sent).toHaveLength(1);
      expect(text(result)).toContain(`${code}: ${message}`);
      expect(text(result)).toContain(`request id: req-${code}`);
    });
  }

  it('refuses a call without slug or agent_id before any request', async () => {
    const server = await serve(roomApi);
    const tools = new SolvrTools(API_KEY, server.url);

    const noAgent = await callTool(tools, 'solvr_room_add_member', { slug: SLUG });
    expect(noAgent.isError).toBe(true);
    expect(text(noAgent)).toContain('agent_id is required');
    const noSlug = await callTool(tools, 'solvr_room_add_member', { agent_id: 'agent_x' });
    expect(noSlug.isError).toBe(true);
    expect(text(noSlug)).toContain('slug is required');
    expect(server.sent).toHaveLength(0);
  });
});

describe('member tools credentials', () => {
  it('present the API key, never the room token a join kept for the same room', async () => {
    const server = await serve(roomApi);
    const tools = new SolvrTools(API_KEY, server.url);

    await callTool(tools, 'solvr_room_join', { slug: SLUG });
    await callTool(tools, 'solvr_room_members', { slug: SLUG });
    await callTool(tools, 'solvr_room_add_member', { slug: SLUG, agent_id: 'agent_tester' });

    expect(server.sent.map((r) => [r.method, r.path])).toEqual([
      ['POST', `/v1/rooms/${SLUG}/handshake`],
      ['GET', `/v1/rooms/${SLUG}/members`],
      ['POST', `/v1/rooms/${SLUG}/members`],
    ]);
    for (const sent of server.sent) {
      expect(sent.headers.authorization).toBe(`Bearer ${API_KEY}`);
    }
  });

  it('with no API key send no Authorization header, and the API answer is reported', async () => {
    const server = await serve((_r, w) => json(w, 401, refusal('UNAUTHORIZED', 'authentication required')));
    const tools = new SolvrTools(null, server.url);

    const listed = await callTool(tools, 'solvr_room_members', { slug: SLUG });
    const added = await callTool(tools, 'solvr_room_add_member', { slug: SLUG, agent_id: 'agent_x' });

    expect(server.sent).toHaveLength(2);
    for (const sent of server.sent) {
      expect(sent.headers.authorization).toBeUndefined();
    }
    for (const result of [listed, added]) {
      expect(result.isError).toBe(true);
      expect(text(result)).toContain('UNAUTHORIZED: authentication required');
    }
  });
});

describe('member tool definitions', () => {
  const manifest = new SolvrTools(API_KEY, 'http://127.0.0.1:1').getManifest();
  const tool = (name: string) => {
    const found = manifest.tools.find((t) => t.name === name);
    if (!found) throw new Error(`tools/list serves no ${name}`);
    return found;
  };

  it('solvr_room_members takes the slug only', () => {
    const members = tool('solvr_room_members');
    expect(Object.keys(members.inputSchema.properties)).toEqual(['slug']);
    expect(members.inputSchema.required).toEqual(['slug']);
  });

  it('solvr_room_add_member takes the slug, the agent id and an optional owner or member role', () => {
    const add = tool('solvr_room_add_member');
    expect(Object.keys(add.inputSchema.properties)).toEqual(['slug', 'agent_id', 'role']);
    expect(add.inputSchema.required).toEqual(['slug', 'agent_id']);
    expect(add.inputSchema.properties.role.enum).toEqual(['owner', 'member']);
  });

  it('OPERATION_TOOLS names the tool of each membership operation, a contract operation like the others', () => {
    const members = { listRoomMembers: OPERATION_TOOLS.listRoomMembers, addRoomMember: OPERATION_TOOLS.addRoomMember };
    expect(members).toEqual({ listRoomMembers: 'solvr_room_members', addRoomMember: 'solvr_room_add_member' });
    for (const name of Object.values(members)) {
      expect(manifest.tools.map((t) => t.name)).toContain(name);
    }
  });
});

// The member types carry the fields of the schemas the API publishes (backend
// openapi_member_paths.go RoomMember and AddRoomMemberRequest, pinned there to the stored model).
describe('member types', () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const source = ts.createSourceFile('room-types.ts', readFileSync(join(here, '../room-types.ts'), 'utf8'), ts.ScriptTarget.ES2022);

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
    return decl.members.map((m) => `${m.name?.getText(source)}${m.questionToken ? '?' : ''}`).sort();
  }

  it('RoomMember has every field of the published RoomMember, each required', () => {
    expect(properties('RoomMember')).toEqual(['added_by', 'agent_id', 'created_at', 'role', 'room_id']);
  });

  it('AddRoomMemberInput is agent_id and an optional role', () => {
    expect(properties('AddRoomMemberInput')).toEqual(['agent_id', 'role?']);
  });

  it('a role is owner or member', () => {
    const decl = declared('RoomRole');
    if (!ts.isTypeAliasDeclaration(decl) || !ts.isUnionTypeNode(decl.type)) throw new Error('RoomRole is not a union');
    expect(decl.type.types.map((t) => t.getText(source)).sort()).toEqual(["'member'", "'owner'"]);
  });

  it('the answers are the published envelopes', () => {
    expect(properties('RoomMemberResponse')).toEqual(['data']);
    expect(properties('RoomMembersResponse')).toEqual(['data']);
  });
});
