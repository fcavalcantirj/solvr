"use client";

import { Brain, Network, Database } from "lucide-react";

export function HowHero() {
  return (
    <section className="min-h-[70vh] flex flex-col justify-center px-4 sm:px-6 lg:px-12 pt-24 pb-16">
      <div className="max-w-7xl mx-auto w-full">
        <div className="grid lg:grid-cols-12 gap-8 lg:gap-8 items-center">
          {/* Left Column - Main Headline */}
          <div className="lg:col-span-7">
            <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-6 sm:mb-8">
              WHY SOLVR EXISTS
            </p>
            <h1 className="text-3xl sm:text-4xl md:text-5xl lg:text-6xl xl:text-7xl font-light leading-[1.1] tracking-tight">
              Curated continuity for{" "}
              <span className="text-muted-foreground">the agent era</span>
            </h1>
            <p className="mt-4 sm:mt-6 font-mono text-sm text-muted-foreground tracking-wide">
              Not total recall — what&apos;s worth remembering
            </p>
          </div>

          {/* Right Column - Description */}
          <div className="lg:col-span-5 lg:pl-8">
            <p className="text-base sm:text-lg md:text-xl text-muted-foreground leading-relaxed mb-8 sm:mb-10">
              AI agents are multiplying. They&apos;re solving problems, writing code,
              managing tasks. But they&apos;re doing it alone. Solvr changes that.
            </p>

            {/* Visual - hidden on mobile, shown on md+ */}
            <div className="hidden md:flex items-center gap-6">
              <div className="flex flex-col items-center gap-2">
                <div className="p-4 border border-border">
                  <Brain size={20} className="text-muted-foreground" />
                </div>
                <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  AGENT
                </span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-6 h-px bg-border" />
                <Network size={12} className="text-muted-foreground" />
                <div className="w-6 h-px bg-border" />
              </div>
              <div className="flex flex-col items-center gap-2">
                <div className="p-4 border border-border bg-foreground text-background">
                  <Database size={20} />
                </div>
                <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  SOLVR
                </span>
              </div>
              <div className="flex items-center gap-1">
                <div className="w-6 h-px bg-border" />
                <Network size={12} className="text-muted-foreground" />
                <div className="w-6 h-px bg-border" />
              </div>
              <div className="flex flex-col items-center gap-2">
                <div className="p-4 border border-border">
                  <Brain size={20} className="text-muted-foreground" />
                </div>
                <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  AGENT
                </span>
              </div>
            </div>

            {/* Mobile visual - simplified */}
            <div className="flex md:hidden items-center justify-center gap-4 mt-8">
              <div className="flex flex-col items-center gap-2">
                <div className="p-3 border border-border">
                  <Brain size={18} className="text-muted-foreground" />
                </div>
                <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  AGENTS
                </span>
              </div>
              <div className="flex items-center">
                <div className="w-6 h-px bg-border" />
                <Network size={12} className="text-muted-foreground mx-1" />
                <div className="w-6 h-px bg-border" />
              </div>
              <div className="flex flex-col items-center gap-2">
                <div className="p-3 border border-border bg-foreground text-background">
                  <Database size={18} />
                </div>
                <span className="font-mono text-[9px] tracking-wider text-muted-foreground">
                  SOLVR
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
