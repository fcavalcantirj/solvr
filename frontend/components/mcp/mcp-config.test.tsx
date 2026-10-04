import { render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { McpHero } from './mcp-hero';
import { McpSetup } from './mcp-setup';
import { ApiMcp } from '../api/api-mcp';

// The hosted MCP server is POST /v1/mcp on the API (MCP over HTTP). Every config and command
// the site offers points there; nothing names an unpublished package or a made-up mcp:// url.

const ENDPOINT = 'https://api.solvr.dev/v1/mcp';
const COMMAND = `claude mcp add --transport http solvr ${ENDPOINT}`;

const surfaces: [string, () => ReactElement][] = [
  ['/mcp hero', () => <McpHero />],
  ['/mcp setup', () => <McpSetup />],
  ['/api-docs MCP section', () => <ApiMcp />],
];

describe('MCP setup on /mcp and /api-docs', () => {
  let originalFetch: typeof globalThis.fetch;
  beforeEach(() => {
    originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true }) as unknown as typeof fetch;
  });
  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('the API serves MCP over HTTP at /v1/mcp', () => {
    const router = readFileSync(resolve(__dirname, '../../../backend/internal/api/router.go'), 'utf8');
    expect(router).toMatch(/r\.Route\("\/v1", func[\s\S]*r\.Post\("\/mcp", mcpHandler\.Handle\)/);
  });

  for (const [name, ui] of surfaces) {
    it(`${name} points every client at the hosted endpoint and nothing else`, () => {
      const text = render(ui()).container.textContent ?? '';
      expect(text).toContain(ENDPOINT);
      expect(text).not.toMatch(/mcp:\/\/|@solvr\/mcp-server|solvr-mcp-server|\bnpx\b/);
    });
  }

  it('the config is an MCP HTTP server entry with the API key as a bearer header', () => {
    const { container } = render(<McpHero />);
    const config = [...container.querySelectorAll('code')].map((c) => c.textContent ?? '').find((t) => t.includes('mcpServers'));
    expect(config).toBeDefined();
    expect(JSON.parse(config!)).toEqual({
      mcpServers: {
        solvr: { type: 'http', url: ENDPOINT, headers: { Authorization: 'Bearer ${SOLVR_API_KEY}' } },
      },
    });
  });

  it('the Claude Code command adds the hosted server over the http transport', () => {
    expect(render(<McpHero />).container.textContent).toContain(COMMAND);
    const setup = render(<McpSetup />).container.textContent ?? '';
    expect(setup).toContain(COMMAND);
    expect(setup).toContain('--header "Authorization: Bearer $SOLVR_API_KEY"');
  });
});
