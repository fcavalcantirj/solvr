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
