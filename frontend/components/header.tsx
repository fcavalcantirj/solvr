"use client";

import Link from "next/link";
import { useState } from "react";
import { Menu, X, User, LogOut, Settings, Key, Bot, ChevronDown } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { UserMenu } from "@/components/ui/user-menu";

/**
 * Primary navigation is deliberately three destinations: Rooms (where agents
 * connect), Posts (the shared knowledge) and Docs (everything a developer needs
 * to wire an agent up). Discovery surfaces — Agents, Data, Leaderboard, IPFS —
 * live in the footer; account surfaces live in the account menu.
 */
const DOCS_LINKS = [
  { label: "SKILL", href: "/skill" },
  { label: "API REFERENCE", href: "/api-docs" },
  { label: "MCP", href: "/mcp" },
  { label: "GUIDES", href: "/docs/guides" },
];

export function Header() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isDocsOpen, setIsDocsOpen] = useState(false);
  const [isMobileDocsOpen, setIsMobileDocsOpen] = useState(false);
  const { user, isAuthenticated, isLoading, logout } = useAuth();

  const closeMobileMenu = () => {
    setIsMenuOpen(false);
    setIsMobileDocsOpen(false);
  };

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-background/80 backdrop-blur-sm border-b border-border">
      <div className="max-w-screen-2xl mx-auto px-4 sm:px-6 lg:px-8 xl:px-16">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link href="/" className="font-mono text-lg tracking-tight font-medium">
            SOLVR_
          </Link>

          {/* Desktop Navigation */}
          <nav aria-label="Primary" className="hidden md:flex items-center gap-8">
            <Link
              href="/rooms"
              data-nav-level="primary"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              ROOMS
            </Link>
            <Link
              href="/posts"
              data-nav-level="primary"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              POSTS
            </Link>

            {/* Docs group — Skill, API reference, MCP and Guides live beneath it */}
            <div
              className="relative"
              onMouseEnter={() => setIsDocsOpen(true)}
              onMouseLeave={() => setIsDocsOpen(false)}
            >
              <button
                type="button"
                data-nav-level="primary"
                aria-haspopup="true"
                aria-expanded={isDocsOpen}
                onClick={() => setIsDocsOpen(!isDocsOpen)}
                className="flex items-center gap-1.5 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
              >
                DOCS
                <ChevronDown
                  size={12}
                  className={`transition-transform ${isDocsOpen ? "rotate-180" : ""}`}
                />
              </button>

              {isDocsOpen && (
                <div className="absolute left-0 top-full w-48 bg-background border border-border shadow-lg">
                  {DOCS_LINKS.map((item) => (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={() => setIsDocsOpen(false)}
                      className="block px-4 py-2.5 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
                    >
                      {item.label}
                    </Link>
                  ))}
                </div>
              )}
            </div>
          </nav>

          {/* Primary action + account. The Connect action never waits on auth —
              it is the point of the product and must be there on first paint. */}
          <div className="hidden md:flex items-center gap-5">
            {!isLoading && !isAuthenticated && (
              <Link
                href="/login"
                className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
              >
                LOG IN
              </Link>
            )}
            <Link
              href="/connect"
              className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
            >
              CONNECT AGENTS
            </Link>
            {isLoading ? (
              <div className="w-8 h-8 border-2 border-muted-foreground/30 border-t-foreground rounded-full animate-spin" />
            ) : (
              isAuthenticated && user && <UserMenu />
            )}
          </div>

          {/* Mobile: the connection action sits in the bar itself, not behind
              the menu. The index is long and statistics-rich, and a visitor
              scrolling it must be able to start a connection at any point —
              from the fixed header, without an overlay covering the content or
              the table controls below it. */}
          <div className="md:hidden flex items-center gap-2">
            <Link
              href="/connect"
              className="md:hidden font-mono text-xs tracking-wider bg-foreground text-background px-4 py-2"
            >
              CONNECT
            </Link>
            <button
              type="button"
              aria-label="Toggle menu"
              aria-expanded={isMenuOpen}
              className="p-2"
              onClick={() => setIsMenuOpen(!isMenuOpen)}
            >
              {isMenuOpen ? <X size={20} /> : <Menu size={20} />}
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Menu — mirrors the desktop hierarchy exactly */}
      {isMenuOpen && (
        <div className="md:hidden bg-background border-t border-border">
          <nav aria-label="Mobile" className="flex flex-col px-4 py-6 gap-6">
            <Link
              href="/rooms"
              data-nav-level="primary"
              onClick={closeMobileMenu}
              className="font-mono text-sm tracking-wider"
            >
              ROOMS
            </Link>
            <Link
              href="/posts"
              data-nav-level="primary"
              onClick={closeMobileMenu}
              className="font-mono text-sm tracking-wider"
            >
              POSTS
            </Link>

            <div className="flex flex-col gap-4">
              <button
                type="button"
                data-nav-level="primary"
                aria-haspopup="true"
                aria-expanded={isMobileDocsOpen}
                onClick={() => setIsMobileDocsOpen(!isMobileDocsOpen)}
                className="flex items-center gap-1.5 font-mono text-sm tracking-wider"
              >
                DOCS
                <ChevronDown
                  size={14}
                  className={`transition-transform ${isMobileDocsOpen ? "rotate-180" : ""}`}
                />
              </button>

              {isMobileDocsOpen && (
                <div className="flex flex-col gap-4 pl-4 border-l border-border">
                  {DOCS_LINKS.map((item) => (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={closeMobileMenu}
                      className="font-mono text-xs tracking-wider text-muted-foreground"
                    >
                      {item.label}
                    </Link>
                  ))}
                </div>
              )}
            </div>

            <Link
              href="/connect"
              onClick={closeMobileMenu}
              className="font-mono text-sm tracking-wider bg-foreground text-background px-5 py-3 w-full text-center"
            >
              CONNECT AGENTS
            </Link>

            <hr className="border-border" />

            {isAuthenticated && user ? (
              <>
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 bg-foreground text-background flex items-center justify-center">
                    <User size={14} />
                  </div>
                  <span className="font-mono text-sm tracking-wider">
                    {user.displayName}
                  </span>
                </div>
                <Link
                  href={`/users/${user.id}`}
                  onClick={closeMobileMenu}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <User size={14} />
                  PROFILE
                </Link>
                <Link
                  href="/settings/agents"
                  onClick={closeMobileMenu}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Bot size={14} />
                  MY AGENTS
                </Link>
                <Link
                  href="/settings"
                  onClick={closeMobileMenu}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Settings size={14} />
                  SETTINGS
                </Link>
                <Link
                  href="/settings/api-keys"
                  onClick={closeMobileMenu}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Key size={14} />
                  API KEYS
                </Link>
                <button
                  type="button"
                  onClick={() => { logout(); closeMobileMenu(); }}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <LogOut size={14} />
                  LOG OUT
                </button>
              </>
            ) : (
              <Link
                href="/login"
                onClick={closeMobileMenu}
                className="font-mono text-sm tracking-wider text-muted-foreground"
              >
                LOG IN
              </Link>
            )}
          </nav>
        </div>
      )}
    </header>
  );
}
