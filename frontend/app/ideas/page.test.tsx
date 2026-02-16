import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import IdeasPage from './page';

// Mock hooks
vi.mock('@/hooks/use-ideas-stats', () => ({
  useIdeasStats: vi.fn(() => ({
    stats: { total: 100, countsByStatus: { spark: 40, developing: 30, mature: 15, realized: 10, archived: 5 } },
    loading: false,
    error: null,
  })),
}));

vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    isAuthenticated: false,
    user: null,
    loading: false,
  })),
}));

// Track what options IdeasList receives
const mockIdeasListOptions = vi.fn();
vi.mock('@/components/ideas/ideas-list', () => ({
  IdeasList: ({ options }: { options?: Record<string, unknown> }) => {
    mockIdeasListOptions(options);
    return <div data-testid="ideas-list" data-options={JSON.stringify(options)} />;
  },
}));

// Track what props IdeasFilters receives
const mockFiltersOnChange = vi.fn();
vi.mock('@/components/ideas/ideas-filters', () => ({
  IdeasFilters: ({ onFiltersChange, stats, stage, sort, tags, onStageChange, onSortChange, onTagsChange }: Record<string, unknown>) => {
    // Store the callbacks so tests can invoke them
    if (onStageChange) (window as Record<string, unknown>).__testOnStageChange = onStageChange;
    if (onSortChange) (window as Record<string, unknown>).__testOnSortChange = onSortChange;
    if (onTagsChange) (window as Record<string, unknown>).__testOnTagsChange = onTagsChange;
    return (
      <div
        data-testid="ideas-filters"
        data-stage={stage as string}
        data-sort={sort as string}
        data-tags={JSON.stringify(tags)}
      />
    );
  },
  IdeasFilterStats: vi.fn(),
}));

vi.mock('@/components/ideas/ideas-sidebar', () => ({
  IdeasSidebar: () => <div data-testid="ideas-sidebar" />,
}));

vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header" />,
}));

const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

describe('IdeasPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIdeasListOptions.mockClear();
    delete (window as Record<string, unknown>).__testOnStageChange;
    delete (window as Record<string, unknown>).__testOnSortChange;
    delete (window as Record<string, unknown>).__testOnTagsChange;
  });

  it('passes filter state props to IdeasFilters', () => {
    render(<IdeasPage />);

    const filters = screen.getByTestId('ideas-filters');
    // Initially: stage undefined (all), sort votes, tags empty
    expect(filters.getAttribute('data-sort')).toBe('votes');
    expect(filters.getAttribute('data-tags')).toBe('[]');
  });

  it('passes filter state as options to IdeasList', () => {
    render(<IdeasPage />);

    // IdeasList should receive options with default values
    expect(mockIdeasListOptions).toHaveBeenCalled();
    const lastCall = mockIdeasListOptions.mock.calls[mockIdeasListOptions.mock.calls.length - 1][0];
    expect(lastCall).toBeDefined();
    expect(lastCall.sort).toBe('votes');
    expect(lastCall.tags).toEqual([]);
  });

  it('changing stage filter updates IdeasList with matching status param', async () => {
    render(<IdeasPage />);

    // Simulate stage change from IdeasFilters
    const onStageChange = (window as Record<string, unknown>).__testOnStageChange as (stage: string) => void;
    expect(onStageChange).toBeDefined();

    // Change to "spark" stage
    onStageChange('spark');

    await waitFor(() => {
      const lastCall = mockIdeasListOptions.mock.calls[mockIdeasListOptions.mock.calls.length - 1][0];
      // "spark" stage maps to status "open" in API
      expect(lastCall.status).toBe('open');
    });
  });

  it('changing sort filter updates IdeasList with matching sort param', async () => {
    render(<IdeasPage />);

    const onSortChange = (window as Record<string, unknown>).__testOnSortChange as (sort: string) => void;
    expect(onSortChange).toBeDefined();

    onSortChange('votes');

    await waitFor(() => {
      const lastCall = mockIdeasListOptions.mock.calls[mockIdeasListOptions.mock.calls.length - 1][0];
      expect(lastCall.sort).toBe('votes');
    });
  });

  it('changing tags updates IdeasList', async () => {
    render(<IdeasPage />);

    const onTagsChange = (window as Record<string, unknown>).__testOnTagsChange as (tags: string[]) => void;
    expect(onTagsChange).toBeDefined();

    onTagsChange(['ai', 'security']);

    await waitFor(() => {
      const lastCall = mockIdeasListOptions.mock.calls[mockIdeasListOptions.mock.calls.length - 1][0];
      expect(lastCall.tags).toEqual(['ai', 'security']);
    });
  });
});
