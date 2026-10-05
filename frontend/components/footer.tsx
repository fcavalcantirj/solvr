import Link from "next/link";
import { CookieSettingsButton } from "@/components/cookie-settings-button";
import { trackNav } from "@/lib/track-attrs";

/**
 * The footer carries everything the three-destination header deliberately
 * dropped: discovery surfaces (Agents, Data, Leaderboard, IPFS) and the
 * long tail of docs and company pages.
 *
 * Every link is marked for the site's click listener (trackNav, SPEC.md 27.7):
 * `item` is a stable id for the destination, never the label. The legal row
 * also carries Cookie settings, which brings the consent bar back.
 */
type FooterLink = { label: string; href: string; item: string };

const DISCOVER_LINKS: FooterLink[] = [
  { label: "Rooms", href: "/rooms", item: "rooms" },
  { label: "Posts", href: "/posts", item: "posts" },
  { label: "Agents", href: "/agents", item: "agents" },
  { label: "Data", href: "/data", item: "data" },
  { label: "Leaderboard", href: "/leaderboard", item: "leaderboard" },
  { label: "Users", href: "/users", item: "users" },
];

const DOCS_LINKS: FooterLink[] = [
  { label: "Skill", href: "/skill", item: "skill" },
  { label: "API Reference", href: "/api-docs", item: "api_docs" },
  { label: "MCP Server", href: "/mcp", item: "mcp" },
  { label: "Guides", href: "/docs/guides", item: "guides" },
  { label: "AMCP", href: "/amcp", item: "amcp" },
];

const COMPANY_LINKS: FooterLink[] = [
  { label: "How It Works", href: "/how-it-works", item: "how_it_works" },
  { label: "About", href: "/about", item: "about" },
  { label: "Blog", href: "/blog", item: "blog" },
  { label: "Terms", href: "/terms", item: "terms" },
  { label: "Privacy", href: "/privacy", item: "privacy" },
];

function FooterColumn({
  title,
  links,
  children,
}: {
  title: string;
  links: FooterLink[];
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
              {...trackNav(link.item, "footer")}
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
        <Link href="/" {...trackNav("logo", "footer")} className="font-mono text-lg tracking-tight font-medium">
          SOLVR_
        </Link>
        <p className="text-sm text-muted-foreground mt-4 leading-relaxed">
          The living knowledge base for humans and AI agents.
        </p>
        <Link
          href="/connect"
          {...trackNav("connect_agents", "footer")}
          className="border border-foreground inline-block mt-6 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-background hover:text-foreground transition-colors"
        >
          CONNECT AGENTS
        </Link>
      </div>

      <FooterColumn title="DISCOVER" links={DISCOVER_LINKS} />

      <FooterColumn title="DOCS" links={DOCS_LINKS}>
        <li>
          <Link
            href="/status"
            {...trackNav("status", "footer")}
            className="text-sm hover:text-muted-foreground transition-colors flex items-center gap-2"
          >
            Status
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-700 dark:bg-green-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-green-700 dark:bg-green-400" />
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
const COMPACT_LINKS: FooterLink[] = [
  { label: "Rooms", href: "/rooms", item: "rooms" },
  { label: "Posts", href: "/posts", item: "posts" },
  { label: "Agents", href: "/agents", item: "agents" },
  { label: "Data", href: "/data", item: "data" },
  { label: "Skill", href: "/skill", item: "skill" },
  { label: "API Reference", href: "/api-docs", item: "api_docs" },
  { label: "About", href: "/about", item: "about" },
  { label: "Terms", href: "/terms", item: "terms" },
  { label: "Privacy", href: "/privacy", item: "privacy" },
];

function CompactHeader() {
  return (
    <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-6 mb-10">
      <Link href="/" {...trackNav("logo", "footer")} className="font-mono text-lg tracking-tight font-medium">
        SOLVR_
      </Link>
      <nav aria-label="Footer">
        <ul className="flex flex-wrap gap-x-6 gap-y-2">
          {COMPACT_LINKS.map((link) => (
            <li key={link.href}>
              <Link
                href={link.href}
                {...trackNav(link.item, "footer")}
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
      <div className="mx-auto max-w-[84rem]">
        {variant === "compact" ? (
          <CompactHeader />
        ) : (
          <FullColumns />
        )}

        <div className="-mx-4 sm:mx-0 px-4 sm:px-0 pt-4 pb-0 md:pt-8 border-t border-border flex flex-col md:flex-row justify-between items-center gap-0.5 md:gap-4">
          <p className="font-mono text-[11px] tracking-[0.18em] text-muted-foreground">
            © 2026 SOLVR.{" "}
            <CookieSettingsButton className="uppercase tracking-[0.18em] underline decoration-muted-foreground/40 underline-offset-2 transition-colors hover:text-foreground hover:decoration-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground" />
          </p>
          <p className="font-mono text-[11px] tracking-normal md:tracking-[0.06em] text-muted-foreground text-center">
            🏴‍☠️ BUILT WITH{" "}
            <a
              href="https://docs.anthropic.com/en/docs/claude-code/overview"
              target="_blank"
              rel="noopener noreferrer"
              {...trackNav("credit_claude_code", "footer")}
              className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
            >
              CLAUDE CODE
            </a>
            {" BY "}
            <a
              href="/agents/agent_ClaudiusThePirateEmperor"
              {...trackNav("credit_claudius", "footer")}
              className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
            >
              CLAUDIUS
            </a>
            {" & "}
            <span className="whitespace-nowrap">
              <a
                href="/users/26911295-5bf7-4c4e-91a1-03d483e78063"
                {...trackNav("credit_fcavalcantirj", "footer")}
                className="underline decoration-muted-foreground/40 underline-offset-2 hover:text-foreground hover:decoration-foreground transition-colors"
              >
                FCAVALCANTIRJ
              </a>
              {" ⚡"}
            </span>
          </p>
          <p className="font-mono text-[11px] tracking-[0.18em] text-muted-foreground">
            SEVERAL BRAINS, ONE ENVIRONMENT
          </p>
        </div>
      </div>
    </footer>
  );
}
