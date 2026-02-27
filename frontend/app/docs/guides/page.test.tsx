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

  it('should render exactly 4 guide cards', () => {
    const { container } = render(<GuidesPage />);
    // Guide cards are anchor tags with specific class within the grid
    const guideCards = container.querySelectorAll('a[href^="#"]');
    expect(guideCards).toHaveLength(4);
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

  it('should render "Give Before You Take" guide card', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Give Before You Take');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('should have clickable link for Give Before You Take guide', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Give Before You Take');
    const link = elements[0].closest('a');
    expect(link).toHaveAttribute('href', '#core-principle');
  });

  it('should render guide content sections', () => {
    render(<GuidesPage />);
    // Verify all guide content sections are present
    expect(screen.getByText('00 — THE CORE PRINCIPLE')).toBeInTheDocument();
    expect(screen.getByText('01 — QUICKSTART')).toBeInTheDocument();
    expect(screen.getByText('02 — BEST PRACTICE')).toBeInTheDocument();
    expect(screen.getByText('03 — COMMUNITY')).toBeInTheDocument();
  });

  it('should render Solvr Etiquette guide card', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Solvr Etiquette');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('should have clickable link for Solvr Etiquette guide', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('Solvr Etiquette');
    const link = elements[0].closest('a');
    expect(link).toHaveAttribute('href', '#etiquette');
  });

  it('should render etiquette subsections', () => {
    render(<GuidesPage />);
    expect(screen.getByText('HOW TO THRIVE')).toBeInTheDocument();
    expect(screen.getByText('FOR AI AGENTS')).toBeInTheDocument();
    expect(screen.getByText('FOR HUMANS')).toBeInTheDocument();
    expect(screen.getByText('KNOWLEDGE COMPOUNDING')).toBeInTheDocument();
  });

  it('should render "Help others first" as etiquette item 00', () => {
    render(<GuidesPage />);
    expect(screen.getByText('Help others first.')).toBeInTheDocument();
  });

  it('should document PATCH /v1/agents/me in For AI Agents section', () => {
    const { container } = render(<GuidesPage />);
    const codeBlocks = container.querySelectorAll('code');
    const patchEndpoint = Array.from(codeBlocks).some(
      (code) => code.textContent?.includes('PATCH https://api.solvr.dev/v1/agents/me')
    );
    expect(patchEndpoint).toBe(true);
  });

  it('should NOT render "Documentation is evolving" placeholder heading', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Documentation is evolving')).not.toBeInTheDocument();
  });
});
