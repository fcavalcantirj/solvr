/**
 * Tests for StatusBadge component
 * Per PRD requirement: Different colors per status
 */

import { render, screen } from '@testing-library/react';

import StatusBadge from '../components/StatusBadge';
import { PostStatus } from '../lib/types';

describe('StatusBadge', () => {
  describe('basic rendering', () => {
    it('displays status text', () => {
      render(<StatusBadge status="open" />);
      expect(screen.getByText('open')).toBeInTheDocument();
    });

    it('formats status with underscores', () => {
      render(<StatusBadge status="in_progress" />);
      expect(screen.getByText('in progress')).toBeInTheDocument();
    });
  });

  describe('color variants', () => {
    it('renders green for open status', () => {
      render(<StatusBadge status="open" />);
      const badge = screen.getByText('open');
      expect(badge).toHaveClass('bg-green-100');
    });

    it('renders emerald for solved status', () => {
      render(<StatusBadge status="solved" />);
      const badge = screen.getByText('solved');
      expect(badge).toHaveClass('bg-emerald-100');
    });

    it('renders emerald for answered status', () => {
      render(<StatusBadge status="answered" />);
      const badge = screen.getByText('answered');
      expect(badge).toHaveClass('bg-emerald-100');
    });

    it('renders yellow for in_progress status', () => {
      render(<StatusBadge status="in_progress" />);
      const badge = screen.getByText('in progress');
      expect(badge).toHaveClass('bg-yellow-100');
    });

    it('renders yellow for active status', () => {
      render(<StatusBadge status="active" />);
      const badge = screen.getByText('active');
      expect(badge).toHaveClass('bg-yellow-100');
    });

    it('renders gray for closed status', () => {
      render(<StatusBadge status="closed" />);
      const badge = screen.getByText('closed');
      expect(badge).toHaveClass('bg-gray-100');
    });

    it('renders gray for stale status', () => {
      render(<StatusBadge status="stale" />);
      const badge = screen.getByText('stale');
      expect(badge).toHaveClass('bg-gray-100');
    });

    it('renders gray for dormant status', () => {
      render(<StatusBadge status="dormant" />);
      const badge = screen.getByText('dormant');
      expect(badge).toHaveClass('bg-gray-100');
    });

    it('renders indigo for evolved status', () => {
      render(<StatusBadge status="evolved" />);
      const badge = screen.getByText('evolved');
      expect(badge).toHaveClass('bg-indigo-100');
    });

    it('renders slate for draft status', () => {
      render(<StatusBadge status="draft" />);
      const badge = screen.getByText('draft');
      expect(badge).toHaveClass('bg-slate-100');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(<StatusBadge status="open" size="sm" />);
      expect(screen.getByText('open')).toBeInTheDocument();
    });

    it('renders large size', () => {
      render(<StatusBadge status="open" size="lg" />);
      expect(screen.getByText('open')).toBeInTheDocument();
    });
  });

  describe('styling', () => {
    it('has rounded styling', () => {
      render(<StatusBadge status="open" />);
      const badge = screen.getByText('open');
      expect(badge).toHaveClass('rounded');
    });

    it('text is capitalized', () => {
      render(<StatusBadge status="open" />);
      const badge = screen.getByText('open');
      expect(badge).toHaveClass('capitalize');
    });
  });
});
