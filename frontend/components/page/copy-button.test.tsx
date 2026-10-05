import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import { CopyButton } from './copy-button';

// The copy control of the marketing pages said "Copied" the moment it was pressed,
// whether or not the browser let it write the clipboard. It now waits for the write
// and says "Copied" only when the text really is on the clipboard.

// Every Copy button says what it copies (SPEC.md 27.7); what it says is tested in
// components/code-copy.events.test.tsx.
const REPORT = { surface: 'skill', item: 'install_command' } as const;

const button = () => screen.getByRole('button');
const label = () => button().textContent;

function clipboard(writeText: unknown) {
  Object.defineProperty(navigator, 'clipboard', { value: writeText ? { writeText } : undefined, configurable: true });
}

beforeEach(() => {
  clipboard(vi.fn().mockResolvedValue(undefined));
});

afterEach(() => {
  vi.useRealTimers();
});

describe('CopyButton', () => {
  it('copies the text it was given and then says Copied', async () => {
    render(<CopyButton text="npx skills add solvr" report={REPORT} />);
    expect(label()).toBe('Copy');

    fireEvent.click(button());

    await waitFor(() => expect(label()).toBe('Copied'));
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('npx skills add solvr');
  });

  it('does not say Copied while the browser is still deciding', async () => {
    let allow: () => void = () => {};
    clipboard(vi.fn().mockReturnValue(new Promise<void>((resolve) => { allow = resolve; })));
    render(<CopyButton text="x" report={REPORT} />);

    fireEvent.click(button());
    await act(async () => {});
    expect(label()).toBe('Copy');

    await act(async () => allow());
    expect(label()).toBe('Copied');
  });

  it('never says Copied when the browser refuses the clipboard', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    clipboard(writeText);
    render(<CopyButton text="x" report={REPORT} />);

    fireEvent.click(button());
    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(label()).toBe('Copy');
  });

  it('never says Copied where there is no clipboard at all', async () => {
    clipboard(null);
    render(<CopyButton text="x" report={REPORT} />);

    fireEvent.click(button());
    await act(async () => {});

    expect(label()).toBe('Copy');
  });

  it('goes back to Copy two seconds after a successful copy', async () => {
    vi.useFakeTimers();
    render(<CopyButton text="x" report={REPORT} />);

    fireEvent.click(button());
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(label()).toBe('Copied');

    await act(async () => { await vi.advanceTimersByTimeAsync(1999); });
    expect(label()).toBe('Copied');
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(label()).toBe('Copy');
  });
});
