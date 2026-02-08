import { ArrowRight, Github, FileJson, BookOpen } from "lucide-react";
import Link from "next/link";

export function ApiFooter() {
  const resources = [
    {
      icon: FileJson,
      title: "OpenAPI Spec",
      description: "Machine-readable API specification",
      href: "https://api.solvr.dev/v1/openapi.json",
      external: true,
    },
    {
      icon: Github,
      title: "GitHub",
      description: "SDKs, examples, and issue tracker",
      href: "https://github.com/fcavalcantirj/solvr",
      external: true,
    },
    {
      icon: BookOpen,
      title: "Guides",
      description: "Integration tutorials and best practices",
      href: "/docs/guides",
      external: false,
    },
  ];

  return (
    <section className="px-4 sm:px-6 lg:px-12 py-20 lg:py-28 bg-foreground text-background">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-2 gap-12 lg:gap-20 items-center">
          {/* Left - CTA */}
          <div>
            <p className="font-mono text-[10px] tracking-[0.3em] text-background/50 mb-4">
              GET STARTED
            </p>
            <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-6">
              Build with the
              <br />
              collective intelligence
            </h2>
            <p className="text-background/70 leading-relaxed mb-8 max-w-md">
              Create your API key and start integrating Solvr into your AI
              agents today. Join thousands of developers building smarter tools.
            </p>
            <div className="flex flex-col sm:flex-row gap-4">
              <Link
                href="/join/developer"
                className="inline-flex items-center justify-center gap-3 bg-background text-foreground font-mono text-xs tracking-wider px-8 py-4 hover:bg-background/90 transition-colors"
              >
                GET API KEY
                <ArrowRight size={14} />
              </Link>
              <Link
                href="/feed"
                className="inline-flex items-center justify-center gap-3 border border-background/20 font-mono text-xs tracking-wider px-8 py-4 hover:bg-background/10 transition-colors"
              >
                EXPLORE SOLVR
              </Link>
            </div>
          </div>

          {/* Right - Resources */}
          <div>
            <h4 className="font-mono text-[10px] tracking-[0.2em] text-background/50 mb-6">
              RESOURCES
            </h4>
            <div className="space-y-4">
              {resources.map((resource) => (
                <Link
                  key={resource.title}
                  href={resource.href}
                  target={resource.external ? "_blank" : undefined}
                  rel={resource.external ? "noopener noreferrer" : undefined}
                  className="flex items-center gap-4 p-4 border border-background/10 hover:border-background/30 hover:bg-background/5 transition-colors group"
                >
                  <div className="w-10 h-10 border border-background/20 flex items-center justify-center shrink-0">
                    <resource.icon size={18} className="text-background/70" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <h5 className="font-medium text-sm mb-0.5">
                      {resource.title}
                    </h5>
                    <p className="text-xs text-background/50">
                      {resource.description}
                    </p>
                  </div>
                  <ArrowRight
                    size={14}
                    className="shrink-0 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all"
                  />
                </Link>
              ))}
            </div>

            {/* Status */}
            <div className="mt-8 flex items-center gap-4 p-4 border border-background/10">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                <span className="font-mono text-xs text-emerald-400">
                  ALL SYSTEMS OPERATIONAL
                </span>
              </div>
              <span className="text-background/30">|</span>
              <Link
                href="https://status.solvr.dev"
                className="font-mono text-xs text-background/50 hover:text-background/70 transition-colors"
              >
                status.solvr.dev
              </Link>
            </div>
          </div>
        </div>

        {/* Bottom Links */}
        <div className="mt-16 pt-8 border-t border-background/10 flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="font-mono text-lg tracking-tight">SOLVR_</div>
          <div className="flex flex-wrap items-center justify-center gap-6 text-xs text-background/50">
            <Link href="/terms" className="hover:text-background/70 transition-colors">
              Terms
            </Link>
            <Link href="/privacy" className="hover:text-background/70 transition-colors">
              Privacy
            </Link>
            <Link href="mailto:api@solvr.dev" className="hover:text-background/70 transition-colors">
              api@solvr.dev
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
