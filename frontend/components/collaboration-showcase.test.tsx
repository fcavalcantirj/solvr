import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CollaborationShowcase } from './collaboration-showcase';

// Mock the hooks
const mockUseProblemsStats = vi.fn();
vi.mock('@/hooks/use-problems-stats', () => ({
  useProblemsStats: () => mockUseProblemsStats(),
}));

const mockUseTrending = vi.fn();
vi.mock('@/hooks/use-stats', () => ({
  useTrending: () => mockUseTrending(),
}));

describe('CollaborationShowcase', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders section heading and marketing copy', () => {
    mockUseProblemsStats.mockReturnValue({ stats: null, loading: false });
    mockUseTrending.mockReturnValue({ trending: null, loading: false });

    render(<CollaborationShowcase />);

    expect(screen.getByText('Watch knowledge compound in real-time')).toBeInTheDocument();
    expect(screen.getByText('Human Contributors')).toBeInTheDocument();
    expect(screen.getByText('AI Agents')).toBeInTheDocument();
  });

  it('shows loading state when data is being fetched', () => {
    mockUseProblemsStats.mockReturnValue({ stats: null, loading: true });
    mockUseTrending.mockReturnValue({ trending: null, loading: true });

    render(<CollaborationShowcase />);

    expect(screen.getAllByText('Loading...').length).toBeGreaterThan(0);
  });

  it('renders recently solved problems with solver info', () => {
    mockUseProblemsStats.mockReturnValue({
      stats: {
        total_problems: 50,
        solved_count: 30,
        active_approaches: 10,
        avg_solve_time_days: 3,
        recently_solved: [
          { id: '1', title: 'Race condition in async/await', solver_name: 'claude_agent', solver_type: 'agent', time_to_solve_days: 2 },
          { id: '2', title: 'Memory leak in useEffect', solver_name: 'sarah_dev', solver_type: 'human', time_to_solve_days: 5 },
        ],
        top_solvers: [],
      },
      loading: false,
    });
    mockUseTrending.mockReturnValue({ trending: null, loading: false });

    render(<CollaborationShowcase />);

    expect(screen.getByText('Race condition in async/await')).toBeInTheDocument();
    expect(screen.getByText('claude_agent')).toBeInTheDocument();
    expect(screen.getByText('2d')).toBeInTheDocument();
    expect(screen.getByText('Memory leak in useEffect')).toBeInTheDocument();
    expect(screen.getByText('sarah_dev')).toBeInTheDocument();
  });

  it('renders trending posts when available', () => {
    mockUseProblemsStats.mockReturnValue({ stats: null, loading: false });
    mockUseTrending.mockReturnValue({
      trending: {
        posts: [
          { id: 'p1', title: 'CORS in Go Chi middleware', type: 'problem', response_count: 12, vote_score: 5 },
          { id: 'p2', title: 'pgvector index tuning', type: 'question', response_count: 8, vote_score: 3 },
        ],
        tags: [],
      },
      loading: false,
    });

    render(<CollaborationShowcase />);

    expect(screen.getByText('CORS in Go Chi middleware')).toBeInTheDocument();
    expect(screen.getByText('pgvector index tuning')).toBeInTheDocument();
    expect(screen.getByText('12 responses')).toBeInTheDocument();
  });

  it('shows empty state when no data', () => {
    mockUseProblemsStats.mockReturnValue({
      stats: {
        recently_solved: [],
        top_solvers: [],
        total_problems: 0,
        solved_count: 0,
        active_approaches: 0,
        avg_solve_time_days: 0,
      },
      loading: false,
    });
    mockUseTrending.mockReturnValue({
      trending: { posts: [], tags: [] },
      loading: false,
    });

    render(<CollaborationShowcase />);

    expect(screen.getByText('Be the first to solve a problem.')).toBeInTheDocument();
  });

  it('links recently solved problems to their detail pages', () => {
    mockUseProblemsStats.mockReturnValue({
      stats: {
        recently_solved: [
          { id: 'abc123', title: 'Test problem', solver_name: 'bot', solver_type: 'agent', time_to_solve_days: 1 },
        ],
        top_solvers: [],
        total_problems: 1,
        solved_count: 1,
        active_approaches: 0,
        avg_solve_time_days: 1,
      },
      loading: false,
    });
    mockUseTrending.mockReturnValue({ trending: null, loading: false });

    render(<CollaborationShowcase />);

    const link = screen.getByText('Test problem').closest('a');
    expect(link).toHaveAttribute('href', '/problems/abc123');
  });
});
