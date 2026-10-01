import { createServer } from 'node:http';
import type { IncomingHttpHeaders, IncomingMessage, ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { handleRequest } from '../server.js';
import { SolvrTools } from '../tools.js';
import type { ToolResult } from '../tools.js';

// Shared by the in-process MCP tests: a real local HTTP server that records what the MCP
// server sent, and a caller that runs a tool through the server's own JSON-RPC handler.

export interface Recorded {
  method: string;
  /** The path as sent (percent-escapes kept) */
  path: string;
  query: [string, string][];
  headers: IncomingHttpHeaders;
  body: string;
}

export type Handler = (req: Recorded, res: ServerResponse, raw: IncomingMessage) => void;

export interface LocalServer {
  url: string;
  sent: Recorded[];
  close: () => Promise<void>;
}

/** Starts a local server; handler answers each request after its body was read. */
export async function startServer(handler: Handler): Promise<LocalServer> {
  const sent: Recorded[] = [];
  const server = createServer((r, w) => {
    const chunks: Buffer[] = [];
    r.on('data', (chunk: Buffer) => chunks.push(chunk));
    r.on('end', () => {
      const url = new URL(r.url ?? '/', 'http://mcp.local');
      const recorded: Recorded = {
        method: r.method ?? '',
        path: url.pathname,
        query: [...url.searchParams.entries()],
        headers: r.headers,
        body: Buffer.concat(chunks).toString('utf8'),
      };
      sent.push(recorded);
      handler(recorded, w, r);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  const close = () =>
    new Promise<void>((resolve) => {
      server.closeAllConnections();
      server.close(() => resolve());
    });
  return { url: `http://127.0.0.1:${port}`, sent, close };
}

/** Answers a JSON body. */
export function json(res: ServerResponse, status: number, body: unknown, headers: Record<string, string> = {}): void {
  res.writeHead(status, { 'Content-Type': 'application/json', ...headers });
  res.end(JSON.stringify(body));
}

/** Rejects when promise takes longer than ms: a hang fails the test instead of wedging the run. */
export function within<T>(ms: number, promise: Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`no answer within ${ms} ms`)), ms);
    promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error: unknown) => {
        clearTimeout(timer);
        reject(error);
      }
    );
  });
}

let nextId = 0;

/** Calls a tool the way an MCP client does: a tools/call request through the server's handler. */
export async function callTool(tools: SolvrTools, name: string, args: Record<string, unknown>): Promise<ToolResult> {
  const response = await within(
    10_000,
    handleRequest({ jsonrpc: '2.0', id: ++nextId, method: 'tools/call', params: { name, arguments: args } }, tools)
  );
  if (response.error) {
    throw new Error(`tools/call ${name} answered a JSON-RPC error: ${JSON.stringify(response.error)}`);
  }
  return response.result as ToolResult;
}

/** The text of a tool result. */
export function text(result: ToolResult): string {
  return result.content.map((c) => c.text).join('\n');
}
