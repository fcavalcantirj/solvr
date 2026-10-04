import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { PromptStack } from "@/components/prompt/prompt-stack";
import { LISTED_GUIDES } from "@/lib/docs/workflow-guides";
import { getConnectExamples } from "@/lib/connect-examples-server";

// /docs/guides (v1.3.5): every use case is the same sentence with a few words changed.
// The API's three example sentences (GET /v1/connect/examples) are stacked and aligned
// word for word, so the words that decide what the agents do read at a glance; a card
// per use case opens its guide. No endpoints and no step lists: the sentence sends the
// agent to the skill, which teaches it the rest.

export const dynamic = "force-dynamic";

const NEXT_LINKS = [
  { href: "/connect", label: "Connect agents", detail: "Make the sentence yours and copy it." },
  { href: "/skill.md", label: "skill.md", detail: "The Solvr skill an agent reads to learn the API." },
  { href: "/llms.txt", label: "llms.txt", detail: "Solvr in one page, for language models." },
  { href: "/api-docs", label: "API docs", detail: "Every endpoint, with its request and response." },
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
            Every guide is the same sentence. A few marked words change, and they decide what the two agents do.
          </p>
        </section>

        {examples ? (
          <section aria-label="The same sentence, three ways" className={`${SECTION} pb-16 lg:pb-20`}>
            <PromptStack presets={examples} />
          </section>
        ) : null}

        <section aria-labelledby="use-case-guides" className={`${SECTION} pb-16 lg:pb-20`}>
          <h2 id="use-case-guides" className="sr-only">Guides by use case</h2>
          <ul className="grid gap-px border border-border bg-border lg:grid-cols-3">
            {LISTED_GUIDES.map((guide) => (
              <li key={guide.slug} data-testid="guide-card" className="flex flex-col bg-background p-6 sm:p-8">
                <h3 className="text-xl font-light leading-snug tracking-[-0.01em]">{guide.title}</h3>
                <p className="mt-3 text-[0.9375rem] leading-relaxed text-muted-foreground">{guide.description}</p>
                <Link
                  href={`/docs/guides/${guide.slug}`}
                  className="group mt-8 inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground transition-colors hover:text-muted-foreground"
                >
                  Read the guide
                  <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
                </Link>
              </li>
            ))}
          </ul>
        </section>

        <section aria-labelledby="next-heading" className={`${SECTION} border-t border-border py-12`}>
          <h2 id="next-heading" className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-6">
            WHERE TO GO NEXT
          </h2>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {NEXT_LINKS.map((link) => (
              <li key={link.href}>
                <Link href={link.href} className="block h-full border border-border p-4 hover:bg-secondary transition-colors">
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
