import { describe, it, expect } from 'vitest';
import { GET } from './[...path]/route';

// /scripts/* is not where Solvr's scripts live (SPEC.md 18.3): Solvr needs no install. Every
// path under it (/scripts/solvr.sh, /scripts/install.sh, ...) reaches this one handler, and an
// agent that saves and runs what it answers gets a sentence that says so and an exit code,
// never an HTML page it would try to execute.

const BODY =
  '#!/bin/sh\necho "This is not a script. Solvr needs no install: read https://solvr.dev/skill.md (plain HTTPS)." >&2\nexit 1\n';

describe('GET /scripts/*', () => {
  it('answers a 404 in plain text that tells the agent what to read instead', async () => {
    const res = GET();
    expect(res.status).toBe(404);
    expect(res.headers.get('content-type')).toBe('text/plain; charset=utf-8');
    expect(await res.text()).toBe(BODY);
  });
});
