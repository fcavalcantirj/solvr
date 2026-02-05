/**
 * Tests for AuthorBadge component
 * Per PRD requirement: Show avatar, name, indicate human/agent type, link to profile
 */

import { render, screen } from '@testing-library/react';

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) {
    return <a href={href}>{children}</a>;
  };
});

import AuthorBadge from '../components/AuthorBadge';
import { PostAuthor } from '../lib/types';

const humanAuthor: PostAuthor = {
  type: 'human',
  id: 'user-123',
  display_name: 'John Developer',
  avatar_url: '/avatars/john.jpg',
};

const agentAuthor: PostAuthor = {
  type: 'agent',
  id: 'claude_assistant',
  display_name: 'Claude',
  avatar_url: '/avatars/claude.png',
};

const humanBackedAgent: PostAuthor = {
  type: 'agent',
  id: 'verified_agent',
  display_name: 'Verified Agent',
  avatar_url: '/avatars/verified.png',
  has_human_backed_badge: true,
  human_username: 'john_owner',
};

const agentWithBadgeNoUsername: PostAuthor = {
  type: 'agent',
  id: 'badge_agent',
  display_name: 'Badge Agent',
  has_human_backed_badge: true,
};

const authorNoAvatar: PostAuthor = {
  type: 'human',
  id: 'user-456',
  display_name: 'Jane Doe',
};

describe('AuthorBadge', () => {
  describe('basic rendering', () => {
    it('displays author name', () => {
      render(<AuthorBadge author={humanAuthor} />);
      expect(screen.getByText('John Developer')).toBeInTheDocument();
    });

    it('displays avatar image when provided', () => {
      render(<AuthorBadge author={humanAuthor} />);
      const avatar = screen.getByRole('img', { name: /john developer/i });
      expect(avatar).toBeInTheDocument();
      expect(avatar).toHaveAttribute('src', '/avatars/john.jpg');
    });

    it('displays avatar placeholder when no avatar_url', () => {
      render(<AuthorBadge author={authorNoAvatar} />);
      // Should show first letter as placeholder
      expect(screen.getByText('J')).toBeInTheDocument();
    });
  });

  describe('author type indication', () => {
    it('shows AI badge for agent authors', () => {
      render(<AuthorBadge author={agentAuthor} />);
      expect(screen.getByText('AI')).toBeInTheDocument();
    });

    it('does not show AI badge for human authors', () => {
      render(<AuthorBadge author={humanAuthor} />);
      expect(screen.queryByText('AI')).not.toBeInTheDocument();
    });
  });

  describe('profile linking', () => {
    it('links to user profile for humans', () => {
      render(<AuthorBadge author={humanAuthor} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/users/user-123');
    });

    it('links to agent profile for agents', () => {
      render(<AuthorBadge author={agentAuthor} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/agents/claude_assistant');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(<AuthorBadge author={humanAuthor} size="sm" />);
      expect(screen.getByText('John Developer')).toBeInTheDocument();
    });

    it('renders large size', () => {
      render(<AuthorBadge author={humanAuthor} size="lg" />);
      expect(screen.getByText('John Developer')).toBeInTheDocument();
    });
  });

  describe('showName option', () => {
    it('hides name when showName is false', () => {
      render(<AuthorBadge author={humanAuthor} showName={false} />);
      expect(screen.queryByText('John Developer')).not.toBeInTheDocument();
    });

    it('shows name by default', () => {
      render(<AuthorBadge author={humanAuthor} />);
      expect(screen.getByText('John Developer')).toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('avatar has alt text', () => {
      render(<AuthorBadge author={humanAuthor} />);
      const avatar = screen.getByRole('img');
      expect(avatar).toHaveAttribute('alt', 'John Developer');
    });

    it('link is accessible', () => {
      render(<AuthorBadge author={humanAuthor} />);
      const link = screen.getByRole('link');
      expect(link).toBeInTheDocument();
    });
  });

  describe('Human-Backed badge display', () => {
    it('shows Human-Backed badge for agents with has_human_backed_badge', () => {
      render(<AuthorBadge author={humanBackedAgent} />);
      expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
    });

    it('does not show Human-Backed badge for agents without it', () => {
      render(<AuthorBadge author={agentAuthor} />);
      expect(screen.queryByTestId('human-backed-badge')).not.toBeInTheDocument();
    });

    it('does not show Human-Backed badge for human authors', () => {
      render(<AuthorBadge author={humanAuthor} />);
      expect(screen.queryByTestId('human-backed-badge')).not.toBeInTheDocument();
    });

    it('shows Human-Backed badge even without human_username', () => {
      render(<AuthorBadge author={agentWithBadgeNoUsername} />);
      expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
    });

    it('shows both AI badge and Human-Backed badge for verified agents', () => {
      render(<AuthorBadge author={humanBackedAgent} />);
      expect(screen.getByText('AI')).toBeInTheDocument();
      expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
    });
  });
});
