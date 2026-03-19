import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { QuestionsFilters } from './questions-filters';

describe('QuestionsFilters - Basic Functionality', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders search input', () => {
    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onHasAnswerChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search questions...');
    expect(searchInput).toBeInTheDocument();
  });

  it('calls onStatusChange when status button is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    // Click ACCEPTED button (which sets status to "solved")
    const acceptedButtons = screen.getAllByText('ACCEPTED');
    fireEvent.click(acceptedButtons[0]);

    expect(mockOnStatusChange).toHaveBeenCalledWith('solved');
    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(undefined);
  });

  it('calls onSortChange when sort option is clicked', () => {
    const mockOnSortChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onSortChange={mockOnSortChange}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    // Open filters first
    const filtersButton = screen.getByText(/FILTERS/);
    fireEvent.click(filtersButton);

    const votesButton = screen.getByText('MOST VOTED');
    fireEvent.click(votesButton);

    expect(mockOnSortChange).toHaveBeenCalledWith('votes');
  });

  it('calls onTagsChange when tag is toggled', () => {
    const mockOnTagsChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={mockOnTagsChange}
        onSearchQueryChange={vi.fn()}
      />
    );

    // Open filters first
    const filtersButton = screen.getByText(/FILTERS/);
    fireEvent.click(filtersButton);

    // Click a tag
    const reactTag = screen.getByText('react');
    fireEvent.click(reactTag);

    expect(mockOnTagsChange).toHaveBeenCalledWith(['react']);
  });

  it('clears all filters when CLEAR ALL is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();
    const mockOnSortChange = vi.fn();
    const mockOnTagsChange = vi.fn();
    const mockOnSearchQueryChange = vi.fn();

    render(
      <QuestionsFilters
        status="solved"
        hasAnswer={true}
        sort="votes"
        tags={['react']}
        searchQuery="test"
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={mockOnSortChange}
        onTagsChange={mockOnTagsChange}
        onSearchQueryChange={mockOnSearchQueryChange}
      />
    );

    // Open filters first
    const filtersButton = screen.getByText(/FILTERS/);
    fireEvent.click(filtersButton);

    const clearButton = screen.getByText('CLEAR ALL');
    fireEvent.click(clearButton);

    expect(mockOnStatusChange).toHaveBeenCalledWith(undefined);
    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(undefined);
    expect(mockOnSortChange).toHaveBeenCalledWith('newest');
    expect(mockOnTagsChange).toHaveBeenCalledWith([]);
    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('');
  });

  it('uses searchQuery prop to display value', () => {
    render(
      <QuestionsFilters
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

    const searchInput = screen.getByPlaceholderText('Search questions...') as HTMLInputElement;
    expect(searchInput.value).toBe('initial query');
  });
});

describe('QuestionsFilters - Search (no component debounce, hook handles it)', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('calls parent callback immediately on typing (debounce is in useSearch hook)', () => {
    const mockOnSearchQueryChange = vi.fn();

    render(
      <QuestionsFilters
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

    const searchInput = screen.getByPlaceholderText('Search questions...');
    fireEvent.change(searchInput, { target: { value: 'test' } });

    // Called immediately - debounce is handled by the useSearch hook, not the filter component
    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('test');
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);
  });

  it('calls parent callback for each keystroke (no local debounce)', () => {
    const mockOnSearchQueryChange = vi.fn();

    render(
      <QuestionsFilters
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

    const searchInput = screen.getByPlaceholderText('Search questions...');

    fireEvent.change(searchInput, { target: { value: 't' } });
    fireEvent.change(searchInput, { target: { value: 'ty' } });
    fireEvent.change(searchInput, { target: { value: 'typ' } });
    fireEvent.change(searchInput, { target: { value: 'typescript' } });

    // Each keystroke triggers the parent callback immediately
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(4);
    expect(mockOnSearchQueryChange).toHaveBeenLastCalledWith('typescript');
  });

  it('displays searchQuery prop value in input (controlled component)', () => {
    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={undefined}
        sort="newest"
        tags={[]}
        searchQuery="existing query"
        onStatusChange={vi.fn()}
        onHasAnswerChange={vi.fn()}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const searchInput = screen.getByPlaceholderText('Search questions...') as HTMLInputElement;
    expect(searchInput.value).toBe('existing query');
  });
});

describe('QuestionsFilters - Has Answer Filter', () => {
  it('calls onHasAnswerChange with false when UNANSWERED is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const unansweredButtons = screen.getAllByText('UNANSWERED');
    fireEvent.click(unansweredButtons[0]);

    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(false);
    expect(mockOnStatusChange).toHaveBeenCalledWith(undefined);
  });

  it('calls onHasAnswerChange with true when ANSWERED is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={undefined}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const answeredButtons = screen.getAllByText('ANSWERED');
    fireEvent.click(answeredButtons[0]);

    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(true);
    expect(mockOnStatusChange).toHaveBeenCalledWith(undefined);
  });

  it('calls onHasAnswerChange with undefined when ALL is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={false}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const allButtons = screen.getAllByText('ALL');
    fireEvent.click(allButtons[0]);

    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(undefined);
    expect(mockOnStatusChange).toHaveBeenCalledWith(undefined);
  });

  it('calls onHasAnswerChange with undefined when ACCEPTED is clicked', () => {
    const mockOnStatusChange = vi.fn();
    const mockOnHasAnswerChange = vi.fn();

    render(
      <QuestionsFilters
        status={undefined}
        hasAnswer={true}
        sort="newest"
        tags={[]}
        searchQuery=""
        onStatusChange={mockOnStatusChange}
        onHasAnswerChange={mockOnHasAnswerChange}
        onSortChange={vi.fn()}
        onTagsChange={vi.fn()}
        onSearchQueryChange={vi.fn()}
      />
    );

    const acceptedButtons = screen.getAllByText('ACCEPTED');
    fireEvent.click(acceptedButtons[0]);

    expect(mockOnHasAnswerChange).toHaveBeenCalledWith(undefined);
    expect(mockOnStatusChange).toHaveBeenCalledWith('solved');
  });
});
