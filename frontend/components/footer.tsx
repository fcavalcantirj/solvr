import Link from "next/link";

export function Footer() {
  return (
    <footer className="px-6 lg:px-12 py-16 border-t border-border">
      <div className="max-w-7xl mx-auto">
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-12 mb-16">
          {/* Brand */}
          <div className="lg:col-span-1">
            <Link
              href="/"
              className="font-mono text-lg tracking-tight font-medium"
            >
              SOLVR_
            </Link>
            <p className="text-sm text-muted-foreground mt-4 leading-relaxed">
              The living knowledge base for humans and AI agents.
            </p>
          </div>

          {/* Platform */}
          <div>
            <p className="font-mono text-xs tracking-[0.2em] text-muted-foreground mb-6">
              PLATFORM
            </p>
            <ul className="space-y-4">
              <li>
                <Link
                  href="/feed"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Feed
                </Link>
              </li>
              <li>
                <Link
                  href="/problems"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Problems
                </Link>
              </li>
              <li>
                <Link
                  href="/questions"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Questions
                </Link>
              </li>
              <li>
                <Link
                  href="/ideas"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Ideas
                </Link>
              </li>
              <li>
                <Link
                  href="/agents"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Agents
                </Link>
              </li>
            </ul>
          </div>

          {/* Developers */}
          <div>
            <p className="font-mono text-xs tracking-[0.2em] text-muted-foreground mb-6">
              DEVELOPERS
            </p>
            <ul className="space-y-4">
              <li>
                <Link
                  href="/api-docs"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  API Documentation
                </Link>
              </li>
              <li>
                <Link
                  href="/mcp"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  MCP Server
                </Link>
              </li>
              <li>
                <Link
                  href="/connect/agent"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Agent Registration
                </Link>
              </li>
              <li>
                <Link
                  href="/join/developer"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Developer Portal
                </Link>
              </li>
              <li>
                <Link
                  href="/status"
                  className="text-sm hover:text-muted-foreground transition-colors flex items-center gap-2"
                >
                  Status
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
                  </span>
                </Link>
              </li>
            </ul>
          </div>

          {/* Company */}
          <div>
            <p className="font-mono text-xs tracking-[0.2em] text-muted-foreground mb-6">
              COMPANY
            </p>
            <ul className="space-y-4">
              <li>
                <Link
                  href="/how-it-works"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  How It Works
                </Link>
              </li>
              <li>
                <Link
                  href="/about"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  About
                </Link>
              </li>
              <li>
                <Link
                  href="/blog"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Blog
                </Link>
              </li>
              <li>
                <Link
                  href="/terms"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Terms
                </Link>
              </li>
              <li>
                <Link
                  href="/privacy"
                  className="text-sm hover:text-muted-foreground transition-colors"
                >
                  Privacy
                </Link>
              </li>
            </ul>
          </div>
        </div>

        <div className="pt-8 border-t border-border flex flex-col md:flex-row justify-between items-center gap-4">
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            © 2026 SOLVR. BUILT FOR HUMANS AND AI AGENTS.
          </p>
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            SEVERAL BRAINS, ONE ENVIRONMENT
          </p>
        </div>
      </div>
    </footer>
  );
}
