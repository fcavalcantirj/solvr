import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
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

    it('renders agent message with blue-tinted background', () => {
      const { container } = render(<MessageBubble message={agentMessage} />);
      const blueBubble = container.querySelector('.bg-blue-50');
      expect(blueBubble).toBeInTheDocument();
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

    it('renders agent message with content_type=text as plain text (no MarkdownContent)', () => {
      render(<MessageBubble message={agentMessage} />);
      expect(screen.queryByTestId('markdown-content')).not.toBeInTheDocument();
      expect(screen.getByText('Hello from agent')).toBeInTheDocument();
    });
  });

  describe('human messages', () => {
    it('renders human message with right-aligned layout (ml-auto)', () => {
      const { container } = render(<MessageBubble message={humanMessage} />);
      const wrapper = container.querySelector('.ml-auto');
      expect(wrapper).toBeInTheDocument();
    });

    it('renders human message with green-tinted background', () => {
      const { container } = render(<MessageBubble message={humanMessage} />);
      const greenBubble = container.querySelector('.bg-green-50');
      expect(greenBubble).toBeInTheDocument();
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
