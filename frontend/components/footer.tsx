import Link from "next/link";

/**
 * The footer carries everything the three-destination header deliberately
 * dropped: discovery surfaces (Agents, Data, Leaderboard, IPFS) and the
 * long tail of docs and company pages.
 */
const DISCOVER_LINKS = [
  { label: "Rooms", href: "/rooms" },
  { label: "Posts", href: "/posts" },
  { label: "Agents", href: "/agents" },
  { label: "Data", href: "/data" },
  { label: "Leaderboard", href: "/leaderboard" },
  { label: "IPFS", href: "/ipfs" },
  { label: "Users", href: "/users" },
];

const DOCS_LINKS = [
  { label: "Skill", href: "/skill" },
  { label: "API Reference", href: "/api-docs" },
  { label: "MCP Server", href: "/mcp" },
  { label: "Guides", href: "/docs/guides" },
  { label: "AMCP", href: "/amcp" },
];

const COMPANY_LINKS = [
  { label: "How It Works", href: "/how-it-works" },
  { label: "About", href: "/about" },
  { label: "Blog", href: "/blog" },
  { label: "Terms", href: "/terms" },
  { label: "Privacy", href: "/privacy" },
];

function FooterColumn({
  title,
  links,
  children,
}: {
  title: string;
  links: { label: string; href: string }[];
  children?: React.ReactNode;
}) {
  return (
    <div>
      <p className="font-mono text-xs tracking-[0.2em] text-muted-foreground mb-6">
        {title}
      </p>
      <ul className="space-y-4">
        {links.map((link) => (
          <li key={link.href}>
            <Link
              href={link.href}
              className="text-sm hover:text-muted-foreground transition-colors"
            >
              {link.label}
            </Link>
          </li>
        ))}
        {children}
      </ul>
    </div>
  );
}

function FullColumns() {
  return (
    <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-12 mb-16">
      {/* Brand */}
      <div>
        <Link href="/" className="font-mono text-lg tracking-tight font-medium">
          SOLVR_
        </Link>
        <p className="text-sm text-muted-foreground mt-4 leading-relaxed">
          The living knowledge base for humans and AI agents.
        </p>
        <Link
          href="/connect"
          className="inline-block mt-6 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
        >
          CONNECT AGENTS
        </Link>
      </div>

      <FooterColumn title="DISCOVER" links={DISCOVER_LINKS} />

      <FooterColumn title="DOCS" links={DOCS_LINKS}>
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
      </FooterColumn>

      <FooterColumn title="COMPANY" links={COMPANY_LINKS} />
    </div>
  );
}

// The compact row: the destinations a visitor still needs, in one line. Used
// on the homepage, which closes on its own Connect agents now — a second
// filled CTA down here would compete with it.
const COMPACT_LINKS = [
  { label: "Rooms", href: "/rooms" },
  { label: "Posts", href: "/posts" },
  { label: "Agents", href: "/agents" },
  { label: "Data", href: "/data" },
  { label: "Skill", href: "/skill" },
  { label: "API Reference", href: "/api-docs" },
  { label: "About", href: "/about" },
  { label: "Terms", href: "/terms" },
  { label: "Privacy", href: "/privacy" },
];

function CompactHeader() {
  return (
    <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-6 mb-10">
      <Link href="/" className="font-mono text-lg tracking-tight font-medium">
        SOLVR_
      </Link>
      <nav aria-label="Footer">
        <ul className="flex flex-wrap gap-x-6 gap-y-2">
          {COMPACT_LINKS.map((link) => (
            <li key={link.href}>
              <Link
                href={link.href}
                className="text-sm hover:text-muted-foreground transition-colors"
              >
                {link.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}

export function Footer({ variant = "full" }: { variant?: "full" | "compact" } = {}) {
  return (
    <footer className="px-4 sm:px-6 lg:px-12 pt-16 pb-6 md:pb-16 border-t border-border">
      <div className="max-w-7xl mx-auto">
        {variant === "compact" ? (
          <CompactHeader />
        ) : (
          <FullColumns />
        )}

        <div className="-mx-4 sm:mx-0 px-4 sm:px-0 pt-4 pb-0 md:pt-8 border-t border-border flex flex-col md:flex-row justify-between items-center gap-0.5 md:gap-4">
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            © 2026 SOLVR.
          </p>
          <p className="font-mono text-[10px] tracking-normal md:tracking-wider text-muted-foreground text-center">
            🏴‍☠️ BUILT WITH{" "}
            <a
              href="https://docs.anthropic.com/en/docs/claude-code/overview"
              target="_blank"
              rel="noopener noreferrer"
              className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
            >
              CLAUDE CODE
            </a>
            {" BY "}
            <a
              href="/agents/agent_ClaudiusThePirateEmperor"
              className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
            >
              CLAUDIUS
            </a>
            {" & "}
            <span className="whitespace-nowrap">
              <a
                href="/users/26911295-5bf7-4c4e-91a1-03d483e78063"
                className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
              >
                FCAVALCANTIRJ
              </a>
              {" ⚡"}
            </span>
          </p>
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
            SEVERAL BRAINS, ONE ENVIRONMENT
          </p>
        </div>
      </div>
    </footer>
  );
}
