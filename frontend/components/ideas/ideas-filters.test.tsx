import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { IdeasFilters } from './ideas-filters';

// Mock useTrending
vi.mock('@/hooks/use-stats', () => ({
  useTrending: vi.fn(() => ({
    trending: {
      tags: [
        { name: 'react', count: 42 },
        { name: 'go', count: 35 },
        { name: 'ai', count: 28 },
      ],
    },
    loading: false,
    error: null,
  })),
}));

import { useTrending } from '@/hooks/use-stats';

describe('IdeasFilters', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses trending tags from useTrending hook instead of hardcoded tags', () => {
    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
      />
    );

    // Click FILTERS button to expand
    const filtersButton = screen.getByText('FILTERS');
    fireEvent.click(filtersButton);

    // Should show trending tags from hook, not hardcoded ones
    expect(screen.getByText('react')).toBeDefined();
    expect(screen.getByText('go')).toBeDefined();
    expect(screen.getByText('ai')).toBeDefined();

    // Hardcoded tags from original should NOT appear
    expect(screen.queryByText('architecture')).toBeNull();
    expect(screen.queryByText('collaboration')).toBeNull();
  });

  it('calls onStageChange when stage tab is clicked', () => {
    const onStageChange = vi.fn();
    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        onStageChange={onStageChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
      />
    );

    fireEvent.click(screen.getByText('SPARK'));
    expect(onStageChange).toHaveBeenCalledWith('spark');
  });

  it('calls onSortChange when sort option is clicked', () => {
    const onSortChange = vi.fn();
    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        onStageChange={vi.fn()}
        onSortChange={onSortChange}
        onTagsChange={vi.fn()}
      />
    );

    // Expand filters
    fireEvent.click(screen.getByText('FILTERS'));

    fireEvent.click(screen.getByText('NEWEST'));
    expect(onSortChange).toHaveBeenCalledWith('newest');
  });

  it('calls onTagsChange when tag is toggled', () => {
    const onTagsChange = vi.fn();
    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={onTagsChange}
      />
    );

    // Expand filters
    fireEvent.click(screen.getByText('FILTERS'));

    // Click a trending tag
    fireEvent.click(screen.getByText('react'));
    expect(onTagsChange).toHaveBeenCalledWith(['react']);
  });

  it('reflects active stage from props', () => {
    const { container } = render(
      <IdeasFilters
        stage="spark"
        sort="newest"
        tags={[]}
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
      />
    );

    // The SPARK button should have active styling (bg-foreground)
    const sparkButton = screen.getByText((content, element) => {
      return element?.tagName === 'BUTTON' && content.includes('SPARK');
    });
    expect(sparkButton.className).toContain('bg-foreground');
  });

  it('reflects active sort from props', () => {
    render(
      <IdeasFilters
        stage="all"
        sort="votes"
        tags={[]}
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
      />
    );

    // Expand filters
    fireEvent.click(screen.getByText('FILTERS'));

    // MOST SUPPORT (mapped from 'votes') should have active styling
    const votesButton = screen.getByText('MOST SUPPORT');
    expect(votesButton.className).toContain('bg-foreground');
  });
});
