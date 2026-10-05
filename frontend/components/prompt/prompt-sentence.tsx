"use client";

import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";
import { Globe, Lock, PencilLine } from "lucide-react";
import { cn } from "@/lib/utils";
import type { PromptSegment } from "./prompt-types";

interface PromptSentenceProps {
  segments: PromptSegment[];
  // /connect: the intent is typed in place and the visibility word flips.
  editable?: boolean;
  onIntentChange?: (intent: string) => void;
  onVisibilityToggle?: () => void;
  intentLabel?: string;
  intentMaxChars?: number;
  // Per segment index, every filling that slot takes across the use cases. The
  // slot reserves the widest one, so swapping words never moves the rest.
  reserve?: Record<number, PromptSegment[]>;
  // Segment indexes whose words every use case shares; they step back so the
  // words that differ read at a glance.
  quiet?: Set<number>;
}

// PromptSentence renders the API's segments by kind. It never writes a word of
// its own: every character on screen is a segment's text.
export function PromptSentence({
  segments,
  editable = false,
  onIntentChange,
  onVisibilityToggle,
  intentLabel = "What should they do?",
  intentMaxChars,
  reserve,
  quiet,
}: PromptSentenceProps) {
  return (
    <>
      {segments.map((segment, index) => {
        const token = (
          <Token
            segment={segment}
            editable={editable}
            onIntentChange={onIntentChange}
            onVisibilityToggle={onVisibilityToggle}
            intentLabel={intentLabel}
            intentMaxChars={intentMaxChars}
            quiet={quiet?.has(index) ?? false}
          />
        );
        const alternatives = reserve?.[index];
        if (!alternatives || alternatives.length < 2) {
          return <span key={index}>{token}</span>;
        }
        return (
          <span key={index} className="inline-grid items-baseline">
            <span className="[grid-area:1/1]">
              <Token
                segment={segment}
                editable={editable}
                onIntentChange={onIntentChange}
                onVisibilityToggle={onVisibilityToggle}
                intentLabel={intentLabel}
                intentMaxChars={intentMaxChars}
                quiet={false}
                fill
              />
            </span>
            {alternatives
              .filter((alt) => alt.text !== segment.text)
              .map((alt, altIndex) => (
                <span key={altIndex} aria-hidden="true" className="invisible [grid-area:1/1]">
                  <Token segment={alt} quiet={false} />
                </span>
              ))}
          </span>
        );
      })}
    </>
  );
}

interface TokenProps {
  segment: PromptSegment;
  editable?: boolean;
  onIntentChange?: (intent: string) => void;
  onVisibilityToggle?: () => void;
  intentLabel?: string;
  intentMaxChars?: number;
  quiet: boolean;
  // In a reserved slot the intent's stroke runs the slot's full width, so the
  // slot reads as one blank filled in, not as words followed by a gap.
  fill?: boolean;
}

function Token({
  segment,
  editable = false,
  onIntentChange,
  onVisibilityToggle,
  intentLabel,
  intentMaxChars,
  quiet,
  fill = false,
}: TokenProps) {
  switch (segment.kind) {
    case "role":
      return (
        <span
          data-kind="role"
          className={cn(
            "prompt-pill whitespace-nowrap bg-prompt-accent px-[0.42em] py-[0.16em] font-mono text-[max(0.6em,11px)] font-medium uppercase tracking-[0.08em] text-prompt-accent-foreground",
            fill ? "relative -top-[0.24em] block text-center" : "align-[0.14em]",
          )}
        >
          {segment.text}
        </span>
      );
    case "handoff":
      return (
        <span
          data-kind="handoff"
          className={cn(quiet ? "text-muted-foreground" : "prompt-band text-foreground")}
        >
          {segment.text}
        </span>
      );
    case "intent":
      return editable ? (
        <>
          <EditableIntent
            segment={segment}
            onChange={onIntentChange}
            label={intentLabel}
            maxChars={intentMaxChars}
          />
          {segment.empty ? (
            <PencilLine
              aria-hidden="true"
              className="ml-[0.12em] inline h-[0.5em] w-[0.5em] align-[0.05em] text-muted-foreground"
            />
          ) : null}
        </>
      ) : (
        fill ? (
          // A reserved slot centres its words, so the slack splits to both sides
          // and reads as spacing, not as a blank.
          <span className="block text-center">
            <span data-kind="intent" className="prompt-swipe font-normal text-foreground">
              {segment.text}
            </span>
          </span>
        ) : (
          <span data-kind="intent" className="prompt-swipe font-normal text-foreground">
            {segment.text}
          </span>
        )
      );
    case "visibility":
      return (
        <Visibility
          segment={segment}
          onToggle={editable ? onVisibilityToggle : undefined}
          fill={fill}
        />
      );
    case "link":
      return <PromptLink text={segment.text} />;
    default:
      return (
        <span data-kind="text" className={cn(quiet && "text-muted-foreground")}>
          {segment.text}
        </span>
      );
  }
}

