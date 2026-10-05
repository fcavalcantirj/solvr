import { describe, it, expect } from 'vitest';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';

// A page rendered on the server must never mint a flow (SPEC.md 25.6): its HTML can be
// cached and shared, and no browser step stands behind the code. GET /v1/connect mints a
// flow code with every answer unless it is asked not to (flow=none), so every reader of it
// is one of two things:
//
//   - the browser client (lib/api-base.ts, getConnectStart), called from modules that run
//     in a browser, which keep minting: that code is the visit's flow;
//   - a server-side read, which must ask for flow=none.
//
// GET /v1/connect/examples and GET /v1/rooms/{slug}/connect mint nothing and are not this
// test's concern.

const ROOT = path.resolve(__dirname, '..');
const BROWSER_CLIENT = 'lib/api-base.ts';

function sourceFiles(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name.startsWith('.')) continue;
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) sourceFiles(full, out);
    else if (/\.(ts|tsx|mjs)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name)) out.push(full);
  }
  return out;
}

const files = ['app', 'components', 'hooks', 'lib']
  .flatMap((dir) => sourceFiles(path.join(ROOT, dir)))
  .concat(path.join(ROOT, 'middleware.ts'))
  .map((full) => ({ file: path.relative(ROOT, full).split(path.sep).join('/'), text: readFileSync(full, 'utf8') }));

// A request for the contract itself: the path, then its query or the end of the string
// literal. Prose about the endpoint (a comment) is followed by a space or punctuation, and
// /v1/connect/examples by a slash.
const READS_CONNECT = /\/v1\/connect(?=[?`'"$])/;

describe('readers of GET /v1/connect', () => {
  it('outside the browser client, every read asks for flow=none', () => {
    const serverReads = files
      .filter(({ file }) => file !== BROWSER_CLIENT)
      .flatMap(({ file, text }) =>
        text
          .split('\n')
          .filter((line) => READS_CONNECT.test(line))
          .map((line) => ({ file, line })),
      );

    // The resume guide is one of them, so this is looking at real requests.
    expect(serverReads.map(({ file }) => file)).toContain('app/docs/guides/[slug]/page.tsx');
    for (const { file, line } of serverReads) {
      expect(line, `${file} reads GET /v1/connect on the server without flow=none`).toMatch(/[?&]flow=none\b/);
    }
  });

  it('the browser client itself never asks for flow=none: a visit gets its code', () => {
    const client = files.find(({ file }) => file === BROWSER_CLIENT);

    expect(client?.text).toMatch(READS_CONNECT);
    expect(client?.text).not.toContain('flow=none');
  });

  it('the browser client is called only from modules that run in a browser', () => {
    const callers = files.filter(({ file, text }) => file !== BROWSER_CLIENT && /\bgetConnectStart\s*\(/.test(text));

    expect(callers.map(({ file }) => file)).toContain('hooks/use-connect-start.ts');
    for (const { file, text } of callers) {
      expect(/^\s*(['"])use client\1/.test(text), `${file} calls getConnectStart and is not a client module`).toBe(true);
    }
  });
});
