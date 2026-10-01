import { describe, it, expect, afterEach } from 'vitest';
import type { ServerResponse } from 'node:http';
import { SolvrTools } from '../tools.js';
import { callTool, json, startServer, text, within } from './harness.js';
import type { LocalServer, Recorded } from './harness.js';

// The room tools against a real local server: which credential each one presents, the room
// tokens a join keeps per room, what each tool sends and shows, and how a watch ends.

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

const entry = (id: number, body: string, extra: Record<string, unknown> = {}) => ({
  id,
  room_id: 'room-1',
  sequence: id,
  kind: 'message',
  author_type: 'agent',
  author_id: 'agent_planner',
  actor_label: 'agent_planner',
  body,
  content_type: 'text',
  extension: {},
  created_at: '2026-10-01T00:00:00Z',
  ...extra,
});

/** Answers a handshake with a token named after the request count, everything else with an entry. */
function roomApi(req: Recorded, res: ServerResponse, sent: Recorded[]) {
  if (req.path.endsWith('/handshake')) {
    const slug = decodeURIComponent(req.path.split('/')[3]);
    json(res, 201, { data: { agent_id: 'agent_planner', room_slug: slug, room_token: `solvr_rt_${sent.length}`, rotated: false } });
    return;
  }
  json(res, 201, { data: entry(7, 'hello'), meta: { idempotent_replay: false } });
}

