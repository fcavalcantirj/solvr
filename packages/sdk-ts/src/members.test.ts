import { describe, it, expect } from 'vitest';
import { createServer } from 'node:http';
import type { IncomingMessage } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { Solvr } from './client.js';
import { SolvrError } from './types.js';

// A room's participants are a collection the owner reads (listRoomMembers) and adds to
// (addRoomMember): a third and any later agent is admitted to the SAME room, then joins it
// with its own handshakeRoom.

interface Sent {
  method?: string;
  url?: string;
  auth?: string;
  body: string;
}

async function withAnswer(status: number, answer: string, run: (baseUrl: string, sent: Sent[]) => Promise<void>) {
  const sent: Sent[] = [];
  const server = createServer((req: IncomingMessage, res) => {
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      sent.push({ method: req.method, url: req.url, auth: req.headers.authorization, body });
      res.writeHead(status, { 'Content-Type': 'application/json' });
      res.end(answer);
    });
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  try {
    await run(`http://127.0.0.1:${port}`, sent);
  } finally {
    server.closeAllConnections();
    await new Promise<void>(resolve => server.close(() => resolve()));
  }
}

const ROOM = '7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f';
const member = (agent_id: string, role: string, created_at: string) =>
  ({ room_id: ROOM, agent_id, role, added_by: 'planner', created_at });

describe('listRoomMembers', () => {
  it('reads the participants with the agent key, owners first, every field', async () => {
    const participants = [
      member('planner', 'owner', '2026-10-02T11:00:00Z'),
      member('executor', 'member', '2026-10-02T11:30:00Z'),
      member('reviewer', 'member', '2026-10-02T12:00:00Z'),
    ];
    await withAnswer(200, JSON.stringify({ data: participants }), async (baseUrl, sent) => {
      const result = await new Solvr({ baseUrl, apiKey: 'solvr_planner_key' }).listRoomMembers('handoff room/1');
      expect(sent).toHaveLength(1);
      expect(sent[0].method).toBe('GET');
      expect(sent[0].url).toBe('/v1/rooms/handoff%20room%2F1/members');
      expect(sent[0].auth).toBe('Bearer solvr_planner_key');
      expect(sent[0].body).toBe('');
      expect(result.data).toEqual(participants);
    });
  });
});

describe('addRoomMember', () => {
  const admitted = JSON.stringify({ data: member('reviewer', 'member', '2026-10-02T12:00:00Z') });

  it('admits an agent: without a role the body is the agent id alone', async () => {
    await withAnswer(201, admitted, async (baseUrl, sent) => {
      const result = await new Solvr({ baseUrl, apiKey: 'solvr_planner_key' }).addRoomMember('handoff room/1', { agent_id: 'reviewer' });
      expect(sent).toHaveLength(1);
      expect(sent[0].method).toBe('POST');
      expect(sent[0].url).toBe('/v1/rooms/handoff%20room%2F1/members');
      expect(sent[0].auth).toBe('Bearer solvr_planner_key');
      expect(JSON.parse(sent[0].body)).toEqual({ agent_id: 'reviewer' });
      expect(result.data).toEqual(member('reviewer', 'member', '2026-10-02T12:00:00Z'));
    });
  });

  it('sends the role it is given', async () => {
    await withAnswer(201, admitted, async (baseUrl, sent) => {
      await new Solvr({ baseUrl, apiKey: 'solvr_planner_key' }).addRoomMember('demo', { agent_id: 'reviewer', role: 'owner' });
      expect(JSON.parse(sent[0].body)).toEqual({ agent_id: 'reviewer', role: 'owner' });
    });
  });
});

describe('member errors', () => {
  const cases: [string, number, string, (c: Solvr) => Promise<unknown>][] = [
    ['a participant that is not an owner lists', 403, 'FORBIDDEN', c => c.listRoomMembers('demo')],
    ['an unknown agent is admitted', 400, 'INVALID_AGENT', c => c.addRoomMember('demo', { agent_id: 'ghost' })],
    ['the last owner is demoted', 409, 'LAST_OWNER', c => c.addRoomMember('demo', { agent_id: 'planner', role: 'member' })],
  ];
  for (const [name, status, code, call] of cases) {
    it(`${name}: ${status} ${code} is a SolvrError, sent once`, async () => {
      const answer = JSON.stringify({ error: { code, message: `${code} message`, request_id: 'req-7' } });
      await withAnswer(status, answer, async (baseUrl, sent) => {
        const error = await call(new Solvr({ baseUrl, apiKey: 'solvr_agent' })).catch((e: unknown) => e);
        expect(error).toBeInstanceOf(SolvrError);
        expect((error as SolvrError).status).toBe(status);
        expect((error as SolvrError).code).toBe(code);
        expect((error as SolvrError).message).toBe(`${code} message`);
        expect((error as SolvrError).requestId).toBe('req-7');
        expect(sent).toHaveLength(1);
      });
    });
  }
});

// The member types carry the fields of the schemas the API publishes (backend
// openapi_member_paths.go RoomMember and AddRoomMemberRequest, pinned there to the stored model).
describe('member types', () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const source = ts.createSourceFile('room-types.ts', readFileSync(join(here, 'room-types.ts'), 'utf8'), ts.ScriptTarget.ES2022);

  function declared(name: string): ts.InterfaceDeclaration | ts.TypeAliasDeclaration {
    const found = source.statements.find(s =>
      (ts.isInterfaceDeclaration(s) || ts.isTypeAliasDeclaration(s)) && s.name.text === name);
    if (!found || !(ts.isInterfaceDeclaration(found) || ts.isTypeAliasDeclaration(found))) {
      throw new Error(`room-types.ts declares no ${name}`);
    }
    return found;
  }

  /** "name" or "name?" for each property of an interface, sorted. */
  function properties(name: string): string[] {
    const decl = declared(name);
    if (!ts.isInterfaceDeclaration(decl)) throw new Error(`${name} is not an interface`);
    return decl.members.map(m => `${m.name?.getText(source)}${m.questionToken ? '?' : ''}`).sort();
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
    expect(decl.type.types.map(t => t.getText(source)).sort()).toEqual(["'member'", "'owner'"]);
  });

  it('the answers are the published envelopes', () => {
    expect(properties('RoomMemberResponse')).toEqual(['data']);
    expect(properties('RoomMembersResponse')).toEqual(['data']);
  });
});
