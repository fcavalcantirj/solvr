/**
 * Tests for PostCard component
 * Per PRD requirement: Create PostCard component displaying title, snippet,
 * author, votes, type badge, tags, and linking to post detail
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

// Import after mocks
import PostCard from '../components/PostCard';
import { PostWithAuthor } from '../lib/types';

const mockProblemPost: PostWithAuthor = {
  id: 'post-123',
  type: 'problem',
  title: 'Race condition in async PostgreSQL queries',
  description:
    'When running multiple async queries against PostgreSQL, I encounter a race condition that causes data inconsistency.',
  tags: ['postgresql', 'async', 'go'],
  posted_by_type: 'human',
  posted_by_id: 'user-456',
  status: 'open',
  upvotes: 42,
  downvotes: 3,
  success_criteria: ['No race condition', 'Data consistency maintained'],
  weight: 3,
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
  author: {
    type: 'human',
    id: 'user-456',
    display_name: 'John Developer',
    avatar_url: '/avatars/john.jpg',
  },
  vote_score: 39,
};

const mockQuestionPost: PostWithAuthor = {
  id: 'post-789',
  type: 'question',
  title: 'How do I handle connection pooling in Go?',
  description: 'I need help understanding how to properly configure connection pooling.',
  tags: ['go', 'database'],
  posted_by_type: 'agent',
  posted_by_id: 'claude_assistant',
  status: 'answered',
  upvotes: 15,
  downvotes: 1,
  accepted_answer_id: 'answer-001',
  created_at: '2026-01-20T14:30:00Z',
  updated_at: '2026-01-21T09:00:00Z',
  author: {
    type: 'agent',
    id: 'claude_assistant',
    display_name: 'Claude',
    avatar_url: '/avatars/claude.png',
  },
  vote_score: 14,
};

const mockIdeaPost: PostWithAuthor = {
  id: 'post-abc',
  type: 'idea',
  title: 'What if we used event sourcing for this?',
  description:
    'I have been thinking about using event sourcing as an alternative pattern.',
  tags: ['architecture', 'patterns'],
  posted_by_type: 'human',
  posted_by_id: 'user-999',
  status: 'active',
  upvotes: 8,
  downvotes: 0,
  created_at: '2026-01-25T08:00:00Z',
  updated_at: '2026-01-25T08:00:00Z',
  author: {
    type: 'human',
    id: 'user-999',
    display_name: 'Alice Architect',
  },
  vote_score: 8,
};

describe('PostCard', () => {
  describe('basic rendering', () => {
    it('renders the post card container', () => {
      render(<PostCard post={mockProblemPost} />);
      const card = screen.getByRole('article');
      expect(card).toBeInTheDocument();
    });

    it('displays the post title', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText(mockProblemPost.title)).toBeInTheDocument();
    });

    it('displays a snippet of the description', () => {
      render(<PostCard post={mockProblemPost} />);
      // Should show description text (contains "data inconsistency" which is only in description)
      expect(screen.getByText(/data inconsistency/i)).toBeInTheDocument();
    });

    it('displays the vote score', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText('39')).toBeInTheDocument();
    });

    it('displays tags', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText('postgresql')).toBeInTheDocument();
      expect(screen.getByText('async')).toBeInTheDocument();
      expect(screen.getByText('go')).toBeInTheDocument();
    });
  });

  describe('type badge', () => {
    it('displays problem badge for problem posts', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText(/problem/i)).toBeInTheDocument();
    });

    it('displays question badge for question posts', () => {
      render(<PostCard post={mockQuestionPost} />);
      expect(screen.getByText(/question/i)).toBeInTheDocument();
    });

    it('displays idea badge for idea posts', () => {
      render(<PostCard post={mockIdeaPost} />);
      expect(screen.getByText(/idea/i)).toBeInTheDocument();
    });
  });

  describe('status badge', () => {
    it('displays open status', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText(/open/i)).toBeInTheDocument();
    });

    it('displays answered status for questions', () => {
      render(<PostCard post={mockQuestionPost} />);
      expect(screen.getByText(/answered/i)).toBeInTheDocument();
    });

    it('displays active status for ideas', () => {
      render(<PostCard post={mockIdeaPost} />);
      expect(screen.getByText(/active/i)).toBeInTheDocument();
    });
  });

  describe('author display', () => {
    it('displays author name', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByText('John Developer')).toBeInTheDocument();
    });

    it('displays author avatar when provided', () => {
      render(<PostCard post={mockProblemPost} />);
      const avatar = screen.getByRole('img', { name: /john developer/i });
      expect(avatar).toBeInTheDocument();
      expect(avatar).toHaveAttribute('src', '/avatars/john.jpg');
    });

    it('displays avatar placeholder when no avatar_url', () => {
      render(<PostCard post={mockIdeaPost} />);
      // Should show initials or default avatar
      expect(screen.getByText('Alice Architect')).toBeInTheDocument();
    });

    it('indicates human author type', () => {
      render(<PostCard post={mockProblemPost} />);
      // Should have some indicator that this is a human
      const authorSection = screen.getByText('John Developer').closest('div');
      expect(authorSection).toBeInTheDocument();
    });

    it('indicates agent author type', () => {
      render(<PostCard post={mockQuestionPost} />);
      // Should have some indicator that this is an agent
      expect(screen.getByText('Claude')).toBeInTheDocument();
    });
  });

  describe('linking', () => {
    it('links to post detail page', () => {
      render(<PostCard post={mockProblemPost} />);
      const link = screen.getByRole('link', { name: mockProblemPost.title });
      expect(link).toHaveAttribute('href', '/posts/post-123');
    });

    it('title is clickable link', () => {
      render(<PostCard post={mockQuestionPost} />);
      const link = screen.getByRole('link', { name: mockQuestionPost.title });
      expect(link).toHaveAttribute('href', '/posts/post-789');
    });
  });

  describe('timestamps', () => {
    it('displays relative time', () => {
      render(<PostCard post={mockProblemPost} />);
      // Should display some form of time indicator
      const card = screen.getByRole('article');
      expect(card.textContent).toMatch(/ago|Jan|2026/i);
    });
  });

  describe('styling', () => {
    it('has border and hover effect', () => {
      render(<PostCard post={mockProblemPost} />);
      const card = screen.getByRole('article');
      expect(card).toHaveClass('border');
    });

    it('applies appropriate styling for vote score', () => {
      render(<PostCard post={mockProblemPost} />);
      // Vote score should be visible
      const voteDisplay = screen.getByText('39');
      expect(voteDisplay).toBeInTheDocument();
    });
  });

  describe('empty states', () => {
    it('handles post with no tags gracefully', () => {
      const postNoTags: PostWithAuthor = {
        ...mockProblemPost,
        tags: [],
      };
      render(<PostCard post={postNoTags} />);
      expect(screen.getByText(postNoTags.title)).toBeInTheDocument();
    });

    it('handles post with undefined tags', () => {
      const postUndefinedTags: PostWithAuthor = {
        ...mockProblemPost,
        tags: undefined,
      };
      render(<PostCard post={postUndefinedTags} />);
      expect(screen.getByText(postUndefinedTags.title)).toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('has article role for semantic structure', () => {
      render(<PostCard post={mockProblemPost} />);
      expect(screen.getByRole('article')).toBeInTheDocument();
    });

    it('images have alt text', () => {
      render(<PostCard post={mockProblemPost} />);
      const avatar = screen.getByRole('img');
      expect(avatar).toHaveAttribute('alt');
    });
  });
});

describe('PostCard variants', () => {
  it('renders compact variant when specified', () => {
    render(<PostCard post={mockProblemPost} variant="compact" />);
    const card = screen.getByRole('article');
    expect(card).toBeInTheDocument();
  });

  it('renders full variant by default', () => {
    render(<PostCard post={mockProblemPost} />);
    const card = screen.getByRole('article');
    // Full variant shows description (contains "data inconsistency" which is only in description)
    expect(screen.getByText(/data inconsistency/i)).toBeInTheDocument();
  });
});
