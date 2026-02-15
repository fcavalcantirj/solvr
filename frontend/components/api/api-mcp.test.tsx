import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
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
    expect(screen.getByText('solvr_answer')).toBeInTheDocument();
    expect(screen.getByText('solvr_claim')).toBeInTheDocument();

    // Wait for health check to settle
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
