import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { CONNECT_FLOW_ID, CONNECT_START, CONNECT_START_BUILD_AND_REVIEW } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// The connection panel is the ONE surface that starts a connection (v1.3.5). It renders
// the API's sentence by its segments, sends back what the visitor types or flips, swaps
// the use case among the sentences the API already sent, and copies the text it was given.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

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

const sentence = () => screen.getByTestId('prompt-sentence');

describe('ConnectPanel renders the API sentence', () => {
  it('shows exactly the text the API served, segment by segment', async () => {
    await renderPanel();
    expect(sentence().textContent).toBe(CONNECT_START.prompt.text);
  });

  it('gives each deciding word its own kind of token', async () => {
    await renderPanel();
    const s = sentence();
    const roles = [...s.querySelectorAll('[data-kind="role"]')].map((n) => n.textContent);
    expect(roles).toEqual(['PLANNER', 'EXECUTOR']);
    expect(s.querySelector('[data-kind="handoff"]')).toHaveTextContent('answer me with a prompt for the');
    expect(within(s).getByRole('textbox', { name: 'What should they do?' })).toHaveTextContent('work on what I tell you next');
    expect(within(s).getByRole('switch', { name: 'Private room' })).toHaveAttribute('aria-checked', 'false');
    // The skill link carries the visit's flow code, exactly as the API served it.
    const skill = `https://solvr.dev/skill.md?f=${CONNECT_FLOW_ID}`;
    expect(within(s).getByRole('link', { name: skill })).toHaveAttribute('href', skill);
  });

  it('offers every use case the API sent, its closing cell, and the line on what happens next', async () => {
    await renderPanel();
    const radios = screen.getAllByRole('radio');
    expect(radios.map((r) => (r as HTMLInputElement).value)).toEqual(['plan-and-build', 'collaborate', 'build-and-review']);
    expect(screen.getByTestId('role-pair-more')).toHaveTextContent('Your imagination');
    expect(screen.getByTestId('role-pair-more')).toHaveTextContent('Any number of agents');
    expect(screen.getByText(CONNECT_START.next)).toBeInTheDocument();
    expect(screen.getByText(`${CONNECT_START.prompt.word_count} words, plain text`)).toBeInTheDocument();
  });

  it('links the real example the API chose', async () => {
    await renderPanel();
    expect(screen.getByRole('link', { name: /watch two agents do it/i })).toHaveAttribute(
      'href',
      '/rooms/tictactoe-human-vs-computer-20260920',
    );
  });
});

describe('ConnectPanel copying', () => {
  it('copies exactly the plain text the API served', async () => {
    await renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(CONNECT_START.prompt.text));
    expect(await screen.findByRole('button', { name: /copied/i })).toBeInTheDocument();
  });

  it('does not report Copied and says how to copy by hand when the clipboard is denied', async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } });
    await renderPanel();
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/copy it by hand/i);
    expect(screen.queryByRole('button', { name: /copied/i })).not.toBeInTheDocument();
  });
});

describe('ConnectPanel sends every change back to the API', () => {
  it('asks the API for its own defaults instead of assuming any', async () => {
    await renderPanel();
    expect(api.getConnectStart).toHaveBeenCalledWith({});
  });

  it('re-reads the sentence when the visitor types the intent', async () => {
    await renderPanel();
    const slot = within(sentence()).getByRole('textbox');
    slot.textContent = 'ship the signup page';
    fireEvent.input(slot);
    // Every later read echoes the flow of the first answer, so the visit stays one flow.
    await waitFor(() =>
      expect(api.getConnectStart).toHaveBeenLastCalledWith({ intent: 'ship the signup page', flow: CONNECT_FLOW_ID }),
    );
  });

  it('re-reads the sentence with the other visibility when the visitor flips it', async () => {
    await renderPanel();
    fireEvent.click(within(sentence()).getByRole('switch'));
    await waitFor(() =>
      expect(api.getConnectStart).toHaveBeenLastCalledWith({ visibility: 'private', flow: CONNECT_FLOW_ID }),
    );
  });

  it('swaps the use case among the sentences it already has, without asking again', async () => {
    await renderPanel();
    const calls = vi.mocked(api.getConnectStart).mock.calls.length;
    fireEvent.click(screen.getByRole('radio', { name: /share context/i }));
    await waitFor(() => expect(sentence().textContent).toBe(CONNECT_START.presets[1].prompt.text));
    expect(within(sentence()).getAllByText('LEARNER')).toHaveLength(1);
    expect(screen.getByText(CONNECT_START.presets[1].next)).toBeInTheDocument();
    expect(api.getConnectStart).toHaveBeenCalledTimes(calls);
  });

  it('holds every use case in the same place, so switching moves nothing else', async () => {
    const { container } = await renderPanel();
    const held = container.querySelectorAll('p.prompt-sentence[aria-hidden="true"]');
    expect(held).toHaveLength(2);
    expect([...held].map((p) => p.textContent)).toEqual([
      CONNECT_START.presets[1].prompt.text,
      CONNECT_START.presets[2].prompt.text,
    ]);
  });

  it('renders the selected use case the API answered with', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START_BUILD_AND_REVIEW });
    await renderPanel();
    expect(sentence().textContent).toBe(CONNECT_START_BUILD_AND_REVIEW.prompt.text);
    expect(screen.getByRole('radio', { name: /build & review/i })).toBeChecked();
  });
});

describe('ConnectPanel states', () => {
  it('says it is loading before the first contract arrives', () => {
    vi.mocked(api.getConnectStart).mockReturnValue(new Promise(() => {}));
    render(<ConnectPanel />);
    expect(screen.getByTestId('connect-loading')).toBeInTheDocument();
  });

  it('reports a failure instead of inventing a sentence', async () => {
    vi.mocked(api.getConnectStart).mockRejectedValue(new Error('The API is down'));
    render(<ConnectPanel />);
    expect(await screen.findByRole('alert')).toHaveTextContent('The API is down');
    expect(screen.queryByTestId('prompt-sentence')).not.toBeInTheDocument();
  });

  // /connect states its own <h1> in the server HTML, so on the page the panel renders no
  // heading and the page keeps exactly one; opened inline on the index it titles itself.
  it('leaves the heading to the page on /connect, and titles itself as a section on the index', async () => {
    const page = await renderPanel('page');
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull();
    expect(screen.queryByRole('heading', { name: CONNECT_START.heading })).toBeNull();
    page.unmount();
    await renderPanel('panel');
    expect(screen.getByRole('heading', { level: 2, name: CONNECT_START.heading })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull();
  });
});

describe('ConnectPanel is a dumb terminal', () => {
  it('writes no sentence, no endpoint and no use case of its own', () => {
    for (const file of [
      'components/connect/connect-panel.tsx',
      'components/prompt/prompt.tsx',
      'components/prompt/prompt-sentence.tsx',
      'components/prompt/prompt-stack.tsx',
      'components/prompt/role-pair-switch.tsx',
      'hooks/use-connect-start.ts',
    ]) {
      const source = read(file);
      for (const forbidden of ['api.solvr.dev', 'Learn Solvr', 'skill.md', 'join it as the', 'PLANNER', 'EXECUTOR', 'is_private', "'plan-and-build'"]) {
        expect(source, `${file} contains ${forbidden}`).not.toContain(forbidden);
      }
    }
  });
});
