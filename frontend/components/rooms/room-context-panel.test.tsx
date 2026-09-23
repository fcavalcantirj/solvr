import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { RoomContextPanel } from './room-context-panel';
import type { APIRoomMessage } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

function makeMessage(overrides: Partial<APIRoomMessage> = {}): APIRoomMessage {
  return {
    id: 1,
    room_id: 'r1',
    author_type: 'agent',
    agent_name: 'planner',
    content: 'TASK: build tic-tac-toe',
    content_type: 'text',
    metadata: {},
    sequence_num: 1,
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('RoomContextPanel (task 33, step 4)', () => {
  // Nothing to summarise -> render nothing rather than an empty box.
  it('renders nothing when there is no task and no pinned directive', () => {
    const { container } = render(<RoomContextPanel />);
    expect(container.firstChild).toBeNull();
  });

  // The initial task is the room's first message; the reader sees it without
  // scrolling to the top of a long transcript.
  it('shows the initial task with a label', () => {
    render(<RoomContextPanel initialTask={makeMessage({ content: 'build tic-tac-toe first' })} />);
    expect(screen.getByText(/build tic-tac-toe first/)).toBeInTheDocument();
    // The label element renders literally "Task" (the content does not).
    expect(screen.getByText('Task')).toBeInTheDocument();
  });

  // The latest pinned directive is surfaced separately from the initial task.
  it('shows the latest pinned directive with a label', () => {
    render(
      <RoomContextPanel
        latestPinned={makeMessage({ id: 9, sequence_num: 9, content: 'DIRECTIVE v2: validate input first', pinned_at: new Date().toISOString() })}
      />,
    );
    expect(screen.getByText(/validate input first/)).toBeInTheDocument();
    expect(screen.getByText(/pinned directive/i)).toBeInTheDocument();
  });

  // Each context item can be expanded IN PLACE — a toggle button, never a
  // navigation away from the room.
  it('expands and collapses without leaving the room', () => {
    render(<RoomContextPanel initialTask={makeMessage()} />);
    const toggle = screen.getByRole('button', { name: /expand/i });
    // A button (not a link) keeps the reader in the room.
    expect(toggle.tagName).toBe('BUTTON');
    fireEvent.click(toggle);
    expect(screen.getByRole('button', { name: /collapse/i })).toBeInTheDocument();
  });

  // Each item links back to its exact message in the transcript by sequence anchor.
  it('links the pinned directive to its message in the conversation', () => {
    render(
      <RoomContextPanel latestPinned={makeMessage({ id: 9, sequence_num: 9, content: 'DIRECTIVE' })} />,
    );
    const link = screen.getByRole('link', { name: /view in conversation/i });
    expect(link).toHaveAttribute('href', '#message-9');
  });
});
