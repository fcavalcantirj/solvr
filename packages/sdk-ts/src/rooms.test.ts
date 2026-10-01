import { describe, it, expect } from 'vitest';
import { createServer } from 'node:http';
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { Solvr } from './client.js';
import { SolvrError } from './types.js';

// Room client behaviors the contract examples do not cover, against a local server (the
// SDK uses the real fetch): stream parsing, stream ends, resume, credential scoping.

type Handler = (req: IncomingMessage, res: ServerResponse) => void;

async function withServer(handler: Handler, run: (baseUrl: string, seen: IncomingMessage[]) => Promise<void>) {
  const seen: IncomingMessage[] = [];
  const server = createServer((req, res) => {
    seen.push(req);
    handler(req, res);
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  try {
    await run(`http://127.0.0.1:${port}`, seen);
  } finally {
    server.closeAllConnections();
    await new Promise<void>(resolve => server.close(() => resolve()));
  }
}

function eventStream(text: string): Handler {
  return (_req, res) => {
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    res.end(text);
  };
}

const frame = (id: number, content: string) => JSON.stringify({
  id, sequence: id, type: 'message', room_id: 'room-1', agent_name: 'planner',
  payload: { id, room_id: 'room-1', author_type: 'agent', agent_name: 'planner', content, content_type: 'text',
    metadata: {}, sequence_num: id, created_at: '2026-10-01T18:41:41.203117Z' },
  timestamp: '2026-10-01T18:41:41.203117Z',
});

describe('streamRoom', () => {
  it('skips heartbeats and retry lines, joins multi-line data, reads CRLF, and tracks the last event id', async () => {
    const text = ': heartbeat\n\nretry: 3000\n\n'
      + `id: 7\r\nevent: message\r\ndata: ${frame(7, 'plan')}\r\n\r\n`
      + 'event: presence_join\ndata: {"type":"presence_join",\ndata: "room_id":"room-1","agent_name":"executor","timestamp":"2026-10-01T18:41:41Z"}\n\n'
      + `id: 8\ndata:${frame(8, 'built')}\n\n`;
    await withServer(eventStream(text), async baseUrl => {
      const stream = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).streamRoom('demo', { lastEventId: '6' });
      expect(stream.lastEventId).toBe('6');

      const first = await stream.next();
      expect(first?.id).toBe('7');
      expect(first?.event).toBe('message');
      expect(first?.message?.content).toBe('plan');
      expect(stream.lastEventId).toBe('7');

      const presence = await stream.next();
      expect(presence?.event).toBe('presence_join');
      expect(presence?.id).toBe('');
      expect(presence?.frame.agent_name).toBe('executor');
      expect(presence?.message).toBeUndefined();
      expect(stream.lastEventId).toBe('7');

      const third = await stream.next();
      expect(third?.event).toBe('message');
      expect(third?.message?.content).toBe('built');
      expect(stream.lastEventId).toBe('8');

      expect(await stream.next()).toBeNull();
    });
  });

  it('is async-iterable until the server closes it', async () => {
    await withServer(eventStream(`id: 1\nevent: message\ndata: ${frame(1, 'a')}\n\nid: 2\nevent: message\ndata: ${frame(2, 'b')}\n\n`), async baseUrl => {
      const stream = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).streamRoom('demo');
      const contents: (string | undefined)[] = [];
      for await (const event of stream) {
        contents.push(event.message?.content);
      }
      expect(contents).toEqual(['a', 'b']);
    });
  });

  it.each([
    ['credential_rotated', 'CREDENTIAL_ROTATED'],
    ['access_revoked', 'ACCESS_REVOKED'],
  ])('ends with a SolvrError when the server ends access (%s)', async (event, code) => {
    const text = `id: 1\nevent: message\ndata: ${frame(1, 'a')}\n\nevent: ${event}\ndata: {"code":"${code}","message":"handshake again"}\n\n`;
    await withServer(eventStream(text), async baseUrl => {
      const stream = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).streamRoom('demo');
      expect((await stream.next())?.id).toBe('1');
      const error = await stream.next().catch((e: unknown) => e);
      expect(error).toBeInstanceOf(SolvrError);
      expect((error as SolvrError).code).toBe(code);
      expect((error as SolvrError).message).toBe('handshake again');
      expect((error as SolvrError).status).toBe(0);
    });
  });

  it('rejects a refused stream with the API error, its status and request id', async () => {
    const refuse: Handler = (_req, res) => {
      res.writeHead(401, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: { code: 'STREAM_TICKET_INVALID', message: 'mint a new one', request_id: 'req-1' } }));
    };
    await withServer(refuse, async (baseUrl, seen) => {
      const error = await new Solvr({ baseUrl, apiKey: null }).streamRoom('demo', { ticket: 'solvr_st_x' }).catch((e: unknown) => e);
      expect(error).toBeInstanceOf(SolvrError);
      expect((error as SolvrError).status).toBe(401);
      expect((error as SolvrError).code).toBe('STREAM_TICKET_INVALID');
      expect((error as SolvrError).requestId).toBe('req-1');
      expect(seen).toHaveLength(1);
    });
  });

  it('rejects a frame that is not JSON', async () => {
    await withServer(eventStream('id: 1\nevent: message\ndata: {not json\n\n'), async baseUrl => {
      const stream = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).streamRoom('demo');
      await expect(stream.next()).rejects.toThrow('message frame');
    });
  });

  it('sends the resume id, the filters and the room token, and accepts an event stream', async () => {
    await withServer(eventStream(''), async (baseUrl, seen) => {
      const agent = new Solvr({ baseUrl, apiKey: 'solvr_agent' });
      const stream = await agent.withRoomToken('solvr_rt_one').streamRoom('demo room', {
        lastEventId: '41', type: 'event', issue: 'ISSUE-1',
      });
      expect(await stream.next()).toBeNull();
      const req = seen[0];
      expect(req.url).toBe('/v1/rooms/demo%20room/stream?type=event&issue=ISSUE-1');
      expect(req.headers.authorization).toBe('Bearer solvr_rt_one');
      expect(req.headers['last-event-id']).toBe('41');
      expect(req.headers.accept).toBe('text/event-stream');
    });
  });

  it('closes a stream the server keeps open', async () => {
    const keepOpen: Handler = (_req, res) => {
      res.writeHead(200, { 'Content-Type': 'text/event-stream' });
      res.write(`id: 1\nevent: message\ndata: ${frame(1, 'a')}\n\n`);
    };
    await withServer(keepOpen, async baseUrl => {
      const stream = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).streamRoom('demo');
      expect((await stream.next())?.id).toBe('1');
      await stream.close();
      expect(await stream.next()).toBeNull();
    });
  }, 5_000);
});

