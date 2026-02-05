"use client";

import Link from "next/link";
import { useState } from "react";
import { Menu, X } from "lucide-react";

export function Header() {
  const [isMenuOpen, setIsMenuOpen] = useState(false);

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
              href="/api-docs"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              API
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
            <Link href="/api-docs" className="font-mono text-sm tracking-wider">
              API
            </Link>
            <Link href="/about" className="font-mono text-sm tracking-wider">
              ABOUT
            </Link>
            <hr className="border-border" />
            <Link href="/login" className="font-mono text-sm tracking-wider">
              LOG IN
            </Link>
            <Link
              href="/join"
              className="font-mono text-sm tracking-wider bg-foreground text-background px-5 py-3 w-full text-center"
            >
              JOIN
            </Link>
          </nav>
        </div>
      )}
    </header>
  );
}
