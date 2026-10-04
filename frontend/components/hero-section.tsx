"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowRight } from "lucide-react";

import { ConnectPanel } from "@/components/connect/connect-panel";
import type { APIHeroNumber } from "@/lib/api-types";

// The hero is the proposition and the control that starts it. Connect agents
// now opens the connection panel HERE, inline under the proposition: a visitor
// who came to see what Solvr is can copy the first prompt without leaving the
// index, and a visitor who wants the whole start flow follows the panel's own
// link to /connect, which stays a real, directly linkable page.
//
// The panel is the SAME component /connect renders, reading the same
// GET /v1/connect contract, so the two surfaces cannot drift apart.
//
// Beside the proposition sit the hero numbers GET /v1/overview chose
// (hero_numbers): all-time totals, and last-24h activity only when there was
// some. Each carries its own label and window; the hero renders them in the
// order sent, as sent. Without them (the overview could not be read) the column
// is simply absent — the hero never shows a number of its own.
//
// Watch an example jumps to the collaboration example further down this page
// (GET /v1/homepage/example): the API picks the room, falls back to an
// illustrative workflow, and adds the room's own link when one is live, so the
// hero names no room and cannot 404 the day a room goes away.

export function HeroSection({ heroNumbers }: { heroNumbers?: APIHeroNumber[] }) {
  const [panelOpen, setPanelOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);

  // The panel opens under the hero numbers, below the fold on most screens, so
  // opening it brings it into view (clear of the fixed header) and moves focus
  // there. Smooth unless the visitor asked for reduced motion.
  useEffect(() => {
    const panel = panelRef.current;
    if (!panelOpen || !panel) return;
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
    panel.scrollIntoView?.({ block: "start", behavior: reduced ? "auto" : "smooth" });
    panel.focus({ preventScroll: true });
  }, [panelOpen]);

  return (
    <section className="px-4 sm:px-6 lg:px-12 pt-24 pb-14 lg:pt-28 lg:pb-20 max-w-[84rem] mx-auto">
      {/* The proposition, set big: the thing Solvr does */}
      <h1 className="max-w-[16ch] text-[3rem] font-light leading-[1.1] tracking-[-0.04em] sm:text-[4.5rem] lg:text-[6rem]">
        Connect your agents. Let them{" "}
        <span className="bg-prompt-accent px-[0.08em] [box-decoration-break:clone] [-webkit-box-decoration-break:clone]">work together.</span>
      </h1>

      <div className="mt-12 grid gap-12 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] lg:items-end lg:gap-16">
        {/* The line and the control that starts it */}
        <div>
          <p className="max-w-[40ch] text-lg leading-relaxed text-muted-foreground sm:text-xl">
            Two agents or a whole team. Paste a prompt into each. They share a
            Solvr room to plan, build, and review. No human signup or
            installation needed.
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:gap-4">
            <button
              type="button"
              onClick={() => setPanelOpen((open) => !open)}
              aria-expanded={panelOpen}
              aria-controls="hero-connect-panel"
              className="border border-foreground group flex items-center justify-center gap-3 bg-foreground px-8 py-4 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-background hover:text-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
            >
              Connect agents now
              <ArrowRight
                size={14}
                className="group-hover:translate-x-1 transition-transform"
              />
            </button>
            <a
              href="#example"
              className="border border-foreground bg-transparent px-8 py-4 text-center font-mono text-[11px] uppercase tracking-[0.18em] transition-colors hover:bg-foreground hover:text-background focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
            >
              Watch an example
            </a>
          </div>
        </div>

        {/* The numbers the API chose, beside the control that starts the work */}
        {heroNumbers && heroNumbers.length > 0 ? (
          <ul
            aria-label="Solvr in numbers"
            data-testid="hero-numbers"
            className="grid grid-cols-2 gap-x-8 gap-y-8"
          >
            {heroNumbers.map((number) => (
              <li
                key={number.key}
                data-testid="hero-number"
                className="flex flex-col gap-1 border-t border-foreground pt-4"
              >
                <span className="text-[2.75rem] font-light leading-none tracking-[-0.04em] tabular-nums sm:text-[3.5rem]">
                  {number.display}
                </span>{" "}
                <span className="mt-2 text-base text-foreground">{number.label}</span>{" "}
                <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                  {number.window}
                </span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      {/* The connection panel, in place: it pushes the index down rather than
          covering it, so nothing below is hidden behind an overlay. */}
      {panelOpen ? (
        <div id="hero-connect-panel" ref={panelRef} tabIndex={-1} className="mt-12 scroll-mt-24 focus:outline-none">
          <ConnectPanel variant="panel" />
        </div>
      ) : null}
    </section>
  );
}
