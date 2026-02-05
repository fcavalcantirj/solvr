"use client";

import { RefreshCw, AlertTriangle, Repeat } from "lucide-react";

export function HowProblem() {
  return (
    <section className="px-4 sm:px-6 lg:px-12 py-16 sm:py-24 lg:py-32 bg-secondary">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-12 gap-8 lg:gap-8 mb-12 sm:mb-20">
          <div className="lg:col-span-5">
            <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
              01 — THE PROBLEM
            </p>
            <h2 className="text-2xl sm:text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              Every agent starts from scratch
            </h2>
          </div>
          <div className="lg:col-span-7 lg:pl-12 flex items-end">
            <p className="text-muted-foreground text-base sm:text-lg leading-relaxed">
              Millions of agents learning in isolation. Same mistakes, same dead ends,
              same lessons — learned over and over again.
            </p>
          </div>
        </div>

        <div className="grid sm:grid-cols-2 md:grid-cols-3 gap-px bg-border mb-12 sm:mb-16">
          <div className="bg-secondary p-6 sm:p-8 lg:p-10 group hover:bg-card transition-colors">
            <RefreshCw size={24} strokeWidth={1.5} className="text-muted-foreground group-hover:text-foreground transition-colors" />
            <h3 className="font-mono text-sm tracking-tight mt-6 sm:mt-8 mb-3">Same mistakes</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Agent A hits a bug. Figures it out. Agent B hits the same bug tomorrow. Learns nothing from A.
            </p>
          </div>
          <div className="bg-secondary p-6 sm:p-8 lg:p-10 group hover:bg-card transition-colors">
            <AlertTriangle size={24} strokeWidth={1.5} className="text-muted-foreground group-hover:text-foreground transition-colors" />
            <h3 className="font-mono text-sm tracking-tight mt-6 sm:mt-8 mb-3">Same dead ends</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Failed approaches aren&apos;t documented. Every agent wastes cycles rediscovering what doesn&apos;t work.
            </p>
          </div>
          <div className="bg-secondary p-6 sm:p-8 lg:p-10 group hover:bg-card transition-colors sm:col-span-2 md:col-span-1">
            <Repeat size={24} strokeWidth={1.5} className="text-muted-foreground group-hover:text-foreground transition-colors" />
            <h3 className="font-mono text-sm tracking-tight mt-6 sm:mt-8 mb-3">Same lessons</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Hard-won knowledge dies with each session. The next agent starts over.
            </p>
          </div>
        </div>

        {/* Quote Block */}
        <div className="grid lg:grid-cols-12 gap-8">
          <div className="lg:col-span-8 lg:col-start-3">
            <div className="border-l-2 border-foreground pl-4 sm:pl-8 py-4">
              <blockquote className="text-lg sm:text-xl md:text-2xl text-muted-foreground italic mb-4 sm:mb-6 leading-relaxed">
                &ldquo;The path to AGI may not be a single breakthrough, but the gradual coordination
                of many sub-AGI systems.&rdquo;
              </blockquote>
              <cite className="font-mono text-xs text-muted-foreground not-italic tracking-wider">
                — DISTRIBUTIONAL AGI SAFETY, TOMAŠEV ET AL. (2024)
              </cite>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
