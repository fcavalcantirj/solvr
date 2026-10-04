import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { CONNECT_START } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// The connection panel reports two BROWSER funnel steps: connection_started when
// its contract loads (the panel/page is meaningfully opened) and
// starter_prompt_copied only after the clipboard write succeeds. Both carry the
// flow id the API minted for this contract; the copied sentence itself never carries it.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

async function renderPanel(variant: 'panel' | 'page' = 'panel') {
  const result = render(<ConnectPanel variant={variant} />);
  await screen.findByTestId('connect-panel');
  return result;
}

describe('ConnectPanel connection funnel', () => {
  it('reports connection_started with the flow id when the /connect page loads', async () => {
    await renderPanel('page');

    await waitFor(() => {
      expect(vi.mocked(api.postFunnelEvent)).toHaveBeenCalledWith(
        expect.objectContaining({
          event: 'connection_started',
          flow_id: 'f_0123456789abcdef01234567',
          entry_surface: 'connect_page',
          preset: 'plan-and-build',
          instruction_version: '2.0',
        }),
      );
    });
  });

  it('labels the homepage panel with its own entry surface', async () => {
    await renderPanel('panel');

    await waitFor(() => {
      expect(vi.mocked(api.postFunnelEvent)).toHaveBeenCalledWith(
        expect.objectContaining({
          event: 'connection_started',
          entry_surface: 'homepage_panel',
        }),
      );
    });
  });

  it('reports starter_prompt_copied only after a successful copy', async () => {
    await renderPanel('page');
    vi.mocked(api.postFunnelEvent).mockClear();

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(CONNECT_START.prompt.text);
    });
    await waitFor(() => {
      expect(vi.mocked(api.postFunnelEvent)).toHaveBeenCalledWith(
        expect.objectContaining({
          event: 'starter_prompt_copied',
          flow_id: 'f_0123456789abcdef01234567',
          entry_surface: 'connect_page',
          preset: 'plan-and-build',
          role: 'planner',
        }),
      );
    });
  });

  it('does not report starter_prompt_copied when the clipboard write fails', async () => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
    });
    await renderPanel('page');
    vi.mocked(api.postFunnelEvent).mockClear();

    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    // Let the rejected clipboard promise settle.
    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalled();
    });
    const copiedCalls = vi
      .mocked(api.postFunnelEvent)
      .mock.calls.filter(([arg]) => arg.event === 'starter_prompt_copied');
    expect(copiedCalls).toHaveLength(0);
  });
});
