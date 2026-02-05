/**
 * Footer component
 * Site footer with links to Terms, Privacy, About, API Docs, and GitHub
 * Per SPEC.md Part 4.2: Global Elements
 */

import Link from 'next/link';

interface FooterLink {
  href: string;
  label: string;
  external?: boolean;
}

/**
 * Footer component for Solvr
 * Provides links to legal pages, documentation, and social
 */
export default function Footer() {
  const currentYear = new Date().getFullYear();

  const links: FooterLink[] = [
    { href: '/about', label: 'About' },
    { href: '/docs/api', label: 'API Docs' },
    { href: 'https://github.com/fcavalcantirj/solvr', label: 'GitHub', external: true },
    { href: '/terms', label: 'Terms' },
    { href: '/privacy', label: 'Privacy' },
  ];

  return (
    <footer className="border-t border-[var(--border)] bg-[var(--background)] py-8">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col items-center space-y-4 md:flex-row md:justify-between md:space-y-0">
          {/* Brand and tagline */}
          <div className="text-center md:text-left">
            <p className="text-sm font-medium text-[var(--foreground)]">
              &copy; {currentYear} Solvr
            </p>
            <p className="mt-1 text-sm text-[var(--foreground-muted)]">
              Built for humans and AI agents
            </p>
          </div>

          {/* Navigation links */}
          <nav aria-label="Footer navigation" className="flex flex-wrap justify-center gap-6">
            {links.map((link) =>
              link.external ? (
                <a
                  key={link.href}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-[var(--foreground-secondary)] transition-colors hover:text-[var(--foreground)]"
                >
                  {link.label}
                </a>
              ) : (
                <Link
                  key={link.href}
                  href={link.href}
                  className="text-sm text-[var(--foreground-secondary)] transition-colors hover:text-[var(--foreground)]"
                >
                  {link.label}
                </Link>
              )
            )}
          </nav>
        </div>
      </div>
    </footer>
  );
}
