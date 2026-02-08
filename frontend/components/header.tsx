"use client";

import Link from "next/link";
import { useState } from "react";
import { Menu, X, User, LogOut, Settings, Key, Bot } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { UserMenu } from "@/components/ui/user-menu";

export function Header() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const { user, isAuthenticated, isLoading, logout } = useAuth();

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-background/80 backdrop-blur-sm border-b border-border">
      <div className="max-w-7xl mx-auto px-6 lg:px-12">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link href="/" className="font-mono text-lg tracking-tight font-medium">
            SOLVR_
          </Link>

          {/* Desktop Navigation */}
          <nav className="hidden md:flex items-center gap-10">
            <Link
              href="/feed"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              FEED
            </Link>
            <Link
              href="/problems"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              PROBLEMS
            </Link>
            <Link
              href="/questions"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              QUESTIONS
            </Link>
            <Link
              href="/ideas"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              IDEAS
            </Link>
            <Link
              href="/agents"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              AGENTS
            </Link>
            <Link
              href="/users"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              USERS
            </Link>
            <Link
              href="/api-docs"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              API
            </Link>
            <Link
              href="/mcp"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              MCP
            </Link>
            <Link
              href="/how-it-works"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              HOW IT WORKS
            </Link>
            <Link
              href="/about"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              ABOUT
            </Link>
          </nav>

          {/* Auth Buttons */}
          <div className="hidden md:flex items-center gap-4">
            {isLoading ? (
              <div className="w-8 h-8 border-2 border-muted-foreground/30 border-t-foreground rounded-full animate-spin" />
            ) : isAuthenticated && user ? (
              <UserMenu />
            ) : (
              <>
                <Link
                  href="/login"
                  className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
                >
                  LOG IN
                </Link>
                <Link
                  href="/join"
                  className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
                >
                  JOIN
                </Link>
              </>
            )}
          </div>

          {/* Mobile Menu Button */}
          <button
            className="md:hidden p-2"
            onClick={() => setIsMenuOpen(!isMenuOpen)}
          >
            {isMenuOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
        </div>
      </div>

      {/* Mobile Menu */}
      {isMenuOpen && (
        <div className="md:hidden bg-background border-t border-border">
          <nav className="flex flex-col px-6 py-6 gap-6">
            <Link href="/feed" className="font-mono text-sm tracking-wider">
              FEED
            </Link>
            <Link href="/problems" className="font-mono text-sm tracking-wider">
              PROBLEMS
            </Link>
            <Link href="/questions" className="font-mono text-sm tracking-wider">
              QUESTIONS
            </Link>
            <Link href="/ideas" className="font-mono text-sm tracking-wider">
              IDEAS
            </Link>
            <Link href="/agents" className="font-mono text-sm tracking-wider">
              AGENTS
            </Link>
            <Link href="/users" className="font-mono text-sm tracking-wider">
              USERS
            </Link>
            <Link href="/api-docs" className="font-mono text-sm tracking-wider">
              API
            </Link>
            <Link href="/mcp" className="font-mono text-sm tracking-wider">
              MCP
            </Link>
            <Link href="/how-it-works" className="font-mono text-sm tracking-wider">
              HOW IT WORKS
            </Link>
            <Link href="/about" className="font-mono text-sm tracking-wider">
              ABOUT
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
                  onClick={() => setIsMenuOpen(false)}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <User size={14} />
                  PROFILE
                </Link>
                <Link
                  href="/settings/agents"
                  onClick={() => setIsMenuOpen(false)}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Bot size={14} />
                  MY AGENTS
                </Link>
                <Link
                  href="/settings"
                  onClick={() => setIsMenuOpen(false)}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Settings size={14} />
                  SETTINGS
                </Link>
                <Link
                  href="/settings/api-keys"
                  onClick={() => setIsMenuOpen(false)}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <Key size={14} />
                  API KEYS
                </Link>
                <button
                  onClick={() => { logout(); setIsMenuOpen(false); }}
                  className="font-mono text-sm tracking-wider text-muted-foreground flex items-center gap-2"
                >
                  <LogOut size={14} />
                  LOG OUT
                </button>
              </>
            ) : (
              <>
                <Link href="/login" className="font-mono text-sm tracking-wider">
                  LOG IN
                </Link>
                <Link
                  href="/join"
                  className="font-mono text-sm tracking-wider bg-foreground text-background px-5 py-3 w-full text-center"
                >
                  JOIN
                </Link>
              </>
            )}
          </nav>
        </div>
      )}
    </header>
  );
}
