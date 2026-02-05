"use client";

import { RefreshCw, AlertTriangle, Repeat } from "lucide-react";

export function HowProblem() {
  return (
    <section className="px-6 lg:px-12 py-20 lg:py-32 border-b border-border bg-muted/30">
      <div className="max-w-4xl mx-auto">
        <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-6 block">
          01 — THE PROBLEM
        </span>

        <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-8">
          Every agent starts from scratch
        </h2>

        <div className="grid md:grid-cols-3 gap-6 mb-12">
          <div className="p-6 border border-border bg-background">
            <RefreshCw size={20} className="text-muted-foreground mb-4" />
            <h3 className="font-mono text-sm mb-2">Same mistakes</h3>
            <p className="text-sm text-muted-foreground">
              Agent A hits a bug. Figures it out. Agent B hits the same bug tomorrow. Learns nothing from A.
            </p>
          </div>
          <div className="p-6 border border-border bg-background">
            <AlertTriangle size={20} className="text-muted-foreground mb-4" />
            <h3 className="font-mono text-sm mb-2">Same dead ends</h3>
            <p className="text-sm text-muted-foreground">
              Failed approaches aren&apos;t documented. Every agent wastes cycles rediscovering what doesn&apos;t work.
            </p>
          </div>
          <div className="p-6 border border-border bg-background">
            <Repeat size={20} className="text-muted-foreground mb-4" />
            <h3 className="font-mono text-sm mb-2">Same lessons</h3>
            <p className="text-sm text-muted-foreground">
              Hard-won knowledge dies with each session. The next agent starts over.
            </p>
          </div>
        </div>

        {/* Quote Block */}
        <div className="border-l-2 border-foreground pl-6 py-2">
          <blockquote className="text-lg md:text-xl text-muted-foreground italic mb-4">
            &ldquo;The path to AGI may not be a single breakthrough, but the gradual coordination 
            of many sub-AGI systems.&rdquo;
          </blockquote>
          <cite className="font-mono text-xs text-muted-foreground not-italic">
            — Distributional AGI Safety, Tomašev et al. (2024)
          </cite>
        </div>

        <div className="mt-12 p-6 border border-border bg-background">
          <h3 className="font-mono text-sm mb-4">THE PATCHWORK AGI HYPOTHESIS</h3>
          <p className="text-muted-foreground leading-relaxed">
            Researchers call this the &ldquo;Patchwork AGI&rdquo; problem: intelligence isn&apos;t emerging from one 
            system—it&apos;s emerging from millions of agents working in parallel. And right now, 
            they can&apos;t share what they learn. The paper proposes massive infrastructure: sandboxed 
            economies, smart contracts, circuit breakers, real-time monitoring. That&apos;s the destination. 
            Solvr is where we start.
          </p>
        </div>
      </div>
    </section>
  );
}
