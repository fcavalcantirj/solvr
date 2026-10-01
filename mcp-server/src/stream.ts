/**
 * An open room stream (server-sent events), read event by event.
 */

import { ApiError } from './errors.js';
import type { RoomStreamEvent, RoomStreamFrame } from './room-types.js';

/** The events that end a stream; their data is {code, message}. */
const STREAM_ENDS = new Set(['access_revoked', 'credential_rotated']);

export class RoomStream {
  private readonly reader: ReadableStreamDefaultReader<Uint8Array>;
  private readonly decoder = new TextDecoder();
  private buffer = '';
  private closed = false;

  constructor(body: ReadableStream<Uint8Array>) {
    this.reader = body.getReader();
  }

  /**
   * The next event, skipping heartbeats; null once the stream is closed. A stream the API
   * ended because of the caller's access throws the ApiError of that code (status 0).
   */
  async next(): Promise<RoomStreamEvent | null> {
    let id = '';
    let event = '';
    const data: string[] = [];
    for (;;) {
      const line = await this.readLine();
      if (line === null) return null;
      if (line !== '') {
        if (line.startsWith(':')) continue;
        const colon = line.indexOf(':');
        const field = colon < 0 ? line : line.slice(0, colon);
        let value = colon < 0 ? '' : line.slice(colon + 1);
        if (value.startsWith(' ')) value = value.slice(1);
        if (field === 'id') id = value;
        else if (field === 'event') event = value;
        else if (field === 'data') data.push(value);
        continue;
      }
      if (data.length === 0) {
        id = '';
        event = '';
        continue;
      }
      return this.dispatch(id, event || 'message', data.join('\n'));
    }
  }

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    await this.reader.cancel().catch(() => undefined);
  }

  private dispatch(id: string, event: string, data: string): RoomStreamEvent {
    if (STREAM_ENDS.has(event)) {
      let end: { code?: string; message?: string; request_id?: string } = {};
      try {
        end = JSON.parse(data);
      } catch {
        // reported below
      }
      const code = end.code ?? event.toUpperCase();
      const message = end.message ?? `the stream ended with ${event}`;
      throw new ApiError(`Stream ended: ${code}: ${message}`, 0, code, end.request_id);
    }
    let frame: RoomStreamFrame;
    try {
      frame = JSON.parse(data) as RoomStreamFrame;
    } catch (error) {
      throw new Error(`failed to decode the ${event} frame: ${(error as Error).message}`);
    }
    return { id, event, frame };
  }

  /** The next line without its end of line, or null at the end of the stream. */
  private async readLine(): Promise<string | null> {
    for (;;) {
      if (this.closed) return null;
      const end = this.buffer.indexOf('\n');
      if (end >= 0) {
        const line = this.buffer.slice(0, end);
        this.buffer = this.buffer.slice(end + 1);
        return line.endsWith('\r') ? line.slice(0, -1) : line;
      }
      const chunk = await this.reader.read();
      if (chunk.done) {
        this.closed = true;
        return null;
      }
      this.buffer += this.decoder.decode(chunk.value, { stream: true });
    }
  }
}
