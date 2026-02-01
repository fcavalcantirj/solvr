/**
 * Tests for TagChip component
 * Per PRD requirement: Display tag text and link to search with tag filter
 */

import { render, screen } from '@testing-library/react';

// Mock next/link with className support
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) {
    return <a href={href} className={className}>{children}</a>;
  };
});

import TagChip from '../components/TagChip';

describe('TagChip', () => {
  describe('basic rendering', () => {
    it('displays tag text', () => {
      render(<TagChip tag="javascript" />);
      expect(screen.getByText('javascript')).toBeInTheDocument();
    });

    it('links to search with tag filter', () => {
      render(<TagChip tag="react" />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/search?tags=react');
    });
  });

  describe('styling', () => {
    it('has rounded styling', () => {
      render(<TagChip tag="go" />);
      const chip = screen.getByText('go');
      expect(chip).toHaveClass('rounded-full');
    });

    it('applies custom className', () => {
      render(<TagChip tag="python" className="custom-class" />);
      const chip = screen.getByText('python');
      expect(chip.closest('a')).toHaveClass('custom-class');
    });
  });

  describe('size variants', () => {
    it('renders small size', () => {
      render(<TagChip tag="rust" size="sm" />);
      expect(screen.getByText('rust')).toBeInTheDocument();
    });

    it('renders large size', () => {
      render(<TagChip tag="typescript" size="lg" />);
      expect(screen.getByText('typescript')).toBeInTheDocument();
    });
  });

  describe('clickable option', () => {
    it('is clickable by default', () => {
      render(<TagChip tag="node" />);
      expect(screen.getByRole('link')).toBeInTheDocument();
    });

    it('is not a link when clickable is false', () => {
      render(<TagChip tag="express" clickable={false} />);
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
      expect(screen.getByText('express')).toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('link is accessible', () => {
      render(<TagChip tag="docker" />);
      const link = screen.getByRole('link');
      expect(link).toBeInTheDocument();
    });
  });

  describe('special characters', () => {
    it('handles tags with special characters', () => {
      render(<TagChip tag="c++" />);
      expect(screen.getByText('c++')).toBeInTheDocument();
    });

    it('encodes tag in URL', () => {
      render(<TagChip tag="c#" />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', '/search?tags=c%23');
    });
  });
});
