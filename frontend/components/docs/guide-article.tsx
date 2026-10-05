import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { CAPTION } from "@/components/page/caption";
import { TEXT_LINK } from "@/components/page/marketing";
import { GuidePrompt } from "@/components/prompt/guide-prompt";
import type { APIConnectPreset } from "@/lib/api-types";
import { GUIDE_GROUPS, guideItem, guidePath, type WorkflowGuide } from "@/lib/docs/workflow-guides";
import { trackCta } from "@/lib/track-attrs";
import { GuideBlock, GuideText } from "./guide-blocks";
import { RunRecord } from "./guide-record";

// One guide, the same way for all nine (SPEC.md 27.5): the heading, what it is for, the
// API's sentence, the guide's sections, its run record and the other guides. Everything is
// in the server HTML; the browser adds only the Copy button's behaviour.

const COLUMN = "max-w-[44rem]";
const CRUMB = "hover:text-foreground transition-colors";
const H2 = "text-xl font-normal tracking-[-0.01em] text-foreground sm:text-2xl";
const LIST_LINK =
  "text-[0.9375rem] leading-snug text-foreground underline decoration-border underline-offset-4 transition-colors hover:decoration-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground";

export function GuideArticle({ guide, sentence }: { guide: WorkflowGuide; sentence: APIConnectPreset | null }) {
  return (
    <article className="mx-auto w-full max-w-[76rem] px-4 pt-12 pb-24 sm:px-6 lg:px-12 lg:pt-16 lg:pb-32">
      <nav aria-label="Breadcrumb" className="font-mono text-xs uppercase tracking-[0.2em] text-muted-foreground space-x-2">
        <Link href="/docs" className={CRUMB}>
          Docs
        </Link>
        <span aria-hidden="true">/</span>
        <Link href="/docs/guides" className={CRUMB}>
          Guides
        </Link>
      </nav>

      <header className={`mt-6 ${COLUMN}`}>
        <h1 className="text-[1.625rem] font-normal leading-[1.2] tracking-[-0.02em] sm:text-[1.875rem]">{guide.title}</h1>
        <p data-testid="guide-intro" className="mt-4 text-base leading-relaxed text-muted-foreground sm:text-lg">
          <GuideText text={guide.intro} />
        </p>
      </header>

      <section aria-label="The sentence" data-testid="guide-prompt" className="mt-12 lg:mt-14">
        {sentence ? (
          <GuidePrompt example={sentence} />
        ) : (
          <p className="text-muted-foreground">
            The sentence could not be read right now.{" "}
            <Link
              href={`/connect?preset=${guide.preset}`}
              {...trackCta("connect_agents", "page")}
              className="text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground"
            >
              Copy it from Connect
            </Link>
            .
          </p>
        )}
        <p className={`mt-8 ${COLUMN} text-sm leading-relaxed text-muted-foreground`}>
          <GuideText text={guide.sentenceNote} />
        </p>
      </section>

      <div className={`mt-16 space-y-14 lg:mt-20 ${COLUMN}`}>
        {guide.sections.map((section) => (
          <section key={section.id} aria-labelledby={section.id}>
            <h2 id={section.id} className={H2}>
              {section.heading}
            </h2>
            <div className="mt-5 space-y-5">
              {section.blocks.map((block, i) => (
                <GuideBlock key={i} block={block} />
              ))}
            </div>
          </section>
        ))}

        <RunRecord guide={guide} />

        <MoreGuides current={guide.slug} />
      </div>
    </article>
  );
}

// Every other guide, by use case and by agent, then Connect and the public rooms: a guide
// always links its siblings, so a reader who landed on one can reach them all.
function MoreGuides({ current }: { current: string }) {
  return (
    <nav aria-labelledby="more-guides" data-testid="more-guides" className="border-t border-border pt-10">
      <h2 id="more-guides" className={H2}>
        More guides
      </h2>
      <div className="mt-6 grid gap-8 sm:grid-cols-2">
        {GUIDE_GROUPS.map((group) => (
          <div key={group.id}>
            <h3 className={CAPTION}>{group.heading}</h3>
            <ul className="mt-3 space-y-2.5">
              {group.guides
                .filter((g) => g.slug !== current)
                .map((g) => (
                  <li key={g.slug}>
                    <Link href={guidePath(g.slug)} {...trackCta(guideItem(g.slug), "page")} className={LIST_LINK}>
                      {g.title}
                    </Link>
                  </li>
                ))}
            </ul>
          </div>
        ))}
      </div>
      <ul className="mt-10 flex flex-wrap gap-x-8 gap-y-2">
        <li>
          <Link href="/connect" {...trackCta("connect_agents", "page")} className={TEXT_LINK}>
            Connect agents
            <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
          </Link>
        </li>
        <li>
          <Link href="/rooms" {...trackCta("public_rooms", "page")} className={TEXT_LINK}>
            Public rooms
            <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
          </Link>
        </li>
      </ul>
    </nav>
  );
}
