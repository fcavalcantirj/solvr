import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import { CONNECT_START, CONNECT_START_PRIVATE_COLLABORATE, CONNECT_START_BUILD_AND_REVIEW } from './connect-fixture';
import { ConnectPanel } from './connect-panel';

// The connection panel is the ONE surface that starts a connection. The index
// opens it inline and /connect renders the same component full-page, so every
// string, every option and the prompt itself come from GET /v1/connect and
// nothing is decided in the browser.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: { getConnectStart: vi.fn() },
}));

import { api } from '@/lib/api';

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START });
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

// renderPanel waits for the first contract to arrive.
async function renderPanel(variant: 'panel' | 'page' = 'panel') {
  const result = render(<ConnectPanel variant={variant} />);
  await screen.findByTestId('connect-panel');
  return result;
}

describe('ConnectPanel renders the API contract', () => {
  it('publishes the heading, the intro and the note the API wrote', async () => {
    await renderPanel();
    expect(screen.getByText(CONNECT_START.heading)).toBeInTheDocument();
    expect(screen.getByText(CONNECT_START.intro)).toBeInTheDocument();
    expect(screen.getByText(CONNECT_START.note)).toBeInTheDocument();
  });

  it('shows the optional task field with the API label, placeholder, note and bound', async () => {
    await renderPanel();
    const field = screen.getByLabelText(CONNECT_START.task_field.label);
    expect(field).toHaveAttribute('placeholder', CONNECT_START.task_field.placeholder);
    expect(field).toHaveAttribute('maxlength', String(CONNECT_START.task_field.max_chars));
    expect(screen.getByText(CONNECT_START.task_field.note)).toBeInTheDocument();
  });

  it('offers every preset and every visibility the API sent, with its explanation', async () => {
    await renderPanel();
    for (const option of [...CONNECT_START.presets, ...CONNECT_START.visibility_options]) {
      const control = screen.getByRole('radio', { name: new RegExp(option.label, 'i') });
      expect((control as HTMLInputElement).checked).toBe(option.selected);
      expect(screen.getByText(option.description)).toBeInTheDocument();
    }
  });

  it('names the two copy/paste actions in order, before any statistic has to be read', async () => {
    await renderPanel();
    const steps = within(screen.getByTestId('connect-steps')).getAllByRole('listitem');
    expect(steps).toHaveLength(CONNECT_START.steps.length);
    CONNECT_START.steps.forEach((step, i) => {
      expect(steps[i]).toHaveTextContent(step.label);
      expect(steps[i]).toHaveTextContent(step.detail);
    });
  });

  it('links the real example the API chose', async () => {
    await renderPanel();
    const link = screen.getByRole('link', { name: CONNECT_START.example.label });
    expect(link).toHaveAttribute('href', CONNECT_START.example.url);
    expect(screen.getByText(CONNECT_START.example.detail)).toBeInTheDocument();
  });

  it('renders the Add another agent control with its role prompt from the API', async () => {
    await renderPanel();
    expect(screen.getByText(CONNECT_START.add_agent.label)).toBeInTheDocument();
    expect(screen.getByText(CONNECT_START.add_agent.detail)).toBeInTheDocument();
    expect(screen.getByTestId('connect-add-agent-prompt')).toHaveTextContent(
      CONNECT_START.add_agent.role_prompt,
    );
  });

  it('renders the Customize section with advanced instructions and API examples', async () => {
    await renderPanel();
    expect(screen.getByText(CONNECT_START.customize.label)).toBeInTheDocument();
    expect(screen.getByText(CONNECT_START.customize.detail)).toBeInTheDocument();
    for (const instruction of CONNECT_START.customize.advanced_instructions) {
      expect(screen.getByText(instruction)).toBeInTheDocument();
    }
    expect(screen.getByTestId('connect-api-examples')).toBeInTheDocument();
    for (const example of CONNECT_START.customize.api_examples) {
      expect(screen.getByTestId('connect-api-examples').textContent).toContain(example);
    }
  });
});

describe('ConnectPanel copying', () => {
  it('copies the prompt with no task entered and explains where to paste it', async () => {
    await renderPanel();

    expect(screen.getByText(CONNECT_START.prompt.instruction)).toBeInTheDocument();
    expect(screen.getByText(CONNECT_START.prompt.next_step)).toBeInTheDocument();

    const copy = screen.getByRole('button', { name: CONNECT_START.prompt.label });
    fireEvent.click(copy);

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(CONNECT_START.prompt.text);
    });
    await screen.findByText(CONNECT_START.prompt.copied_label);
  });

  it('shows the prompt itself, so it can be read and selected without the clipboard', async () => {
    await renderPanel();
    expect(screen.getByTestId('connect-prompt-text')).toHaveTextContent('You are the PLANNER agent');
  });
});

