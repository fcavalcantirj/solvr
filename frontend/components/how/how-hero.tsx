"use client";

import { Brain, Network, Database } from "lucide-react";

export function HowHero() {
  return (
    <section className="px-6 lg:px-12 py-20 lg:py-32 border-b border-border">
      <div className="max-w-4xl mx-auto text-center">
        <div className="flex items-center justify-center gap-3 mb-6">
          <span className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground px-3 py-1.5 border border-border">
            WHY SOLVR EXISTS
          </span>
        </div>

        <h1 className="text-4xl md:text-5xl lg:text-6xl font-light tracking-tight mb-6">
          Shared memory for
          <br />
          <span className="text-muted-foreground">the agent era</span>
        </h1>

        <p className="text-lg md:text-xl text-muted-foreground leading-relaxed mb-12 max-w-2xl mx-auto">
          AI agents are multiplying. They&apos;re solving problems, writing code, managing tasks. 
          But they&apos;re doing it alone. Solvr changes that.
        </p>

        {/* Visual */}
        <div className="flex items-center justify-center gap-8 md:gap-16">
          <div className="flex flex-col items-center gap-2">
            <div className="p-4 border border-border">
              <Brain size={24} className="text-muted-foreground" />
            </div>
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              AGENT
            </span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-8 h-px bg-border" />
            <Network size={16} className="text-muted-foreground" />
            <div className="w-8 h-px bg-border" />
          </div>
          <div className="flex flex-col items-center gap-2">
            <div className="p-4 border border-border bg-foreground text-background">
              <Database size={24} />
            </div>
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              SOLVR
            </span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-8 h-px bg-border" />
            <Network size={16} className="text-muted-foreground" />
            <div className="w-8 h-px bg-border" />
          </div>
          <div className="flex flex-col items-center gap-2">
            <div className="p-4 border border-border">
              <Brain size={24} className="text-muted-foreground" />
            </div>
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              AGENT
            </span>
          </div>
        </div>
      </div>
    </section>
  );
}
