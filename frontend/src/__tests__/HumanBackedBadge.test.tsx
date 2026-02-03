/**
 * Tests for HumanBackedBadge component
 * Per PRD requirement: Human-Backed badge display on agent profile and posts
 *
 * Requirements:
 * - Show badge on agent profile page
 * - Show badge next to agent name on posts/answers
 * - Optionally show human's handle if human opts in
 *
 * TDD approach: RED -> GREEN -> REFACTOR
 */

import { render, screen } from '@testing-library/react';

// Import the component (will fail initially - RED phase)
import HumanBackedBadge from '../components/HumanBackedBadge';

describe('HumanBackedBadge', () => {
  describe('basic rendering', () => {
    it('renders the badge with checkmark icon', () => {
      render(<HumanBackedBadge />);
      // Should show verified/human-backed indicator
      expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
    });

    it('displays "Human-Backed" text', () => {
      render(<HumanBackedBadge />);
      expect(screen.getByText(/Human-Backed/i)).toBeInTheDocument();
    });

    it('has appropriate styling for verified status', () => {
      render(<HumanBackedBadge />);
      const badge = screen.getByTestId('human-backed-badge');
      // Should have green/verified color scheme
      expect(badge).toHaveClass('bg-emerald-100');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(<HumanBackedBadge size="sm" />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveClass('text-xs');
    });

    it('renders medium size (default)', () => {
      render(<HumanBackedBadge />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveClass('text-xs');
    });

    it('renders large size', () => {
      render(<HumanBackedBadge size="lg" />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveClass('text-sm');
    });
  });

  describe('human handle display', () => {
    it('does not show human handle by default', () => {
      render(<HumanBackedBadge humanUsername="john_dev" />);
      expect(screen.queryByText('john_dev')).not.toBeInTheDocument();
    });

    it('shows human handle when showHumanHandle is true', () => {
      render(<HumanBackedBadge humanUsername="john_dev" showHumanHandle />);
      expect(screen.getByText(/john_dev/)).toBeInTheDocument();
    });

    it('formats human handle with @ prefix', () => {
      render(<HumanBackedBadge humanUsername="jane_coder" showHumanHandle />);
      expect(screen.getByText(/@jane_coder/)).toBeInTheDocument();
    });

    it('does not show human section when humanUsername is not provided', () => {
      render(<HumanBackedBadge showHumanHandle />);
      // Should still render badge without crashing
      expect(screen.getByTestId('human-backed-badge')).toBeInTheDocument();
      expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    });
  });

  describe('tooltip/title', () => {
    it('has descriptive title attribute', () => {
      render(<HumanBackedBadge />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveAttribute('title', 'This agent is verified by a human');
    });

    it('includes human name in title when available', () => {
      render(<HumanBackedBadge humanUsername="john_dev" showHumanHandle />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge.getAttribute('title')).toContain('john_dev');
    });
  });

  describe('compact mode', () => {
    it('shows only icon in compact mode', () => {
      render(<HumanBackedBadge compact />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toBeInTheDocument();
      // Text should be visually hidden but still accessible
      expect(screen.getByText(/Human-Backed/i)).toHaveClass('sr-only');
    });
  });

  describe('accessibility', () => {
    it('has appropriate aria-label', () => {
      render(<HumanBackedBadge />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveAttribute('aria-label', 'Human-Backed verified agent');
    });

    it('is announced correctly by screen readers', () => {
      render(<HumanBackedBadge humanUsername="john_dev" showHumanHandle />);
      const badge = screen.getByTestId('human-backed-badge');
      expect(badge).toHaveAttribute('role', 'status');
    });
  });
});
