"use client";

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { UserPostsList } from './user-posts-list';
import type { UserPostData } from '@/hooks/use-user';

const mockPosts: UserPostData[] = [
  {
    id: 'post-1',
    type: 'question',
    title: 'How do I handle async errors in Go?',
    description: 'I am trying to handle async errors...',
    status: 'open',
    voteScore: 10,
    upvotes: 12,
    downvotes: 2,
    views: 150,
    tags: ['go', 'errors'],
    createdAt: '2025-01-15T10:00:00Z',
    time: '2d ago',
  },
  {
    id: 'post-2',
    type: 'problem',
    title: 'Optimize database queries',
    description: 'Need to improve performance...',
    status: 'active',
    voteScore: 25,
    upvotes: 28,
    downvotes: 3,
    views: 500,
    tags: ['sql', 'performance'],
    createdAt: '2025-01-10T08:00:00Z',
    time: '5d ago',
  },
  {
    id: 'post-3',
    type: 'idea',
    title: 'AI-powered code reviews',
    description: 'What if we had AI review our code...',
    status: 'open',
    voteScore: 50,
    upvotes: 55,
    downvotes: 5,
    views: 1000,
    tags: ['ai', 'tooling'],
    createdAt: '2025-01-05T12:00:00Z',
    time: '1w ago',
  },
];

describe('UserPostsList', () => {
  it('should render list of posts', () => {
    render(<UserPostsList posts={mockPosts} />);

    expect(screen.getByText('How do I handle async errors in Go?')).toBeInTheDocument();
    expect(screen.getByText('Optimize database queries')).toBeInTheDocument();
    expect(screen.getByText('AI-powered code reviews')).toBeInTheDocument();
  });

  it('should render post types', () => {
    render(<UserPostsList posts={mockPosts} />);

    expect(screen.getByText('question')).toBeInTheDocument();
    expect(screen.getByText('problem')).toBeInTheDocument();
    expect(screen.getByText('idea')).toBeInTheDocument();
  });

  it('should render post vote scores', () => {
    render(<UserPostsList posts={mockPosts} />);

    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('25')).toBeInTheDocument();
    expect(screen.getByText('50')).toBeInTheDocument();
  });

  it('should render empty state when no posts', () => {
    render(<UserPostsList posts={[]} />);

    expect(screen.getByText(/no posts yet/i)).toBeInTheDocument();
  });

  it('should render loading state', () => {
    render(<UserPostsList posts={[]} loading={true} />);

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('should link posts to their detail pages', () => {
    render(<UserPostsList posts={mockPosts} />);

    const questionLink = screen.getByRole('link', { name: /how do i handle async errors/i });
    expect(questionLink).toHaveAttribute('href', '/questions/post-1');

    const problemLink = screen.getByRole('link', { name: /optimize database queries/i });
    expect(problemLink).toHaveAttribute('href', '/problems/post-2');

    const ideaLink = screen.getByRole('link', { name: /ai-powered code reviews/i });
    expect(ideaLink).toHaveAttribute('href', '/ideas/post-3');
  });

  it('should render post tags', () => {
    render(<UserPostsList posts={mockPosts} />);

    expect(screen.getByText('go')).toBeInTheDocument();
    expect(screen.getByText('errors')).toBeInTheDocument();
    expect(screen.getByText('sql')).toBeInTheDocument();
  });
});
