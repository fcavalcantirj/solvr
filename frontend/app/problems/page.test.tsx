import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import ProblemsPage from './page';

// Mock hooks
vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    isAuthenticated: false,
    user: null,
    loading: false,
  })),
}));

// Track what props ProblemsList receives
const mockProblemsListProps = vi.fn();
vi.mock('@/components/problems/problems-list', () => ({
  ProblemsList: ({ sort, status, tags, searchQuery }: Record<string, unknown>) => {
    mockProblemsListProps({ sort, status, tags, searchQuery });
    return (
      <div
        data-testid="problems-list"
        data-sort={sort as string}
        data-status={status as string}
        data-tags={JSON.stringify(tags)}
      />
    );
  },
}));

// Track what props ProblemsFilters receives
vi.mock('@/components/problems/problems-filters', () => ({
  ProblemsFilters: ({ sort, status, tags }: Record<string, unknown>) => (
    <div
      data-testid="problems-filters"
      data-sort={sort as string}
      data-status={status as string}
      data-tags={JSON.stringify(tags)}
    />
  ),
}));

vi.mock('@/components/problems/problems-sidebar', () => ({
  ProblemsSidebar: () => <div data-testid="problems-sidebar" />,
}));

vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header" />,
}));

const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

describe('ProblemsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockProblemsListProps.mockClear();
  });

  it('defaults to votes sort on initial render', () => {
    render(<ProblemsPage />);

    const list = screen.getByTestId('problems-list');
    expect(list.getAttribute('data-sort')).toBe('votes');
  });

  it('passes sort=votes to ProblemsList on mount', () => {
    render(<ProblemsPage />);

    expect(mockProblemsListProps).toHaveBeenCalled();
    const lastCall = mockProblemsListProps.mock.calls[mockProblemsListProps.mock.calls.length - 1][0];
    expect(lastCall.sort).toBe('votes');
  });

  it('passes sort=votes to ProblemsFilters on mount', () => {
    render(<ProblemsPage />);

    const filters = screen.getByTestId('problems-filters');
    expect(filters.getAttribute('data-sort')).toBe('votes');
  });
});