describe('credentials and room paths', () => {
  const ok: Handler = (_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ data: [], meta: { has_more: false, limit: 50 } }));
  };

  it('withRoomToken returns a client that presents the room token and leaves the agent client alone', async () => {
    await withServer(ok, async (baseUrl, seen) => {
      const agent = new Solvr({ baseUrl, apiKey: 'solvr_agent' });
      await agent.withRoomToken('solvr_rt_one').listRoomEntries('demo');
      await agent.listRoomEntries('demo');
      expect(seen.map(r => r.headers.authorization)).toEqual(['Bearer solvr_rt_one', 'Bearer solvr_agent']);
    });
  });

  it('refuses an empty room token', () => {
    expect(() => new Solvr({ apiKey: 'solvr_agent' }).withRoomToken('')).toThrow('Room token is required');
  });

  it('an anonymous client (apiKey null) sends no Authorization', async () => {
    await withServer(ok, async (baseUrl, seen) => {
      await new Solvr({ baseUrl, apiKey: null }).listRoomEntries('demo');
      expect(seen[0].headers.authorization).toBeUndefined();
    });
  });

  it('escapes the slug and sends the timeline filters', async () => {
    await withServer(ok, async (baseUrl, seen) => {
      await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).listRoomEntries('a/b', {
        cursor: 'c1', limit: 10, kind: 'event', issue: 'ISSUE-1',
      });
      expect(seen[0].url).toBe('/v1/rooms/a%2Fb/entries?cursor=c1&limit=10&kind=event&issue=ISSUE-1');
    });
  });

  it('search omits an empty query and sends the sort', async () => {
    await withServer(ok, async (baseUrl, seen) => {
      await new Solvr({ baseUrl, apiKey: null }).search('', { sort: 'newest' });
      expect(seen[0].url).toBe('/v1/search?sort=newest');
    });
  });

  it('updateReply sends If-Match and surfaces the new ETag', async () => {
    const edited: Handler = (_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json', ETag: '"2"' });
      res.end(JSON.stringify({ data: { id: 'r1', body: 'new' } }));
    };
    await withServer(edited, async (baseUrl, seen) => {
      const reply = await new Solvr({ baseUrl, apiKey: 'solvr_agent' }).updateReply('r1', '"1"', { body: 'new' });
      expect(seen[0].headers['if-match']).toBe('"1"');
      expect(seen[0].method).toBe('PATCH');
      expect(reply.etag).toBe('"2"');
    });
  });
});
