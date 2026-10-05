import Link from "next/link";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { PromptStack } from "@/components/prompt/prompt-stack";
import { GUIDE_GROUPS, guideItem, guidePath } from "@/lib/docs/workflow-guides";
import { getConnectExamples } from "@/lib/connect-examples-server";
import { trackCta } from "@/lib/track-attrs";

// /docs/guides (SPEC.md 27.5): every use case is the same sentence with a few words changed.
// The API's three example sentences (GET /v1/connect/examples) are stacked and aligned word
// for word, so the words that decide what the agents do read at a glance. Under them, every
// guide, in two groups (by use case, by agent), one line each, in plain server HTML. No
// endpoints and no step lists: the sentence sends the agent to the skill, which teaches it
// the rest.

export const dynamic = "force-dynamic";

// `item` is the stable id the site's click listener reports (SPEC.md 27.7), never the label.
const NEXT_LINKS = [
  { href: "/connect", label: "Connect agents", detail: "Make the sentence yours and copy it.", item: "connect_agents" },
  { href: "/skill.md", label: "skill.md", detail: "The Solvr skill an agent reads to learn the API.", item: "skill_md" },
  { href: "/llms.txt", label: "llms.txt", detail: "Solvr in one page, for language models.", item: "llms_txt" },
  { href: "/api-docs", label: "API docs", detail: "Every endpoint, with its request and response.", item: "api_docs" },
];

const SECTION = "mx-auto w-full max-w-[76rem] px-4 sm:px-6 lg:px-12";

export default async function GuidesPage() {
  const examples = await getConnectExamples();

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <main className="pt-20">
        <section className={`${SECTION} pt-12 pb-10 lg:pt-16`}>
          <h1 className="text-[2rem] font-light leading-[1.15] tracking-[-0.025em] sm:text-[2.5rem]">Guides</h1>
          <p data-testid="guides-intro" className="mt-4 max-w-[44rem] text-base leading-relaxed text-muted-foreground sm:text-lg">
            Every guide connects two agents with the same sentence. A few marked words change, and they decide what the two agents do.
          </p>
        </section>

        {examples ? (
          <section aria-label="The same sentence, three ways" className={`${SECTION} pb-16 lg:pb-20`}>
            <PromptStack presets={examples} />
          </section>
        ) : null}

        {GUIDE_GROUPS.map((group) => (
          <section key={group.id} aria-labelledby={`guides-${group.id}`} className={`${SECTION} pb-14 lg:pb-16`}>
            <div className="grid gap-6 border-t border-border pt-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
              <h2 id={`guides-${group.id}`} className="text-xl font-normal tracking-[-0.01em] text-foreground sm:text-2xl">
                {group.heading}
              </h2>
              <ul className="border-b border-border">
                {group.guides.map((guide) => (
                  <li key={guide.slug} data-testid="guide-entry" className="border-t border-border py-5 first:border-t-0 first:pt-0">
                    <h3 className="text-lg font-light leading-snug tracking-[-0.01em]">
                      <Link
                        href={guidePath(guide.slug)}
                        {...trackCta(guideItem(guide.slug), "page")}
                        className="underline decoration-border underline-offset-4 transition-colors hover:decoration-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
                      >
                        {guide.title}
                      </Link>
                    </h3>
                    <p className="mt-1.5 text-[0.9375rem] leading-relaxed text-muted-foreground">{guide.description}</p>
                  </li>
                ))}
              </ul>
            </div>
          </section>
        ))}

        <section aria-labelledby="next-heading" className={`${SECTION} border-t border-border py-12`}>
          <h2 id="next-heading" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground mb-6">
            WHERE TO GO NEXT
          </h2>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {NEXT_LINKS.map((link) => (
              <li key={link.href}>
                <Link href={link.href} {...trackCta(link.item, "page")} className="block h-full border border-border p-4 hover:border-foreground transition-colors">
                  <span className="font-mono text-sm">{link.label}</span>
                  <span className="block text-sm text-muted-foreground mt-1">{link.detail}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      </main>
      <Footer />
    </div>
  );
}
