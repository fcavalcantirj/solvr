import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApiSection } from './api-section';

// Mock the hooks
const mockUseStats = vi.fn();
vi.mock('@/hooks/use-stats', () => ({
  useStats: () => mockUseStats(),
}));

const mockUseSearchStats = vi.fn();
vi.mock('@/hooks/use-search-stats', () => ({
  useSearchStats: () => mockUseSearchStats(),
}));

describe('ApiSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the code example and section heading', () => {
    mockUseStats.mockReturnValue({ stats: null, loading: false });
    mockUseSearchStats.mockReturnValue({ searchStats: null, loading: false });

    render(<ApiSection />);

    expect(screen.getByText('FOR AI AGENTS')).toBeInTheDocument();
    expect(screen.getByText('API-first. Agent-native.')).toBeInTheDocument();
    expect(screen.getByText('VIEW API DOCUMENTATION')).toBeInTheDocument();
  });

  it('shows loading placeholders while fetching stats', () => {
    mockUseStats.mockReturnValue({ stats: null, loading: true });
    mockUseSearchStats.mockReturnValue({ searchStats: null, loading: true });

    render(<ApiSection />);

    const dashes = screen.getAllByText('--');
    expect(dashes.length).toBe(3);
  });

  it('renders live agent stats when loaded', () => {
    mockUseStats.mockReturnValue({
      stats: { total_agents: 24, total_contributions: 1234 },
      loading: false,
    });
    mockUseSearchStats.mockReturnValue({
      searchStats: { total_searches_7d: 890, agent_searches_7d: 500, human_searches_7d: 390, trending_queries: [] },
      loading: false,
    });

    render(<ApiSection />);

    expect(screen.getByText('24')).toBeInTheDocument();
    expect(screen.getByText('890')).toBeInTheDocument();
    expect(screen.getByText('1.2K')).toBeInTheDocument();
    expect(screen.getByText('AI AGENTS ACTIVE')).toBeInTheDocument();
    expect(screen.getByText('SEARCHES THIS WEEK')).toBeInTheDocument();
    expect(screen.getByText('CONTRIBUTIONS')).toBeInTheDocument();
  });

  it('renders bullet points for API endpoints', () => {
    mockUseStats.mockReturnValue({ stats: null, loading: false });
    mockUseSearchStats.mockReturnValue({ searchStats: null, loading: false });

    render(<ApiSection />);

    expect(screen.getByText(/GET \/search/)).toBeInTheDocument();
    expect(screen.getByText(/POST \/posts/)).toBeInTheDocument();
    expect(screen.getByText(/POST \/approaches/)).toBeInTheDocument();
  });
});
