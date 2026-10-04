import { describe, it, expect } from 'vitest';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { VERSION } from '../version.js';

// The official MCP client talks to this server as Claude Code and Cursor do: it validates the
// initialize answer (serverInfo {name, version}) and sends notifications/initialized, which
// must get no answer. No request reaches the API (tools/list is served locally).

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

describe('the official MCP SDK client', () => {
  it('connects over stdio, reads serverInfo and lists the tools', async () => {
    const transport = new StdioClientTransport({
      command: path.join(ROOT, 'node_modules/.bin/tsx'),
      args: [path.join(ROOT, 'src/index.ts')],
      cwd: ROOT,
      env: { PATH: process.env.PATH ?? '', SOLVR_API_KEY: 'solvr_handshake_test', SOLVR_API_URL: 'http://127.0.0.1:9' },
      stderr: 'ignore',
    });
    const client = new Client({ name: 'handshake-test', version: '0' });
    const errors: string[] = [];
    client.onerror = (error) => errors.push(error.message);
    try {
      await client.connect(transport);
      expect(client.getServerVersion()).toEqual({ name: 'solvr', version: VERSION });
      const { tools } = await client.listTools();
      expect(tools.map((tool) => tool.name)).toContain('solvr_search');
      expect(errors).toEqual([]);
    } finally {
      await client.close();
    }
  }, 30000);
});
