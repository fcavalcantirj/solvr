import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import GuidesPage from './page';

// Mock Header and Footer components
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));

vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe('GuidesPage', () => {
  it('should not render "MORE GUIDES COMING SOON" placeholder text', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('MORE GUIDES COMING SOON')).not.toBeInTheDocument();
  });

  it('should render exactly 2 guide cards', () => {
    const { container } = render(<GuidesPage />);
    // Guide cards are anchor tags with specific class within the grid
    const guideCards = container.querySelectorAll('a[href^="#"]');
    expect(guideCards).toHaveLength(2);
  });

  it('should render "Getting Started with AI Agents" guide card', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Getting Started with AI Agents');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('should render "Search Before You Solve" guide card', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Search Before You Solve');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('should NOT render "Contributing Solutions" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Contributing Solutions')).not.toBeInTheDocument();
  });

  it('should NOT render "Authentication Flows" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Authentication Flows')).not.toBeInTheDocument();
  });

  it('should NOT render "MCP Server Integration" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('MCP Server Integration')).not.toBeInTheDocument();
  });

  it('should NOT render "Rate Limits & Best Practices" unlinked guide', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Rate Limits & Best Practices')).not.toBeInTheDocument();
  });

  it('should have clickable link for Getting Started guide', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Getting Started with AI Agents');
    const link = elements[0].closest('a');
    expect(link).toHaveAttribute('href', '#agent-quickstart');
  });

  it('should have clickable link for Search Before You Solve guide', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Search Before You Solve');
    const link = elements[0].closest('a');
    expect(link).toHaveAttribute('href', '#search-pattern');
  });

  it('should render page heading "Build with Solvr"', () => {
    render(<GuidesPage />);
    expect(screen.getByText('Build with Solvr')).toBeInTheDocument();
  });

  it('should render "ALL GUIDES" section heading', () => {
    render(<GuidesPage />);
    expect(screen.getByText('ALL GUIDES')).toBeInTheDocument();
  });

  it('should render guide content sections', () => {
    render(<GuidesPage />);
    // Verify both guide content sections are present
    expect(screen.getByText('01 — QUICKSTART')).toBeInTheDocument();
    expect(screen.getByText('02 — BEST PRACTICE')).toBeInTheDocument();
  });

  it('should NOT render "Documentation is evolving" placeholder heading', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Documentation is evolving')).not.toBeInTheDocument();
  });
});
