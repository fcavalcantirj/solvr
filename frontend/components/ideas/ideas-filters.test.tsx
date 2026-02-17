import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
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
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
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
        searchQuery=""
        onStageChange={onStageChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
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
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={onSortChange}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
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
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={onTagsChange}
        onSearchQueryChange={vi.fn()}
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
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
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
        searchQuery=""
        onSearchQueryChange={vi.fn()}
      />
    );

    // Expand filters
    fireEvent.click(screen.getByText('FILTERS'));

    // MOST SUPPORT (mapped from 'votes') should have active styling
    const votesButton = screen.getByText('MOST SUPPORT');
    expect(votesButton.className).toContain('bg-foreground');
  });
});

describe('IdeasFilters - Search Debouncing', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('prevents immediate API calls when typing (debounces)', async () => {
    vi.useFakeTimers();
    const mockOnSearchQueryChange = vi.fn();

    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search ideas...');
    fireEvent.change(searchInput, { target: { value: 'test' } });

    // Immediately after typing: parent callback should NOT be called
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // Even after 100ms: still not called (debounce is 500ms)
    vi.advanceTimersByTime(100);
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    vi.useRealTimers();
  });

  it('triggers parent callback after 500ms debounce period', async () => {
    vi.useFakeTimers();
    const mockOnSearchQueryChange = vi.fn();

    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search ideas...');
    fireEvent.change(searchInput, { target: { value: 'test query' } });

    // Not called immediately
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // After 500ms: parent callback SHOULD be called with the correct value
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('test query');
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('cancels previous timers on rapid typing (debounce reset)', async () => {
    vi.useFakeTimers();
    const mockOnSearchQueryChange = vi.fn();

    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search ideas...');

    // Type multiple characters rapidly (each keystroke resets the timer)
    fireEvent.change(searchInput, { target: { value: 'a' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'ai' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'ai ' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'ai code' } });

    // Only 300ms has passed total, no callback yet
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // Now wait 500ms from the LAST keystroke
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('ai code');
    // Should be called only ONCE with the final value (not 4 times)
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('updates input value immediately without lag (responsive UX)', () => {
    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search ideas...') as HTMLInputElement;

    // Type characters
    fireEvent.change(searchInput, { target: { value: 'r' } });
    expect(searchInput.value).toBe('r');

    fireEvent.change(searchInput, { target: { value: 're' } });
    expect(searchInput.value).toBe('re');

    fireEvent.change(searchInput, { target: { value: 'rea' } });
    expect(searchInput.value).toBe('rea');

    fireEvent.change(searchInput, { target: { value: 'react' } });
    expect(searchInput.value).toBe('react');

    // Input shows typed text immediately (no debounce delay on display)
  });

  it('bypasses debounce when Enter key is pressed', async () => {
    vi.useFakeTimers();
    const mockOnSearchQueryChange = vi.fn();

    render(
      <IdeasFilters
        stage="all"
        sort="newest"
        tags={[]}
        searchQuery=""
        onStageChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search ideas...');

    // Type text
    fireEvent.change(searchInput, { target: { value: 'urgent idea' } });

    // Not called immediately (debounce active)
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // Press Enter key
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    // Enter should trigger immediate callback (no wait needed)
    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('urgent idea');
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });
});
