import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import React from 'react';
import { CommentInput } from './comment-input';

// Mock useAuth hook
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock API client
vi.mock('@/lib/api', () => ({
  api: {
    postRoomMessage: vi.fn(),
  },
}));

const unauthenticatedUser = {
  user: null,
  isAuthenticated: false,
  isLoading: false,
};

const authenticatedUser = {
  user: { id: 'user-1', displayName: 'Alice', type: 'human' as const },
  isAuthenticated: true,
  isLoading: false,
};

describe('CommentInput', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('unauthenticated state', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue(unauthenticatedUser);
    });

    it('renders "Join the conversation" text when not authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      expect(screen.getByText('Join the conversation')).toBeInTheDocument();
    });

    it('renders LOG IN link when not authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const loginLink = screen.getByRole('link', { name: 'LOG IN' });
      expect(loginLink).toBeInTheDocument();
      expect(loginLink).toHaveAttribute('href', '/login');
    });

    it('does NOT render textarea when not authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    });

    it('does NOT render send button when not authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
  });

  describe('authenticated state', () => {
    beforeEach(() => {
      mockUseAuth.mockReturnValue(authenticatedUser);
    });

    it('renders textarea with placeholder "Type a message..." when authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const textarea = screen.getByPlaceholderText('Type a message...');
      expect(textarea).toBeInTheDocument();
    });

    it('renders send button when authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const button = screen.getByRole('button');
      expect(button).toBeInTheDocument();
    });

    it('renders user displayName when authenticated', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      expect(screen.getByText('Alice')).toBeInTheDocument();
    });

    it('send button is disabled when textarea is empty', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });

    it('send button is disabled when content is only whitespace', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const textarea = screen.getByPlaceholderText('Type a message...');
      fireEvent.change(textarea, { target: { value: '   ' } });
      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });

    it('character counter is NOT visible when content is short (< 1800 chars)', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const textarea = screen.getByPlaceholderText('Type a message...');
      fireEvent.change(textarea, { target: { value: 'short message' } });
      // Character counter should not be present
      expect(screen.queryByText(/\/ 2000 characters/)).not.toBeInTheDocument();
    });

    it('character counter IS visible when content length >= 1800 chars', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const textarea = screen.getByPlaceholderText('Type a message...');
      const longContent = 'a'.repeat(1800);
      fireEvent.change(textarea, { target: { value: longContent } });
      expect(screen.getByText(/\/ 2000 characters/)).toBeInTheDocument();
    });

    it('character counter shows red text when content length >= 2000 chars', () => {
      render(<CommentInput slug="test-room" onMessageSent={vi.fn()} />);
      const textarea = screen.getByPlaceholderText('Type a message...');
      const veryLongContent = 'a'.repeat(2000);
      fireEvent.change(textarea, { target: { value: veryLongContent } });
      const counter = screen.getByText(/\/ 2000 characters/);
      expect(counter).toBeInTheDocument();
      expect(counter.className).toMatch(/text-red-500/);
    });
  });
});
