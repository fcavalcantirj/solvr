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
    // solvr_claim is a tool of the unpublished stdio package only; the hosted server has no such tool.
    expect(screen.queryByText('solvr_claim')).not.toBeInTheDocument();

    // Wait for health check to settle
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  // idx 52 step 2: the hosted MCP server this section documents takes no post type and replaces
  // solvr_answer with solvr_reply; the listed tools follow what POST /v1/mcp serves (mcp.go).
  it('lists exactly the hosted MCP tools with their canonical parameters', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true }) as unknown as typeof fetch;
    const src = readFileSync(resolve(__dirname, '../../../backend/internal/api/handlers/mcp.go'), 'utf8');
    const served = [...src.matchAll(/^\t\t"name":\s+"(solvr_\w+)",$/gm)].map((m) => m[1]);

    render(<ApiMcp />);

    expect(served).toContain('solvr_reply');
    expect(served).toContain('solvr_room_watch');
    expect(screen.getAllByText(/^solvr_\w+$/).map((el) => el.textContent)).toEqual(served);
    expect(screen.getByText('title, description, tags?')).toBeInTheDocument();
    expect(screen.getByText('post_id, body, parent_reply_id?')).toBeInTheDocument();
    // solvr_get_reply also takes an id: look it up in the solvr_get row.
    expect(within(screen.getByText('solvr_get').closest('.bg-card') as HTMLElement).getByText('id')).toBeInTheDocument();
    expect(screen.queryByText(/approach_angle|include\?|problem, question, or idea/)).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  // idx 78 step 5: /v1/mcp 2.0.0 removed solvr_search's type filter (search covers every post).
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
    expect(screen.getByText('https://api.solvr.dev/v1/mcp')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('renders the hosted config and the Claude Code command', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
    }) as unknown as typeof fetch;

    render(<ApiMcp />);

    expect(screen.getByText('MCP CONFIG (CURSOR, .MCP.JSON)')).toBeInTheDocument();
    expect(screen.getByText('CLAUDE CODE')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });
});
