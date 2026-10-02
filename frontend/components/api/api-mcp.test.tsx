import { render, screen, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { ApiMcp } from './api-mcp';

describe('ApiMcp', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('renders MCP section with tools', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    expect(screen.getByText('MCP SERVER')).toBeInTheDocument();
    expect(screen.getByText('Model Context Protocol')).toBeInTheDocument();
    expect(screen.getByText('solvr_search')).toBeInTheDocument();
    expect(screen.getByText('solvr_get')).toBeInTheDocument();
    expect(screen.getByText('solvr_post')).toBeInTheDocument();
    expect(screen.getByText('solvr_reply')).toBeInTheDocument();
    expect(screen.queryByText('solvr_answer')).not.toBeInTheDocument();
    expect(screen.getByText('solvr_claim')).toBeInTheDocument();

    // Wait for health check to settle
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  // idx 52 step 2: the npm mcp-server this section documents takes no post type and replaces
  // solvr_answer with solvr_reply; the listed tools and parameters follow mcp-server/src/tools.ts.
  it('lists exactly the npm mcp-server tools with their canonical parameters', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true }) as unknown as typeof fetch;
    // idx 78: the room tools are defined in mcp-server/src/room-tools.ts (served after the others).
    const src = ['tools.ts', 'room-tools.ts']
      .map((file) => readFileSync(resolve(__dirname, '../../../mcp-server/src', file), 'utf8'))
      .join('\n');
    const served = [...src.matchAll(/^\s+name: '(solvr_\w+)',$/gm)].map((m) => m[1]);

    render(<ApiMcp />);

    expect(served).toContain('solvr_reply');
    expect(served).toContain('solvr_room_watch');
    expect(screen.getAllByText(/^solvr_\w+$/).map((el) => el.textContent)).toEqual(served);
    expect(screen.getByText('title, description, tags?, visibility?')).toBeInTheDocument();
    expect(screen.getByText('post_id, body, parent_reply_id?')).toBeInTheDocument();
    // solvr_get_reply also takes an id: look it up in the solvr_get row.
    expect(within(screen.getByText('solvr_get').closest('.bg-card') as HTMLElement).getByText('id')).toBeInTheDocument();
    expect(screen.queryByText(/approach_angle|include\?|problem, question, or idea/)).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  // idx 78 step 5: @solvr/mcp-server 2.0.0 removed solvr_search's type filter (search covers every post).
  it('documents solvr_search without the type filter 2.0.0 removed', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true }) as unknown as typeof fetch;

    render(<ApiMcp />);

    expect(screen.getByText('query, limit?, page?, sort?')).toBeInTheDocument();
    expect(screen.queryByText(/\btype\?/)).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('shows ONLINE status when health check succeeds', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });

    // Verify the health endpoint was called
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/health'),
      expect.anything()
    );
  });

  it('shows OFFLINE status when health check fails', async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error')) as unknown as typeof fetch;

    render(<ApiMcp />);

    await waitFor(() => {
      expect(screen.getByText('OFFLINE')).toBeInTheDocument();
    });
  });

  it('shows OFFLINE status when health check returns non-ok response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    await waitFor(() => {
      expect(screen.getByText('OFFLINE')).toBeInTheDocument();
    });
  });

  it('renders MCP server URL', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    expect(screen.getByText('MCP SERVER URL')).toBeInTheDocument();
    expect(screen.getByText('mcp://solvr.dev')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('renders cloud and self-hosted config blocks', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    expect(screen.getByText('CLOUD CONFIG (RECOMMENDED)')).toBeInTheDocument();
    expect(screen.getByText('SELF-HOSTED CONFIG')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });
});
