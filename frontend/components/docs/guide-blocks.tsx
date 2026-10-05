import Link from "next/link";
import { CAPTION } from "@/components/page/caption";
import { RUNS } from "@/lib/docs/guide-runs";
import type { Block, RunId, Span, Text } from "@/lib/docs/guide-types";
import { trackCta } from "@/lib/track-attrs";

// The guides' running text and blocks (lib/docs/guide-types.ts), rendered on the server.
// Every link is a call to action of the page, marked for the click listener (SPEC.md 27.7).

const INLINE_LINK =
  "text-foreground underline decoration-border underline-offset-4 transition-colors hover:decoration-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground";

function SpanView({ span }: { span: Span }) {
  if (typeof span === "string") return <>{span}</>;
  if ("strong" in span) return <strong className="font-medium text-foreground">{span.strong}</strong>;
  if ("code" in span) return <code className="font-mono text-[0.875em] text-foreground">{span.code}</code>;
  const marks = trackCta(span.item, "page");
  // The contact address is a mail link, not a page of the site.
  if (span.href.startsWith("mailto:")) {
    return (
      <a href={span.href} {...marks} className={INLINE_LINK}>
        {span.text}
      </a>
    );
  }
  return (
    <Link href={span.href} {...marks} className={INLINE_LINK}>
      {span.text}
    </Link>
  );
}

export function GuideText({ text }: { text: Text }) {
  if (typeof text === "string") return <>{text}</>;
  return (
    <>
      {text.map((span, i) => (
        <SpanView key={i} span={span} />
      ))}
    </>
  );
}

const PARAGRAPH = "text-[0.9375rem] leading-relaxed sm:text-base";

function RoomExcerpt({ run, caption }: { run: RunId; caption: Text }) {
  const lines = RUNS[run].excerpt ?? [];
  return (
    <figure>
      <ol data-testid="room-excerpt" className="border-t border-border">
        {lines.map((line, i) => (
          <li key={i} className="grid gap-2 border-b border-border py-4 sm:grid-cols-[9rem_minmax(0,1fr)] sm:gap-6">
            <p className={`${CAPTION} text-foreground`}>
              {line.role}
              <span className="block text-muted-foreground">
                {line.agent}
                {line.pinned ? ", pinned" : ""}
              </span>
            </p>
            <blockquote className="text-[0.9375rem] leading-relaxed [overflow-wrap:anywhere]">{line.text}</blockquote>
          </li>
        ))}
      </ol>
      <figcaption className="mt-4 text-sm leading-relaxed text-muted-foreground">
        <GuideText text={caption} />
      </figcaption>
    </figure>
  );
}

export function GuideBlock({ block }: { block: Block }) {
  switch (block.kind) {
    case "paragraph":
      return (
        <p className={PARAGRAPH}>
          <GuideText text={block.text} />
        </p>
      );
    case "steps":
      return (
        <ol className="space-y-4">
          {block.items.map((item, i) => (
            <li key={i} className={`grid grid-cols-[2.25rem_minmax(0,1fr)] gap-x-3 ${PARAGRAPH}`}>
              <span aria-hidden="true" className="pt-[0.3rem] font-mono text-[11px] tracking-[0.18em] text-muted-foreground tabular-nums">
                {String(i + 1).padStart(2, "0")}
              </span>
              <span>
                <GuideText text={item} />
              </span>
            </li>
          ))}
        </ol>
      );
    case "list":
      return (
        <ul className="space-y-4">
          {block.items.map((item, i) => (
            <li key={i} className={`border-l border-border pl-4 ${PARAGRAPH}`}>
              <GuideText text={item} />
            </li>
          ))}
        </ul>
      );
    case "command":
      return (
        <pre className="border border-border bg-card px-4 py-3 font-mono text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
          <code>{block.code}</code>
        </pre>
      );
    case "excerpt":
      return <RoomExcerpt run={block.run} caption={block.caption} />;
  }
}
