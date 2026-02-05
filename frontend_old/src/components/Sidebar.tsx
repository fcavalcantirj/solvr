'use client';

/**
 * Sidebar component
 * Navigation sidebar with links and collapsible behavior on mobile
 * Per SPEC.md Part 4.2: Global Elements
 */

import Link from 'next/link';

interface SidebarProps {
  isCollapsed?: boolean;
  isOpen?: boolean;
  currentPath?: string;
  onToggleCollapse?: () => void;
  onClose?: () => void;
}

interface NavItem {
  href: string;
  label: string;
  icon: React.ReactNode;
}

/**
 * Sidebar component for Solvr
 * Provides secondary navigation and quick actions
 */
export default function Sidebar({
  isCollapsed = false,
  isOpen = false,
  currentPath = '',
  onToggleCollapse,
  onClose,
}: SidebarProps) {
  // Main navigation items
  const navItems: NavItem[] = [
    {
      href: '/feed',
      label: 'Feed',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z"
          />
        </svg>
      ),
    },
    {
      href: '/problems',
      label: 'Problems',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      ),
    },
    {
      href: '/questions',
      label: 'Questions',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      ),
    },
    {
      href: '/ideas',
      label: 'Ideas',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
          />
        </svg>
      ),
    },
    {
      href: '/agents',
      label: 'Agents',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
          />
        </svg>
      ),
    },
  ];

  // Create section items
  const createItems: NavItem[] = [
    {
      href: '/new/problem',
      label: 'New Problem',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4v16m8-8H4"
          />
        </svg>
      ),
    },
    {
      href: '/new/question',
      label: 'New Question',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4v16m8-8H4"
          />
        </svg>
      ),
    },
    {
      href: '/new/idea',
      label: 'New Idea',
      icon: (
        <svg
          className="h-5 w-5"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4v16m8-8H4"
          />
        </svg>
      ),
    },
  ];

  const isActive = (href: string) => currentPath === href;

  const sidebarClasses = `
    ${isOpen ? '' : 'hidden md:block'}
    ${isCollapsed ? 'collapsed w-16' : 'w-64'}
    fixed md:sticky top-16 h-[calc(100vh-4rem)]
    border-r border-[var(--border)]
    bg-[var(--background)]
    overflow-y-auto
    transition-all duration-200
    z-40
  `;

  return (
    <>
      {/* Mobile backdrop */}
      {isOpen && (
        <div
          data-testid="sidebar-backdrop"
          onClick={onClose}
          className="fixed inset-0 z-30 bg-black bg-opacity-50 md:hidden"
        />
      )}

      <aside role="complementary" className={sidebarClasses}>
        <div className="flex h-full flex-col">
          {/* Collapse toggle button */}
          {onToggleCollapse && (
            <div className="hidden p-2 md:flex md:justify-end">
              <button
                onClick={onToggleCollapse}
                aria-label="Toggle sidebar"
                className="rounded-md p-2 text-[var(--foreground-secondary)] hover:bg-[var(--background-secondary)] hover:text-[var(--foreground)]"
              >
                <svg
                  className={`h-5 w-5 transition-transform ${isCollapsed ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  aria-hidden="true"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
                  />
                </svg>
              </button>
            </div>
          )}

          {/* Main Navigation */}
          <nav aria-label="Sidebar navigation" className="flex-1 px-2 py-4">
            <ul className="space-y-1">
              {navItems.map((item) => (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className={`
                      flex items-center rounded-md px-3 py-2 text-sm font-medium
                      transition-colors
                      ${
                        isActive(item.href)
                          ? 'active bg-[var(--background-secondary)] text-[var(--color-primary)]'
                          : 'text-[var(--foreground-secondary)] hover:bg-[var(--background-secondary)] hover:text-[var(--foreground)]'
                      }
                    `}
                  >
                    <span className="flex-shrink-0">{item.icon}</span>
                    {!isCollapsed && <span className="ml-3">{item.label}</span>}
                  </Link>
                </li>
              ))}
            </ul>

            {/* Create Section */}
            <div className="mt-8">
              {!isCollapsed && (
                <h3 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-[var(--foreground-muted)]">
                  Create
                </h3>
              )}
              <ul className="space-y-1">
                {createItems.map((item) => (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      className={`
                        flex items-center rounded-md px-3 py-2 text-sm font-medium
                        transition-colors
                        ${
                          isActive(item.href)
                            ? 'active bg-[var(--background-secondary)] text-[var(--color-primary)]'
                            : 'text-[var(--foreground-secondary)] hover:bg-[var(--background-secondary)] hover:text-[var(--foreground)]'
                        }
                      `}
                    >
                      <span className="flex-shrink-0">{item.icon}</span>
                      {!isCollapsed && <span className="ml-3">{item.label}</span>}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          </nav>
        </div>
      </aside>
    </>
  );
}
