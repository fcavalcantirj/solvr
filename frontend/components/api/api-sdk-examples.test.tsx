import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { sdks } from './api-sdks';
import { ApiQuickstart } from './api-quickstart';

// idx 52: the /api-docs samples teach the canonical write shapes (a post has no
// type; every contribution is a reply) and call only what the first-party SDKs
// and CLI in packages/ actually export — each sample is checked against the
// SDK source, never against a hand-written list.

const PACKAGES = resolve(__dirname, '../../../packages');
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

// The arguments of every `callee(` call in the code.
const everyCallArgs = (code: string, callee: string) =>
  Array.from(code.matchAll(new RegExp(`\\b${callee.replace('.', '\\.')}\\(`, 'g')), (m) =>
    callArgs(code, callee, m.index),
  );

const calls = (code: string, receiver: string) =>
  Array.from(code.matchAll(new RegExp(`\\b${receiver}\\.(\\w+)\\(`, 'g')), (m) => m[1]);

const LEGACY_WRITES = /\b(approach|answer|approach_angle|angle|include)\b|\/answers|\/approaches/;

describe('/api-docs SDK samples use the canonical write shapes the SDKs export (idx 52)', () => {
  it('TypeScript: every solvr.* call is a Solvr method; post takes no type; contributions are replies', () => {
    const code = example('JavaScript / TypeScript');
    const client = read('sdk-ts/src/client.ts');
    const methods = calls(code, 'solvr');
    expect(methods).toContain('post');
    expect(methods).toContain('reply');
    for (const m of methods) expect(client).toMatch(new RegExp(`\\n  async ${m}\\(`));

    const input = read('sdk-ts/src/types.ts').match(/interface CreatePostInput \{([^}]*)\}/)![1];
    const keys = Array.from(callArgs(code, 'solvr.post').matchAll(/(\w+):/g), (m) => m[1]);
    expect(keys).not.toContain('type');
    for (const k of keys) expect(input).toMatch(new RegExp(`\\b${k}\\??:`));
    expect(code).not.toMatch(LEGACY_WRITES);
  });

  it('Python: every client.* call and keyword is in the SDK signature; post takes no type', () => {
    const code = example('Python');
    const client = read('sdk-python/solvr/client.py');
    const methods = calls(code, 'client');
    expect(methods).toContain('post');
    expect(methods).toContain('reply');
    for (const m of methods) {
      const signature = client.match(new RegExp(`\\n    def ${m}\\(([^)]*)\\)`));
      expect(signature, `client.${m} is not a Solvr method`).not.toBeNull();
      const params = Array.from(signature![1].matchAll(/(\w+)\s*:/g), (p) => p[1]);
      for (const args of everyCallArgs(code, `client.${m}`)) {
        for (const [, k] of args.matchAll(/(\w+)\s*=/g)) expect(params, `client.${m}(${k}=...)`).toContain(k);
      }
    }
    expect(callArgs(code, 'client.post')).not.toMatch(/\btype\s*=/);
    expect(code).not.toMatch(LEGACY_WRITES);
  });

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

  it('CLI: every solvr command and flag exists in @solvr/cli; post takes no type', () => {
    const code = example('CLI');
    // idx 78 slice 7: the commands are registered in program.ts and room-commands.ts.
    const cli = ['index.ts', 'program.ts', 'room-commands.ts'].map((f) => read(`cli/src/${f}`)).join('\n');
    const invocations = code
      .replace(/\\\n\s*/g, ' ')
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l.startsWith('solvr '));
    const commands = invocations.map((l) => l.split(/\s+/)[1]);
    expect(commands).toContain('post');
    expect(commands).toContain('reply');
    for (const c of commands) expect(cli).toMatch(new RegExp(`\\.command\\("${c}[ "]`));
    for (const flag of invocations.flatMap((l) => l.match(/--[\w-]+/g) ?? [])) {
      expect(cli, flag).toMatch(new RegExp(`${flag}[ "]`));
    }
    // idx 78 slice 18: 2.0.0 keeps the removed 1.x commands and options only to refuse them.
    const removedCommands = [...cli.matchAll(/removedCommand\(program, "([\w-]+)"/g)].map((m) => m[1]);
    const removedFlags = [...cli.matchAll(/removedOption\(\w+, "([^"]+)"/g)].flatMap((m) => m[1].match(/--[\w-]+/g) ?? []);
    expect(removedCommands).toEqual(['answer', 'approach']);
    expect(removedFlags).toEqual(expect.arrayContaining(['--type', '--status', '--include', '--criteria']));
    for (const c of commands) expect(removedCommands, `solvr ${c} was removed`).not.toContain(c);
    for (const l of invocations) {
      for (const flag of l.match(/--[\w-]+/g) ?? []) expect(removedFlags, `${l}: ${flag} was removed`).not.toContain(flag);
    }
    expect(invocations.find((l) => l.startsWith('solvr post '))).toMatch(/^solvr post\s+--/);
    expect(code).not.toMatch(LEGACY_WRITES);
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
