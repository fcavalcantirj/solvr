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

  it('should render 4 unique guide cards (duplicated for carousel)', () => {
    const { container } = render(<GuidesPage />);
    // Carousel duplicates cards for infinite loop — 4 unique guides × 2 = 8 anchors
    const guideCards = container.querySelectorAll('a[href^="#"]');
    expect(guideCards).toHaveLength(8);
    // Verify 4 unique hrefs
    const uniqueHrefs = new Set(Array.from(guideCards).map(a => a.getAttribute('href')));
    expect(uniqueHrefs.size).toBe(4);
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
    expect(screen.getByText('03 — OPENCLAW')).toBeInTheDocument();
  });

  it('should render OpenClaw guide card', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('OpenClaw: 4-Layer Auth Gotcha');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('should have clickable link for OpenClaw guide', () => {
    render(<GuidesPage />);
    const elements = screen.getAllByText('OpenClaw: 4-Layer Auth Gotcha');
    const link = elements[0].closest('a');
    expect(link).toHaveAttribute('href', '#openclaw');
  });

  it('should render OpenClaw section content', () => {
    render(<GuidesPage />);
    expect(screen.getByText('THE 4 LAYERS (HIGHEST PRIORITY FIRST)')).toBeInTheDocument();
    expect(screen.getByText('THE FIX: SEARCH SOLVR FIRST')).toBeInTheDocument();
    expect(screen.getByText('FULL REFERENCE ON SOLVR')).toBeInTheDocument();
  });

  it('should render the 4-layer auth override example prompt', () => {
    const { container } = render(<GuidesPage />);
    const codeBlocks = container.querySelectorAll('code');
    const hasPrompt = Array.from(codeBlocks).some(
      (code) => code.textContent?.includes('ONLY START DOING WORK AFTER FINDING THE POST')
    );
    expect(hasPrompt).toBe(true);
  });

  it('should use prompt-first content instead of curl commands', () => {
    const { container } = render(<GuidesPage />);
    const codeBlocks = container.querySelectorAll('code');
    const hasCurl = Array.from(codeBlocks).some(
      (code) => code.textContent?.includes('curl')
    );
    expect(hasCurl).toBe(false);
  });

  it('should render Solvr skill install reference', () => {
    render(<GuidesPage />);
    expect(screen.getByText('INSTALL THE SOLVR SKILL')).toBeInTheDocument();
  });

  it('should link to Solvr post for full 4-layer reference', () => {
    const { container } = render(<GuidesPage />);
    const link = container.querySelector('a[href="https://solvr.dev/ideas/44781b98-68f2-4ffb-9ad0-cbec604393a4"]');
    expect(link).toBeTruthy();
  });

  it('should NOT render "Documentation is evolving" placeholder heading', () => {
    render(<GuidesPage />);
    expect(screen.queryByText('Documentation is evolving')).not.toBeInTheDocument();
  });
});
