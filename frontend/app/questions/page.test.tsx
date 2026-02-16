import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import QuestionsPage from './page';

// Mock hooks
vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    isAuthenticated: false,
    user: null,
    loading: false,
  })),
}));

// Track what props QuestionsList receives
const mockQuestionsListProps = vi.fn();
vi.mock('@/components/questions/questions-list', () => ({
  QuestionsList: ({ sort, status, tags, searchQuery }: Record<string, unknown>) => {
    mockQuestionsListProps({ sort, status, tags, searchQuery });
    return (
      <div
        data-testid="questions-list"
        data-sort={sort as string}
        data-status={status as string}
        data-tags={JSON.stringify(tags)}
      />
    );
  },
}));

// Track what props QuestionsFilters receives
vi.mock('@/components/questions/questions-filters', () => ({
  QuestionsFilters: ({ sort, status, tags }: Record<string, unknown>) => (
    <div
      data-testid="questions-filters"
      data-sort={sort as string}
      data-status={status as string}
      data-tags={JSON.stringify(tags)}
    />
  ),
}));

vi.mock('@/components/questions/questions-sidebar', () => ({
  QuestionsSidebar: () => <div data-testid="questions-sidebar" />,
}));

vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header" />,
}));

const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

describe('QuestionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQuestionsListProps.mockClear();
  });

  it('defaults to votes sort on initial render', () => {
    render(<QuestionsPage />);

    const list = screen.getByTestId('questions-list');
    expect(list.getAttribute('data-sort')).toBe('votes');
  });

  it('passes sort=votes to QuestionsList on mount', () => {
    render(<QuestionsPage />);

    expect(mockQuestionsListProps).toHaveBeenCalled();
    const lastCall = mockQuestionsListProps.mock.calls[mockQuestionsListProps.mock.calls.length - 1][0];
    expect(lastCall.sort).toBe('votes');
  });

  it('passes sort=votes to QuestionsFilters on mount', () => {
    render(<QuestionsPage />);

    const filters = screen.getByTestId('questions-filters');
    expect(filters.getAttribute('data-sort')).toBe('votes');
  });
});
