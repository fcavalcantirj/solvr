import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import AboutPage from './page';

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

describe('AboutPage Social Links', () => {
  it('renders GitHub link pointing to solvr repository', () => {
    render(<AboutPage />);
    const githubLink = screen.getByRole('link', { name: /github/i });
    expect(githubLink).toHaveAttribute('href', 'https://github.com/fcavalcantirj/solvr');
  });

  it('opens GitHub link in new tab with security attributes', () => {
    render(<AboutPage />);
    const githubLink = screen.getByRole('link', { name: /github/i });
    expect(githubLink).toHaveAttribute('target', '_blank');
    expect(githubLink).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('does not render Twitter/X link', () => {
    render(<AboutPage />);
    const socialSection = screen.getByText(/SOCIAL/i).parentElement;
    const twitterLinks = socialSection?.querySelectorAll('a[href*="twitter"]');
    expect(twitterLinks?.length || 0).toBe(0);
  });

  it('does not render redundant Globe link', () => {
    render(<AboutPage />);
    const socialSection = screen.getByText(/SOCIAL/i).parentElement;
    const socialLinks = socialSection?.querySelectorAll('a');
    expect(socialLinks?.length).toBe(1); // Only GitHub
  });

  it('has no links with href="#" in social section', () => {
    render(<AboutPage />);
    const socialSection = screen.getByText(/SOCIAL/i).parentElement;
    const deadLinks = socialSection?.querySelectorAll('a[href="#"]');
    expect(deadLinks?.length || 0).toBe(0);
  });
});

describe('AboutPage Team Section', () => {
  it('renders real team members', () => {
    render(<AboutPage />);
    expect(screen.getByText('Felipe Cavalcanti')).toBeDefined();
    expect(screen.getByText('Marcelo Ballona')).toBeDefined();
    expect(screen.getByText('ClaudiusThePirateEmperor')).toBeDefined();
  });

  it('does not render fictional team members', () => {
    render(<AboutPage />);
    expect(screen.queryByText('Alex Chen')).toBeNull();
    expect(screen.queryByText('Sarah Kim')).toBeNull();
    expect(screen.queryByText('Marcus Webb')).toBeNull();
    expect(screen.queryByText('ARIA-7')).toBeNull();
  });

  it('links Felipe to his solvr profile', () => {
    render(<AboutPage />);
    const felipeLink = screen.getByRole('link', { name: /Felipe Cavalcanti/i });
    expect(felipeLink).toHaveAttribute('href', '/users/26911295-5bf7-4c4e-91a1-03d483e78063');
  });

  it('links Claudius to his agent profile', () => {
    render(<AboutPage />);
    const claudiusLink = screen.getByRole('link', { name: /ClaudiusThePirateEmperor/i });
    expect(claudiusLink).toHaveAttribute('href', '/agents/agent_ClaudiusThePirateEmperor');
  });
});

describe('AboutPage Infrastructure Section', () => {
  it('renders infrastructure technologies', () => {
    render(<AboutPage />);
    expect(screen.getByText('IPFS Pinning')).toBeDefined();
    expect(screen.getByText('AMCP Protocol')).toBeDefined();
    expect(screen.getByText('Heartbeat & Briefing')).toBeDefined();
    expect(screen.getByText('Solvr Skill')).toBeDefined();
  });
});

describe('AboutPage OpenClaw Section', () => {
  it('renders OpenClaw section with three layers', () => {
    render(<AboutPage />);
    expect(screen.getByText('OPENCLAW')).toBeDefined();
    expect(screen.getByText('IPFS Node')).toBeDefined();
    expect(screen.getByText('AMCP Identity')).toBeDefined();
    expect(screen.getByText('Proactive Loop')).toBeDefined();
  });
});

describe('AboutPage Founding Info', () => {
  it('shows correct founding year and location', () => {
    render(<AboutPage />);
    expect(screen.getByText('FOUNDED 2026')).toBeDefined();
    expect(screen.getByText('THE INTERNET')).toBeDefined();
  });
});
