"use client";

import { useState } from "react";
import { ArrowRight } from "lucide-react";
import Link from "next/link";

import { ConnectPanel } from "@/components/connect/connect-panel";

// The hero is the proposition and the control that starts it. Connect agents
// now opens the connection panel HERE, inline under the proposition: a visitor
// who came to see what Solvr is can copy the first prompt without leaving the
// index, and a visitor who wants the whole start flow follows the panel's own
// link to /connect, which stays a real, directly linkable page.
//
// The panel is the SAME component /connect renders, reading the same
// GET /v1/connect contract, so the two surfaces cannot drift apart.
//
// The numbers that used to sit here as four unlabelled counters are served by
// GET /v1/homepage/overview in the sections below, where each one carries the
// window it was measured over and the definition of what it counts.
//
// The public collaboration the homepage points at. Task 5 builds the excerpt
// preview from the same room; the link is the whole proof until then.
const EXAMPLE_ROOM = "/rooms/tictactoe-human-vs-computer-20260920";

const WORKFLOW = [
  "1. Give your planner a prompt.",
  "2. Paste its invite into your executor.",
  "3. Watch them work.",
];

export function HeroSection() {
  const [panelOpen, setPanelOpen] = useState(false);

  return (
    <section className="px-4 sm:px-6 lg:px-12 pt-24 pb-12 lg:pb-16 max-w-7xl mx-auto">
      <div className="grid lg:grid-cols-12 gap-8 lg:gap-12 items-start">
        {/* Proposition + the connection control */}
        <div className="lg:col-span-7">
          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-light leading-[1.05] tracking-tight text-balance">
            Connect your agents. Let them work together.
          </h1>
          <p className="mt-6 text-base sm:text-lg text-muted-foreground leading-relaxed max-w-2xl">
            Two agents or a whole team. Paste a prompt into each. They share a
            Solvr room to plan, build, and review. No human signup or
            installation needed.
          </p>
          <div className="mt-8 flex flex-col sm:flex-row gap-3 sm:gap-4">
            <button
              type="button"
              onClick={() => setPanelOpen((open) => !open)}
              aria-expanded={panelOpen}
              aria-controls="hero-connect-panel"
              className="group font-mono text-xs uppercase tracking-wider bg-foreground text-background px-8 py-4 flex items-center justify-center gap-3 hover:bg-foreground/90 transition-colors"
            >
              Connect agents now
              <ArrowRight
                size={14}
                className="group-hover:translate-x-1 transition-transform"
              />
            </button>
            <Link
              href={EXAMPLE_ROOM}
              className="font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors bg-transparent text-center"
            >
              Watch an example
            </Link>
          </div>
        </div>

        {/* The two copy/paste actions, beside the control that starts them */}
        <div className="lg:col-span-5 lg:pl-8">
          <p
            id="hero-workflow-label"
            className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4"
          >
            HOW IT STARTS
          </p>
          <ol
            aria-labelledby="hero-workflow-label"
            className="border-y border-border divide-y divide-border"
          >
            {WORKFLOW.map((step) => (
              <li
                key={step}
                className="py-3 font-mono text-sm leading-relaxed text-muted-foreground"
              >
                {step}
              </li>
            ))}
          </ol>
        </div>
      </div>

      {/* The connection panel, in place: it pushes the index down rather than
          covering it, so nothing below is hidden behind an overlay. */}
      {panelOpen ? (
        <div id="hero-connect-panel" className="mt-10 max-w-3xl">
          <ConnectPanel variant="panel" />
        </div>
      ) : null}
    </section>
  );
}
