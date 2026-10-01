/**
 * An open room stream (server-sent events), read with next() or for await.
 */

import type { RoomStreamEvent, RoomStreamFrame, RoomStreamMessage } from './room-types.js';
import { SolvrError } from './types.js';

/** The events that end a stream; their data is {code, message}. */
const STREAM_ENDS = new Set(['access_revoked', 'credential_rotated']);

export class RoomStream implements AsyncIterable<RoomStreamEvent> {
  private readonly reader: ReadableStreamDefaultReader<Uint8Array>;
  private readonly decoder = new TextDecoder();
  private buffer = '';
  private lastId: string;
  private closed = false;

  constructor(body: ReadableStream<Uint8Array>, lastEventId = '') {
    this.reader = body.getReader();
    this.lastId = lastEventId;
  }

  /**
   * The id of the last event received with one (or the one the stream resumed
   * from): reconnect with it as lastEventId to replay what was missed.
   */
  get lastEventId(): string {
    return this.lastId;
  }

  /**
   * Reads the next event, skipping heartbeats. Answers null once the stream is
   * closed (reconnect with lastEventId). A stream the server ended because the
   * caller's access did rejects with the SolvrError of that code
   * (CREDENTIAL_ROTATED: handshake again; ACCESS_REVOKED).
   */
  async next(): Promise<RoomStreamEvent | null> {
    let id = '';
    let event = '';
    let hasId = false;
    const data: string[] = [];
    for (;;) {
      const line = await this.readLine();
      if (line === null) {
        return null;
      }
      if (line !== '') {
        if (line.startsWith(':')) continue;
        const colon = line.indexOf(':');
        const field = colon < 0 ? line : line.slice(0, colon);
        let value = colon < 0 ? '' : line.slice(colon + 1);
        if (value.startsWith(' ')) value = value.slice(1);
        if (field === 'id') {
          id = value;
          hasId = true;
        } else if (field === 'event') {
          event = value;
        } else if (field === 'data') {
          data.push(value);
        }
        continue;
      }
      if (data.length === 0) {
        id = '';
        event = '';
        hasId = false;
        continue;
      }
      if (hasId) {
        this.lastId = id;
      }
      return this.dispatch(id, event || 'message', data.join('\n'));
    }
  }

  /** Closes the stream. */
  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    await this.reader.cancel().catch(() => undefined);
  }

  async *[Symbol.asyncIterator](): AsyncIterator<RoomStreamEvent> {
    try {
      for (;;) {
        const event = await this.next();
        if (event === null) return;
        yield event;
      }
    } finally {
      await this.close();
    }
  }

  private dispatch(id: string, event: string, data: string): RoomStreamEvent {
    if (STREAM_ENDS.has(event)) {
      let end: { code?: string; message?: string; request_id?: string } = {};
      try {
        end = JSON.parse(data);
      } catch {
        // falls through to the generic error below
      }
      if (!end.code) {
        throw new Error(`the stream ended with ${event}: ${data}`);
      }
      throw new SolvrError(end.message ?? event, 0, end.code, undefined, end.request_id);
    }
    let frame: RoomStreamFrame;
    try {
      frame = JSON.parse(data) as RoomStreamFrame;
    } catch (error) {
      throw new Error(`failed to decode the ${event} frame: ${(error as Error).message}`);
    }
    const message = frame.type === 'message' ? (frame.payload as RoomStreamMessage) : undefined;
    return message ? { id, event, frame, message } : { id, event, frame };
  }

  /** The next line without its end of line, or null at the end of the stream. */
  private async readLine(): Promise<string | null> {
    for (;;) {
      // Closed by close() or by the server (whose remaining buffer holds no whole line).
      if (this.closed) return null;
      const end = this.buffer.indexOf('\n');
      if (end >= 0) {
        const line = this.buffer.slice(0, end);
        this.buffer = this.buffer.slice(end + 1);
        return line.endsWith('\r') ? line.slice(0, -1) : line;
      }
      const chunk = await this.reader.read().catch((error: unknown) => {
        if (this.closed) return { done: true as const, value: undefined };
        throw error;
      });
      if (chunk.done) {
        this.closed = true;
        return null;
      }
      this.buffer += this.decoder.decode(chunk.value, { stream: true });
    }
  }
}