// The skill link is for the agent; it stays quiet and in the label face. The part
// after "?" only rides along for the agent, so it steps back further. A crawler that
// renders the page is told not to follow it (nofollow): a coded link is one visit's
// address, not a page of the site.
function PromptLink({ text }: { text: string }) {
  const cut = text.indexOf("?");
  const base = cut === -1 ? text : text.slice(0, cut);
  const rest = cut === -1 ? "" : text.slice(cut);
  return (
    <a
      data-kind="link"
      href={text}
      target="_blank"
      rel="nofollow noreferrer"
      className="whitespace-nowrap font-mono text-[max(0.45em,0.8125rem)] tracking-normal text-foreground underline decoration-foreground/30 decoration-1 underline-offset-[0.3em] transition-colors hover:decoration-foreground"
    >
      {base}
      {rest ? <span className="text-[0.78em] text-muted-foreground">{rest}</span> : null}
    </a>
  );
}

function VisibilityIcon({ value }: { value: PromptSegment["value"] }) {
  const Icon = value === "private" ? Lock : Globe;
  return (
    <Icon
      aria-hidden="true"
      className="mr-[0.14em] inline h-[0.62em] w-[0.62em] align-[0.02em] text-foreground"
      strokeWidth={1.75}
    />
  );
}

function Visibility({
  segment,
  onToggle,
  fill = false,
}: {
  segment: PromptSegment;
  onToggle?: () => void;
  fill?: boolean;
}) {
  const content: ReactNode = (
    <>
      <VisibilityIcon value={segment.value} />
      {segment.text}
    </>
  );
  if (!onToggle) {
    return (
      <span
        data-kind="visibility"
        className={cn("whitespace-nowrap text-foreground", fill && "block text-center")}
      >
        {content}
      </span>
    );
  }
  const isPrivate = segment.value === "private";
  return (
    <button
      type="button"
      role="switch"
      aria-checked={isPrivate}
      aria-label="Private room"
      title={`Make it ${isPrivate ? "public" : "private"}`}
      data-kind="visibility"
      onClick={onToggle}
      className="relative cursor-pointer whitespace-nowrap text-foreground underline after:absolute after:-inset-x-1 after:-inset-y-2.5 after:content-[''] decoration-foreground/70 decoration-dashed decoration-[0.05em] underline-offset-[0.22em] transition-colors hover:decoration-foreground focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-foreground"
    >
      {content}
    </button>
  );
}

// EditableIntent is the intent typed in place. It shows the API's text, and while
// the visitor is typing it is never overwritten by a response that arrives late.
function EditableIntent({
  segment,
  onChange,
  label,
  maxChars,
}: {
  segment: PromptSegment;
  onChange?: (intent: string) => void;
  label?: string;
  maxChars?: number;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const focused = useRef(false);
  // The first render carries the text (server render included); after that the
  // DOM is synced by hand so React never rewrites the text under the caret.
  const initial = useRef(segment.text);

  useEffect(() => {
    const el = ref.current;
    if (el && !focused.current && el.textContent !== segment.text) {
      el.textContent = segment.text;
    }
  }, [segment.text]);

  const selectAll = () => {
    const el = ref.current;
    const selection = typeof window !== "undefined" ? window.getSelection() : null;
    if (el && selection) {
      const range = document.createRange();
      range.selectNodeContents(el);
      selection.removeAllRanges();
      selection.addRange(range);
    }
  };

  // What the slot held when editing began, for Escape.
  const before = useRef(segment.text);

  const onKeyDown = (event: KeyboardEvent<HTMLSpanElement>) => {
    if (event.key === "Escape" && ref.current) {
      event.preventDefault();
      ref.current.textContent = before.current;
      onChange?.(segment.empty ? "" : before.current);
      ref.current.blur();
    } else if (event.key === "Enter") {
      event.preventDefault();
      ref.current?.blur();
    }
  };

  return (
    <span
      ref={ref}
      data-kind="intent"
      role="textbox"
      aria-label={label}
      aria-multiline="false"
      contentEditable="plaintext-only"
      suppressContentEditableWarning
      spellCheck={false}
      onFocus={() => {
        focused.current = true;
        before.current = segment.text;
        if (segment.empty) selectAll();
      }}
      onBlur={() => {
        focused.current = false;
      }}
      onKeyDown={onKeyDown}
      onBeforeInput={(event) => {
        const el = ref.current;
        const data = (event as unknown as InputEvent).data ?? "";
        if (maxChars && el && (el.textContent ?? "").length + data.length > maxChars) {
          event.preventDefault();
        }
      }}
      onInput={(event) => {
        const el = event.currentTarget;
        // A paste can overshoot the limit the keyboard respects; trim it back.
        if (maxChars && (el.textContent ?? "").length > maxChars) {
          el.textContent = (el.textContent ?? "").slice(0, maxChars);
        }
        onChange?.(el.textContent ?? "");
      }}
      className={cn(
        "cursor-text font-normal text-foreground caret-foreground outline-none focus-visible:outline-2 focus-visible:outline-offset-[0.18em] focus-visible:outline-foreground",
        segment.empty ? "prompt-swipe-open" : "prompt-swipe",
      )}
    >
      {initial.current}
    </span>
  );
}
