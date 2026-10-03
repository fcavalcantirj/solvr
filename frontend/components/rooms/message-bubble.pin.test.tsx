import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import type { APIRoomMessage } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock('@/components/shared/markdown-content', () => ({
  MarkdownContent: ({ content }: { content: string }) => <div>{content}</div>,
}));

import { MessageBubble } from './message-bubble';

// Pinning a directive (idx 92): the control appears only when the API says this viewer
// may pin, and it names the action the message's own state calls for.
const msg = (over: Partial<APIRoomMessage> = {}): APIRoomMessage => ({
  id: 7,
  room_id: 'r',
  author_type: 'agent',
  agent_name: 'planner',
  content: 'Directive: ship it',
  content_type: 'text',
  metadata: {},
  created_at: '2026-09-01T00:00:00Z',
  ...over,
});

describe('MessageBubble pin control', () => {
  it('is absent unless the viewer may pin', () => {
    render(<MessageBubble message={msg()} />);
    expect(screen.queryByRole('button', { name: /pin/i })).not.toBeInTheDocument();
  });

  it('pins an unpinned message', () => {
    const onTogglePin = vi.fn();
    render(<MessageBubble message={msg()} canPin onTogglePin={onTogglePin} />);
    fireEvent.click(screen.getByRole('button', { name: 'Pin as directive' }));
    expect(onTogglePin).toHaveBeenCalledWith(expect.objectContaining({ id: 7 }));
  });

  it('unpins a pinned message, human or agent', () => {
    const onTogglePin = vi.fn();
    render(
      <MessageBubble message={msg({ author_type: 'human', pinned_at: '2026-09-01T01:00:00Z' })} canPin onTogglePin={onTogglePin} />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'Unpin' }));
    expect(onTogglePin).toHaveBeenCalledTimes(1);
  });
});
