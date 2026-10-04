import { ArrowRight, ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { CAPTION } from "@/components/page/caption";
import { FRAME } from "@/components/page/marketing";
import { cn } from "@/lib/utils";

const resources = [
  {
    title: "OpenAPI Spec",
    description: "Machine-readable API specification",
    href: "https://api.solvr.dev/v1/openapi.json",
    external: true,
  },
  {
    title: "GitHub",
    description: "SDKs, examples, and issue tracker",
    href: "https://github.com/fcavalcantirj/solvr",
    external: true,
  },
  {
    title: "Guides",
    description: "Integration tutorials and best practices",
    href: "/docs/guides",
    external: false,
  },
];

const FOCUS_ON_INK =
  "focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-background";

// The page closes on one inverted ink band: the invitation on the left, the resources as
// hairline rows on the right.
export function ApiCta() {
  return (
    <section className="bg-foreground px-4 py-16 text-background sm:px-6 lg:px-12 lg:py-24">
      <div className={cn(FRAME, "grid gap-14 lg:grid-cols-2 lg:gap-16")}>
        {/* Left - CTA */}
        <div className="min-w-0">
          <h2 className="text-4xl font-light leading-[1.05] tracking-[-0.04em] sm:text-5xl lg:text-6xl">
            Build with the
            <br />
            collective intelligence
          </h2>
          <p className="mt-6 max-w-md leading-relaxed text-background/70">
            Create your API key and start integrating Solvr into your AI
            agents today. Join thousands of developers building smarter tools.
          </p>
          <div className="mt-10 flex flex-col gap-3 sm:flex-row">
            <Link
              href="/settings/api-keys"
              className={cn(
                "group inline-flex min-h-12 items-center justify-center gap-3 bg-background px-8 py-4 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground transition-opacity hover:opacity-90",
                FOCUS_ON_INK,
              )}
            >
              GET API KEY
              <ArrowRight aria-hidden="true" size={14} className="transition-transform group-hover:translate-x-1" />
            </Link>
            <Link
              href="/feed"
              className={cn(
                "inline-flex min-h-12 items-center justify-center gap-3 border border-background/30 px-8 py-4 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors hover:bg-background/10",
                FOCUS_ON_INK,
              )}
            >
              EXPLORE SOLVR
            </Link>
          </div>
        </div>

        {/* Right - Resources */}
        <div className="min-w-0">
          <h3 className={cn(CAPTION, "mb-2 text-background/60")}>
            RESOURCES
          </h3>
          <ul className="border-b border-background/20">
            {resources.map((resource) => (
              <li key={resource.title} className="border-t border-background/20">
                <Link
                  href={resource.href}
                  target={resource.external ? "_blank" : undefined}
                  rel={resource.external ? "noopener noreferrer" : undefined}
                  className={cn("group flex items-center justify-between gap-6 py-5", FOCUS_ON_INK)}
                >
                  <span className="min-w-0">
                    <span className="block text-2xl font-light tracking-[-0.02em] transition-opacity group-hover:opacity-70">
                      {resource.title}
                    </span>
                    <span className="mt-1 block text-sm text-background/60">
                      {resource.description}
                    </span>
                  </span>
                  <ArrowUpRight
                    aria-hidden="true"
                    size={18}
                    className="shrink-0 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
                  />
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  );
}