describe('room tokens', () => {
  it('join presents the API key, sends {} and keeps the token; send presents that token, not the key', async () => {
    const server: LocalServer = await serve((r, w) => roomApi(r, w, server.sent));
    const tools = new SolvrTools(API_KEY, server.url);

    const joined = await callTool(tools, 'solvr_room_join', { slug: 'plan' });
    expect(joined.isError).toBeFalsy();
    expect(text(joined)).toContain('solvr_rt_1');
    expect(server.sent[0]).toMatchObject({ method: 'POST', path: '/v1/rooms/plan/handshake', body: '{}' });
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${API_KEY}`);

    const sent = await callTool(tools, 'solvr_room_send', { slug: 'plan', body: 'hello' });
    expect(sent.isError, text(sent)).toBeFalsy();
    expect(server.sent[1]).toMatchObject({ method: 'POST', path: '/v1/rooms/plan/entries' });
    expect(server.sent[1].headers.authorization).toBe('Bearer solvr_rt_1');
    expect(JSON.parse(server.sent[1].body)).toEqual({ body: 'hello' });
  });

  it('keeps one token per room; a later join (rotate, ttl) replaces it', async () => {
    const server: LocalServer = await serve((r, w) => roomApi(r, w, server.sent));
    const tools = new SolvrTools(API_KEY, server.url);

    await callTool(tools, 'solvr_room_join', { slug: 'one' });
    await callTool(tools, 'solvr_room_join', { slug: 'two' });
    await callTool(tools, 'solvr_room_ticket', { slug: 'one' });
    expect(server.sent[2].headers.authorization).toBe('Bearer solvr_rt_1');

    await callTool(tools, 'solvr_room_join', { slug: 'one', rotate: true, ttl_seconds: 600 });
    expect(JSON.parse(server.sent[3].body)).toEqual({ rotate: true, ttl_seconds: 600 });
    await callTool(tools, 'solvr_room_read', { slug: 'one' });
    await callTool(tools, 'solvr_room_read', { slug: 'two' });
    expect(server.sent[4].headers.authorization).toBe('Bearer solvr_rt_4');
    expect(server.sent[5].headers.authorization).toBe('Bearer solvr_rt_2');
  });

  it('read, send, ticket and watch without a token fail before any request and point at solvr_room_join', async () => {
    const server = await serve((_r, w) => json(w, 500, {}));
    const tools = new SolvrTools(API_KEY, server.url);
    for (const [name, args] of [
      ['solvr_room_read', { slug: 'plan' }],
      ['solvr_room_send', { slug: 'plan', body: 'x' }],
      ['solvr_room_ticket', { slug: 'plan' }],
      ['solvr_room_watch', { slug: 'plan' }],
    ] as const) {
      const result = await callTool(tools, name, args);
      expect(result.isError, name).toBe(true);
      expect(text(result), name).toContain('No room token for plan');
      expect(text(result), name).toContain('solvr_room_join');
    }
    expect(server.sent).toHaveLength(0);
  });

  it('a room_token argument beats the kept one; the slug is escaped', async () => {
    const server: LocalServer = await serve((r, w) => roomApi(r, w, server.sent));
    const tools = new SolvrTools(API_KEY, server.url);

    await callTool(tools, 'solvr_room_join', { slug: 'a b/c' });
    expect(server.sent[0].path).toBe('/v1/rooms/a%20b%2Fc/handshake');
    await callTool(tools, 'solvr_room_send', { slug: 'a b/c', body: 'x', room_token: 'solvr_rt_given' });
    expect(server.sent[1].path).toBe('/v1/rooms/a%20b%2Fc/entries');
    expect(server.sent[1].headers.authorization).toBe('Bearer solvr_rt_given');
  });

  it('a tool missing a required argument fails before any request', async () => {
    const server = await serve((_r, w) => json(w, 500, {}));
    const tools = new SolvrTools(API_KEY, server.url);
    const result = await callTool(tools, 'solvr_room_send', { slug: 'plan', room_token: 'solvr_rt_x' });
    expect(result.isError).toBe(true);
    expect(text(result)).toContain('body is required');
    const noSlug = await callTool(tools, 'solvr_room_join', {});
    expect(text(noSlug)).toContain('slug is required');
    expect(server.sent).toHaveLength(0);
  });
});

describe('room tools', () => {
  it('create sends only the fields given and shows the slug', async () => {
    const server = await serve((_r, w) =>
      json(w, 201, { data: { id: 'r1', slug: 'my-room', display_name: 'My room', tags: [], is_private: true, message_count: 0 } })
    );
    const tools = new SolvrTools(API_KEY, server.url);
    const result = await callTool(tools, 'solvr_room_create', { display_name: 'My room', is_private: true });
    expect(JSON.parse(server.sent[0].body)).toEqual({ display_name: 'My room', is_private: true });
    expect(server.sent[0].headers.authorization).toBe(`Bearer ${API_KEY}`);
    expect(text(result)).toContain('my-room');
    expect(text(result)).toContain('(private)');
    expect(text(result)).toContain('solvr_room_join');
  });

  it('read sends its filters and shows messages, events and the next cursor', async () => {
    const server = await serve((_r, w) =>
      json(w, 200, {
        data: [
          entry(1, 'Plan: build it'),
          { ...entry(2, ''), kind: 'event', body: undefined, event_type: 'task.claimed', issue: 'parser', actor_label: 'agent_exec' },
        ],
        meta: { limit: 2, has_more: true, next_cursor: 'c2' },
      })
    );
    const tools = new SolvrTools(API_KEY, server.url);
    const result = await callTool(tools, 'solvr_room_read', {
      slug: 'plan', room_token: 'solvr_rt_x', limit: 2, cursor: 'c1', kind: 'event', issue: 'parser',
    });
    expect(Object.fromEntries(server.sent[0].query)).toEqual({ limit: '2', cursor: 'c1', kind: 'event', issue: 'parser' });
    const shown = text(result);
    expect(shown).toContain('Plan: build it');
    expect(shown).toContain('[task.claimed]');
    expect(shown).toContain('agent_exec');
    expect(shown).toContain('cursor c2');
  });

  it('read shows an empty room as such', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: [], meta: { limit: 50, has_more: false, next_cursor: null } }));
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_read', { slug: 'plan', room_token: 'solvr_rt_x' });
    expect(text(result)).toContain('No entries yet');
  });

  it('send forwards reply_to_entry_id and addressed_member_ids and reports a replay', async () => {
    const server = await serve((_r, w) => json(w, 200, { data: entry(9, 'again'), meta: { idempotent_replay: true } }));
    const tools = new SolvrTools(API_KEY, server.url);
    const result = await callTool(tools, 'solvr_room_send', {
      slug: 'plan', room_token: 'solvr_rt_x', body: 'again', client_entry_id: 'c-1',
      reply_to_entry_id: 3, addressed_member_ids: ['agent_exec', 'agent_review'],
    });
    expect(JSON.parse(server.sent[0].body)).toEqual({
      body: 'again', client_entry_id: 'c-1', reply_to_entry_id: 3, addressed_member_ids: ['agent_exec', 'agent_review'],
    });
    expect(text(result)).toContain('Message 9 was already sent');
  });

  it('ticket shows the ticket and the stream it opens', async () => {
    const server = await serve((_r, w) =>
      json(w, 201, { data: { ticket: 'solvr_st_t', expires_at: '2026-10-01T00:01:00Z', ttl_seconds: 60, stream: '/v1/rooms/plan/stream' } })
    );
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_ticket', { slug: 'plan', room_token: 'solvr_rt_x' });
    expect(server.sent[0].body).toBe('');
    expect(text(result)).toContain('solvr_st_t');
    expect(text(result)).toContain('/v1/rooms/plan/stream');
  });
});

const frame = (id: number, content: string) =>
  `id: ${id}\nevent: message\ndata: ${JSON.stringify({
    id, sequence: id, type: 'message', room_id: 'room-1', agent_name: 'agent_planner',
    payload: { id, author_type: 'agent', agent_name: 'agent_planner', content, sequence_num: id, created_at: 't' },
    timestamp: 't',
  })}\n\n`;

describe('solvr_room_watch', () => {
  it('skips heartbeats, stops after max_events on a stream the server keeps open, and names the last event id', async () => {
    const server = await serve((_r, w) => {
      w.writeHead(200, { 'Content-Type': 'text/event-stream' });
      w.write(': heartbeat\n\n');
      w.write(frame(11, 'first'));
      w.write(': heartbeat\n\n');
      w.write(frame(12, 'second'));
      w.write(frame(13, 'third'));
      // never ends
    });
    const tools = new SolvrTools(API_KEY, server.url);
    const result = await within(5_000, callTool(tools, 'solvr_room_watch', {
      slug: 'plan', room_token: 'solvr_rt_x', max_events: 2, last_event_id: '10', event_type: 'message',
    }));
    expect(result.isError, text(result)).toBeFalsy();
    const shown = text(result);
    expect(shown).toContain('first');
    expect(shown).toContain('second');
    expect(shown).not.toContain('third');
    expect(shown).toContain('last_event_id 12');
    const req = server.sent[0];
    expect(req.headers.accept).toBe('text/event-stream');
    expect(req.headers['last-event-id']).toBe('10');
    expect(req.headers.authorization).toBe('Bearer solvr_rt_x');
    expect(Object.fromEntries(req.query)).toEqual({ type: 'message' });
  });

  it('answers what it has after wait_seconds on a quiet stream, not an error', async () => {
    const server = await serve((_r, w) => {
      w.writeHead(200, { 'Content-Type': 'text/event-stream' });
      w.write(': heartbeat\n\n');
    });
    const started = Date.now();
    const result = await within(5_000, callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_watch', {
      slug: 'plan', room_token: 'solvr_rt_x', wait_seconds: 1,
    }));
    expect(Date.now() - started).toBeLessThan(4_000);
    expect(result.isError, text(result)).toBeFalsy();
    expect(text(result)).toContain('No events within 1 s');
  });

  it('a stream the API ends for the caller (CREDENTIAL_ROTATED) is an error that keeps the events before it', async () => {
    const server = await serve((_r, w) => {
      w.writeHead(200, { 'Content-Type': 'text/event-stream' });
      w.write(frame(21, 'before the rotation'));
      w.end(`event: credential_rotated\ndata: ${JSON.stringify({ code: 'CREDENTIAL_ROTATED', message: 'this room token was rotated' })}\n\n`);
    });
    const result = await within(5_000, callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_watch', {
      slug: 'plan', room_token: 'solvr_rt_x', max_events: 5,
    }));
    expect(result.isError).toBe(true);
    expect(text(result)).toContain('before the rotation');
    expect(text(result)).toContain('CREDENTIAL_ROTATED: this room token was rotated');
  });

  it('a watch with a ticket is anonymous even with an API key and a kept room token', async () => {
    const server: LocalServer = await serve((r, w) => {
      if (r.path.endsWith('/handshake')) {
        roomApi(r, w, server.sent);
        return;
      }
      w.writeHead(200, { 'Content-Type': 'text/event-stream' });
      w.end(frame(31, 'seen with a ticket'));
    });
    const tools = new SolvrTools(API_KEY, server.url);
    await callTool(tools, 'solvr_room_join', { slug: 'plan' });
    const result = await callTool(tools, 'solvr_room_watch', { slug: 'plan', ticket: 'solvr_st_t' });
    expect(text(result)).toContain('seen with a ticket');
    expect(server.sent[1].headers.authorization).toBeUndefined();
    expect(Object.fromEntries(server.sent[1].query)).toEqual({ ticket: 'solvr_st_t' });
  });

  it('a stream that ends with no event says so', async () => {
    const server = await serve((_r, w) => {
      w.writeHead(200, { 'Content-Type': 'text/event-stream' });
      w.end(': heartbeat\n\n');
    });
    const result = await callTool(new SolvrTools(API_KEY, server.url), 'solvr_room_watch', { slug: 'plan', room_token: 'solvr_rt_x' });
    expect(result.isError).toBeFalsy();
    expect(text(result)).toContain('No events');
  });
});
