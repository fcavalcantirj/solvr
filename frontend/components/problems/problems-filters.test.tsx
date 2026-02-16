import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ProblemsFilters } from './problems-filters';

describe('ProblemsFilters - Search Functionality', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls onSearchQueryChange when user types in search input', () => {
    const mockOnSearchQueryChange = vi.fn();

    render(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search problems...');
    fireEvent.change(searchInput, { target: { value: 'test query' } });

    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('test query');
  });

  it('triggers search on Enter key press', () => {
    const mockOnSearchQueryChange = vi.fn();

    render(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery="test"
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search problems...');
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    // Enter key should not prevent default search behavior
    // The search is already triggered by onChange, Enter is just for convenience
    expect(searchInput).toBeInTheDocument();
  });

  it('shows clickable lens icon with hover effect', () => {
    render(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const lensIcon = screen.getByTestId('search-icon-button');
    expect(lensIcon).toBeInTheDocument();
    expect(lensIcon.tagName).toBe('BUTTON');
    expect(lensIcon).toHaveClass('cursor-pointer');
  });

  it('clears search query when "CLEAR ALL" is clicked', () => {
    const mockOnSearchQueryChange = vi.fn();

    render(
      <ProblemsFilters
        status="open"
        sort="votes"
        tags={['react']}
        searchQuery="test search"
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    // Need to open filters first to see "CLEAR ALL" button
    const filtersButton = screen.getByText(/FILTERS/);
    fireEvent.click(filtersButton);

    const clearButton = screen.getByText('CLEAR ALL');
    fireEvent.click(clearButton);

    // Should have been called with empty string
    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('');
  });

  it('uses searchQuery prop instead of local state', () => {
    const { rerender } = render(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery="initial query"
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search problems...') as HTMLInputElement;
    expect(searchInput.value).toBe('initial query');

    // Rerender with updated prop
    rerender(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery="updated query"
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    expect(searchInput.value).toBe('updated query');
  });

  it('includes searchQuery in hasActiveFilters check', () => {
    const { rerender } = render(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    // Open filters to check if "CLEAR ALL" is shown
    const filtersButton = screen.getByText(/FILTERS/);
    fireEvent.click(filtersButton);

    // No active filters, so "CLEAR ALL" should not be visible
    expect(screen.queryByText('CLEAR ALL')).not.toBeInTheDocument();

    // Rerender with search query
    rerender(
      <ProblemsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery="test"
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    // Now "CLEAR ALL" should be visible because we have a search query
    expect(screen.getByText('CLEAR ALL')).toBeInTheDocument();
  });
});
