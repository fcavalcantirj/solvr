import { vi } from "vitest";
import { createServer } from "node:http";
import type { IncomingHttpHeaders, IncomingMessage, ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import { run } from "../program.js";

// Shared by the in-process CLI tests: a real local HTTP server that records what the CLI
// sent, and a runner that executes the CLI's own program with a hermetic configuration.

export interface Recorded {
  method: string;
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
    r.on("data", (chunk: Buffer) => chunks.push(chunk));
    r.on("end", () => {
      const url = new URL(r.url ?? "/", "http://cli.local");
      const recorded: Recorded = {
        method: r.method ?? "",
        path: url.pathname,
        query: [...url.searchParams.entries()],
        headers: r.headers,
        body: Buffer.concat(chunks).toString("utf8"),
      };
      sent.push(recorded);
      handler(recorded, w, r);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
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
  res.writeHead(status, { "Content-Type": "application/json", ...headers });
  res.end(JSON.stringify(body));
}

export interface CliResult {
  stdout: string;
  stderr: string;
  code: number;
}

export interface CliEnv {
  baseUrl: string;
  /** SOLVR_API_KEY; absent = no API key anywhere */
  apiKey?: string;
  /** The config file; absent = a fresh path no file exists at */
  configPath?: string;
}

/** A config path in a fresh temporary directory (no file yet). */
export function freshConfigPath(): string {
  return path.join(fs.mkdtempSync(path.join(os.tmpdir(), "solvr-cli-test-")), "config.json");
}

/**
 * Runs `solvr <args>` in this process with only the given configuration: no config file but
 * env.configPath, SOLVR_API_KEY only when env.apiKey is given, SOLVR_BASE_URL = env.baseUrl.
 */
export async function runCli(args: string[], env: CliEnv): Promise<CliResult> {
  const saved = {
    SOLVR_API_KEY: process.env.SOLVR_API_KEY,
    SOLVR_BASE_URL: process.env.SOLVR_BASE_URL,
    SOLVR_CONFIG_PATH: process.env.SOLVR_CONFIG_PATH,
  };
  const stdout: string[] = [];
  const stderr: string[] = [];
  const log = vi.spyOn(console, "log").mockImplementation((...a: unknown[]) => {
    stdout.push(a.map(String).join(" "));
  });
  const err = vi.spyOn(console, "error").mockImplementation((...a: unknown[]) => {
    stderr.push(a.map((v) => (typeof v === "string" ? v : JSON.stringify(v))).join(" "));
  });
  const write = vi.spyOn(process.stderr, "write").mockImplementation((chunk: string | Uint8Array) => {
    stderr.push(String(chunk).replace(/\n$/, ""));
    return true;
  });
  const writeOut = vi.spyOn(process.stdout, "write").mockImplementation((chunk: string | Uint8Array) => {
    stdout.push(String(chunk).replace(/\n$/, ""));
    return true;
  });
  try {
    process.env.SOLVR_BASE_URL = env.baseUrl;
    process.env.SOLVR_CONFIG_PATH = env.configPath ?? freshConfigPath();
    if (env.apiKey === undefined) {
      delete process.env.SOLVR_API_KEY;
    } else {
      process.env.SOLVR_API_KEY = env.apiKey;
    }
    const code = await run(["node", "solvr", ...args]);
    return { stdout: stdout.join("\n"), stderr: stderr.join("\n"), code };
  } finally {
    log.mockRestore();
    err.mockRestore();
    write.mockRestore();
    writeOut.mockRestore();
    for (const [name, value] of Object.entries(saved)) {
      if (value === undefined) {
        delete process.env[name];
      } else {
        process.env[name] = value;
      }
    }
  }
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
