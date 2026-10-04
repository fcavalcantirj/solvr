import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { sdks } from './api-sdks';
import { ApiQuickstart } from './api-quickstart';

// idx 52: the /api-docs samples teach the canonical write shapes (a post has no
// type; every contribution is a reply) and call only what the Go SDK (packages/sdk-go)
// and the Go CLI (cli/cmd/solvr) actually export — each sample is checked against the
// source, never against a hand-written list.

const PACKAGES = resolve(__dirname, '../../../packages');
const CLI = resolve(__dirname, '../../../cli/cmd/solvr');
const read = (p: string) => readFileSync(resolve(PACKAGES, p), 'utf8');
const example = (language: string) => {
  const sdk = sdks.find((s) => s.language === language);
  if (!sdk) throw new Error(`no ${language} sample`);
  return sdk.code;
};

// The text between the parentheses of the `callee(` call starting at or after `from`.
function callArgs(code: string, callee: string, from = 0): string {
  const start = code.indexOf(callee + '(', from);
  if (start < 0) throw new Error(`${callee}( not found`);
  let depth = 0;
  for (let i = start + callee.length; i < code.length; i++) {
    if (code[i] === '(') depth++;
    if (code[i] === ')' && --depth === 0) return code.slice(start + callee.length + 1, i);
  }
  throw new Error(`${callee}( is not closed`);
}

const calls = (code: string, receiver: string) =>
  Array.from(code.matchAll(new RegExp(`\\b${receiver}\\.(\\w+)\\(`, 'g')), (m) => m[1]);

const LEGACY_WRITES = /\b(approach|answer|approach_angle|angle|include)\b|\/answers|\/approaches/;

describe('/api-docs SDK samples use the canonical write shapes the SDKs export (idx 52)', () => {
  it('Go: every client.* call is a Client method and every solvr.T{} literal uses fields of T', () => {
    const code = example('Go');
    const sdk = readdirSync(resolve(PACKAGES, 'sdk-go'))
      .filter((f) => f.endsWith('.go') && !f.endsWith('_test.go'))
      .map((f) => read(`sdk-go/${f}`))
      .join('\n');
    const methods = calls(code, 'client');
    expect(methods).toContain('CreatePost');
    expect(methods).toContain('CreateReply');
    for (const m of methods) expect(sdk).toMatch(new RegExp(`func \\(c \\*Client\\) ${m}\\(`));
    for (const f of Array.from(code.matchAll(/\bsolvr\.(\w+)\(/g), (x) => x[1])) {
      expect(sdk).toMatch(new RegExp(`\\nfunc ${f}\\(`));
    }

    const literals = Array.from(code.matchAll(/solvr\.(\w+)\{([^}]*)\}/g));
    expect(literals.map((l) => l[1])).toContain('CreatePostRequest');
    for (const [, type, body] of literals) {
      const struct = sdk.match(new RegExp(`type ${type} struct \\{([^}]*)\\}`));
      expect(struct, `solvr.${type} is not an SDK struct`).not.toBeNull();
      for (const [, field] of body.matchAll(/(\w+):/g)) {
        expect(struct![1], `solvr.${type}.${field}`).toMatch(new RegExp(`\\n\\s*${field}\\s`));
      }
    }
    expect(literals.find((l) => l[1] === 'CreatePostRequest')![2]).not.toMatch(/\bType:/);
    expect(code).not.toMatch(LEGACY_WRITES);
  });

  it('CLI: every solvr command and flag exists in the Go CLI; post takes no type', () => {
    const code = example('CLI');
    const cli = readdirSync(CLI)
      .filter((f) => f.endsWith('.go') && !f.endsWith('_test.go'))
      .map((f) => readFileSync(resolve(CLI, f), 'utf8'))
      .join('\n');
    const invocations = code
      .replace(/\\\n\s*/g, ' ')
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l.startsWith('solvr '));
    const commands = invocations.map((l) => l.split(/\s+/)[1]);
    expect(commands).toContain('post');
    expect(commands).toContain('reply');
    for (const c of commands) expect(cli, `solvr ${c}`).toMatch(new RegExp(`Use:\\s+"${c}[ "]`));
    for (const flag of invocations.flatMap((l) => l.match(/--[\w-]+/g) ?? [])) {
      expect(cli, flag).toMatch(new RegExp(`Flags\\(\\)\\.\\w+\\(&\\w+, "${flag.slice(2)}"`));
    }
    // 0.2.0 keeps the removed 0.1 command and flags only to refuse them.
    const removedCommands = [...cli.matchAll(/removedCommand\("([\w-]+)"/g)].map((m) => m[1]);
    const removedFlags = [...cli.matchAll(/removeFlag\(\w+, "([\w-]+)"/g)].map((m) => `--${m[1]}`);
    expect(removedCommands).toEqual(['answer']);
    expect(removedFlags).toEqual(expect.arrayContaining(['--type', '--include']));
    for (const c of commands) expect(removedCommands, `solvr ${c} was removed`).not.toContain(c);
    for (const l of invocations) {
      for (const flag of l.match(/--[\w-]+/g) ?? []) expect(removedFlags, `${l}: ${flag} was removed`).not.toContain(flag);
    }
    expect(invocations.find((l) => l.startsWith('solvr post '))).toMatch(/^solvr post\s+--/);
    expect(code).not.toMatch(LEGACY_WRITES);
  });
});

// Every install line on /api-docs works today: the Go SDK is the module in packages/sdk-go, and
// the CLI is the main package in cli/cmd/solvr, so `go install` gives a binary named solvr.
// Nothing unpublished (an npm or PyPI package) is offered.
describe('/api-docs install lines', () => {
  it('install only what exists in this repository', () => {
    expect(sdks.map((s) => s.language)).toEqual(['Go', 'CLI']);
    for (const s of sdks) expect(s.install).not.toMatch(/^(npm|npx|pip|yarn|pnpm)\b/);

    const sdkModule = read('sdk-go/go.mod').match(/^module (\S+)/)![1];
    const go = sdks.find((s) => s.language === 'Go')!;
    expect(go.install).toBe(`go get ${sdkModule}`);
    expect(go.package).toBe(sdkModule);
    expect(go.code).toContain(`solvr "${sdkModule}"`);

    const cliModule = readFileSync(resolve(CLI, '../../go.mod'), 'utf8').match(/^module (\S+)/)![1];
    const cli = sdks.find((s) => s.language === 'CLI')!;
    expect(cli.package).toBe(cliModule);
    expect(cli.install).toBe(`go install ${cliModule}/cmd/solvr@latest`);
    expect(readFileSync(resolve(CLI, 'main.go'), 'utf8')).toMatch(/^package main\n/);
  });
});

describe('/api-docs quickstart teaches the canonical post (idx 52)', () => {
  it('contributes back with a post that has no type, through a real SDK method', () => {
    const { container } = render(<ApiQuickstart />);
    const text = container.textContent ?? '';
    const args = callArgs(text, 'solvr.post');
    expect(args).toContain('title');
    expect(args).not.toMatch(/\btype\s*:/);
  });
});
