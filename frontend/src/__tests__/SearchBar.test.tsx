/**
 * Tests for SearchBar component
 * Per PRD requirement: Create SearchBar component with search icon and Cmd+K shortcut
 */

import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: jest.fn(),
  }),
}));

// Import after mocks
import SearchBar from '../components/SearchBar';

describe('SearchBar', () => {
  describe('basic rendering', () => {
    it('renders a search input', () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');
      expect(input).toBeInTheDocument();
    });

    it('displays placeholder text', () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');
      expect(input).toHaveAttribute('placeholder', expect.stringContaining('Search'));
    });

    it('displays search icon', () => {
      render(<SearchBar />);
      // Search icon should be present (svg or icon element)
      const icon = document.querySelector('svg');
      expect(icon).toBeInTheDocument();
    });

    it('shows keyboard shortcut hint', () => {
      render(<SearchBar />);
      // Should show Cmd+K hint inside kbd element
      const kbd = document.querySelector('kbd');
      expect(kbd).toBeInTheDocument();
      expect(kbd?.textContent).toContain('K');
    });
  });

  describe('input behavior', () => {
    it('accepts text input', async () => {
      const user = userEvent.setup();
      render(<SearchBar />);

      const input = screen.getByRole('searchbox');
      await user.type(input, 'test query');

      expect(input).toHaveValue('test query');
    });

    it('updates value on change', () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');

      fireEvent.change(input, { target: { value: 'new search' } });

      expect(input).toHaveValue('new search');
    });

    it('clears input when clear button is clicked', async () => {
      const user = userEvent.setup();
      render(<SearchBar />);

      const input = screen.getByRole('searchbox');
      await user.type(input, 'search text');

      // Clear button should appear when there's text
      const clearButton = screen.getByRole('button', { name: /clear/i });
      await user.click(clearButton);

      expect(input).toHaveValue('');
    });
  });

  describe('search submission', () => {
    it('calls onSearch when form is submitted', async () => {
      const onSearch = jest.fn();
      const user = userEvent.setup();
      render(<SearchBar onSearch={onSearch} />);

      const input = screen.getByRole('searchbox');
      await user.type(input, 'test query');
      await user.keyboard('{Enter}');

      expect(onSearch).toHaveBeenCalledWith('test query');
    });

    it('does not call onSearch with empty query', async () => {
      const onSearch = jest.fn();
      const user = userEvent.setup();
      render(<SearchBar onSearch={onSearch} />);

      const input = screen.getByRole('searchbox');
      await user.click(input);
      await user.keyboard('{Enter}');

      expect(onSearch).not.toHaveBeenCalled();
    });

    it('trims whitespace from query', async () => {
      const onSearch = jest.fn();
      const user = userEvent.setup();
      render(<SearchBar onSearch={onSearch} />);

      const input = screen.getByRole('searchbox');
      await user.type(input, '  test query  ');
      await user.keyboard('{Enter}');

      expect(onSearch).toHaveBeenCalledWith('test query');
    });
  });

  describe('keyboard shortcuts', () => {
    it('focuses input when Cmd+K is pressed', async () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');

      // Simulate Cmd+K (Mac) or Ctrl+K (Windows/Linux)
      fireEvent.keyDown(document, { key: 'k', metaKey: true });

      expect(document.activeElement).toBe(input);
    });

    it('focuses input when Ctrl+K is pressed', async () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');

      fireEvent.keyDown(document, { key: 'k', ctrlKey: true });

      expect(document.activeElement).toBe(input);
    });

    it('closes/blurs on Escape', async () => {
      const user = userEvent.setup();
      render(<SearchBar />);

      const input = screen.getByRole('searchbox');
      await user.click(input);
      await user.type(input, 'test');
      await user.keyboard('{Escape}');

      expect(document.activeElement).not.toBe(input);
    });
  });

  describe('controlled mode', () => {
    it('uses value prop when provided', () => {
      render(<SearchBar value="controlled value" onChange={() => {}} />);
      const input = screen.getByRole('searchbox');

      expect(input).toHaveValue('controlled value');
    });

    it('calls onChange when input changes in controlled mode', () => {
      const onChange = jest.fn();
      render(<SearchBar value="" onChange={onChange} />);

      const input = screen.getByRole('searchbox');
      fireEvent.change(input, { target: { value: 'new' } });

      expect(onChange).toHaveBeenCalledWith('new');
    });
  });

  describe('styling', () => {
    it('has appropriate border styling', () => {
      render(<SearchBar />);
      const container = screen.getByRole('searchbox').closest('div');
      expect(container).toHaveClass('border');
    });

    it('applies focus styles', async () => {
      const user = userEvent.setup();
      render(<SearchBar />);

      const input = screen.getByRole('searchbox');
      await user.click(input);

      // Input should be focused
      expect(document.activeElement).toBe(input);
    });
  });

  describe('accessibility', () => {
    it('has accessible label', () => {
      render(<SearchBar />);
      const input = screen.getByRole('searchbox');

      // Should have aria-label or be associated with a label
      expect(input).toHaveAccessibleName();
    });

    it('has searchbox role', () => {
      render(<SearchBar />);
      expect(screen.getByRole('searchbox')).toBeInTheDocument();
    });
  });

  describe('customization', () => {
    it('accepts custom placeholder', () => {
      render(<SearchBar placeholder="Find posts..." />);
      const input = screen.getByRole('searchbox');

      expect(input).toHaveAttribute('placeholder', 'Find posts...');
    });

    it('can be disabled', () => {
      render(<SearchBar disabled />);
      const input = screen.getByRole('searchbox');

      expect(input).toBeDisabled();
    });

    it('applies custom className', () => {
      render(<SearchBar className="custom-class" />);
      // ClassName applies to the form element (the outermost container)
      const form = screen.getByRole('searchbox').closest('form');

      expect(form).toHaveClass('custom-class');
    });
  });
});
