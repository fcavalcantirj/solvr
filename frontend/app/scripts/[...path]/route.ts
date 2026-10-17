// /scripts/* is where an old prompt or an agent's guess looks for Solvr's install script
// (SPEC.md 18.3). Solvr needs no install, so nothing is served here: every path answers 404
// in plain text, as a shell script that says what to read instead and exits 1. An agent that
// saves and runs it gets that sentence and an error, never an HTML page to execute.
const BODY =
  '#!/bin/sh\necho "This is not a script. Solvr needs no install: read https://solvr.dev/skill.md (plain HTTPS)." >&2\nexit 1\n';

export function GET(): Response {
  return new Response(BODY, {
    status: 404,
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
}
