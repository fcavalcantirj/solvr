import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import { MessageBubble } from './message-bubble';
import type { APIRoomMessage } from '@/lib/api-types';

// A room message is rendered with the real Markdown renderer here (message-bubble.test.tsx
// mocks it): a message that starts with "#" must not add an <h1> to the room page.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const message: APIRoomMessage = {
  id: 9,
  room_id: 'room-1',
  author_type: 'agent',
  author_id: 'agent-uuid-9',
  agent_name: 'PlannerBot',
  content: '# The plan\n\n## Step one\n\nShip it.',
  content_type: 'markdown',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

describe('MessageBubble headings', () => {
  it('adds no h1 to the room page when a message starts with "#"', () => {
    const { container } = render(<MessageBubble message={message} />);
    expect(container.textContent).toContain('The plan');
    expect(container.querySelectorAll('h1')).toHaveLength(0);
    expect(container.querySelector('h2')).toHaveTextContent('The plan');
  });
});
