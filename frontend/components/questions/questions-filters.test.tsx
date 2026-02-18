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

describe('QuestionsFilters - Search Debouncing', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('prevents immediate API calls when typing (debounces)', async () => {
    vi.useFakeTimers();
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

    // Type multiple characters rapidly (each keystroke resets the timer)
    fireEvent.change(searchInput, { target: { value: 't' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'ty' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'typ' } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });

    fireEvent.change(searchInput, { target: { value: 'typescript' } });

    // Only 300ms has passed total, no callback yet
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // Now wait 500ms from the LAST keystroke
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('typescript');
    // Should be called only ONCE with the final value (not 4 times)
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
  });

  it('updates input value immediately without lag (responsive UX)', () => {
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

    const searchInput = screen.getByPlaceholderText('Search questions...') as HTMLInputElement;

    // Type characters
    fireEvent.change(searchInput, { target: { value: 't' } });
    expect(searchInput.value).toBe('t');

    fireEvent.change(searchInput, { target: { value: 'te' } });
    expect(searchInput.value).toBe('te');

    fireEvent.change(searchInput, { target: { value: 'tes' } });
    expect(searchInput.value).toBe('tes');

    fireEvent.change(searchInput, { target: { value: 'test' } });
    expect(searchInput.value).toBe('test');

    // Input shows typed text immediately (no debounce delay on display)
  });

  it('bypasses debounce when Enter key is pressed', async () => {
    vi.useFakeTimers();
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

    // Type text
    fireEvent.change(searchInput, { target: { value: 'urgent question' } });

    // Not called immediately (debounce active)
    expect(mockOnSearchQueryChange).not.toHaveBeenCalled();

    // Press Enter key
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    // Enter should trigger immediate callback (no wait needed)
    expect(mockOnSearchQueryChange).toHaveBeenCalledWith('urgent question');
    expect(mockOnSearchQueryChange).toHaveBeenCalledTimes(1);

    vi.useRealTimers();
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
