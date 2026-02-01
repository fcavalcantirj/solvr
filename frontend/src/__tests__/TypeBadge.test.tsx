/**
 * Tests for TypeBadge component
 * Per PRD requirement: Different icons for problem/question/idea
 */

import { render, screen } from '@testing-library/react';

import TypeBadge from '../components/TypeBadge';

describe('TypeBadge', () => {
  describe('basic rendering', () => {
    it('displays type text for problem', () => {
      render(<TypeBadge type="problem" />);
      expect(screen.getByText('problem')).toBeInTheDocument();
    });

    it('displays type text for question', () => {
      render(<TypeBadge type="question" />);
      expect(screen.getByText('question')).toBeInTheDocument();
    });

    it('displays type text for idea', () => {
      render(<TypeBadge type="idea" />);
      expect(screen.getByText('idea')).toBeInTheDocument();
    });
  });

  describe('icons', () => {
    it('renders icon for problem', () => {
      render(<TypeBadge type="problem" />);
      const icon = document.querySelector('svg');
      expect(icon).toBeInTheDocument();
    });

    it('renders icon for question', () => {
      render(<TypeBadge type="question" />);
      const icon = document.querySelector('svg');
      expect(icon).toBeInTheDocument();
    });

    it('renders icon for idea', () => {
      render(<TypeBadge type="idea" />);
      const icon = document.querySelector('svg');
      expect(icon).toBeInTheDocument();
    });
  });

  describe('color variants', () => {
    it('renders red for problem type', () => {
      render(<TypeBadge type="problem" />);
      // Get the outermost span (the one with bg-red-100)
      const badge = screen.getByText('problem').parentElement;
      expect(badge).toHaveClass('bg-red-100');
    });

    it('renders blue for question type', () => {
      render(<TypeBadge type="question" />);
      const badge = screen.getByText('question').parentElement;
      expect(badge).toHaveClass('bg-blue-100');
    });

    it('renders purple for idea type', () => {
      render(<TypeBadge type="idea" />);
      const badge = screen.getByText('idea').parentElement;
      expect(badge).toHaveClass('bg-purple-100');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(<TypeBadge type="problem" size="sm" />);
      expect(screen.getByText('problem')).toBeInTheDocument();
    });

    it('renders large size', () => {
      render(<TypeBadge type="question" size="lg" />);
      expect(screen.getByText('question')).toBeInTheDocument();
    });
  });

  describe('icon only mode', () => {
    it('can render icon only when showLabel is false', () => {
      render(<TypeBadge type="problem" showLabel={false} />);
      expect(screen.queryByText('problem')).not.toBeInTheDocument();
      const icon = document.querySelector('svg');
      expect(icon).toBeInTheDocument();
    });
  });

  describe('styling', () => {
    it('has rounded styling', () => {
      render(<TypeBadge type="problem" />);
      const badge = screen.getByText('problem').parentElement;
      expect(badge).toHaveClass('rounded');
    });

    it('text is capitalized', () => {
      render(<TypeBadge type="problem" />);
      const badge = screen.getByText('problem').parentElement;
      expect(badge).toHaveClass('capitalize');
    });
  });
});
