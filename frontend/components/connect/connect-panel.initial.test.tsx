import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { renderToString } from 'react-dom/server';
import { hydrateRoot, type Root } from 'react-dom/client';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { CONNECT_FLOW_ID, CONNECT_START, CONNECT_START_NO_FLOW, connectStartFor } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// /connect hands the panel the default sentence its server read (`initial`: GET /v1/connect
// with flow=none, SPEC.md 25.6), and the panel shows it at once: the use cases, the sentence,
// the line beside it and Copy are in the server HTML. The browser still reads the contract
// exactly as before, which mints the visit's flow, and its answer replaces what is shown.
// connection_started waits for that answer; a copy made before it carries no flow id.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn(), postFunnelEvent: vi.fn() },
}));

import { api } from '@/lib/api';

type Answer = { data: typeof CONNECT_START };
// The browser's read is held until a test lets it arrive (or fail).
let arrive: (value: Answer) => void = () => {};
let fail: (error: Error) => void = () => {};

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockImplementation(
    () => new Promise((resolve, reject) => { arrive = resolve as (value: Answer) => void; fail = reject; }),
  );
  vi.mocked(api.postFunnelEvent).mockReset();
  vi.mocked(api.postFunnelEvent).mockResolvedValue(undefined);
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});
// A root hydrated by hand is not one testing-library cleans up: removed here, pass or fail.
let hydrated: { container: HTMLElement; root?: Root } | null = null;
afterEach(() => {
  if (hydrated) {
    const { container, root } = hydrated;
    act(() => root?.unmount());
    container.remove();
    hydrated = null;
  }
  window.history.replaceState(null, '', '/');
});

const PLAIN = 'https://solvr.dev/skill.md';
const sentence = () => screen.getByTestId('prompt-sentence');
const steps = (event: string) =>
  vi.mocked(api.postFunnelEvent).mock.calls.map(([step]) => step).filter((step) => step.event === event);
const browserAnswers = async (data: typeof CONNECT_START) => {
  await act(async () => arrive({ data }));
};
const renderPage = () => render(<ConnectPanel variant="page" initial={CONNECT_START_NO_FLOW} />);

