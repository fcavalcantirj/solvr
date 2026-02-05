/**
 * Tests for Header component
 * Tests per PRD requirement: Create Header component with logo, nav links, search bar, user menu
 */

import { render, screen, fireEvent } from '@testing-library/react';

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) {
    return <a href={href}>{children}</a>;
  };
});

// Import component after mocks
import Header from '../components/Header';

describe('Header', () => {
  it('renders the header element', () => {
    render(<Header />);

    const header = screen.getByRole('banner');
    expect(header).toBeInTheDocument();
  });

  it('displays the Solvr logo/brand', () => {
    render(<Header />);

    const logo = screen.getByRole('link', { name: /solvr/i });
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute('href', '/');
  });

  it('renders navigation links', () => {
    render(<Header />);

    const nav = screen.getByRole('navigation');
    expect(nav).toBeInTheDocument();

    // Verify core navigation items exist
    expect(screen.getByRole('link', { name: /feed/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /problems/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /questions/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /ideas/i })).toBeInTheDocument();
  });

  it('renders a search bar', () => {
    render(<Header />);

    const searchInput = screen.getByRole('searchbox');
    expect(searchInput).toBeInTheDocument();
    expect(searchInput).toHaveAttribute('placeholder', expect.stringContaining('Search'));
  });

  it('renders login button when not authenticated', () => {
    render(<Header isAuthenticated={false} />);

    const loginButton = screen.getByRole('link', { name: /login/i });
    expect(loginButton).toBeInTheDocument();
    expect(loginButton).toHaveAttribute('href', '/login');
  });

  it('renders user menu when authenticated', () => {
    render(
      <Header
        isAuthenticated={true}
        user={{ displayName: 'John Doe', avatarUrl: '/avatar.jpg' }}
      />
    );

    // Should not show login button
    expect(screen.queryByRole('link', { name: /login/i })).not.toBeInTheDocument();

    // Should show user menu
    const userButton = screen.getByRole('button', { name: /user menu/i });
    expect(userButton).toBeInTheDocument();
  });

  it('shows user dropdown when user menu is clicked', () => {
    render(
      <Header
        isAuthenticated={true}
        user={{ displayName: 'John Doe', avatarUrl: '/avatar.jpg' }}
      />
    );

    const userButton = screen.getByRole('button', { name: /user menu/i });
    fireEvent.click(userButton);

    // Dropdown should be visible
    expect(screen.getByRole('link', { name: /dashboard/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /settings/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /logout/i })).toBeInTheDocument();
  });

  it('calls onLogout when logout button is clicked', () => {
    const onLogout = jest.fn();
    render(
      <Header
        isAuthenticated={true}
        user={{ displayName: 'John Doe', avatarUrl: '/avatar.jpg' }}
        onLogout={onLogout}
      />
    );

    // Open dropdown
    const userButton = screen.getByRole('button', { name: /user menu/i });
    fireEvent.click(userButton);

    // Click logout
    const logoutButton = screen.getByRole('button', { name: /logout/i });
    fireEvent.click(logoutButton);

    expect(onLogout).toHaveBeenCalledTimes(1);
  });

  it('renders mobile menu button on small screens', () => {
    render(<Header />);

    // Mobile menu button should exist (visible on mobile)
    const mobileMenuButton = screen.getByRole('button', { name: /menu/i });
    expect(mobileMenuButton).toBeInTheDocument();
  });

  it('toggles mobile navigation when hamburger menu is clicked', () => {
    render(<Header />);

    const mobileMenuButton = screen.getByRole('button', { name: /menu/i });

    // Initially, mobile nav should not be visible
    expect(screen.queryByTestId('mobile-nav')).not.toBeInTheDocument();

    // Click to open
    fireEvent.click(mobileMenuButton);
    expect(screen.getByTestId('mobile-nav')).toBeInTheDocument();

    // Click to close
    fireEvent.click(mobileMenuButton);
    expect(screen.queryByTestId('mobile-nav')).not.toBeInTheDocument();
  });

  it('calls onSearch when search is submitted', () => {
    const onSearch = jest.fn();
    render(<Header onSearch={onSearch} />);

    const searchInput = screen.getByRole('searchbox');
    fireEvent.change(searchInput, { target: { value: 'test query' } });

    const searchForm = searchInput.closest('form');
    fireEvent.submit(searchForm!);

    expect(onSearch).toHaveBeenCalledWith('test query');
  });

  it('has appropriate aria labels for accessibility', () => {
    render(<Header />);

    // Main nav should have aria-label
    const nav = screen.getByRole('navigation');
    expect(nav).toHaveAttribute('aria-label');

    // Search should have appropriate labeling
    const searchInput = screen.getByRole('searchbox');
    expect(searchInput).toHaveAccessibleName();
  });
});

describe('Header navigation links', () => {
  it('has correct href for Feed link', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: /feed/i })).toHaveAttribute('href', '/feed');
  });

  it('has correct href for Problems link', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: /problems/i })).toHaveAttribute(
      'href',
      '/problems'
    );
  });

  it('has correct href for Questions link', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: /questions/i })).toHaveAttribute(
      'href',
      '/questions'
    );
  });

  it('has correct href for Ideas link', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: /ideas/i })).toHaveAttribute('href', '/ideas');
  });
});

describe('Header styling', () => {
  it('has sticky positioning class', () => {
    render(<Header />);

    const header = screen.getByRole('banner');
    expect(header).toHaveClass('sticky');
  });

  it('is positioned at top', () => {
    render(<Header />);

    const header = screen.getByRole('banner');
    expect(header).toHaveClass('top-0');
  });

  it('has appropriate z-index for layering', () => {
    render(<Header />);

    const header = screen.getByRole('banner');
    expect(header.className).toMatch(/z-\d+/);
  });
});
