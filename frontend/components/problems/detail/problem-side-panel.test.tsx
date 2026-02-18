import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { ProblemSidePanel } from './problem-side-panel';
import type { ProblemData } from '@/hooks/use-problem';

const mockProblem: ProblemData = {
  id: 'problem-123',
  title: 'Test Problem',
  description: 'Test description',
  status: 'OPEN',
  voteScore: 42,
  upvotes: 50,
  downvotes: 8,
  author: {
    id: 'user-1',
    type: 'human',
    displayName: 'TestUser',
  },
  tags: ['go', 'postgres'],
  createdAt: '2026-01-15T10:00:00Z',
  updatedAt: '2026-01-16T14:30:00Z',
  time: '2 days ago',
  approachesCount: 3,
  views: 100,
};

describe('ProblemSidePanel', () => {
  it('renders approaches count and posted time', () => {
    render(<ProblemSidePanel problem={mockProblem} approachesCount={5} />);

    expect(screen.getByText('APPROACHES')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('POSTED')).toBeInTheDocument();
    expect(screen.getByText('2 days ago')).toBeInTheDocument();
  });

  it('renders author info', () => {
    render(<ProblemSidePanel problem={mockProblem} approachesCount={3} />);

    expect(screen.getByText('POSTED BY')).toBeInTheDocument();
    expect(screen.getByText('TestUser')).toBeInTheDocument();
    expect(screen.getByText('Human')).toBeInTheDocument();
  });

  it('renders tags', () => {
    render(<ProblemSidePanel problem={mockProblem} approachesCount={3} />);

    expect(screen.getByText('TAGS')).toBeInTheDocument();
    expect(screen.getByText('go')).toBeInTheDocument();
    expect(screen.getByText('postgres')).toBeInTheDocument();
  });

  it('does not render crystallization section when CID is not present', () => {
    render(<ProblemSidePanel problem={mockProblem} approachesCount={3} />);

    expect(screen.queryByText('IPFS ARCHIVE')).not.toBeInTheDocument();
  });

  it('renders crystallization section when CID is present', () => {
    const crystallizedProblem: ProblemData = {
      ...mockProblem,
      status: 'SOLVED',
      crystallizationCid: 'QmTestCid123456789abcdef',
      crystallizedAt: '2026-02-15T10:30:00Z',
    };

    render(<ProblemSidePanel problem={crystallizedProblem} approachesCount={3} />);

    expect(screen.getByText('IPFS ARCHIVE')).toBeInTheDocument();
    expect(screen.getByText('CRYSTALLIZED')).toBeInTheDocument();
  });

  it('renders IPFS gateway link in crystallization section', () => {
    const crystallizedProblem: ProblemData = {
      ...mockProblem,
      status: 'SOLVED',
      crystallizationCid: 'QmTestCid123456789abcdef',
      crystallizedAt: '2026-02-15T10:30:00Z',
    };

    render(<ProblemSidePanel problem={crystallizedProblem} approachesCount={3} />);

    const link = screen.getByRole('link', { name: /view on ipfs/i });
    expect(link).toHaveAttribute(
      'href',
      'https://ipfs.io/ipfs/QmTestCid123456789abcdef'
    );
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('renders AI agent author correctly', () => {
    const agentProblem: ProblemData = {
      ...mockProblem,
      author: {
        id: 'claude-1',
        type: 'ai',
        displayName: 'Claude',
      },
    };

    render(<ProblemSidePanel problem={agentProblem} approachesCount={3} />);

    expect(screen.getByText('Claude')).toBeInTheDocument();
    expect(screen.getByText('AI Agent')).toBeInTheDocument();
  });
});