describe('ConnectPanel with the sentence the server read', () => {
  it('shows it in its first render, then the browser\'s answer when it arrives', async () => {
    renderPage();

    // Nothing awaited: the first render already has the sentence, its use cases and Copy.
    expect(screen.queryByTestId('connect-loading')).toBeNull();
    expect(sentence().textContent).toBe(CONNECT_START_NO_FLOW.prompt.text);
    expect(within(sentence()).getByRole('link', { name: PLAIN })).toHaveAttribute('href', PLAIN);
    expect(screen.getAllByRole('radio').map((r) => (r as HTMLInputElement).value)).toEqual(['plan-and-build', 'collaborate', 'build-and-review']);
    expect(screen.getByText(CONNECT_START_NO_FLOW.next)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /copy prompt/i })).toBeInTheDocument();

    // The browser reads the contract as it always did: the API's defaults, no flow sent.
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({});

    await browserAnswers(CONNECT_START);
    expect(sentence().textContent).toBe(CONNECT_START.prompt.text);
    const coded = `${PLAIN}?f=${CONNECT_FLOW_ID}`;
    expect(within(sentence()).getByRole('link', { name: coded })).toHaveAttribute('href', coded);
  });

  it('reports connection_started once, only after the browser\'s read, with the flow id it minted', async () => {
    renderPage();
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(steps('connection_started')).toEqual([]);

    await browserAnswers(CONNECT_START);
    await waitFor(() => expect(steps('connection_started')).toHaveLength(1));
    expect(steps('connection_started')[0]).toMatchObject({
      flow_id: CONNECT_FLOW_ID,
      entry_surface: 'connect_page',
      preset: 'plan-and-build',
      instruction_version: '2.1',
    });

    // A later read of the same visit starts nothing new, and still echoes the one flow.
    fireEvent.click(within(sentence()).getByRole('switch', { name: 'Private room' }));
    await waitFor(() =>
      expect(api.getConnectStart).toHaveBeenLastCalledWith({ visibility: 'private', flow: CONNECT_FLOW_ID }),
    );
    await browserAnswers(connectStartFor('plan-and-build', '', 'private'));
    await waitFor(() => expect(within(sentence()).getByRole('switch')).toHaveAttribute('aria-checked', 'true'));
    expect(steps('connection_started')).toHaveLength(1);
  });

  it('copies the sentence shown when Copy comes first, and reports it without a flow id', async () => {
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(CONNECT_START_NO_FLOW.prompt.text));
    await waitFor(() => expect(steps('starter_prompt_copied')).toHaveLength(1));
    const early = steps('starter_prompt_copied')[0];
    expect(early).toMatchObject({ entry_surface: 'connect_page', preset: 'plan-and-build', role: 'planner', instruction_version: '2.1' });
    expect(early).not.toHaveProperty('flow_id');
    expect(steps('connection_started')).toEqual([]);

    // Once the browser's answer is shown, a copy carries the visit's flow, as before.
    await browserAnswers(CONNECT_START);
    await waitFor(() => expect(steps('connection_started')).toHaveLength(1));
    fireEvent.click(screen.getByRole('button', { name: /copy prompt/i }));
    await waitFor(() => expect(steps('starter_prompt_copied')).toHaveLength(2));
    expect(vi.mocked(navigator.clipboard.writeText).mock.calls[1][0]).toBe(CONNECT_START.prompt.text);
    expect(steps('starter_prompt_copied')[1]).toMatchObject({ flow_id: CONNECT_FLOW_ID });
  });

  it('shows a use case picked before the browser\'s read at once, and keeps it when the read arrives', async () => {
    renderPage();
    fireEvent.click(screen.getByRole('radio', { name: /build & review/i }));
    expect(sentence().textContent).toBe(CONNECT_START_NO_FLOW.presets[2].prompt.text);

    await browserAnswers(CONNECT_START);
    expect(sentence().textContent).toBe(CONNECT_START.presets[2].prompt.text);
    expect(screen.getByRole('radio', { name: /build & review/i })).toBeChecked();
  });

  // The server renders /connect without its query string (the page is static), so a linked
  // ?preset= is not in the server HTML: the default shows first, the browser asks for the
  // linked use case, and its answer replaces it. Hydration must find the HTML it rendered.
  it('hydrates the server HTML without a mismatch when the address carries ?preset=, then shows the linked use case', async () => {
    window.history.replaceState(null, '', '/connect');
    const html = renderToString(<ConnectPanel variant="page" initial={CONNECT_START_NO_FLOW} />);
    window.history.replaceState(null, '', '/connect?preset=collaborate');
    const container = document.createElement('div');
    container.innerHTML = html;
    document.body.appendChild(container);
    hydrated = { container };
    const shown = () => container.querySelector('[data-testid="prompt-sentence"]')?.textContent;

    const recoverable = vi.fn();
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      await act(async () => {
        hydrated!.root = hydrateRoot(container, <ConnectPanel variant="page" initial={CONNECT_START_NO_FLOW} />, {
          onRecoverableError: recoverable,
        });
      });
      expect(recoverable).not.toHaveBeenCalled();
      expect(consoleError).not.toHaveBeenCalled();
    } finally {
      consoleError.mockRestore();
    }
    expect(shown()).toBe(CONNECT_START_NO_FLOW.prompt.text);
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({ preset: 'collaborate' });

    const linked = connectStartFor('collaborate', '');
    await browserAnswers(linked);
    expect(shown()).toBe(linked.prompt.text);
  });

  it('keeps the sentence the server read when the browser\'s read fails, says so, and reports no start', async () => {
    renderPage();
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(1));
    await act(async () => fail(new Error('The API is down')));

    expect(await screen.findByRole('alert')).toHaveTextContent('The API is down');
    expect(sentence().textContent).toBe(CONNECT_START_NO_FLOW.prompt.text);
    expect(steps('connection_started')).toEqual([]);
  });

  // Until the browser's answer brings the code, every sentence's link holds its width empty
  // (reserveFlowCode), so the code arriving moves no word; then the code takes the place.
  it('holds the code\'s width in every link until the browser\'s answer, and never after', async () => {
    const { container } = renderPage();
    const placeholders = () => container.querySelectorAll('a[data-kind="link"] > span[aria-hidden="true"]');
    expect(placeholders()).toHaveLength(3);
    expect(sentence().textContent).toBe(CONNECT_START_NO_FLOW.prompt.text);

    // A use case picked meanwhile keeps it: the answer has still not arrived.
    fireEvent.click(screen.getByRole('radio', { name: /share context/i }));
    expect(placeholders()).toHaveLength(3);

    await browserAnswers(CONNECT_START);
    expect(placeholders()).toHaveLength(0);
    expect(within(sentence()).getByText(`?f=${CONNECT_FLOW_ID}`)).toBeInTheDocument();
  });

  it('holds nothing in the home page\'s panel, nor on the page when the server read nothing', async () => {
    const home = render(<ConnectPanel variant="panel" initial={CONNECT_START_NO_FLOW} />);
    expect(home.container.querySelectorAll('a[data-kind="link"] span[aria-hidden="true"]')).toHaveLength(0);
    home.unmount();

    const page = render(<ConnectPanel variant="page" initial={null} />);
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(2));
    await browserAnswers(CONNECT_START);
    expect(sentence().textContent).toBe(CONNECT_START.prompt.text);
    expect(page.container.querySelectorAll('a[data-kind="link"] span[aria-hidden="true"]')).toHaveLength(0);
  });

  it('shows the loading state, as before, when the server read nothing', async () => {
    render(<ConnectPanel variant="page" initial={null} />);
    expect(screen.getByTestId('connect-loading')).toHaveTextContent('READING THE SENTENCE...');
    expect(screen.queryByTestId('prompt-sentence')).toBeNull();

    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(1));
    await browserAnswers(CONNECT_START);
    expect(sentence().textContent).toBe(CONNECT_START.prompt.text);
    await waitFor(() => expect(steps('connection_started')).toHaveLength(1));
    expect(steps('connection_started')[0]).toMatchObject({ flow_id: CONNECT_FLOW_ID });
  });
});