describe('ConnectPanel sends every choice back to the API', () => {
  it('asks the API for its own defaults instead of assuming any', async () => {
    await renderPanel();
    expect(vi.mocked(api.getConnectStart)).toHaveBeenCalledWith({});
  });

  it('re-reads the prompt when the visitor types a task', async () => {
    await renderPanel();
    fireEvent.change(screen.getByLabelText(CONNECT_START.task_field.label), {
      target: { value: 'Port the billing job' },
    });
    await waitFor(
      () => {
        expect(vi.mocked(api.getConnectStart)).toHaveBeenCalledWith(
          expect.objectContaining({ task: 'Port the billing job' }),
        );
      },
      { timeout: 3000 },
    );
  });

  it('re-reads the prompt when the preset or the visibility changes, and renders what comes back', async () => {
    await renderPanel();

    // What the API answers while only the preset has changed: the room is
    // still public, because the visitor has not chosen otherwise yet.
    const collaboratePublic = {
      ...CONNECT_START_PRIVATE_COLLABORATE,
      visibility_options: CONNECT_START.visibility_options,
      selected: { ...CONNECT_START_PRIVATE_COLLABORATE.selected, visibility: 'public' },
    };
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: collaboratePublic });

    fireEvent.click(screen.getByRole('radio', { name: /Collaborate/i }));
    await waitFor(() => {
      expect(vi.mocked(api.getConnectStart)).toHaveBeenCalledWith(
        expect.objectContaining({ preset: 'collaborate' }),
      );
    });

    await screen.findByRole('radio', { name: /Collaborate/i, checked: true });
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START_PRIVATE_COLLABORATE });

    fireEvent.click(screen.getByRole('radio', { name: /Private/i }));
    await waitFor(() => {
      expect(vi.mocked(api.getConnectStart)).toHaveBeenCalledWith(
        expect.objectContaining({ visibility: 'private' }),
      );
    });

    // The copy control, the instruction and the prompt are the API's new ones,
    // not the first ones relabelled in the browser.
    await screen.findByRole('button', { name: CONNECT_START_PRIVATE_COLLABORATE.prompt.label });
    expect(screen.getByText(CONNECT_START_PRIVATE_COLLABORATE.prompt.instruction)).toBeInTheDocument();
    expect(screen.getByTestId('connect-prompt-text')).toHaveTextContent('You are the FIRST agent');
  });

  it('re-reads the prompt when the build-and-review preset is chosen', async () => {
    await renderPanel();

    // The API answers with the build-and-review variant while the room is still
    // public, because the visitor has not chosen otherwise yet.
    const buildAndReviewPublic = {
      ...CONNECT_START_BUILD_AND_REVIEW,
      visibility_options: CONNECT_START.visibility_options,
      selected: { ...CONNECT_START_BUILD_AND_REVIEW.selected, visibility: 'public' },
    };
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: buildAndReviewPublic });

    fireEvent.click(screen.getByRole('radio', { name: /Build and review/i }));
    await waitFor(() => {
      expect(vi.mocked(api.getConnectStart)).toHaveBeenCalledWith(
        expect.objectContaining({ preset: 'build-and-review' }),
      );
    });

    // The builder prompt replaces the planner prompt.
    await screen.findByRole('button', { name: CONNECT_START_BUILD_AND_REVIEW.prompt.label });
    expect(screen.getByText(CONNECT_START_BUILD_AND_REVIEW.prompt.instruction)).toBeInTheDocument();
    expect(screen.getByTestId('connect-prompt-text')).toHaveTextContent('You are the BUILDER agent');
  });

  it('renders the build-and-review add-agent role as a reviewer', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValue({ data: CONNECT_START_BUILD_AND_REVIEW });
    await renderPanel();

    expect(screen.getByText(CONNECT_START_BUILD_AND_REVIEW.add_agent.label)).toBeInTheDocument();
    expect(screen.getByTestId('connect-add-agent-prompt')).toHaveTextContent('reviewer');
  });
});

describe('ConnectPanel states', () => {
  it('says it is loading before the first contract arrives', () => {
    vi.mocked(api.getConnectStart).mockReturnValue(new Promise(() => {}));
    render(<ConnectPanel />);
    expect(screen.getByTestId('connect-loading')).toBeInTheDocument();
  });

  it('reports a failure instead of inventing a prompt', async () => {
    vi.mocked(api.getConnectStart).mockRejectedValue(new Error('connection contract unavailable'));
    render(<ConnectPanel />);
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('connection contract unavailable');
    expect(screen.queryByTestId('connect-prompt-text')).not.toBeInTheDocument();
  });
});

describe('ConnectPanel variants', () => {
  it('points the compact panel at the full page, which stays directly linkable', async () => {
    await renderPanel('panel');
    const link = screen.getByRole('link', { name: CONNECT_START.page_label });
    expect(link).toHaveAttribute('href', CONNECT_START.page_url);
  });

  it('does not link the page to itself', async () => {
    await renderPanel('page');
    expect(screen.queryByRole('link', { name: CONNECT_START.page_label })).not.toBeInTheDocument();
  });

  it('titles the page variant as the page heading and the panel as a section', async () => {
    const { unmount } = await renderPanel('page');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(CONNECT_START.heading);
    unmount();

    await renderPanel('panel');
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(CONNECT_START.heading);
  });
});

describe('ConnectPanel is a dumb terminal', () => {
  it('writes no prompt, no endpoint and no visibility rule of its own', () => {
    const source = read('components/connect/connect-panel.tsx');
    for (const forbidden of [
      'api.solvr.dev',
      'agents/register',
      'is_private',
      'Copy planner prompt',
      'Paste this into your planner',
      'plan-and-build',
    ]) {
      expect(source).not.toContain(forbidden);
    }
  });
});
