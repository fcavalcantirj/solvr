import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MessageBubble } from './message-bubble';
import type { APIRoomMessage } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/components/shared/markdown-content', () => ({
  MarkdownContent: ({ content }: { content: string }) => <div data-testid="markdown-content">{content}</div>,
}));

const agentMessage: APIRoomMessage = {
  id: 1,
  room_id: 'room-1',
  author_type: 'agent',
  author_id: 'agent-uuid-123',
  agent_name: 'TestBot',
  content: 'Hello from agent',
  content_type: 'text',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

const agentMessageNoId: APIRoomMessage = {
  id: 2,
  room_id: 'room-1',
  author_type: 'agent',
  author_id: undefined,
  agent_name: 'AnonymousBot',
  content: 'Hello from anonymous agent',
  content_type: 'text',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

const agentMarkdownMessage: APIRoomMessage = {
  id: 3,
  room_id: 'room-1',
  author_type: 'agent',
  author_id: 'agent-uuid-456',
  agent_name: 'MarkdownBot',
  content: '**bold content**',
  content_type: 'markdown',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

const humanMessage: APIRoomMessage = {
  id: 4,
  room_id: 'room-1',
  author_type: 'human',
  author_id: 'user-uuid-789',
  agent_name: 'JohnDoe',
  content: 'Hello from human',
  content_type: 'text',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

const systemMessage: APIRoomMessage = {
  id: 5,
  room_id: 'room-1',
  author_type: 'system',
  author_id: undefined,
  agent_name: 'system',
  content: 'Room created',
  content_type: 'text',
  metadata: {},
  created_at: '2026-01-01T00:00:00Z',
};

describe('MessageBubble', () => {
  describe('agent messages', () => {
    it('renders agent message with Bot icon area and agent_name text', () => {
      render(<MessageBubble message={agentMessage} />);
      expect(screen.getByText('TestBot')).toBeInTheDocument();
    });

    // v1.3.7: an agent speaks on the page itself; the bubble is marked by who wrote
    // it, not by a blue tint (the old palette is gone).
    it('marks an agent message as the agent\'s, on the page itself', () => {
      const { container } = render(<MessageBubble message={agentMessage} />);
      const body = container.querySelector('[data-author="agent"]');
      expect(body).toBeInTheDocument();
      expect(container.querySelector('.bg-blue-50')).toBeNull();
    });

    it('renders agent message with author_id as link to /agents/{author_id}', () => {
      render(<MessageBubble message={agentMessage} />);
      const link = screen.getByRole('link', { name: 'TestBot' });
      expect(link).toHaveAttribute('href', '/agents/agent-uuid-123');
    });

    it('renders agent message without author_id as plain text (not a link)', () => {
      render(<MessageBubble message={agentMessageNoId} />);
      const nameElement = screen.getByText('AnonymousBot');
      expect(nameElement.tagName.toLowerCase()).not.toBe('a');
    });

    it('renders agent message with content_type=markdown using MarkdownContent', () => {
      render(<MessageBubble message={agentMarkdownMessage} />);
      expect(screen.getByTestId('markdown-content')).toBeInTheDocument();
    });

    // v1.3.9: agents write Markdown whatever type they declare, so a room renders every
    // spoken message through the shared renderer (no raw "##" in the transcript).
    it('renders a content_type=text message through MarkdownContent too', () => {
      render(<MessageBubble message={agentMessage} />);
      expect(screen.getByTestId('markdown-content')).toBeInTheDocument();
    });
  });

  describe('human messages', () => {
    // v1.3.9: every message is a ledger row, the author on the left and the words on the right.
    it('sets a human message as a ledger row: the author, then the words', () => {
      const { container } = render(<MessageBubble message={humanMessage} />);
      const row = container.querySelector('[data-message-id]')!;
      expect(row.children).toHaveLength(2);
      expect(row.children[0]).toHaveTextContent(humanMessage.agent_name);
      expect(row.querySelector('.ml-auto')).toBeNull();
    });

    it('sets an agent message as the same ledger row', () => {
      const { container } = render(<MessageBubble message={agentMessage} />);
      const row = container.querySelector('[data-message-id]')!;
      expect(row.children).toHaveLength(2);
      expect(row.children[0]).toHaveTextContent(agentMessage.agent_name);
    });

    // v1.3.7: a human's interjection sits on the quiet secondary fill, not a green tint.
    it('marks a human message as the human\'s, on the quiet fill', () => {
      const { container } = render(<MessageBubble message={humanMessage} />);
      const body = container.querySelector('[data-author="human"]');
      expect(body).toHaveClass('bg-secondary');
      expect(container.querySelector('.bg-green-50')).toBeNull();
    });

    it('renders human message with author_id as link to /users/{author_id}', () => {
      render(<MessageBubble message={humanMessage} />);
      const link = screen.getByRole('link', { name: 'JohnDoe' });
      expect(link).toHaveAttribute('href', '/users/user-uuid-789');
    });
  });

  describe('system messages', () => {
    it('renders system message centered with no bubble (border-dashed)', () => {
      const { container } = render(<MessageBubble message={systemMessage} />);
      const systemEl = container.querySelector('.border-dashed');
      expect(systemEl).toBeInTheDocument();
    });

    it('renders system message content as plain text', () => {
      render(<MessageBubble message={systemMessage} />);
      expect(screen.getByText('Room created')).toBeInTheDocument();
    });
  });
});

// The homepage example links each beat to /rooms/<slug>#message-<sequence_num>.
// Without this anchor the link opens the room but lands nowhere in particular.
describe('MessageBubble deep-link anchor', () => {
  it('anchors an agent message by its sequence number', () => {
    const { container } = render(
      <MessageBubble message={{ ...agentMessage, sequence_num: 2 }} />,
    );
    expect(container.querySelector('#message-2')).not.toBeNull();
  });

  it('anchors a human message by its sequence number', () => {
    const { container } = render(
      <MessageBubble message={{ ...humanMessage, sequence_num: 7 }} />,
    );
    expect(container.querySelector('#message-7')).not.toBeNull();
  });

  it('clears the fixed header when the browser jumps to the anchor', () => {
    const { container } = render(
      <MessageBubble message={{ ...agentMessage, sequence_num: 3 }} />,
    );
    expect(container.querySelector('#message-3')?.className).toContain('scroll-mt-24');
  });

  it('adds no anchor when the message has no sequence number', () => {
    const { container } = render(<MessageBubble message={agentMessage} />);
    expect(container.querySelector('[id^="message-"]')).toBeNull();
  });
});

describe('MessageBubble stable id + highlight (deep links)', () => {
  it('exposes the persistent message id as a data attribute for scroll/fetch targeting', () => {
    const { container } = render(<MessageBubble message={agentMessage} />);
    expect(container.querySelector('[data-message-id="1"]')).not.toBeNull();
  });

  it('marks the message as highlighted when it is the deep-link target', () => {
    const { container } = render(<MessageBubble message={agentMessage} highlighted />);
    expect(container.querySelector('[data-highlighted="true"]')).not.toBeNull();
  });

  it('is not highlighted by default', () => {
    const { container } = render(<MessageBubble message={agentMessage} />);
    expect(container.querySelector('[data-highlighted="true"]')).toBeNull();
  });
});

describe('MessageBubble long-message collapse', () => {
  const longContent = 'x'.repeat(1300);
  const longMessage: APIRoomMessage = { ...agentMessage, content: longContent };

  it('collapses a long message behind an explicit expand control', () => {
    const { container } = render(<MessageBubble message={longMessage} />);
    expect(screen.getByRole('button', { name: /show more/i })).toBeInTheDocument();
    expect(container.querySelector('[data-collapsed="true"]')).not.toBeNull();
  });

  // v1.3.9: a collapsed message still shows a screenful (36rem), not a 288px sliver.
  it('collapses at a readable height', () => {
    const { container } = render(<MessageBubble message={longMessage} />);
    const collapsed = container.querySelector('[data-collapsed="true"]')!;
    expect(collapsed.className).toContain('max-h-[36rem]');
    expect(collapsed.className).not.toContain('max-h-72');
  });

  it('expands and re-collapses when the control is toggled', () => {
    const { container } = render(<MessageBubble message={longMessage} />);
    fireEvent.click(screen.getByRole('button', { name: /show more/i }));
    expect(container.querySelector('[data-collapsed="false"]')).not.toBeNull();
    expect(screen.getByRole('button', { name: /show less/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /show less/i }));
    expect(container.querySelector('[data-collapsed="true"]')).not.toBeNull();
  });

  it('does not add an expand control to a short message', () => {
    render(<MessageBubble message={agentMessage} />);
    expect(screen.queryByRole('button', { name: /show more/i })).not.toBeInTheDocument();
  });

  it('wraps long unbroken text so it cannot widen the page', () => {
    const { container } = render(<MessageBubble message={agentMessage} />);
    expect(container.querySelector('.break-words')).not.toBeNull();
  });

  // Review loop (step 5): an unverified completion claim is shown as the AUTHOR's
  // message, never dressed up by the platform as a certified/verified outcome.
  describe('completion claims are the author\'s claim, not platform-certified', () => {
    const completionClaim: APIRoomMessage = {
      id: 42,
      room_id: 'room-1',
      author_type: 'agent',
      author_id: 'agent-uuid-claim',
      agent_name: 'ExecutorBot',
      content: 'Done. All tests pass and the feature is complete.',
      content_type: 'text',
      metadata: {},
      created_at: '2026-01-01T00:00:00Z',
    };

    it('renders the claim as the author\'s message with the author identity', () => {
      render(<MessageBubble message={completionClaim} />);
      expect(screen.getByText('ExecutorBot')).toBeInTheDocument();
      expect(
        screen.getByText('Done. All tests pass and the feature is complete.'),
      ).toBeInTheDocument();
    });

    it('adds no platform certification badge to the message', () => {
      render(<MessageBubble message={completionClaim} />);
      // The platform must not stamp the message as a certified/verified outcome.
      expect(
        screen.queryByText(/certified|verified by solvr|platform[- ]verified|solvr[- ]approved/i),
      ).not.toBeInTheDocument();
    });
  });
});
