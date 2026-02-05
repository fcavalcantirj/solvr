'use client';

/**
 * Header component
 * Main navigation header with logo, nav links, search bar, and user menu
 * Per SPEC.md Part 4.2: Global Elements
 */

import { useState } from 'react';
import Link from 'next/link';

interface User {
  displayName: string;
  avatarUrl?: string;
}

interface HeaderProps {
  isAuthenticated?: boolean;
  user?: User;
  onLogout?: () => void;
  onSearch?: (query: string) => void;
}

/**
 * Main Header component for Solvr
 * Provides navigation, search, and user authentication UI
 */
export default function Header({
  isAuthenticated = false,
  user,
  onLogout,
  onSearch,
}: HeaderProps) {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (onSearch && searchQuery.trim()) {
      onSearch(searchQuery.trim());
    }
  };

  const handleLogout = () => {
    if (onLogout) {
      onLogout();
    }
    setIsUserMenuOpen(false);
  };

  const toggleMobileMenu = () => {
    setIsMobileMenuOpen(!isMobileMenuOpen);
  };

  const toggleUserMenu = () => {
    setIsUserMenuOpen(!isUserMenuOpen);
  };

  const navLinks = [
    { href: '/feed', label: 'Feed' },
    { href: '/problems', label: 'Problems' },
    { href: '/questions', label: 'Questions' },
    { href: '/ideas', label: 'Ideas' },
  ];

  return (
    <header className="sticky top-0 z-50 border-b border-[var(--border)] bg-[var(--background)]">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Logo */}
          <div className="flex items-center">
            <Link href="/" className="flex items-center space-x-2">
              <span className="text-xl font-bold text-[var(--color-primary)]">Solvr</span>
            </Link>
          </div>

          {/* Desktop Navigation */}
          <nav
            aria-label="Main navigation"
            className="hidden md:flex md:items-center md:space-x-6"
          >
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-sm font-medium text-[var(--foreground-secondary)] transition-colors hover:text-[var(--foreground)]"
              >
                {link.label}
              </Link>
            ))}
          </nav>

          {/* Search Bar */}
          <form
            onSubmit={handleSearchSubmit}
            className="mx-4 hidden flex-1 md:flex md:max-w-md"
          >
            <label htmlFor="search" className="sr-only">
              Search Solvr
            </label>
            <div className="relative w-full">
              {/* Search icon */}
              <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <svg
                  className="h-5 w-5 text-[var(--foreground-muted)]"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                  />
                </svg>
              </div>
              <input
                id="search"
                name="search"
                type="search"
                role="searchbox"
                placeholder="Search..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="block w-full rounded-md border border-[var(--border)] bg-[var(--background-secondary)] py-2 pl-10 pr-3 text-sm text-[var(--foreground)] placeholder-[var(--foreground-muted)] transition-colors focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
              />
            </div>
          </form>

          {/* Right side: User Menu or Login */}
          <div className="flex items-center space-x-4">
            {isAuthenticated && user ? (
              <div className="relative">
                <button
                  onClick={toggleUserMenu}
                  aria-label="User menu"
                  aria-expanded={isUserMenuOpen}
                  className="flex items-center space-x-2 rounded-full focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:ring-offset-2"
                >
                  {user.avatarUrl ? (
                    <img
                      src={user.avatarUrl}
                      alt={user.displayName}
                      className="h-8 w-8 rounded-full"
                    />
                  ) : (
                    <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--color-primary)] text-sm font-medium text-white">
                      {user.displayName.charAt(0).toUpperCase()}
                    </div>
                  )}
                </button>

                {/* User Dropdown Menu */}
                {isUserMenuOpen && (
                  <div className="absolute right-0 mt-2 w-48 rounded-md border border-[var(--border)] bg-[var(--background)] py-1 shadow-lg">
                    <Link
                      href="/dashboard"
                      className="block px-4 py-2 text-sm text-[var(--foreground)] hover:bg-[var(--background-secondary)]"
                    >
                      Dashboard
                    </Link>
                    <Link
                      href="/settings"
                      className="block px-4 py-2 text-sm text-[var(--foreground)] hover:bg-[var(--background-secondary)]"
                    >
                      Settings
                    </Link>
                    <hr className="my-1 border-[var(--border)]" />
                    <button
                      onClick={handleLogout}
                      className="block w-full px-4 py-2 text-left text-sm text-[var(--color-error)] hover:bg-[var(--background-secondary)]"
                    >
                      Logout
                    </button>
                  </div>
                )}
              </div>
            ) : (
              <Link
                href="/login"
                className="rounded-md bg-[var(--color-primary)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--color-primary-hover)]"
              >
                Login
              </Link>
            )}

            {/* Mobile Menu Button */}
            <button
              onClick={toggleMobileMenu}
              aria-label="Menu"
              aria-expanded={isMobileMenuOpen}
              className="inline-flex items-center justify-center rounded-md p-2 text-[var(--foreground-secondary)] hover:bg-[var(--background-secondary)] hover:text-[var(--foreground)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] md:hidden"
            >
              <svg
                className="h-6 w-6"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                {isMobileMenuOpen ? (
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                ) : (
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M4 6h16M4 12h16M4 18h16"
                  />
                )}
              </svg>
            </button>
          </div>
        </div>

        {/* Mobile Navigation */}
        {isMobileMenuOpen && (
          <nav
            data-testid="mobile-nav"
            aria-label="Mobile navigation"
            className="border-t border-[var(--border)] py-4 md:hidden"
          >
            {/* Mobile Search */}
            <form onSubmit={handleSearchSubmit} className="mb-4 px-2">
              <label htmlFor="mobile-search" className="sr-only">
                Search Solvr
              </label>
              <div className="relative">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                  <svg
                    className="h-5 w-5 text-[var(--foreground-muted)]"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                    />
                  </svg>
                </div>
                <input
                  id="mobile-search"
                  name="search"
                  type="search"
                  role="searchbox"
                  placeholder="Search..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="block w-full rounded-md border border-[var(--border)] bg-[var(--background-secondary)] py-2 pl-10 pr-3 text-sm text-[var(--foreground)] placeholder-[var(--foreground-muted)] focus:border-[var(--color-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
                />
              </div>
            </form>

            {/* Mobile Nav Links */}
            <div className="space-y-1">
              {navLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className="block rounded-md px-3 py-2 text-base font-medium text-[var(--foreground-secondary)] hover:bg-[var(--background-secondary)] hover:text-[var(--foreground)]"
                >
                  {link.label}
                </Link>
              ))}
            </div>
          </nav>
        )}
      </div>
    </header>
  );
}
